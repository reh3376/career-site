package users

import (
	"context"
	"fmt"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// Metrics is the state of the reviewer, read from the SQL views that
// define each number once (migration 00028).
//
// Nothing here computes a metric. Every field is selected from a view,
// so the page, the log and any future export all say the same thing.
// If a number looks wrong, the definition is in the migration and there
// is exactly one of it.
type Metrics struct {
	// Reliability over the last twenty real runs.
	RecentRuns      int
	RecentCompleted int
	RecentFailed    int
	RecentStuck     int

	// Agreement between the model's verdicts and the owner's labels.
	Reviewed          int
	Agreed            int
	AgreementPct      *float64
	SoftDisagreements int
	HardDisagreements int

	// Time to a result.
	FinishedRuns        int
	MedianMinutes       *float64
	P95Minutes          *float64
	MedianQueuedMinutes *float64

	// Thirty-day landing funnel.
	Landed      int
	ReadWriting int
	Clicked     int
	Registered  int
	Verified    int
	SignedIn    int
	Submitted   int

	// Model volume, last 30 days.
	Calls            int
	CallFailures     int
	PromptTokens     int64
	CompletionTokens int64

	// The newest evaluation, when there is one.
	LatestEval *EvalSummary

	// Fit band against what actually happened.
	Outcomes []OutcomeByFit
}

// EvalSummary is the headline of one golden-set evaluation.
type EvalSummary struct {
	ID              int64
	Note            string
	GateCorrect     int
	Scored          int
	GatePct         *float64
	OrderViolations int
	Margin          *float64
}

// OutcomeByFit is one fit band against one real-world outcome.
type OutcomeByFit struct {
	Fit         string
	Outcome     string
	Submissions int
}

// GetMetrics reads every view. A view with no rows yet (no runs, no
// reviews, no visitors) leaves its fields at zero rather than failing,
// because "nothing has happened yet" is a legitimate state and the
// page should say so plainly.
func (r *Repo) GetMetrics(ctx context.Context) (*Metrics, error) {
	tid := tenant.FromContext(ctx).Int64()
	m := &Metrics{}

	// Each of these is a single-row view keyed by tenant; a missing row
	// means the underlying table is empty.
	_ = r.pool.QueryRow(ctx, `
    SELECT runs, completed, failed, stuck FROM v_reliability WHERE tenant_id = $1`, tid).
		Scan(&m.RecentRuns, &m.RecentCompleted, &m.RecentFailed, &m.RecentStuck)

	_ = r.pool.QueryRow(ctx, `
    SELECT reviewed, agreed, agreement_pct, soft_disagreements, hard_disagreements
      FROM v_judge_agreement_summary WHERE tenant_id = $1`, tid).
		Scan(&m.Reviewed, &m.Agreed, &m.AgreementPct, &m.SoftDisagreements, &m.HardDisagreements)

	_ = r.pool.QueryRow(ctx, `
    SELECT finished_runs, median_minutes, p95_minutes, median_queued_minutes
      FROM v_jd_latency WHERE tenant_id = $1`, tid).
		Scan(&m.FinishedRuns, &m.MedianMinutes, &m.P95Minutes, &m.MedianQueuedMinutes)

	_ = r.pool.QueryRow(ctx, `
    SELECT landed, read_writing, clicked, registered, verified, signed_in, submitted
      FROM v_funnel_30d_summary WHERE tenant_id = $1`, tid).
		Scan(&m.Landed, &m.ReadWriting, &m.Clicked, &m.Registered, &m.Verified, &m.SignedIn, &m.Submitted)

	_ = r.pool.QueryRow(ctx, `
    SELECT coalesce(sum(calls), 0), coalesce(sum(failures), 0),
           coalesce(sum(prompt_tokens), 0), coalesce(sum(completion_tokens), 0)
      FROM v_llm_usage_daily
     WHERE tenant_id = $1 AND day > current_date - 30`, tid).
		Scan(&m.Calls, &m.CallFailures, &m.PromptTokens, &m.CompletionTokens)

	var e EvalSummary
	if err := r.pool.QueryRow(ctx, `
    SELECT id, note, gate_correct, scored, gate_pct, order_violations, margin
      FROM v_eval_history
     WHERE tenant_id = $1 AND status = 'done'
     ORDER BY started_at DESC LIMIT 1`, tid).
		Scan(&e.ID, &e.Note, &e.GateCorrect, &e.Scored, &e.GatePct, &e.OrderViolations, &e.Margin); err == nil {
		m.LatestEval = &e
	}

	rows, err := r.pool.Query(ctx, `
    SELECT fit, outcome, submissions FROM v_outcome_by_fit
     WHERE tenant_id = $1 ORDER BY fit, outcome`, tid)
	if err != nil {
		return nil, fmt.Errorf("read outcomes by fit: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var o OutcomeByFit
		if err := rows.Scan(&o.Fit, &o.Outcome, &o.Submissions); err != nil {
			return nil, fmt.Errorf("read outcomes by fit: scan: %w", err)
		}
		m.Outcomes = append(m.Outcomes, o)
	}
	return m, rows.Err()
}
