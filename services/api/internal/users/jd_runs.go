package users

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/reh3376/career-site/services/api/internal/tenant"
)

// JdRun is one pipeline run: what produced a verdict and what it
// decided. Written once at the start and once at the end, then never
// touched, because its whole purpose is to survive the next re-score
// that overwrites the submission row.
type JdRun struct {
	RunID        string
	SubmissionID int64
	Attempt      int
	Trigger      string // submit | rescore
	TriggeredBy  int64  // admin who pressed re-score; 0 for a submit
	Status       string // running | ready | below_threshold | failed
	Error        string

	// Provenance.
	AppCommit         string
	Host              string
	Model             string
	NumCtx            int
	EmbedderModel     string
	Prompts           map[string]string // prompt id -> "v2:9f1c2e7a"
	CorpusFingerprint string
	// RetrievalScope is what the run could read: all or public.
	RetrievalScope  string
	CorpusDocuments int
	CorpusChunks    int

	// Outcome.
	ScoreFormula     string
	RetrievalScore   *float64
	MatchScore       *float64
	Threshold        *float64
	Fit              string
	RequirementCount int
	MetCount         int
	PartialCount     int
	UnmetCount       int
	ResumeGenerated  bool

	QueuedMs   int64
	DurationMs int64
	StartedAt  time.Time
	FinishedAt *time.Time
}

// StartJdRun opens a run and returns its attempt number. The attempt is
// derived here rather than passed in so two concurrent starts cannot
// both claim the same number.
func (r *Repo) StartJdRun(ctx context.Context, run JdRun) (int, error) {
	prompts := run.Prompts
	if prompts == nil {
		prompts = map[string]string{}
	}
	raw, err := json.Marshal(prompts)
	if err != nil {
		return 0, fmt.Errorf("start jd run: marshal prompts: %w", err)
	}
	var attempt int
	err = r.pool.QueryRow(ctx, `
    INSERT INTO jd_runs
      (run_id, tenant_id, submission_id, attempt, trigger, triggered_by, status,
       app_commit, host, model, num_ctx, embedder_model, prompts,
       corpus_fingerprint, corpus_documents, corpus_chunks, queued_ms,
       retrieval_scope)
    VALUES ($1::uuid, $2, $3,
            (SELECT coalesce(max(attempt), 0) + 1 FROM jd_runs WHERE submission_id = $3),
            $4, NULLIF($5, 0), 'running',
            $6, $7, $8, $9, $10, $11::jsonb, $12, $13, $14, $15, $16)
    RETURNING attempt`,
		run.RunID, tenant.FromContext(ctx).Int64(), run.SubmissionID,
		run.Trigger, run.TriggeredBy,
		run.AppCommit, run.Host, run.Model, run.NumCtx, run.EmbedderModel, string(raw),
		run.CorpusFingerprint, run.CorpusDocuments, run.CorpusChunks, run.QueuedMs,
		run.RetrievalScope,
	).Scan(&attempt)
	if err != nil {
		return 0, fmt.Errorf("start jd run: %w", err)
	}
	return attempt, nil
}

// FinishJdRun closes a run with its outcome. Called from a deferred
// function on a context that outlives cancellation, so a run is never
// left at `running` just because the caller went away.
func (r *Repo) FinishJdRun(ctx context.Context, run JdRun) error {
	_, err := r.pool.Exec(ctx, `
    UPDATE jd_runs SET
      status = $2, error = $3,
      score_formula = $4, retrieval_score = $5, match_score = $6, threshold = $7,
      fit = $8, requirement_count = $9, met_count = $10, partial_count = $11,
      unmet_count = $12, resume_generated = $13,
      model = coalesce(NULLIF($14, ''), model),
      queued_ms = $15, duration_ms = $16, finished_at = now()
    WHERE run_id = $1::uuid AND finished_at IS NULL`,
		run.RunID, run.Status, truncRunErr(run.Error),
		run.ScoreFormula, run.RetrievalScore, run.MatchScore, run.Threshold,
		run.Fit, run.RequirementCount, run.MetCount, run.PartialCount,
		run.UnmetCount, run.ResumeGenerated, run.Model, run.QueuedMs, run.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("finish jd run: %w", err)
	}
	return nil
}

// ListJdRuns returns a submission's runs, newest attempt first. The
// admin JD page shows these so a superseded verdict is still visible.
func (r *Repo) ListJdRuns(ctx context.Context, submissionID int64) ([]JdRun, error) {
	rows, err := r.pool.Query(ctx, `
    SELECT run_id::text, submission_id, attempt, trigger, coalesce(triggered_by, 0),
           status, error, app_commit, host, model, num_ctx, embedder_model,
           prompts::text, corpus_fingerprint, corpus_documents, corpus_chunks,
           score_formula, retrieval_score, match_score, threshold, fit,
           requirement_count, met_count, partial_count, unmet_count,
           resume_generated, queued_ms, duration_ms, started_at, finished_at
      FROM jd_runs
     WHERE submission_id = $1
     ORDER BY attempt DESC`, submissionID)
	if err != nil {
		return nil, fmt.Errorf("list jd runs: %w", err)
	}
	defer rows.Close()

	var out []JdRun
	for rows.Next() {
		var run JdRun
		var promptsJSON string
		if err := rows.Scan(
			&run.RunID, &run.SubmissionID, &run.Attempt, &run.Trigger, &run.TriggeredBy,
			&run.Status, &run.Error, &run.AppCommit, &run.Host, &run.Model, &run.NumCtx,
			&run.EmbedderModel, &promptsJSON, &run.CorpusFingerprint, &run.CorpusDocuments,
			&run.CorpusChunks, &run.ScoreFormula, &run.RetrievalScore, &run.MatchScore,
			&run.Threshold, &run.Fit, &run.RequirementCount, &run.MetCount,
			&run.PartialCount, &run.UnmetCount, &run.ResumeGenerated, &run.QueuedMs,
			&run.DurationMs, &run.StartedAt, &run.FinishedAt,
		); err != nil {
			return nil, fmt.Errorf("list jd runs: scan: %w", err)
		}
		_ = json.Unmarshal([]byte(promptsJSON), &run.Prompts)
		out = append(out, run)
	}
	return out, rows.Err()
}

// FailStrandedRuns closes runs left at `running` by a process that
// died. Called at boot beside FailStrandedJd, so the run table never
// implies work is still happening when nothing is.
func (r *Repo) FailStrandedRuns(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
    UPDATE jd_runs
       SET status = 'failed', error = 'the process restarted while this run was in flight',
           finished_at = now()
     WHERE status = 'running' AND finished_at IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("fail stranded runs: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CorpusFingerprint identifies the corpus a run read: a hash over the
// documents' content hashes plus the counts. Two runs with the same
// fingerprint saw the same corpus, so a score change between them is
// not the corpus.
func (r *Repo) CorpusFingerprint(ctx context.Context) (fingerprint string, docs, chunks int, embedder string, err error) {
	err = r.pool.QueryRow(ctx, `
    SELECT coalesce(md5(string_agg(h, ',' ORDER BY h)), ''),
           count(*)::int
      FROM (SELECT coalesce(encode(content_hash, 'hex'), id::text) AS h
              FROM corpus_documents) d`).Scan(&fingerprint, &docs)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("corpus fingerprint: %w", err)
	}
	// The embedder is whichever model the chunks were built with; more
	// than one means a sweep is half-done, which is worth seeing in the
	// run record rather than hidden behind a "first row wins".
	err = r.pool.QueryRow(ctx, `
    SELECT count(*)::int, coalesce(string_agg(DISTINCT embedder_model, '+'), '')
      FROM corpus_chunks`).Scan(&chunks, &embedder)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("corpus fingerprint: chunks: %w", err)
	}
	if len(fingerprint) > 12 {
		fingerprint = fingerprint[:12]
	}
	return fingerprint, docs, chunks, embedder, nil
}

func truncRunErr(s string) string {
	const max = 2000
	if len(s) > max {
		return s[:max]
	}
	return s
}
