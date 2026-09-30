-- +goose Up
-- +goose StatementBegin

-- Make a graded decision into a usable training example.
--
-- decision_log was built for the JD reviewer, where the task is
-- classification: the owner picks "met", "partial" or "unmet", and that
-- label *is* the gold output. prompt_text plus human_verdict is a
-- complete training pair and nothing else is needed.
--
-- The assistant's answers break that. The task is generation, so the
-- gold output is prose, and there is nowhere to put it. A reviewer who
-- thinks an answer is wrong can currently record only that it is wrong
-- (human_verdict) and why (human_note), and neither of those is what a
-- model would be trained on. The row can be counted and cannot be
-- learned from, which defeats the reason the table exists.
--
-- Three columns close that.

-- The answer the owner would have given. This is the training target.
--
-- Deliberately separate from human_note rather than overloaded onto it,
-- because an exporter cannot tell a rewrite from a remark, and a note
-- like "too long, and he never worked at Amazon" would be exported as
-- if it were a model answer. A note explains the grade to a human; this
-- is text a model is meant to imitate.
--
-- With response_text it is also a preference pair: human_answer is the
-- chosen completion, response_text the rejected one. That is the more
-- valuable of the two, because it carries what was wrong with a
-- specific answer rather than only what a right one looks like.
ALTER TABLE decision_log ADD COLUMN IF NOT EXISTS human_answer text NOT NULL DEFAULT '';

COMMENT ON COLUMN decision_log.human_answer IS
  'The owner''s corrected wording, as a model should have produced it. The SFT target, and the chosen half of a preference pair against response_text. Not a note about the answer: a replacement for it.';

-- Per-dimension grades, as {"grounded": "yes", "voice": "no", ...}.
--
-- One verdict is enough for a requirement judgment and not enough for
-- an answer, which fails in independent ways: it can be factually
-- grounded and in the wrong voice, correct and citing the wrong
-- passage, accurate and three times too long for a reader on this
-- hardware. Collapsing those to one thumb loses exactly the signal that
-- says what to fix.
--
-- Two of these are already acceptance criteria with numbers attached:
-- grounding at 90 % and citation validity at 95 % (FR-CHAT-03,
-- FR-CHAT-04, measured by the golden set in FR-CHAT-15). Those rates
-- cannot be computed from a single overall verdict.
--
-- jsonb rather than columns because the rubric is not settled. It will
-- change as the golden set does, and a rubric change should not be a
-- migration. The vocabulary lives in Go (users.ReviewDimensions).
ALTER TABLE decision_log ADD COLUMN IF NOT EXISTS human_dimensions jsonb NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN decision_log.human_dimensions IS
  'Per-rubric grades for one decision, e.g. {"grounded":"yes","citations":"partial","voice":"no","scope":"yes","length":"yes"}. Vocabulary in users.ReviewDimensions; jsonb because the rubric moves and a rubric change should not be a migration.';

-- How long the reader waited before the first word.
--
-- latency_ms is the whole call, which is the number that matters for a
-- batch job and not the one that matters here. On the production box a
-- chat answer spends most of its time before the first token (measured:
-- 11.4 s of a 22.4 s answer at two passages, 37.1 s of 52.0 s at six),
-- and those two numbers move for different reasons: time to first token
-- tracks how much context was retrieved, total time tracks how much was
-- written. Recording only the sum makes a retrieval regression and a
-- verbosity regression look identical.
--
-- Zero where it does not apply, which is every JD row and every answer
-- served from the Q&A bank without the model.
ALTER TABLE decision_log ADD COLUMN IF NOT EXISTS first_token_ms bigint NOT NULL DEFAULT 0;

COMMENT ON COLUMN decision_log.first_token_ms IS
  'Milliseconds to the first token. Separate from latency_ms because on a CPU-only box first-token time tracks retrieved context and total time tracks answer length; one number cannot distinguish a retrieval regression from a verbose one. Zero where it does not apply.';

-- The review queue for a generative kind is not "everything ungraded"
-- but "everything ungraded, newest first, of this kind", and for chat
-- it is a different queue from the JD one. The existing unreviewed
-- index is already (kind, id DESC) WHERE reviewed_at IS NULL, which
-- serves that. What it does not serve is finding the rows a member
-- reacted to, which is where a reviewer's time is worth most.
CREATE INDEX IF NOT EXISTS idx_decision_log_graded
    ON decision_log (kind, reviewed_at DESC)
 WHERE reviewed_at IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_decision_log_graded;
ALTER TABLE decision_log DROP COLUMN IF EXISTS first_token_ms;
ALTER TABLE decision_log DROP COLUMN IF EXISTS human_dimensions;
ALTER TABLE decision_log DROP COLUMN IF EXISTS human_answer;
-- +goose StatementEnd
