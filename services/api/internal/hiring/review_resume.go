package hiring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// A résumé written after review (owner's decision, 2026-11-13; FSD
// FR-HF-08). When a person's verdicts lift a posting that scored below
// the résumé gate to at or above it, the owner may ask for the tailored
// résumé on the verdicts as they stand. It is his alone, which is why
// it lives in review_resumes and not on the submission: the member's
// review reads its résumé from jd_submissions, and nothing here is ever
// written there.

// Review résumé states.
const (
	ReviewResumeWriting = "writing"
	ReviewResumeReady   = "ready"
	ReviewResumeFailed  = "failed"
)

// ErrReviewResumeBusy is returned when a résumé after review is already
// being written for the run.
var ErrReviewResumeBusy = errors.New("hiring: a résumé is already being written for this run")

// ReviewResume is one row of review_resumes, without the PDF's bytes.
type ReviewResume struct {
	RunID         string
	SubmissionID  int64
	Status        string
	Error         string
	Markdown      string
	Model         string
	PromptID      string
	PromptVersion int
	// ReviewedScore, Threshold and OverrideCount are what the review
	// stood at when the résumé was asked for.
	ReviewedScore float64
	Threshold     float64
	OverrideCount int
	RequestedBy   int64
	RequestedAt   time.Time
	FinishedAt    *time.Time
	HasPDF        bool
}

// ReviewResumeRequest is what opens a write.
type ReviewResumeRequest struct {
	RunID         string
	SubmissionID  int64
	ReviewedScore float64
	Threshold     float64
	OverrideCount int
	RequestedBy   int64
}

// StartReviewResume marks a résumé after review as being written for a
// run, replacing any finished or failed one. It refuses while one is
// already being written: the model host does one thing at a time and a
// second request would only queue the same work twice.
func (r *Repo) StartReviewResume(ctx context.Context, in ReviewResumeRequest) error {
	tag, err := r.pool.Exec(ctx, `
    INSERT INTO review_resumes
      (run_id, tenant_id, submission_id, status, reviewed_score, threshold, override_count, requested_by, requested_at)
    VALUES ($1::uuid, $2, $3, 'writing', $4, $5, $6, $7, now())
    ON CONFLICT (run_id) DO UPDATE SET
      status = 'writing', error = '', resume_json = NULL, markdown = '', pdf = NULL, pdf_pages = 0,
      model = '', prompt_id = '', prompt_version = 0,
      reviewed_score = EXCLUDED.reviewed_score, threshold = EXCLUDED.threshold,
      override_count = EXCLUDED.override_count, requested_by = EXCLUDED.requested_by,
      requested_at = now(), finished_at = NULL
    WHERE review_resumes.status <> 'writing'`,
		in.RunID, tenant.FromContext(ctx).Int64(), in.SubmissionID, in.ReviewedScore, in.Threshold, in.OverrideCount, in.RequestedBy)
	if err != nil {
		return fmt.Errorf("start review résumé: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrReviewResumeBusy
	}
	return nil
}

// FinishReviewResume stores the written résumé and marks it ready.
func (r *Repo) FinishReviewResume(ctx context.Context, runID string, resumeJSON json.RawMessage, markdown, model, promptID string, promptVersion int) error {
	_, err := r.pool.Exec(ctx, `
    UPDATE review_resumes
    SET status = 'ready', error = '', resume_json = $2, markdown = $3,
        model = $4, prompt_id = $5, prompt_version = $6, finished_at = now()
    WHERE run_id = $1::uuid`, runID, []byte(resumeJSON), markdown, model, promptID, promptVersion)
	if err != nil {
		return fmt.Errorf("finish review résumé: %w", err)
	}
	return nil
}

// FailReviewResume records why a write produced nothing.
func (r *Repo) FailReviewResume(ctx context.Context, runID, reason string) error {
	_, err := r.pool.Exec(ctx, `
    UPDATE review_resumes SET status = 'failed', error = $2, finished_at = now()
    WHERE run_id = $1::uuid`, runID, reason)
	if err != nil {
		return fmt.Errorf("fail review résumé: %w", err)
	}
	return nil
}

// SetReviewResumePDF stores the rendered PDF beside a ready résumé.
func (r *Repo) SetReviewResumePDF(ctx context.Context, runID string, pdf []byte, pages int32) error {
	_, err := r.pool.Exec(ctx, `
    UPDATE review_resumes SET pdf = $2, pdf_pages = $3 WHERE run_id = $1::uuid`, runID, pdf, pages)
	if err != nil {
		return fmt.Errorf("review résumé pdf: %w", err)
	}
	return nil
}

// FailInterruptedReviewResumes fails every row left at `writing`. It is
// called once when the API starts: a write is a goroutine of one
// process, so a new process has none in flight, and a row that still
// says so was cut off by a restart. It returns how many it closed.
func (r *Repo) FailInterruptedReviewResumes(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
    UPDATE review_resumes
    SET status = 'failed', error = 'interrupted by a restart of the service; ask for it again', finished_at = now()
    WHERE status = 'writing'`)
	if err != nil {
		return 0, fmt.Errorf("fail interrupted review résumés: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GetReviewResume reads a run's résumé after review; ok is false when
// none has been asked for.
func (r *Repo) GetReviewResume(ctx context.Context, runID string) (ReviewResume, bool, error) {
	var x ReviewResume
	err := r.pool.QueryRow(ctx, `
    SELECT run_id::text, submission_id, status, error, markdown, model, prompt_id, prompt_version,
           reviewed_score, threshold, override_count, COALESCE(requested_by, 0), requested_at, finished_at,
           pdf IS NOT NULL
    FROM review_resumes WHERE run_id = $1::uuid AND tenant_id = $2`,
		runID, tenant.FromContext(ctx).Int64()).Scan(
		&x.RunID, &x.SubmissionID, &x.Status, &x.Error, &x.Markdown, &x.Model, &x.PromptID, &x.PromptVersion,
		&x.ReviewedScore, &x.Threshold, &x.OverrideCount, &x.RequestedBy, &x.RequestedAt, &x.FinishedAt, &x.HasPDF)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewResume{}, false, nil
	}
	if err != nil {
		return ReviewResume{}, false, fmt.Errorf("review résumé: %w", err)
	}
	return x, true, nil
}

// GetReviewResumePDF returns the PDF of a run's résumé after review and
// the submission it belongs to; nil when there is none.
func (r *Repo) GetReviewResumePDF(ctx context.Context, runID string) ([]byte, int64, error) {
	var pdf []byte
	var submissionID int64
	err := r.pool.QueryRow(ctx, `
    SELECT pdf, submission_id FROM review_resumes
    WHERE run_id = $1::uuid AND tenant_id = $2 AND status = 'ready'`,
		runID, tenant.FromContext(ctx).Int64()).Scan(&pdf, &submissionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("review résumé pdf: %w", err)
	}
	return pdf, submissionID, nil
}
