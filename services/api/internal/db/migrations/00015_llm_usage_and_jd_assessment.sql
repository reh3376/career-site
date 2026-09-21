-- +goose Up
-- +goose StatementBegin

-- Every LLM call, logged from day one (Phase 4 guardrail #4). kind is
-- the calling feature (jd_assess, jd_resume, chat); ref_id points at
-- that feature's row. cost_usd stays NULL for on-host Ollama and is
-- filled by a hosted provider adapter.
CREATE TABLE llm_usage (
  id                bigserial PRIMARY KEY,
  kind              text NOT NULL,
  ref_id            bigint,
  model             text NOT NULL,
  prompt_id         text NOT NULL,
  prompt_version    integer NOT NULL,
  prompt_tokens     integer NOT NULL DEFAULT 0,
  completion_tokens integer NOT NULL DEFAULT 0,
  latency_ms        bigint NOT NULL DEFAULT 0,
  cost_usd          numeric(10, 6),
  ok                boolean NOT NULL,
  error             text,
  created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX llm_usage_created_idx ON llm_usage (created_at DESC);
CREATE INDEX llm_usage_ref_idx ON llm_usage (kind, ref_id);

-- The score is computed in code from per-requirement verdicts, and the
-- whole derivation is kept so a number can always be explained:
--   retrieval_score  mean top-K cosine, the cheap pre-score
--   match_score      requirement-weighted score (existing column)
--   assessment       requirements, evidence ids, judgments, prompts
-- result_token is a random secret returned once at submit time; the
-- public poll must present it to read the résumé, so a guessable
-- numeric id alone reveals nothing beyond status and score.
ALTER TABLE jd_submissions
  ADD COLUMN retrieval_score double precision,
  ADD COLUMN assessment      jsonb,
  ADD COLUMN result_token    bytea,
  ADD COLUMN resume_markdown text,
  ADD COLUMN llm_model       text,
  ADD COLUMN prompt_id       text,
  ADD COLUMN prompt_version  integer;

COMMENT ON COLUMN jd_submissions.assessment IS
  'Auditable derivation of match_score: requirements, evidence chunk ids, verdicts, prompt versions.';
COMMENT ON COLUMN jd_submissions.result_token IS
  '16 random bytes issued at submit; required to read resume_markdown via the public poll.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE jd_submissions
  DROP COLUMN retrieval_score,
  DROP COLUMN assessment,
  DROP COLUMN result_token,
  DROP COLUMN resume_markdown,
  DROP COLUMN llm_model,
  DROP COLUMN prompt_id,
  DROP COLUMN prompt_version;

DROP TABLE llm_usage;

-- +goose StatementEnd
