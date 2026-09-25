package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
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
	// Gated is false for a row that is measured on purpose and not
	// judged on purpose.
	Gated bool
}

// Gate returns the criteria in the owner's order.
func (r *Repo) Gate(ctx context.Context) ([]GateRow, error) {
	const q = `
    SELECT criterion, pass, value, target, as_of, detail, gated
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
		if err := rows.Scan(&g.Criterion, &g.Pass, &g.Value, &g.Target, &g.AsOf, &g.Detail, &g.Gated); err != nil {
			return nil, fmt.Errorf("scan gate row: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ReviewerStatus is the part of the gate that can be published: how the
// judge agrees with the owner's own grading, and how the fixed posting
// set scored last time.
//
// It carries nothing about who visited, who registered, or what anyone
// submitted. Those are on the same page of views and are deliberately
// not here: publishing a funnel says something about other people,
// publishing a disagreement rate says something about the reviewer.
type ReviewerStatus struct {
	Graded, Agreed                  int
	AgreementPct                    float64
	HardDisagreements               int
	TooHarsh, TooGenerous           int
	Postings, PostingsRandom        int
	Scored, GateCorrect, Inversions int
	Margin                          *float64
	EvaluatedAt                     *time.Time
	Model                           string
	// Bands are the thresholds in force, so a page without a session can
	// quote the real ones rather than a default that is right only until
	// the owner edits them.
	Bands *FitBands
}

// FitBands mirrors the stored jd_fit_bands setting.
type FitBands struct {
	VeryStrong float64 `json:"very_strong"`
	Strong     float64 `json:"strong"`
	Possible   float64 `json:"possible"`
	Weak       float64 `json:"weak"`
}

// ReviewerStatus reads the publishable numbers from the same views the
// admin gate reads, so the public page cannot quote something the owner
// is not also looking at.
func (r *Repo) ReviewerStatus(ctx context.Context) (*ReviewerStatus, error) {
	out := &ReviewerStatus{}
	tid := tenant.FromContext(ctx).Int64()

	const agreeQ = `
    SELECT COALESCE(gradeable, 0), COALESCE(agreed, 0), COALESCE(agreement_pct, 0),
           COALESCE(hard_disagreements, 0), COALESCE(too_harsh, 0), COALESCE(too_generous, 0)
      FROM v_judge_agreement_summary WHERE tenant_id = $1
  `
	if err := r.pool.QueryRow(ctx, agreeQ, tid).Scan(
		&out.Graded, &out.Agreed, &out.AgreementPct,
		&out.HardDisagreements, &out.TooHarsh, &out.TooGenerous,
	); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("reviewer agreement: %w", err)
	}

	const setQ = `
    SELECT count(*), count(*) FILTER (WHERE selection = 'random')
      FROM golden_postings WHERE tenant_id = $1 AND active AND expected_gate <> ''
  `
	if err := r.pool.QueryRow(ctx, setQ, tid).Scan(&out.Postings, &out.PostingsRandom); err != nil {
		return nil, fmt.Errorf("reviewer posting set: %w", err)
	}

	const evalQ = `
    SELECT COALESCE(scored, 0), COALESCE(gate_correct, 0), COALESCE(order_violations, 0),
           margin, started_at, COALESCE(model, '')
      FROM v_eval_history
     WHERE tenant_id = $1 AND status = 'done' AND scored > 0
     ORDER BY started_at DESC LIMIT 1
  `
	if err := r.pool.QueryRow(ctx, evalQ, tid).Scan(
		&out.Scored, &out.GateCorrect, &out.Inversions,
		&out.Margin, &out.EvaluatedAt, &out.Model,
	); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("reviewer evaluation: %w", err)
	}

	if raw, ok, bErr := r.GetSetting(ctx, "jd_fit_bands"); bErr == nil && ok {
		var b FitBands
		if json.Unmarshal(raw, &b) == nil && b.Strong > 0 {
			out.Bands = &b
		}
	}
	return out, nil
}

// ComparisonRow is one golden posting scored in the two most recent
// completed evaluations.
type ComparisonRow struct {
	Selection string
	RoleHint  string
	Score     float64
	Previous  *float64
	Unchanged bool
}

// ReviewerComparison returns every posting in the last completed
// evaluation beside its score in the one before it.
//
// Empty until two evaluations have produced results, which is the
// honest answer rather than a table of one column pretending to be
// evidence. Completion is deliberately not required: a run stopped part
// way still scored what it reached, and whether those scores reproduce
// is the whole question.
// The rounding to three places happens here so that "unchanged" means
// the same thing as the number a reader sees, rather than differing in
// a digit nobody is shown.
func (r *Repo) ReviewerComparison(ctx context.Context) ([]ComparisonRow, error) {
	const q = `
    WITH done AS (
      -- Runs that produced results, complete or not. A run stopped part
      -- way still scored the postings it reached, and whether those
      -- scores reproduce is exactly the question. Requiring completion
      -- would throw away seven good measurements because an eighth
      -- never ran.
      SELECT r.id FROM eval_runs r
       WHERE r.tenant_id = $1
         AND EXISTS (SELECT 1 FROM eval_items i
                      WHERE i.eval_run_id = r.id AND i.match_score IS NOT NULL)
       ORDER BY r.started_at DESC LIMIT 2
    ), latest AS (SELECT min(id) AS id FROM (SELECT id FROM done ORDER BY id DESC LIMIT 1) x),
      prior AS (SELECT min(id) AS id FROM (SELECT id FROM done ORDER BY id ASC LIMIT 1) y)
    SELECT g.selection,
           COALESCE(g.role_hint, ''),
           round(cur.match_score::numeric, 3)::float8,
           round(prev.match_score::numeric, 3)::float8
      FROM eval_items cur
      JOIN golden_postings g ON g.id = cur.golden_id
      LEFT JOIN eval_items prev
        ON prev.golden_id = cur.golden_id
       AND prev.eval_run_id = (SELECT id FROM prior)
     WHERE cur.eval_run_id = (SELECT id FROM latest)
       AND cur.match_score IS NOT NULL
       AND (SELECT count(*) FROM done) = 2
     ORDER BY cur.match_score DESC
  `
	rows, err := r.pool.Query(ctx, q, tenant.FromContext(ctx).Int64())
	if err != nil {
		return nil, fmt.Errorf("reviewer comparison: %w", err)
	}
	defer rows.Close()

	var out []ComparisonRow
	for rows.Next() {
		var c ComparisonRow
		if err := rows.Scan(&c.Selection, &c.RoleHint, &c.Score, &c.Previous); err != nil {
			return nil, fmt.Errorf("scan comparison row: %w", err)
		}
		c.Unchanged = c.Previous != nil && *c.Previous == c.Score
		out = append(out, c)
	}
	return out, rows.Err()
}

// PublicLabel describes a posting without disclosing anything the owner
// has not chosen to disclose.
//
// A posting he applied for is labelled by the fact that he applied,
// nothing more. Naming it, or even its title, would tell that employer
// he applied and what the reviewer scored him at. A randomly drawn
// posting carries its role, because it is a public advertisement and
// says nothing about him.
func (c ComparisonRow) PublicLabel() string {
	if c.Selection != "random" {
		return "a role he applied for"
	}
	if c.RoleHint == "" {
		return "drawn at random"
	}
	return "drawn at random: " + c.RoleHint
}

// PipelineStatus is how many submissions sit at one status, and how
// long the oldest has been there.
type PipelineStatus struct {
	Status string
	Count  int
	Oldest *time.Time
}

// PipelineCounts groups real submissions by status. Evaluation rows are
// excluded: they are a test of the reviewer rather than a queue of work
// waiting on it, and counting them would make the pipeline look backed
// up every time an evaluation runs.
//
// No tenant filter, because jd_submissions has no tenant_id. Migration
// 00024 added the column to events, decision_log and llm_usage, and to
// everything built after it, but not to the tables that predate it.
// Filtering on a column that does not exist made this return an error
// the handler logged and swallowed, so the console showed an empty
// section rather than a fault. Add the filter here when the column
// arrives, not before.
func (r *Repo) PipelineCounts(ctx context.Context) ([]PipelineStatus, error) {
	const q = `
    SELECT status, count(*), min(created_at)
      FROM jd_submissions
     WHERE NOT is_eval
     GROUP BY status
     ORDER BY count(*) DESC
  `
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("pipeline counts: %w", err)
	}
	defer rows.Close()
	var out []PipelineStatus
	for rows.Next() {
		var p PipelineStatus
		if err := rows.Scan(&p.Status, &p.Count, &p.Oldest); err != nil {
			return nil, fmt.Errorf("scan pipeline count: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// LLMActivity summarises the last hour of model calls: whether the box
// is working, and whether it is failing while it does.
func (r *Repo) LLMActivity(ctx context.Context) (calls, failures int, avgSeconds float64, err error) {
	const q = `
    SELECT count(*), count(*) FILTER (WHERE NOT ok),
           COALESCE(round(avg(latency_ms) / 1000.0, 1), 0)::float8
      FROM llm_usage
     WHERE tenant_id = $1 AND created_at > now() - interval '1 hour'
  `
	if err := r.pool.QueryRow(ctx, q, tenant.FromContext(ctx).Int64()).
		Scan(&calls, &failures, &avgSeconds); err != nil {
		return 0, 0, 0, fmt.Errorf("llm activity: %w", err)
	}
	return calls, failures, avgSeconds, nil
}
