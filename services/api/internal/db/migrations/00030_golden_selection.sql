-- +goose Up
-- +goose StatementBegin

-- How a golden posting got into the set, and whether it has been
-- labelled yet.
--
-- The set held two postings, both chosen by the owner, one of them a
-- job he had applied for. Its expected outcome was therefore his
-- application decision restated rather than an independent judgment,
-- so the set could confirm what he already believed and could not
-- surprise him. An evaluation that cannot produce an unwelcome result
-- is not measuring anything.
--
-- Postings nobody picked fix that, but only if the two groups are
-- reported apart: a gate that is clean on chosen postings and noisy on
-- random ones is the most useful finding the set can produce, and
-- averaging them together hides exactly that.

ALTER TABLE golden_postings
  -- chosen: the owner put it in because he knew the answer.
  -- random: drawn from a search without reading it first.
  ADD COLUMN selection text NOT NULL DEFAULT 'chosen',
  -- Where it came from, so a posting can be traced back and a dead
  -- link does not make the row a mystery.
  ADD COLUMN source text NOT NULL DEFAULT '';

COMMENT ON COLUMN golden_postings.selection IS
  'chosen (the owner picked it) or random (drawn without reading it first). Reported apart.';

-- A random posting arrives unlabelled, because the whole point is that
-- nobody decided the answer in advance. The empty string is that state.
-- Evaluations skip unlabelled postings rather than guessing, and the
-- console asks for a label.
ALTER TABLE golden_postings
  ALTER COLUMN expected_gate DROP NOT NULL;
UPDATE golden_postings SET expected_gate = '' WHERE expected_gate IS NULL;
ALTER TABLE golden_postings
  ALTER COLUMN expected_gate SET DEFAULT '',
  ALTER COLUMN expected_gate SET NOT NULL;

CREATE INDEX idx_golden_unlabelled ON golden_postings (tenant_id)
  WHERE active AND expected_gate = '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_golden_unlabelled;
ALTER TABLE golden_postings DROP COLUMN IF EXISTS source;
ALTER TABLE golden_postings DROP COLUMN IF EXISTS selection;

-- +goose StatementEnd
