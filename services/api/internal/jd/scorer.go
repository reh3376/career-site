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

	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/prompts"
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
	// threshold is the match gate (see DefaultMatchThreshold).
	threshold float64
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
func NewScorer(log *slog.Logger, repo *users.Repo, embed ingest.EmbedClient, assessor *Assessor, resume *ResumeWriter, threshold float64, pipelineTimeout time.Duration) *Scorer {
	if threshold <= 0 || threshold > 1 {
		threshold = DefaultMatchThreshold
	}
	if pipelineTimeout <= 0 {
		pipelineTimeout = 15 * time.Minute
	}
	return &Scorer{
		log: log, users: repo, embed: embed, assessor: assessor, resume: resume, threshold: threshold,
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

// Threshold returns the configured match gate.
func (s *Scorer) Threshold() float64 { return s.threshold }

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
		"threshold":       s.threshold,
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
	persist := context.WithoutCancel(ctx)
	// Whatever path the run takes, the owner hears about the outcome.
	defer s.notifyOutcome(persist, submissionID)
	if err := s.users.UpdateJdScoring(persist, submissionID, "scoring", nil, ""); err != nil {
		s.log.Warn("jd: failed to mark scoring", slog.Int64("id", submissionID), slog.String("error", err.Error()))
	}
	queued := time.Now()
	if !s.acquire(ctx) {
		s.log.Warn("jd: gave up waiting for a pipeline slot", slog.Int64("id", submissionID))
		if uErr := s.users.UpdateJdScoring(persist, submissionID, "failed", nil, "timed out waiting for the pipeline; re-score to retry"); uErr != nil {
			s.log.Warn("jd: failed to record queue timeout", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
		}
		return
	}
	defer s.release()
	if waited := time.Since(queued); waited > time.Second {
		s.log.Info("jd: pipeline slot acquired", slog.Int64("id", submissionID), slog.Duration("waited", waited))
	}
	ctx, cancel := context.WithTimeout(ctx, s.pipelineTimeout)
	defer cancel()

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
		if uErr := s.users.UpdateJdScoring(persist, submissionID, "failed", nil, truncErr(err.Error())); uErr != nil {
			s.log.Warn("jd: failed to record failure", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
		}
		return
	}
	if len(hits) == 0 {
		s.log.Info("jd: no corpus hits", slog.Int64("id", submissionID))
		_ = s.users.UpdateJdScoring(persist, submissionID, "below_threshold", &retrieval,
			"corpus is empty: no chunks available to score against")
		return
	}

	score := retrieval
	var assessment *Assessment
	if s.assessor != nil {
		started := time.Now()
		assessment, err = s.assessor.Assess(ctx, submissionID, jdText, hints)
		if err != nil && isTransient(err) {
			s.log.Warn("jd: assessment transient failure, retrying once", slog.Int64("id", submissionID), slog.String("error", err.Error()))
			if sleepCtx(ctx, retryDelay) {
				assessment, err = s.assessor.Assess(ctx, submissionID, jdText, hints)
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
	if score >= s.threshold {
		next = "generating"
	}
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

	// Grounded résumé: one model call, sources verified in code.
	var usable *Assessment
	if assessment != nil && assessment.Error == "" {
		usable = assessment
	}
	started := time.Now()
	res, markdown, err := s.resume.Write(ctx, submissionID, jdText, hints, usable, hits)
	if err != nil {
		s.log.Warn("jd: résumé failed", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		if uErr := s.users.UpdateJdScoring(persist, submissionID, "failed", &score, "résumé generation failed: "+truncErr(err.Error())); uErr != nil {
			s.log.Warn("jd: failed to record résumé failure", slog.Int64("id", submissionID), slog.String("error", uErr.Error()))
		}
		return
	}
	raw, _ := json.Marshal(res)
	if err := s.users.SetJdResume(persist, submissionID, raw, markdown, res.Model, res.PromptID, res.PromptVers); err != nil {
		s.log.Warn("jd: failed to store résumé", slog.Int64("id", submissionID), slog.String("error", err.Error()))
		return
	}
	s.log.Info("jd: résumé ready",
		slog.Int64("id", submissionID),
		slog.String("model", res.Model),
		slog.Int("bullets_dropped", res.Dropped),
		slog.Int("chars", len(markdown)),
		slog.Duration("took", time.Since(started)),
	)

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
