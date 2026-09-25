// Package jd wires the JD-upload flow through the Ask Roger corpus:
// embed the submitted JD → retrieve → pre-score, then (when an LLM is
// available) extract requirements, judge each against retrieved
// evidence, and compute the match score in code from the verdicts.
// The résumé itself is generated in a follow-up step behind the same
// interface.
//
// The threshold is the gate for the "generate a tailored résumé"
// path. Below that, the caller sees a polite fallback and Roger sees
// the row in /admin/jd for manual triage.
package jd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/build"
	"github.com/reh3376/career-site/services/api/internal/corpusscope"
	"github.com/reh3376/career-site/services/api/internal/events"
	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/runid"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// DefaultMatchThreshold is the gate used when JD_MATCH_THRESHOLD is
// not set. The gate is model-dependent: each judge model reads the
// same evidence with its own strictness, so the calibrated value lives
// in .env.prod next to OLLAMA_LLM_MODEL (api and web read the same
// variable) and docs/llm-tuning-log.md records it per model and
// prompt version (qwen3:4b-q8_0, prompts v2, facts sheet: 0.70 with
// strong 0.857 / mid 0.591). Recalibrate whenever the judge prompt
// version, the model, the score formula or the batching changes.
const DefaultMatchThreshold = 0.55

// TopK caps how many chunks feed the retrieval pre-score.
const TopK = 8

// Scorer runs the JD → score pipeline. Depends on the same
// EmbedClient interface `ingest` uses, so a fake can drive tests
// without booting the gRPC stack.
type Scorer struct {
	log   *slog.Logger
	users *users.Repo
	embed ingest.EmbedClient
	// assessor is optional; nil means the retrieval pre-score is the
	// gate (dev without an LLM provider, or stub provider).
	assessor *Assessor
	// resume is optional; nil leaves above-threshold rows at
	// `generating` for manual attention.
	resume *ResumeWriter
	// bands hold the fit categories; the "strong" edge is the gate.
	bands *BandsStore
	// host names where the model ran, recorded on every run so a change
	// in latency or verdicts can be attributed to the machine rather
	// than guessed at. One value today; the plan is for more.
	host string
	// events is the product event stream (jd.finished); nil is silent.
	events *events.Writer
	// slots serialises pipelines. On the CPX31 two concurrent JDs made
	// the embedder and the LLM swap in and out (one resident model at a
	// time) until an embed call timed out; one JD at a time is also
	// simply faster on four vCPUs than two interleaved. Submissions
	// wait in `scoring` for their turn; the pipeline timeout still
	// bounds each one from the moment it starts.
	slots chan struct{}
	// pipelineTimeout bounds one run from the moment it holds the slot.
	pipelineTimeout time.Duration
	// notifier is told when a run reaches a terminal state (see notify.go).
	notifier OutcomeNotifier
}

// NewScorer wires the deps. assessor and resume may be nil; a
// threshold outside (0, 1] falls back to DefaultMatchThreshold.
func NewScorer(log *slog.Logger, repo *users.Repo, embed ingest.EmbedClient, assessor *Assessor, resume *ResumeWriter, bands *BandsStore, pipelineTimeout time.Duration) *Scorer {
	if bands == nil {
		bands = NewBandsStore(log, repo, DefaultBands(0))
	}
	if pipelineTimeout <= 0 {
		pipelineTimeout = 15 * time.Minute
	}
	return &Scorer{
		log: log, users: repo, embed: embed, assessor: assessor, resume: resume, bands: bands,
		slots:           make(chan struct{}, PipelineConcurrency),
		pipelineTimeout: pipelineTimeout,
	}
}

// maxQueueWait caps how long a submission may wait for a slot before
// it is failed as "re-score to retry". Separate from the pipeline
// timeout: waiting in line must not eat a submission's own budget.
const maxQueueWait = 3 * time.Hour

// PipelineConcurrency is how many JD pipelines may run at once. One,
// because the model host is a single small box; raise it only with a
// GPU backend that can serve concurrent requests.
const PipelineConcurrency = 1

// acquire waits for a pipeline slot, up to maxQueueWait or until ctx
// ends.
func (s *Scorer) acquire(ctx context.Context) bool {
	t := time.NewTimer(maxQueueWait)
	defer t.Stop()
	select {
	case s.slots <- struct{}{}:
		return true
	case <-t.C:
		return false
	case <-ctx.Done():
		return false
	}
}

func (s *Scorer) release() { <-s.slots }

// Threshold returns the match gate in force (the "strong" band edge).
func (s *Scorer) Threshold() float64 { return s.bands.Get(context.Background()).Strong }

// Bands returns the fit bands in force.
func (s *Scorer) Bands(ctx context.Context) Bands { return s.bands.Get(ctx) }

// BandsStore exposes the store for the admin surface.
func (s *Scorer) BandsStore() *BandsStore { return s.bands }

// logGate records the gate decision (a code decision, no model) in the
// decision log so the owner can review the threshold call alongside
// the verdicts that fed it. Best-effort.
func (s *Scorer) logGate(ctx context.Context, submissionID int64, score, retrieval float64, assessment *Assessment, next string) {
	counts := map[string]int{"met": 0, "partial": 0, "unmet": 0}
	assessed := assessment != nil && assessment.Error == ""
	weightTotal, nReq := 0, 0
	if assessed {
		for _, j := range assessment.Judgments {
			counts[j.Verdict]++
		}
		weightTotal, nReq = assessment.WeightTotal, len(assessment.Requirements)
	}
	input, _ := json.Marshal(map[string]any{
		"score":           score,
		"retrieval_score": retrieval,
		"threshold":       s.Threshold(),
		"assessor_ran":    assessed,
		"requirements":    nReq,
		"weight_total":    weightTotal,
		"verdicts":        counts,
	})
	outcome := "below_threshold"
	if next == "generating" {
		outcome = "above_threshold"
	}
	output, _ := json.Marshal(map[string]any{"outcome": outcome})
	row := users.Decision{
		Kind: "jd_gate", RefKind: "jd_submission", RefID: submissionID,
		Model: "code", Input: input, Output: output,
	}
	if err := s.users.InsertDecisions(context.WithoutCancel(ctx), []users.Decision{row}); err != nil {
		s.log.Warn("decision log write failed", slog.Int64("jd_id", submissionID), slog.String("error", err.Error()))
	}
}

// Score embeds the JD, retrieves top-K corpus chunks, and returns the
// mean of their similarities: a cheap retrieval pre-score, monotonic
// in the strength and breadth of the match, kept for diagnostics
// alongside the requirement-weighted score.
func (s *Scorer) Score(ctx context.Context, jdText string) (score float64, hits []users.CorpusHit, err error) {
	if s.embed == nil {
		return 0, nil, errors.New("scorer: no embed client wired")
	}
	vectors, _, err := s.embed.Embed(ctx, []string{jdText}, ingest.PurposeQuery)
	if err != nil {
		return 0, nil, err
	}
	if len(vectors) != 1 {
		return 0, nil, errors.New("scorer: embedder returned no vector")
	}
	hits, err = s.users.SearchCorpus(ctx, vectors[0], TopK)
	if err != nil {
		return 0, nil, err
	}
	if len(hits) == 0 {
		return 0, hits, nil
	}
	var sum float32
	for _, h := range hits {
		sum += h.Similarity
	}
	return float64(sum) / float64(len(hits)), hits, nil
}

// ScoreAndPersist runs the pipeline for one jd_submissions row and
// writes the outcome. Best-effort: persistence failures are logged,
// the row already exists and a re-score can retry. Called from a
// goroutine by the JD handler.
// The caller's ctx should carry no deadline of its own (the handlers
// pass a background context): the queue wait is capped by
// maxQueueWait and the run by pipelineTimeout, applied here once the
// slot is held. Status writes use a context that survives either
// deadline, so a timed-out run is recorded as failed instead of being
// left at `scoring` forever (which is what happened on prod on
// 2026-09-22 when a submission queued for 28 minutes behind another).
func (s *Scorer) ScoreAndPersist(ctx context.Context, submissionID int64, jdText string, hints prompts.Hints) {
	s.scoreAndPersist(ctx, submissionID, jdText, hints, "submit", 0)
}

// RescoreAndPersist is ScoreAndPersist for an admin re-run. The only
// difference is what the run record says about why it happened and who
// asked, which is exactly the sort of thing that is impossible to
// reconstruct later if it is not written down at the time.
func (s *Scorer) RescoreAndPersist(ctx context.Context, submissionID int64, jdText string, hints prompts.Hints, adminID int64) {
	s.scoreAndPersist(ctx, submissionID, jdText, hints, "rescore", adminID)
}

func (s *Scorer) scoreAndPersist(ctx context.Context, submissionID int64, jdText string, hints prompts.Hints, trigger string, adminID int64) {
	persist := context.WithoutCancel(ctx)

	// The run record (data layer D2). A re-score overwrites the
	// submission row, so without this the evidence of what the previous
	// run did, and of what produced it, is gone. Opened before the queue
	// wait so queue time is recorded even when the run never gets a slot.
	run := users.JdRun{
		RunID:        runid.New(),
		SubmissionID: submissionID,
		Trigger:      trigger,
		TriggeredBy:  adminID,
		AppCommit:    build.Commit,
		ScoreFormula: ScoreFormula,
		Prompts:      prompts.Fingerprints(),
		NumCtx:       s.assessor.NumCtx(),
		Host:         s.host,
		// What this run was allowed to read. Recorded because two runs
		// with different scopes were otherwise indistinguishable in
		// provenance, and looked like non-determinism.
		RetrievalScope: scopeName(ctx),
	}
	if fp, docs, chunks, embedder, err := s.users.CorpusFingerprint(persist); err != nil {
		s.log.Warn("jd: corpus fingerprint failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
	} else {
		run.CorpusFingerprint, run.CorpusDocuments, run.CorpusChunks, run.EmbedderModel = fp, docs, chunks, embedder
	}
	runOpen := false
	if attempt, err := s.users.StartJdRun(persist, run); err != nil {
		// A missing run record must not cost the submitter their review,
		// so the pipeline continues without one and says so loudly.
		s.log.Error("jd: could not open a run record", slog.Int64("id", submissionID), slog.String("error", err.Error()))
	} else {
		run.Attempt = attempt
		runOpen = true
		ctx = runid.With(ctx, run.RunID)
		persist = runid.With(persist, run.RunID)
	}
	runStarted := time.Now()
	// Filled in as the run progresses; whatever is set when the function
	// returns is what the record keeps.
	outcome := &users.JdRun{RunID: run.RunID, Status: "failed", ScoreFormula: ScoreFormula}
	defer func() {
		if !runOpen {
			return
		}
		outcome.DurationMs = time.Since(runStarted).Milliseconds()
		if err := s.users.FinishJdRun(persist, *outcome); err != nil {
			s.log.Warn("jd: could not close the run record",
				slog.Int64("id", submissionID), slog.String("run", run.RunID), slog.String("error", err.Error()))
		}
	}()

	// Whatever path the run takes, the owner hears about the outcome,
	// and the progress column reads 100 once the row is terminal.
	defer s.notifyOutcome(persist, submissionID)
	defer func() {
		if err := s.users.FinishJdProgress(persist, submissionID); err != nil {
			s.log.Warn("jd: finish progress failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		}
	}()
	if err := s.users.UpdateJdScoring(persist, submissionID, "scoring", nil, ""); err != nil {
		s.log.Warn("jd: failed to mark scoring", slog.Int64("id", submissionID), slog.String("error", err.Error()))
	}
	queued := time.Now()
	_ = s.users.UpdateJdProgress(persist, submissionID, 1, "queued behind another review")
	if !s.acquire(ctx) {
		s.log.Warn("jd: gave up waiting for a pipeline slot", slog.Int64("id", submissionID))
		outcome.QueuedMs = time.Since(queued).Milliseconds()
		outcome.Error = "timed out waiting for the pipeline"
		if uErr := s.users.UpdateJdScoring(persist, submissionID, "failed", nil, "timed out waiting for the pipeline; re-score to retry"); uErr != nil {
			s.log.Warn("jd: failed to record queue timeout", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
		}
		return
	}
	defer s.release()
	// Queue time is recorded apart from work time. They answer different
	// questions: one says the box is busy, the other says the pipeline is
	// slow, and a single duration hides which.
	outcome.QueuedMs = time.Since(queued).Milliseconds()
	runStarted = time.Now()
	if waited := time.Since(queued); waited > time.Second {
		s.log.Info("jd: pipeline slot acquired", slog.Int64("id", submissionID), slog.Duration("waited", waited))
	}
	ctx, cancel := context.WithTimeout(ctx, s.pipelineTimeout)
	defer cancel()
	progress := Progress(func(pct int32, stage string) {
		if err := s.users.UpdateJdProgress(persist, submissionID, pct, stage); err != nil {
			s.log.Warn("jd: progress write failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		}
	})
	progress(2, "starting")

	// Step zero: is this even a job posting? A score computed from a
	// search-term list or a pasted résumé looks exactly like a real one
	// and means nothing, which is worse than refusing to answer. Runs
	// before retrieval so nothing expensive is spent on it, and fails
	// open so a confused classifier cannot block real work.
	if s.assessor != nil {
		progress(4, "checking the posting")
		if v := s.assessor.CheckPosting(ctx, submissionID, jdText); !v.IsPosting {
			msg := NotAPostingMessage(v)
			outcome.Status = "not_a_posting"
			outcome.Error = "not a posting: " + v.Kind
			if uErr := s.users.UpdateJdScoring(persist, submissionID, "not_a_posting", nil, msg); uErr != nil {
				s.log.Warn("jd: failed to record not-a-posting",
					slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
			}
			return
		}
	}

	retrieval, hits, err := s.Score(ctx, jdText)
	if err != nil && isTransient(err) {
		// One retry after a short pause covers the sidecar restarting or
		// a momentary network blip, which is what has actually failed
		// submissions so far.
		s.log.Warn("jd: score transient failure, retrying once", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		if !sleepCtx(ctx, retryDelay) {
			err = ctx.Err()
		} else {
			retrieval, hits, err = s.Score(ctx, jdText)
		}
	}
	if err != nil {
		s.log.Warn("jd: score failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		outcome.Error = "retrieval: " + truncErr(err.Error())
		if uErr := s.users.UpdateJdScoring(persist, submissionID, "failed", nil, truncErr(err.Error())); uErr != nil {
			s.log.Warn("jd: failed to record failure", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
		}
		return
	}
	outcome.RetrievalScore = &retrieval
	if len(hits) == 0 {
		s.log.Info("jd: no corpus hits", slog.Int64("id", submissionID))
		outcome.Status = "below_threshold"
		outcome.Error = "corpus is empty"
		_ = s.users.UpdateJdScoring(persist, submissionID, "below_threshold", &retrieval,
			"corpus is empty: no chunks available to score against")
		return
	}

	score := retrieval
	var assessment *Assessment
	if s.assessor != nil {
		started := time.Now()
		assessment, err = s.assessor.Assess(ctx, submissionID, jdText, hints, progress)
		if err != nil && isTransient(err) {
			s.log.Warn("jd: assessment transient failure, retrying once", slog.Int64("id", submissionID), slog.String("error", err.Error()))
			if sleepCtx(ctx, retryDelay) {
				assessment, err = s.assessor.Assess(ctx, submissionID, jdText, hints, progress)
			}
		}
		if err != nil {
			// The retrieval pre-score is flat across good and bad JDs
			// (0.58 to 0.71 on the calibration set) and cannot gate a
			// résumé. With the assessor wired, an assessment failure is a
			// failed submission: the retrieval score is kept for the
			// record, the error is visible in /admin/jd, and Re-score
			// runs it again once the cause (usually the model host) is
			// fixed. See docs/llm-tuning-log.md, 2026-09-22.
			s.log.Warn("jd: assessment failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
			outcome.Error = "assessment: " + truncErr(err.Error())
			assessment = &Assessment{Error: truncErr(err.Error())}
			if raw, mErr := json.Marshal(assessment); mErr == nil {
				if uErr := s.users.SetJdAssessment(persist, submissionID, retrieval, raw); uErr != nil {
					s.log.Warn("jd: failed to store assessment", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
				}
			}
			if uErr := s.users.UpdateJdScoring(persist, submissionID, "failed", &retrieval, "assessment failed: "+truncErr(err.Error())); uErr != nil {
				s.log.Warn("jd: failed to record assessment failure", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
			}
			return
		} else {
			score = assessment.Score
			outcome.Model = assessment.Model
			outcome.RequirementCount = len(assessment.Requirements)
			for _, v := range assessment.Judgments {
				switch v.Verdict {
				case "met":
					outcome.MetCount++
				case "partial":
					outcome.PartialCount++
				case "unmet":
					outcome.UnmetCount++
				}
			}
			s.log.Info("jd: assessed",
				slog.Int64("id", submissionID),
				slog.Int("requirements", len(assessment.Requirements)),
				slog.Float64("score", assessment.Score),
				slog.Float64("retrieval_score", retrieval),
				slog.Duration("took", time.Since(started)),
			)
		}
		if raw, mErr := json.Marshal(assessment); mErr == nil {
			if uErr := s.users.SetJdAssessment(persist, submissionID, retrieval, raw); uErr != nil {
				s.log.Warn("jd: failed to store assessment", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
			}
		}
	} else if uErr := s.users.SetJdAssessment(persist, submissionID, retrieval, nil); uErr != nil {
		s.log.Warn("jd: failed to store retrieval score", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
	}

	next := "below_threshold"
	threshold := s.Threshold()
	if score >= threshold {
		next = "generating"
	}
	outcome.MatchScore = &score
	outcome.Threshold = &threshold
	outcome.Fit = s.Bands(ctx).Category(score)
	// Terminal unless the résumé stage runs and changes it.
	outcome.Status = next
	s.logGate(ctx, submissionID, score, retrieval, assessment, next)
	if err := s.users.UpdateJdScoring(persist, submissionID, next, &score, ""); err != nil {
		s.log.Warn("jd: failed to record score",
			slog.Int64("id", submissionID),
			slog.Float64("score", score),
			slog.String("error", err.Error()),
		)
		return
	}
	s.log.Info("jd: scored",
		slog.Int64("id", submissionID),
		slog.Float64("score", score),
		slog.String("status", next),
		slog.Int("top_hits", len(hits)),
	)
	if next != "generating" || s.resume == nil {
		return
	}

	progress(80, "writing the tailored résumé")

	// Grounded résumé: one model call, sources verified in code.
	var usable *Assessment
	if assessment != nil && assessment.Error == "" {
		usable = assessment
	}
	started := time.Now()
	res, markdown, err := s.resume.Write(ctx, submissionID, jdText, hints, usable, hits)
	if err != nil {
		s.log.Warn("jd: résumé failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		outcome.Status = "failed"
		outcome.Error = "résumé: " + truncErr(err.Error())
		if uErr := s.users.UpdateJdScoring(persist, submissionID, "failed", &score, "résumé generation failed: "+truncErr(err.Error())); uErr != nil {
			s.log.Warn("jd: failed to record résumé failure", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
		}
		return
	}
	raw, _ := json.Marshal(res)
	if err := s.users.SetJdResume(persist, submissionID, raw, markdown, res.Model, res.PromptID, res.PromptVers); err != nil {
		s.log.Warn("jd: failed to store résumé", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		outcome.Error = "storing the résumé: " + truncErr(err.Error())
		return
	}
	// The submission reaches `ready` when SetJdResume lands; the run
	// agrees from here, whatever the optional PDF stage does.
	outcome.Status = "ready"
	outcome.ResumeGenerated = true
	s.log.Info("jd: résumé ready",
		slog.Int64("id", submissionID),
		slog.String("model", res.Model),
		slog.Int("bullets_dropped", res.Dropped),
		slog.Int("chars", len(markdown)),
		slog.Duration("took", time.Since(started)),
	)

	progress(94, "rendering the PDF")

	// PDF is optional: the markdown is already deliverable, so a render
	// failure is logged and the row stays ready.
	pdf, pages, err := s.resume.RenderPDF(ctx, submissionID, raw)
	if err != nil {
		s.log.Warn("jd: pdf render failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		return
	}
	if pdf == nil {
		return
	}
	if err := s.users.SetJdResumePDF(persist, submissionID, pdf, pages, s.resume.DownloadPath(submissionID)); err != nil {
		s.log.Warn("jd: failed to store pdf", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		return
	}
	s.log.Info("jd: pdf ready", slog.Int64("id", submissionID), slog.Int("bytes", len(pdf)), slog.Int("pages", int(pages)))
}

// retryDelay is the pause before the single automatic retry.
const retryDelay = 8 * time.Second

// isTransient reports whether an error looks like a sidecar or network
// hiccup worth one retry, as opposed to a bad input or a logic error.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"unavailable", "network is unreachable", "connection refused", "connection reset",
		"broken pipe", "eof", "timeout", "timed out", "temporarily", "no such host", "503", "502",
		// Ollama restarts its model runner after a crash (an OOM kill on
		// a tight box); the next call succeeds, so treat it as transient.
		"unexpectedly stopped", "http 500",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// sleepCtx waits for d unless ctx ends first; returns false when it did.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// truncErr keeps a persisted error short: the row's `error` column is
// UI-facing on /admin/jd; a stack trace is out of place.
func truncErr(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// scopeName renders the retrieval scope for the run record.
func scopeName(ctx context.Context) string {
	if corpusscope.AllowsPrivate(ctx) {
		return "all"
	}
	return "public"
}
