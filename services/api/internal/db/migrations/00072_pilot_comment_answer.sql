-- +goose Up
-- +goose StatementBegin

-- A structured answer on every workflow step (FSD FR-PLT-09 amended).
-- Owner's decision after his third run, 2026-11-09: the open comment
-- box under each of the 26 steps asked nothing concrete. Each step now
-- asks "where you work, this step: happens like this, happens
-- differently, does not happen", stored here so the answer can be
-- counted across participants; the comment box opens only when the
-- answer is "differs" and its text stays in body. The same M / D / X
-- tally the interview guide (11 Part C) kept by hand.
ALTER TABLE pilot_comments
  ADD COLUMN answer text NOT NULL DEFAULT ''
    CONSTRAINT pilot_comments_answer_check CHECK (answer IN ('', 'matches', 'differs', 'absent'));

COMMENT ON COLUMN pilot_comments.answer IS
  'On a workflow step: matches, differs or absent; empty on every other target (FSD FR-PLT-09).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE pilot_comments DROP COLUMN IF EXISTS answer;

-- +goose StatementEnd
