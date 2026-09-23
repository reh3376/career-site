package users

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/reh3376/career-site/services/api/internal/runid"
	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// OutcomeStatuses is the vocabulary for what happened after a review,
// in the order a posting usually moves through it. Fixed, because free
// text cannot be counted and "did the score predict anything" is a
// counting question.
var OutcomeStatuses = []string{
	"not_pursued", // read the review, decided against applying
	"applied",
	"screening",
	"interview",
	"offer",
	"rejected",
	"no_response", // applied, heard nothing back
	"withdrew",
}

// ValidOutcomeStatus reports whether s is in the vocabulary.
func ValidOutcomeStatus(s string) bool {
	return slices.Contains(OutcomeStatuses, s)
}

// JdOutcome is what happened in the world after a review.
type JdOutcome struct {
	SubmissionID int64
	Status       string
	DecidedOn    *time.Time
	Note         string
	RecordedBy   int64
	UpdatedAt    time.Time
}

// SetJdOutcome records or revises the outcome for a posting. One row
// per submission: an outcome is a current fact, not a log, and its
// history lives in the events stream.
func (r *Repo) SetJdOutcome(ctx context.Context, o JdOutcome) error {
	_, err := r.pool.Exec(ctx, `
    INSERT INTO jd_outcomes (tenant_id, submission_id, status, decided_on, note, recorded_by)
    VALUES ($1, $2, $3, $4, $5, NULLIF($6, 0))
    ON CONFLICT (submission_id) DO UPDATE SET
      status = EXCLUDED.status,
      decided_on = EXCLUDED.decided_on,
      note = EXCLUDED.note,
      recorded_by = EXCLUDED.recorded_by,
      updated_at = now()`,
		tenant.FromContext(ctx).Int64(), o.SubmissionID, o.Status, o.DecidedOn,
		truncNote(o.Note), o.RecordedBy)
	if err != nil {
		return fmt.Errorf("set jd outcome: %w", err)
	}
	return nil
}

// GetJdOutcome returns the outcome for a posting, or nil when none has
// been recorded.
func (r *Repo) GetJdOutcome(ctx context.Context, submissionID int64) (*JdOutcome, error) {
	var o JdOutcome
	var recordedBy *int64
	err := r.pool.QueryRow(ctx, `
    SELECT submission_id, status, decided_on, note, recorded_by, updated_at
      FROM jd_outcomes WHERE submission_id = $1`, submissionID).
		Scan(&o.SubmissionID, &o.Status, &o.DecidedOn, &o.Note, &recordedBy, &o.UpdatedAt)
	if err != nil {
		return nil, nil //nolint:nilerr // "no outcome yet" is the normal case, not an error
	}
	if recordedBy != nil {
		o.RecordedBy = *recordedBy
	}
	return &o, nil
}

// FeedbackRatings is the vocabulary per target. A thumb would not do:
// "too generous" and "too harsh" point at opposite fixes, and one bit
// cannot tell them apart.
var FeedbackRatings = map[string][]string{
	"score":  {"accurate", "too_generous", "too_harsh", "unusable"},
	"resume": {"would_send", "needs_edits", "wrong"},
}

// ValidFeedback reports whether the target and rating are a known pair.
func ValidFeedback(target, rating string) bool {
	return slices.Contains(FeedbackRatings[target], rating)
}

// JdFeedback is one person's judgment of one run's output.
type JdFeedback struct {
	SubmissionID int64
	RunID        string
	Source       string // owner | submitter
	UserID       int64
	Target       string // score | resume
	Rating       string
	Note         string
	CreatedAt    time.Time
}

// RecordJdFeedback stores a judgment, replacing this person's previous
// one for the same run and target so counting ratings counts people
// rather than clicks. The run comes from the context when the caller is
// inside a pipeline, and from the field otherwise.
func (r *Repo) RecordJdFeedback(ctx context.Context, f JdFeedback) error {
	run := f.RunID
	if run == "" {
		run = runid.FromContext(ctx)
	}
	_, err := r.pool.Exec(ctx, `
    INSERT INTO jd_feedback (tenant_id, run_id, submission_id, source, user_id, target, rating, note)
    VALUES ($1, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, 0), $6, $7, $8)
    ON CONFLICT (submission_id, coalesce(run_id, '00000000-0000-0000-0000-000000000000'::uuid), source, target, coalesce(user_id, 0))
    DO UPDATE SET rating = EXCLUDED.rating, note = EXCLUDED.note, created_at = now()`,
		tenant.FromContext(ctx).Int64(), run, f.SubmissionID, f.Source, f.UserID,
		f.Target, f.Rating, truncNote(f.Note))
	if err != nil {
		return fmt.Errorf("record jd feedback: %w", err)
	}
	return nil
}

// ListJdFeedback returns every judgment recorded for a posting, newest
// first.
func (r *Repo) ListJdFeedback(ctx context.Context, submissionID int64) ([]JdFeedback, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT coalesce(run_id::text, ''), submission_id, source, coalesce(user_id, 0),
           target, rating, note, created_at
      FROM jd_feedback WHERE submission_id = $1 ORDER BY id DESC`, submissionID)
	if err != nil {
		return nil, fmt.Errorf("list jd feedback: %w", err)
	}
	defer rows.Close()

	var out []JdFeedback
	for rows.Next() {
		var f JdFeedback
		if err := rows.Scan(&f.RunID, &f.SubmissionID, &f.Source, &f.UserID,
			&f.Target, &f.Rating, &f.Note, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("list jd feedback: scan: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func truncNote(s string) string {
	const max = 4000
	if len(s) > max {
		return s[:max]
	}
	return s
}
