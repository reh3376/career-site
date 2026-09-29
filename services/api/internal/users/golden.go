package users

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// GoldenPosting is one job description with a stated expectation.
//
// The expectation answers one question: does the candidate have the
// skill set to do this job. Not whether the level, the pay, the
// location or the hours suit him. Those are parameter filters the
// candidate sets for himself, they need no model, and mixing them in
// here would make the reviewer's verdict mean two things at once.
//
// It is a side of the gate rather than a score, because a score is
// model-dependent and would have to be rewritten on every model change,
// and that rewriting is exactly how a regression hides. "He can do this
// job" stays true when the model changes, so that is what gets
// asserted.
type GoldenPosting struct {
	ID           int64
	Name         string
	JdText       string
	RoleHint     string
	EmployerHint string
	// ExpectedGate is "above", "below", or "" when nobody has labelled
	// it yet. Unlabelled postings are skipped by evaluations rather than
	// guessed at.
	ExpectedGate string
	ExpectedBand string // advisory
	Note         string
	// Selection is "chosen" or "random": how the posting got into the
	// set. Reported apart, because a gate that is clean on chosen
	// postings and noisy on random ones is the finding that matters.
	Selection string
	Source    string
	Active    bool
	CreatedAt time.Time
	// LastScore and LastEvalAt come from the newest evaluation that
	// scored it, for the list view.
	LastScore  *float64
	LastPassed *bool
	LastEvalAt *time.Time
}

// UpsertGoldenPosting creates or replaces a golden posting by name.
func (r *Repo) UpsertGoldenPosting(ctx context.Context, g GoldenPosting) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
    INSERT INTO golden_postings
      (tenant_id, name, jd_text, role_hint, employer_hint, expected_gate, expected_band, note, active, selection, source)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    ON CONFLICT (tenant_id, name) DO UPDATE SET
      jd_text = EXCLUDED.jd_text,
      role_hint = EXCLUDED.role_hint,
      employer_hint = EXCLUDED.employer_hint,
      expected_gate = EXCLUDED.expected_gate,
      expected_band = EXCLUDED.expected_band,
      note = EXCLUDED.note,
      active = EXCLUDED.active,
      selection = EXCLUDED.selection,
      source = EXCLUDED.source
    RETURNING id`,
		tenant.FromContext(ctx).Int64(), g.Name, g.JdText, g.RoleHint, g.EmployerHint,
		g.ExpectedGate, g.ExpectedBand, g.Note, g.Active, g.Selection, g.Source).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert golden posting: %w", err)
	}
	return id, nil
}

// SetGoldenActive retires or restores a posting. Retired postings stay
// for history, because deleting one would silently change what every
// past evaluation was measuring.
func (r *Repo) SetGoldenActive(ctx context.Context, id int64, active bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE golden_postings SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return fmt.Errorf("set golden active: %w", err)
	}
	return nil
}

// LabelGoldenPosting records which side of the gate a posting belongs
// on. Kept separate from the upsert so a label does not require
// resending the posting, and so the unlabelled state is one the console
// can act on rather than a gap to be filled by guessing.
func (r *Repo) LabelGoldenPosting(ctx context.Context, id int64, gate, note string) error {
	tag, err := r.pool.Exec(ctx, `
    UPDATE golden_postings
       SET expected_gate = $2,
           note = CASE WHEN $3 = '' THEN note
                       WHEN note = '' THEN $3
                       ELSE note || ' | ' || $3 END
     WHERE id = $1`, id, gate, note)
	if err != nil {
		return fmt.Errorf("label golden posting: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListGoldenPostings returns the set with each posting's most recent
// result attached.
func (r *Repo) ListGoldenPostings(ctx context.Context, activeOnly bool) ([]GoldenPosting, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT g.id, g.name, g.jd_text, g.role_hint, g.employer_hint,
           g.expected_gate, g.expected_band, g.note, g.selection, g.source, g.active, g.created_at,
           last.match_score, last.passed, last.created_at
      FROM golden_postings g
      LEFT JOIN LATERAL (
        SELECT i.match_score, i.passed, i.created_at
          FROM eval_items i WHERE i.golden_id = g.id
         ORDER BY i.id DESC LIMIT 1
      ) last ON true
     WHERE g.tenant_id = $1 AND ($2 = false OR (g.active AND g.expected_gate <> ''))
     ORDER BY g.expected_gate DESC, g.name`,
		tenant.FromContext(ctx).Int64(), activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list golden postings: %w", err)
	}
	defer rows.Close()

	var out []GoldenPosting
	for rows.Next() {
		var g GoldenPosting
		if err := rows.Scan(&g.ID, &g.Name, &g.JdText, &g.RoleHint, &g.EmployerHint,
			&g.ExpectedGate, &g.ExpectedBand, &g.Note, &g.Selection, &g.Source, &g.Active, &g.CreatedAt,
			&g.LastScore, &g.LastPassed, &g.LastEvalAt); err != nil {
			return nil, fmt.Errorf("list golden postings: scan: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// EvalRun is one scoring of the whole active set under one config.
type EvalRun struct {
	ID                int64
	EvalID            string
	Status            string
	Note              string
	TriggeredBy       int64
	AppCommit         string
	Host              string
	Model             string
	NumCtx            int
	EmbedderModel     string
	Prompts           map[string]string
	CorpusFingerprint string
	Threshold         *float64
	Total             int
	Scored            int
	GateCorrect       int
	OrderViolations   int
	Margin            *float64
	Errors            int
	StartedAt         time.Time
	FinishedAt        *time.Time
	Items             []EvalItem
	// CorpusDocuments is what the corpus held when the run started,
	// captured because the corpus is mutable and has no delete, so a
	// later ingest otherwise makes an older run unreadable. Populated
	// by GetEvalRun only.
	CorpusDocuments []EvalCorpusDoc
}

// EvalCorpusDoc is one document the corpus held when a run began. The
// title and kind are copied rather than joined, so re-indexing a
// document later does not rewrite the history of a run that read the
// version before it.
type EvalCorpusDoc struct {
	DocumentID int64
	Title      string
	SourceKind string
	Visibility string
	ChunkCount int
	// ContentHash is what the document's text hashed to at capture.
	// Empty for runs captured before it was recorded, which means
	// unknown rather than unchanged. Without it the manifest proves
	// which documents a run read and not whether they were the same
	// documents, and an edit that preserves the chunk count is
	// invisible.
	ContentHash string
}

// EvalItem is one posting's result inside one evaluation.
type EvalItem struct {
	GoldenID     int64
	GoldenName   string
	ExpectedGate string
	RunID        string
	SubmissionID int64
	MatchScore   *float64
	Fit          string
	GateSide     string
	Passed       bool
	Error        string
}

// StartEvalRun opens an evaluation and returns its row id.
func (r *Repo) StartEvalRun(ctx context.Context, e EvalRun) (int64, error) {
	raw, err := json.Marshal(e.Prompts)
	if err != nil {
		raw = []byte("{}")
	}
	var id int64
	err = r.pool.QueryRow(ctx, `
    INSERT INTO eval_runs
      (eval_id, tenant_id, note, triggered_by, app_commit, host, model, num_ctx,
       embedder_model, prompts, corpus_fingerprint, threshold, total)
    VALUES ($1::uuid, $2, $3, NULLIF($4, 0), $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13)
    RETURNING id`,
		e.EvalID, tenant.FromContext(ctx).Int64(), e.Note, e.TriggeredBy,
		e.AppCommit, e.Host, e.Model, e.NumCtx, e.EmbedderModel, string(raw),
		e.CorpusFingerprint, e.Threshold, e.Total).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("start eval run: %w", err)
	}
	return id, nil
}

// SnapshotEvalCorpus records what the corpus held for this run.
//
// One statement rather than a row per document: the list is small and
// this runs on the path that starts a four-hour job, so it should cost
// nothing and never be the reason a run fails to start. A failure here
// is returned and logged by the caller but does not stop the run,
// because an evaluation without its manifest is still worth having.
func (r *Repo) SnapshotEvalCorpus(ctx context.Context, evalRunID int64) (int, error) {
	tag, err := r.pool.Exec(ctx, `
    INSERT INTO eval_run_documents (eval_run_id, document_id, title, source_kind, visibility, chunk_count, content_hash)
    SELECT $1, d.id, coalesce(d.title, ''), coalesce(d.source_kind, ''), coalesce(d.visibility, ''),
           (SELECT count(*) FROM corpus_chunks c WHERE c.document_id = d.id),
           d.content_hash
      FROM corpus_documents d
    ON CONFLICT (eval_run_id, document_id) DO NOTHING`, evalRunID)
	if err != nil {
		return 0, fmt.Errorf("snapshot eval corpus: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// RecordEvalItem stores one posting's result.
func (r *Repo) RecordEvalItem(ctx context.Context, evalRunID int64, it EvalItem) error {
	_, err := r.pool.Exec(ctx, `
    INSERT INTO eval_items
      (eval_run_id, golden_id, run_id, submission_id, match_score, fit, gate_side, passed, error)
    VALUES ($1, $2, NULLIF($3, '')::uuid, NULLIF($4, 0), $5, $6, $7, $8, $9)
    ON CONFLICT (eval_run_id, golden_id) DO UPDATE SET
      run_id = EXCLUDED.run_id, submission_id = EXCLUDED.submission_id,
      match_score = EXCLUDED.match_score, fit = EXCLUDED.fit,
      gate_side = EXCLUDED.gate_side, passed = EXCLUDED.passed, error = EXCLUDED.error`,
		evalRunID, it.GoldenID, it.RunID, it.SubmissionID, it.MatchScore,
		it.Fit, it.GateSide, it.Passed, truncNote(it.Error))
	if err != nil {
		return fmt.Errorf("record eval item: %w", err)
	}
	return nil
}

// FinishEvalRun closes an evaluation with its results.
func (r *Repo) FinishEvalRun(ctx context.Context, e EvalRun) error {
	_, err := r.pool.Exec(ctx, `
    UPDATE eval_runs SET
      status = $2, scored = $3, gate_correct = $4, order_violations = $5,
      margin = $6, errors = $7, model = coalesce(NULLIF($8, ''), model),
      finished_at = now()
    WHERE id = $1`,
		e.ID, e.Status, e.Scored, e.GateCorrect, e.OrderViolations,
		e.Margin, e.Errors, e.Model)
	if err != nil {
		return fmt.Errorf("finish eval run: %w", err)
	}
	return nil
}

// ListEvalRuns returns evaluations newest first, without their items.
func (r *Repo) ListEvalRuns(ctx context.Context, limit int) ([]EvalRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := r.pool.Query(ctx, `
    SELECT id, eval_id::text, status, note, coalesce(triggered_by, 0), app_commit, host,
           model, num_ctx, embedder_model, prompts::text, corpus_fingerprint, threshold,
           total, scored, gate_correct, order_violations, margin, errors,
           started_at, finished_at
      FROM eval_runs WHERE tenant_id = $1 ORDER BY id DESC LIMIT $2`,
		tenant.FromContext(ctx).Int64(), limit)
	if err != nil {
		return nil, fmt.Errorf("list eval runs: %w", err)
	}
	defer rows.Close()
	return scanEvalRuns(rows)
}

// GetEvalRun returns one evaluation with its items.
func (r *Repo) GetEvalRun(ctx context.Context, id int64) (*EvalRun, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT id, eval_id::text, status, note, coalesce(triggered_by, 0), app_commit, host,
           model, num_ctx, embedder_model, prompts::text, corpus_fingerprint, threshold,
           total, scored, gate_correct, order_violations, margin, errors,
           started_at, finished_at
      FROM eval_runs WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("get eval run: %w", err)
	}
	list, err := scanEvalRuns(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	run := list[0]

	itemRows, err := r.pool.Query(ctx, `
    SELECT i.golden_id, g.name, g.expected_gate, coalesce(i.run_id::text, ''),
           coalesce(i.submission_id, 0), i.match_score, i.fit, i.gate_side, i.passed, i.error
      FROM eval_items i JOIN golden_postings g ON g.id = i.golden_id
     WHERE i.eval_run_id = $1
     ORDER BY g.expected_gate DESC, g.name`, id)
	if err != nil {
		return nil, fmt.Errorf("get eval items: %w", err)
	}
	defer itemRows.Close()
	for itemRows.Next() {
		var it EvalItem
		if err := itemRows.Scan(&it.GoldenID, &it.GoldenName, &it.ExpectedGate, &it.RunID,
			&it.SubmissionID, &it.MatchScore, &it.Fit, &it.GateSide, &it.Passed, &it.Error); err != nil {
			return nil, fmt.Errorf("get eval items: scan: %w", err)
		}
		run.Items = append(run.Items, it)
	}
	if err := itemRows.Err(); err != nil {
		return nil, err
	}

	// What the corpus held when this run started. Absent for runs that
	// predate the snapshot, which is reported as an empty list rather
	// than an error: those runs are still readable, they just cannot
	// say what they read.
	docRows, err := r.pool.Query(ctx, `
    SELECT document_id, title, source_kind, visibility, chunk_count,
           coalesce(encode(content_hash, 'hex'), '')
      FROM eval_run_documents
     WHERE eval_run_id = $1
     ORDER BY source_kind, title, document_id`, id)
	if err != nil {
		return nil, fmt.Errorf("get eval corpus: %w", err)
	}
	defer docRows.Close()
	for docRows.Next() {
		var d EvalCorpusDoc
		if err := docRows.Scan(&d.DocumentID, &d.Title, &d.SourceKind, &d.Visibility, &d.ChunkCount, &d.ContentHash); err != nil {
			return nil, fmt.Errorf("get eval corpus: scan: %w", err)
		}
		run.CorpusDocuments = append(run.CorpusDocuments, d)
	}
	return &run, docRows.Err()
}

type rowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

func scanEvalRuns(rows rowScanner) ([]EvalRun, error) {
	defer rows.Close()
	var out []EvalRun
	for rows.Next() {
		var e EvalRun
		var promptsJSON string
		if err := rows.Scan(&e.ID, &e.EvalID, &e.Status, &e.Note, &e.TriggeredBy,
			&e.AppCommit, &e.Host, &e.Model, &e.NumCtx, &e.EmbedderModel, &promptsJSON,
			&e.CorpusFingerprint, &e.Threshold, &e.Total, &e.Scored, &e.GateCorrect,
			&e.OrderViolations, &e.Margin, &e.Errors, &e.StartedAt, &e.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan eval run: %w", err)
		}
		_ = json.Unmarshal([]byte(promptsJSON), &e.Prompts)
		out = append(out, e)
	}
	return out, rows.Err()
}
