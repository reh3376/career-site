package users

import (
	"context"
	"fmt"
	"time"

	"github.com/reh3376/career-site/services/api/internal/runid"
	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// LLMUsage is one row of llm_usage: a single call to the LLM gateway,
// successful or not.
type LLMUsage struct {
	Kind             string // "jd_resume" | "chat"
	RefID            int64
	Model            string
	PromptID         string
	PromptVersion    int
	PromptTokens     int32
	CompletionTokens int32
	LatencyMs        int64
	OK               bool
	Error            string
}

// RecordLLMUsage appends one ledger row. Best-effort for callers: the
// generation has already happened, so a ledger failure is logged, not
// surfaced.
func (r *Repo) RecordLLMUsage(ctx context.Context, u LLMUsage) error {
	const q = `
    INSERT INTO llm_usage
      (tenant_id, run_id, kind, ref_id, model, prompt_id, prompt_version,
       prompt_tokens, completion_tokens, latency_ms, ok, error)
    VALUES ($1, NULLIF($2, '')::uuid, $3, NULLIF($4, 0), $5, $6, $7, $8, $9, $10, $11, NULLIF($12, ''))
  `
	if _, err := r.pool.Exec(ctx, q,
		tenant.FromContext(ctx).Int64(), runid.FromContext(ctx),
		u.Kind, u.RefID, u.Model, u.PromptID, u.PromptVersion,
		u.PromptTokens, u.CompletionTokens, u.LatencyMs, u.OK, u.Error,
	); err != nil {
		return fmt.Errorf("record llm usage: %w", err)
	}
	return nil
}

// CountLLMCallsSince counts ledger rows (successful or not) since a
// point in time; the monthly call cap reads it before each generation.
func (r *Repo) CountLLMCallsSince(ctx context.Context, since time.Time) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM llm_usage WHERE created_at >= $1`, since,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count llm calls: %w", err)
	}
	return n, nil
}
