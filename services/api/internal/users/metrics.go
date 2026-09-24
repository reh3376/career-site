package users

import (
	"context"
	"fmt"
	"time"

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
	// Gradeable excludes the rows the reviewer could not judge from
	// what they were shown; the percentage is out of those, not out of
	// everything reviewed.
	Reviewed          int
	Gradeable         int
	Ungradeable       int
	Agreed            int
	AgreementPct      *float64
	SoftDisagreements int
	HardDisagreements int
	// Which way the judge errs. Three harsh and no generous is a bias
	// to fix; a mix is noise, and they call for opposite changes.
	TooHarsh    int
	TooGenerous int

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
    SELECT reviewed, gradeable, ungradeable, agreed, agreement_pct,
           soft_disagreements, hard_disagreements, too_harsh, too_generous
      FROM v_judge_agreement_summary WHERE tenant_id = $1`, tid).
		Scan(&m.Reviewed, &m.Gradeable, &m.Ungradeable, &m.Agreed, &m.AgreementPct,
			&m.SoftDisagreements, &m.HardDisagreements, &m.TooHarsh, &m.TooGenerous)

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

// GateRow is one criterion answered, read from v_gate (migration
// 00031).
//
// Pass is deliberately a pointer. The third state is the point of the
// gate: a criterion with no target set, or with too little data to
// judge, is unanswered rather than failing, and flattening that into
// false would make a young system look broken.
type GateRow struct {
	Criterion string
	Pass      *bool
	Value     string
	Target    string
	AsOf      *time.Time
	Detail    string
}

// Gate returns the criteria in the owner's order.
func (r *Repo) Gate(ctx context.Context) ([]GateRow, error) {
	const q = `
    SELECT criterion, pass, value, target, as_of, detail
      FROM v_gate
     WHERE tenant_id = $1
     ORDER BY ord
  `
	rows, err := r.pool.Query(ctx, q, tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("select gate: %w", err)
	}
	defer rows.Close()

	var out []GateRow
	for rows.Next() {
		var g GateRow
		if err := rows.Scan(&g.Criterion, &g.Pass, &g.Value, &g.Target, &g.AsOf, &g.Detail); err != nil {
			return nil, fmt.Errorf("scan gate row: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
