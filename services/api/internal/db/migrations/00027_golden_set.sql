-- +goose Up
-- +goose StatementBegin

-- The golden set and its evaluations (data layer D4).
--
-- The gate is currently justified by four postings scored by hand and
-- written into a log. That was enough to pick a number once. It cannot
-- answer the question that matters every time a prompt or a model
-- changes: did this change make the reviewer better, worse, or neither?
--
-- A golden posting is one job description with a stated expectation.
-- The expectation is not a score, because a score is model-dependent
-- and would have to be rewritten on every change, which is exactly the
-- rewriting that hides a regression. The expectation is which side of
-- the gate the posting belongs on, which is a claim about the world
-- and stays true when the model changes.

CREATE TABLE golden_postings (
  id             bigserial PRIMARY KEY,
  tenant_id      bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  -- Short human label, unique per tenant, used in summaries.
  name           text        NOT NULL,
  jd_text        text        NOT NULL,
  role_hint      text        NOT NULL DEFAULT '',
  employer_hint  text        NOT NULL DEFAULT '',
  -- above | below: which side of the gate this posting belongs on.
  expected_gate  text        NOT NULL,
  -- Optional, advisory: the band it ought to land in. Not asserted,
  -- because band edges are tuned and the gate side is not.
  expected_band  text        NOT NULL DEFAULT '',
  -- Why this posting is in the set and what it is meant to catch.
  note           text        NOT NULL DEFAULT '',
  -- Retired postings stay for history but are skipped by new runs.
  active         boolean     NOT NULL DEFAULT true,
  created_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name)
);

COMMENT ON TABLE golden_postings IS
  'Fixed job descriptions with a stated expectation, re-scored to measure change (docs/llm-tuning-log.md).';

-- One evaluation: the whole active set scored under one configuration.
CREATE TABLE eval_runs (
  id                 bigserial PRIMARY KEY,
  eval_id            uuid        NOT NULL UNIQUE,
  tenant_id          bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  status             text        NOT NULL DEFAULT 'running', -- running | done | failed
  note               text        NOT NULL DEFAULT '',
  triggered_by       bigint      REFERENCES users(id) ON DELETE SET NULL,

  -- The same provenance a pipeline run records, captured once here so
  -- two evaluations can be compared without reading every item.
  app_commit         text        NOT NULL DEFAULT '',
  host               text        NOT NULL DEFAULT '',
  model              text        NOT NULL DEFAULT '',
  num_ctx            integer     NOT NULL DEFAULT 0,
  embedder_model     text        NOT NULL DEFAULT '',
  prompts            jsonb       NOT NULL DEFAULT '{}'::jsonb,
  corpus_fingerprint text        NOT NULL DEFAULT '',
  threshold          double precision,

  -- Results, computed once when the run closes.
  total              integer     NOT NULL DEFAULT 0,
  scored             integer     NOT NULL DEFAULT 0,
  gate_correct       integer     NOT NULL DEFAULT 0,
  -- Pairs of postings with opposite expectations where the one expected
  -- below outscored the one expected above. Ordering failures are worse
  -- than a gate miss: a gate can be moved, an inversion cannot.
  order_violations   integer     NOT NULL DEFAULT 0,
  -- Smallest gap between an expected-above score and an expected-below
  -- score. A shrinking margin is a regression the gate accuracy hides.
  margin             double precision,
  errors             integer     NOT NULL DEFAULT 0,
  started_at         timestamptz NOT NULL DEFAULT now(),
  finished_at        timestamptz
);

COMMENT ON TABLE eval_runs IS
  'One scoring of the whole golden set under one configuration, for before-and-after comparison.';

CREATE INDEX idx_eval_runs_started ON eval_runs (tenant_id, started_at DESC);

-- One golden posting's result within one evaluation.
CREATE TABLE eval_items (
  id            bigserial PRIMARY KEY,
  eval_run_id   bigint      NOT NULL REFERENCES eval_runs(id) ON DELETE CASCADE,
  golden_id     bigint      NOT NULL REFERENCES golden_postings(id) ON DELETE CASCADE,
  -- The pipeline run this produced, so the evidence behind an eval
  -- result is the same evidence as any other run: verdicts, decisions,
  -- model calls.
  run_id        uuid        REFERENCES jd_runs(run_id) ON DELETE SET NULL,
  submission_id bigint      REFERENCES jd_submissions(id) ON DELETE SET NULL,
  match_score   double precision,
  fit           text        NOT NULL DEFAULT '',
  gate_side     text        NOT NULL DEFAULT '',  -- above | below
  passed        boolean     NOT NULL DEFAULT false,
  error         text        NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (eval_run_id, golden_id)
);

CREATE INDEX idx_eval_items_golden ON eval_items (golden_id, id DESC);

-- Evaluation submissions run through the same pipeline as real ones,
-- because a test that takes a different path tests a different thing.
-- This flag keeps them out of the submission list and stops the owner
-- being emailed five times per evaluation.
ALTER TABLE jd_submissions ADD COLUMN is_eval boolean NOT NULL DEFAULT false;
CREATE INDEX idx_jd_submissions_real ON jd_submissions (created_at DESC) WHERE NOT is_eval;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_jd_submissions_real;
ALTER TABLE jd_submissions DROP COLUMN IF EXISTS is_eval;
DROP TABLE IF EXISTS eval_items;
DROP TABLE IF EXISTS eval_runs;
DROP TABLE IF EXISTS golden_postings;

-- +goose StatementEnd
