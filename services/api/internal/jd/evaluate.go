package jd

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/reh3376/career-site/services/api/internal/build"
	"github.com/reh3376/career-site/services/api/internal/corpusscope"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/runid"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Evaluator scores the golden set and records what it found (data
// layer D4).
//
// It runs each posting through the ordinary pipeline rather than a
// special one. A test that takes a different path tests a different
// thing, and the point of this is to measure the reviewer as it
// actually behaves, including retrieval, judgment, and the arithmetic.
// The cost is that an evaluation takes as long as the set does, which
// on the production box is tens of minutes; it is therefore a job with
// progress, not a request.
type Evaluator struct {
	log    *slog.Logger
	users  *users.Repo
	scorer *Scorer
	// ownerID is the member the evaluation submissions belong to. They
	// need an owner because every submission does; they are flagged so
	// they never reach the submission list or an email.
	ownerID int64
}

// NewEvaluator wires the evaluator.
func NewEvaluator(log *slog.Logger, repo *users.Repo, scorer *Scorer, ownerID int64) *Evaluator {
	return &Evaluator{log: log, users: repo, scorer: scorer, ownerID: ownerID}
}

// Report is the progress callback the job runner supplies.
type Report func(pct int32, summary string)

// Run scores every active golden posting and records the evaluation.
// It returns a one-line summary for the job log.
func (e *Evaluator) Run(ctx context.Context, note string, adminID int64, report Report) (string, error) {
	if e == nil || e.scorer == nil {
		return "", fmt.Errorf("the scoring pipeline is not wired")
	}
	set, err := e.users.ListGoldenPostings(ctx, true)
	if err != nil {
		return "", fmt.Errorf("read the golden set: %w", err)
	}
	if len(set) == 0 {
		// ListGoldenPostings(active only) already excludes unlabelled
		// postings: an evaluation compares against an expectation, and a
		// posting nobody has judged has none. Say that, rather than
		// reporting an empty set when there are postings sitting there.
		return "", fmt.Errorf("no labelled postings to evaluate; label the ones waiting on /admin/evals first")
	}

	threshold := e.scorer.Threshold()
	run := users.EvalRun{
		EvalID:      runid.New(),
		Note:        note,
		TriggeredBy: adminID,
		AppCommit:   build.Commit,
		Host:        e.scorer.host,
		NumCtx:      e.scorer.assessor.NumCtx(),
		Prompts:     prompts.Fingerprints(),
		Threshold:   &threshold,
		Total:       len(set),
	}
	if fp, _, _, embedder, fErr := e.users.CorpusFingerprint(ctx); fErr == nil {
		run.CorpusFingerprint, run.EmbedderModel = fp, embedder
	}
	run.ID, err = e.users.StartEvalRun(ctx, run)
	if err != nil {
		return "", fmt.Errorf("open the evaluation: %w", err)
	}

	started := time.Now()
	for i, g := range set {
		if ctx.Err() != nil {
			// Bookkeeping runs on a context that outlives the
			// cancellation. Closing the run with the dead ctx writes
			// nothing, which on 2026-09-23 left a timed-out run sitting at
			// "running" for ever with no summary and no record of how far
			// it got. A run that died should say so in the table, since
			// that table is the only place anyone looks afterwards.
			done := context.WithoutCancel(ctx)
			run.Status = "failed"
			run.Note = fmt.Sprintf("%s | stopped after %d of %d: %v",
				run.Note, i, len(set), ctx.Err())
			if fErr := e.users.FinishEvalRun(done, run); fErr != nil {
				e.log.Warn("eval: could not close the interrupted run",
					slog.String("error", fErr.Error()))
			}
			return "", ctx.Err()
		}
		report(int32(5+85*i/len(set)), fmt.Sprintf("scoring %s (%d of %d)", g.Name, i+1, len(set)))

		item := e.score(ctx, g, threshold)
		if item.Error != "" {
			run.Errors++
		} else {
			run.Scored++
			if item.Passed {
				run.GateCorrect++
			}
		}
		if item.MatchScore != nil && run.Model == "" {
			// Whatever the judge reported on the first scored posting;
			// they all ran under the same configuration.
			if runs, rErr := e.users.ListJdRuns(ctx, item.SubmissionID); rErr == nil && len(runs) > 0 {
				run.Model = runs[0].Model
			}
		}
		if rErr := e.users.RecordEvalItem(ctx, run.ID, item); rErr != nil {
			e.log.Warn("eval: could not record an item",
				slog.String("posting", g.Name), slog.String("error", rErr.Error()))
		}
		run.Items = append(run.Items, item)
	}

	run.OrderViolations, run.Margin = separation(run.Items)
	run.Status = "done"
	if err := e.users.FinishEvalRun(ctx, run); err != nil {
		return "", fmt.Errorf("close the evaluation: %w", err)
	}
	report(100, "done")

	summary := fmt.Sprintf("%d of %d on the right side of the gate, %d ordering violations, %d errors, %s",
		run.GateCorrect, run.Scored, run.OrderViolations, run.Errors, time.Since(started).Round(time.Second))
	if run.Margin != nil {
		summary += fmt.Sprintf(", margin %.3f", *run.Margin)
	}
	e.log.Info("eval: finished", slog.String("summary", summary))
	return summary, nil
}

// score runs one golden posting through the real pipeline.
func (e *Evaluator) score(ctx context.Context, g users.GoldenPosting, threshold float64) users.EvalItem {
	item := users.EvalItem{GoldenID: g.ID, GoldenName: g.Name, ExpectedGate: g.ExpectedGate}

	sub, err := e.users.CreateJdSubmission(ctx, users.JdSubmitInput{
		SourceKind:   "paste",
		JdText:       g.JdText,
		RoleHint:     g.RoleHint,
		EmployerHint: g.EmployerHint,
		UserID:       e.ownerID,
		IsEval:       true,
	})
	if err != nil {
		item.Error = "could not create the evaluation submission: " + err.Error()
		return item
	}
	item.SubmissionID = sub.ID

	// Synchronous on purpose: the evaluation is already a background
	// job, and the pipeline serialises anyway, so running these in
	// parallel would only make them queue behind each other with worse
	// reporting.
	// An evaluation retrieves from the whole corpus. It exists to measure
	// what the owner gets when he reviews a posting for himself, which is
	// the case the private material is there for. A member's submission
	// is scoped to public documents in the handler, so the two paths
	// differ, and the golden-set numbers describe the owner's path only.
	e.scorer.ScoreAndPersist(corpusscope.With(ctx, corpusscope.All),
		sub.ID, g.JdText, prompts.Hints{Role: g.RoleHint, Employer: g.EmployerHint})

	runs, err := e.users.ListJdRuns(ctx, sub.ID)
	if err != nil || len(runs) == 0 {
		item.Error = "the run produced no record"
		return item
	}
	r := runs[0]
	item.RunID = r.RunID
	item.Fit = r.Fit
	if r.Error != "" && r.MatchScore == nil {
		item.Error = r.Error
		return item
	}
	item.MatchScore = r.MatchScore
	if r.MatchScore == nil {
		item.Error = "the run recorded no score"
		return item
	}
	item.GateSide = "below"
	if *r.MatchScore >= threshold {
		item.GateSide = "above"
	}
	item.Passed = item.GateSide == g.ExpectedGate
	return item
}

// separation measures how well the two groups are held apart.
//
// Gate accuracy alone is a coarse instrument: a set can stay at 100 %
// while every score creeps toward the threshold, and the first posting
// to cross it looks like a sudden failure rather than the end of a
// slide. So two more numbers:
//
//   - violations: pairs where a posting expected below outscored one
//     expected above. An inversion is worse than a gate miss, because a
//     gate can be moved and an inversion cannot be fixed by moving it.
//   - margin: the smallest gap between the two groups. A shrinking
//     margin is a regression that gate accuracy hides entirely.
func separation(items []users.EvalItem) (violations int, margin *float64) {
	var above, below []float64
	for _, it := range items {
		if it.MatchScore == nil || it.Error != "" {
			continue
		}
		if it.ExpectedGate == "above" {
			above = append(above, *it.MatchScore)
		} else {
			below = append(below, *it.MatchScore)
		}
	}
	if len(above) == 0 || len(below) == 0 {
		return 0, nil
	}
	for _, a := range above {
		for _, b := range below {
			if b >= a {
				violations++
			}
		}
	}
	sort.Float64s(above)
	sort.Float64s(below)
	gap := above[0] - below[len(below)-1]
	return violations, &gap
}
