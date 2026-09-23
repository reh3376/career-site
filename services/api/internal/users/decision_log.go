package users

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/internal/runid"
	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// Decision is one row of decision_log: a single decision the reviewer
// pipeline made (by the model or by code), what it was made from, and
// the owner's review of it. See docs/decision-log.md.
type Decision struct {
	ID            int64
	Kind          string // jd_requirement_verdict | jd_gate | ...
	RefKind       string // jd_submission
	RefID         int64
	Key           string // requirement id within the ref; "" when per-ref
	Model         string
	PromptID      string
	PromptVersion int
	NumCtx        int
	Input         json.RawMessage
	Output        json.RawMessage
	PromptText    string
	ResponseText  string
	PromptTokens  int32
	CompletionTok int32
	LatencyMs     int64
	CreatedAt     time.Time

	HumanVerdict string
	HumanNote    string
	ReviewedBy   *int64
	ReviewedAt   *time.Time
}

// Reviewed reports whether the owner has labelled this decision.
func (d Decision) Reviewed() bool { return d.ReviewedAt != nil }

// InsertDecisions appends rows in one transaction. Best-effort for
// callers: the decision has already been made and persisted elsewhere,
// so a logging failure is reported, never fatal to the pipeline.
func (r *Repo) InsertDecisions(ctx context.Context, rows []Decision) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("insert decisions: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const q = `
    INSERT INTO decision_log
      (tenant_id, run_id, kind, ref_kind, ref_id, key, model, prompt_id, prompt_version, num_ctx,
       input, output, prompt_text, response_text,
       prompt_tokens, completion_tokens, latency_ms)
    VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
  `
	tid := tenant.FromContext(ctx).Int64()
	rid := runid.FromContext(ctx)
	for _, d := range rows {
		in := d.Input
		if len(in) == 0 {
			in = json.RawMessage(`{}`)
		}
		out := d.Output
		if len(out) == 0 {
			out = json.RawMessage(`{}`)
		}
		if _, err := tx.Exec(ctx, q,
			tid, rid,
			d.Kind, d.RefKind, d.RefID, d.Key, d.Model, d.PromptID, d.PromptVersion, d.NumCtx,
			in, out, d.PromptText, d.ResponseText,
			d.PromptTokens, d.CompletionTok, d.LatencyMs,
		); err != nil {
			return fmt.Errorf("insert decision %s/%s: %w", d.Kind, d.Key, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("insert decisions: commit: %w", err)
	}
	return nil
}

// DecisionFilter narrows ListDecisions. Zero values mean "any".
type DecisionFilter struct {
	Kind           string
	RefID          int64
	UnreviewedOnly bool
	Limit          int
}

const decisionColumns = `
    id, kind, ref_kind, ref_id, key, model, prompt_id, prompt_version, num_ctx,
    input, output, prompt_text, response_text,
    prompt_tokens, completion_tokens, latency_ms, created_at,
    coalesce(human_verdict, ''), coalesce(human_note, ''), reviewed_by, reviewed_at`

// ListDecisions returns rows newest first for the review surface.
func (r *Repo) ListDecisions(ctx context.Context, f DecisionFilter) ([]Decision, error) {
	var where []string
	var args []any
	if f.Kind != "" {
		args = append(args, f.Kind)
		where = append(where, fmt.Sprintf("kind = $%d", len(args)))
	}
	if f.RefID != 0 {
		args = append(args, f.RefID)
		where = append(where, fmt.Sprintf("ref_id = $%d", len(args)))
	}
	if f.UnreviewedOnly {
		where = append(where, "reviewed_at IS NULL")
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	args = append(args, limit)
	q := "SELECT " + decisionColumns + " FROM decision_log"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += fmt.Sprintf(" ORDER BY id DESC LIMIT $%d", len(args))
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}
	defer rows.Close()
	var out []Decision
	for rows.Next() {
		var d Decision
		if err := rows.Scan(
			&d.ID, &d.Kind, &d.RefKind, &d.RefID, &d.Key, &d.Model, &d.PromptID, &d.PromptVersion, &d.NumCtx,
			&d.Input, &d.Output, &d.PromptText, &d.ResponseText,
			&d.PromptTokens, &d.CompletionTok, &d.LatencyMs, &d.CreatedAt,
			&d.HumanVerdict, &d.HumanNote, &d.ReviewedBy, &d.ReviewedAt,
		); err != nil {
			return nil, fmt.Errorf("scan decision: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CountDecisions returns totals for the review surface header.
func (r *Repo) CountDecisions(ctx context.Context) (total, reviewed int64, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE reviewed_at IS NOT NULL) FROM decision_log`,
	).Scan(&total, &reviewed)
	if err != nil {
		return 0, 0, fmt.Errorf("count decisions: %w", err)
	}
	return total, reviewed, nil
}

// ReviewDecision records the owner's label. Saving again overwrites.
func (r *Repo) ReviewDecision(ctx context.Context, id, reviewerID int64, verdict, note string) error {
	tag, err := r.pool.Exec(ctx, `
    UPDATE decision_log
       SET human_verdict = $2, human_note = NULLIF($3, ''), reviewed_by = $4, reviewed_at = now()
     WHERE id = $1`, id, verdict, note, reviewerID)
	if err != nil {
		return fmt.Errorf("review decision: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReviewVocabulary is what the owner may answer, per decision kind.
//
// It is per kind rather than one flat list because "met" on a gate row
// or "above_threshold" on a requirement is meaningless, and a label
// that means nothing is worse than no label: it still counts.
//
// A kind missing from this map cannot be graded at all, which is
// deliberate. Adding a decision kind without deciding what a human is
// supposed to say about it is the bug that produced this map: the
// posting check started logging rows the console offered no valid
// answer for, and every attempt to grade one failed silently.
//
// `insufficient_evidence` is in every list. It is the reviewer saying
// they could not judge the row from what they were shown, which is
// neither agreement nor disagreement, and it is excluded from the
// agreement rate rather than counted against it.
var ReviewVocabulary = map[string][]string{
	"jd_requirement_verdict": {"met", "partial", "unmet", "insufficient_evidence"},
	"jd_gate":                {"above_threshold", "below_threshold", "insufficient_evidence"},
	"jd_posting_check":       {"posting", "not_posting", "insufficient_evidence"},
}

// DecisionKind returns the kind of one logged decision, so a review can
// be validated against the vocabulary that actually applies to it.
func (r *Repo) DecisionKind(ctx context.Context, id int64) (string, error) {
	var kind string
	if err := r.pool.QueryRow(ctx, `SELECT kind FROM decision_log WHERE id = $1`, id).Scan(&kind); err != nil {
		return "", ErrNotFound
	}
	return kind, nil
}

// ExportDecisions returns rows oldest first, optionally only the
// reviewed ones, for the JSONL training export.
func (r *Repo) ExportDecisions(ctx context.Context, reviewedOnly bool) ([]Decision, error) {
	q := "SELECT " + decisionColumns + " FROM decision_log"
	if reviewedOnly {
		q += " WHERE reviewed_at IS NOT NULL"
	}
	q += " ORDER BY id ASC"
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("export decisions: %w", err)
	}
	defer rows.Close()
	var out []Decision
	for rows.Next() {
		var d Decision
		if err := rows.Scan(
			&d.ID, &d.Kind, &d.RefKind, &d.RefID, &d.Key, &d.Model, &d.PromptID, &d.PromptVersion, &d.NumCtx,
			&d.Input, &d.Output, &d.PromptText, &d.ResponseText,
			&d.PromptTokens, &d.CompletionTok, &d.LatencyMs, &d.CreatedAt,
			&d.HumanVerdict, &d.HumanNote, &d.ReviewedBy, &d.ReviewedAt,
		); err != nil {
			return nil, fmt.Errorf("scan decision: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
