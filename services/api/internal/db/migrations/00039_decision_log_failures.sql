-- +goose Up
-- +goose StatementBegin

-- Keep the model's output when the call fails, not only when it works.
--
-- decision_log holds prompt_text, response_text, input, output, tokens
-- and latency for every judgment, which is exactly the evidence needed
-- to understand a judgment. It was written only after a successful
-- decode, so the one response nobody could examine was the one that
-- broke.
--
-- Run 13 on 2026-09-29 lost the Blue Origin posting four hours in when
-- a judgment filled its 1200-token budget and returned incomplete JSON.
-- The response that did it is gone. All that survives is a row in
-- llm_usage saying 1200 tokens and 448 seconds, from which the cause
-- had to be inferred rather than read. The behaviour at and near a
-- failure is the behaviour least understood and most worth keeping.
--
-- `error` is empty for the successful rows, which is every row written
-- until now, so no backfill is needed and no existing query changes.
-- `ok` is generated rather than stored so a reader cannot forget the
-- convention, and the partial index makes "show me the failures" cheap
-- on a table that is almost entirely successes.
ALTER TABLE decision_log ADD COLUMN IF NOT EXISTS error text NOT NULL DEFAULT '';

ALTER TABLE decision_log
  ADD COLUMN IF NOT EXISTS ok boolean
  GENERATED ALWAYS AS (error = '') STORED;

CREATE INDEX IF NOT EXISTS idx_decision_log_failures
    ON decision_log (created_at DESC)
 WHERE error <> '';

COMMENT ON COLUMN decision_log.error IS
  'Why this call produced no usable verdict. Empty for a successful decode. A row with this set still carries prompt_text and response_text, which is the point: the failing response is the one worth reading.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_decision_log_failures;
ALTER TABLE decision_log DROP COLUMN IF EXISTS ok;
ALTER TABLE decision_log DROP COLUMN IF EXISTS error;
-- +goose StatementEnd
