-- +goose Up
-- +goose StatementBegin

-- One row per pipeline run (data layer D2).
--
-- The problem this fixes: a re-score overwrites `jd_submissions`. The
-- score, the assessment, the status and the résumé are all replaced, so
-- the evidence of what the previous run did, and of what produced it,
-- is gone. That makes the central question unanswerable: did the change
-- to the prompt, the model, or the corpus make the reviewer better or
-- worse? Without a durable per-run record there is nothing to compare.
--
-- So a run is immutable once finished, and it carries everything needed
-- to reproduce the verdict: which build, which model and context size,
-- which prompts at which version and content hash, and which corpus
-- (fingerprinted by document content) was retrieved from. `run_id` then
-- appears on llm_usage and decision_log so every model call and every
-- logged decision belongs to exactly one run.
--
-- Token counts and cost are deliberately NOT copied here. They live in
-- llm_usage and are summed by run_id when wanted. A denormalized copy
-- would be a second source of truth that quietly drifts.

CREATE TABLE jd_runs (
  id                 bigserial PRIMARY KEY,
  -- Stable external id; llm_usage and decision_log reference this, not
  -- the serial, so a row can be exported without leaking sequence order.
  run_id             uuid        NOT NULL UNIQUE,
  tenant_id          bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  submission_id      bigint      NOT NULL REFERENCES jd_submissions(id) ON DELETE CASCADE,
  -- 1 for the first run, 2 for the first re-score, and so on.
  attempt            integer     NOT NULL DEFAULT 1,
  -- submit | rescore
  trigger            text        NOT NULL DEFAULT 'submit',
  triggered_by       bigint      REFERENCES users(id) ON DELETE SET NULL,

  -- running | ready | below_threshold | failed. A row left at `running`
  -- means the process died mid-run, which is itself worth seeing.
  status             text        NOT NULL DEFAULT 'running',
  error              text        NOT NULL DEFAULT '',

  -- What produced the verdict.
  app_commit         text        NOT NULL DEFAULT '',
  host               text        NOT NULL DEFAULT '',  -- which model host served it
  model              text        NOT NULL DEFAULT '',
  num_ctx            integer     NOT NULL DEFAULT 0,
  embedder_model     text        NOT NULL DEFAULT '',
  -- {"requirement_judge": "v2:9f1c2e7a", ...}: version and a hash of the
  -- prompt text, so an edited prompt is visible even at the same version.
  prompts            jsonb       NOT NULL DEFAULT '{}'::jsonb,
  -- Hash over the corpus documents' content hashes, plus the counts. Two
  -- runs with the same fingerprint read the same corpus.
  corpus_fingerprint text        NOT NULL DEFAULT '',
  corpus_documents   integer     NOT NULL DEFAULT 0,
  corpus_chunks      integer     NOT NULL DEFAULT 0,

  -- What it decided. Kept per run because the submission's copy is
  -- overwritten by the next re-score.
  score_formula      text        NOT NULL DEFAULT '',
  retrieval_score    double precision,
  match_score        double precision,
  threshold          double precision,
  fit                text        NOT NULL DEFAULT '',
  requirement_count  integer     NOT NULL DEFAULT 0,
  met_count          integer     NOT NULL DEFAULT 0,
  partial_count      integer     NOT NULL DEFAULT 0,
  unmet_count        integer     NOT NULL DEFAULT 0,
  resume_generated   boolean     NOT NULL DEFAULT false,

  -- How long it took, split so queue time is not mistaken for work.
  queued_ms          bigint      NOT NULL DEFAULT 0,
  duration_ms        bigint      NOT NULL DEFAULT 0,
  started_at         timestamptz NOT NULL DEFAULT now(),
  finished_at        timestamptz
);

COMMENT ON TABLE jd_runs IS
  'One immutable record per JD pipeline run: what produced the verdict and what it decided (docs/decision-log.md).';

CREATE INDEX idx_jd_runs_submission ON jd_runs (tenant_id, submission_id, attempt DESC);
CREATE INDEX idx_jd_runs_started ON jd_runs (tenant_id, started_at DESC);
CREATE INDEX idx_jd_runs_status ON jd_runs (tenant_id, status) WHERE status = 'running';

-- Every model call and every logged decision belongs to a run. NULL for
-- the rows written before this migration, and for future callers that
-- are not part of a JD run (chat, when it lands, gets its own run kind).
ALTER TABLE llm_usage    ADD COLUMN run_id uuid REFERENCES jd_runs(run_id) ON DELETE SET NULL;
ALTER TABLE decision_log ADD COLUMN run_id uuid REFERENCES jd_runs(run_id) ON DELETE SET NULL;

CREATE INDEX idx_llm_usage_run ON llm_usage (run_id) WHERE run_id IS NOT NULL;
CREATE INDEX idx_decision_log_run ON decision_log (run_id) WHERE run_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_decision_log_run;
DROP INDEX IF EXISTS idx_llm_usage_run;
ALTER TABLE decision_log DROP COLUMN IF EXISTS run_id;
ALTER TABLE llm_usage    DROP COLUMN IF EXISTS run_id;
DROP TABLE IF EXISTS jd_runs;

-- +goose StatementEnd
