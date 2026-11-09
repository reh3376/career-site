-- +goose Up
-- +goose StatementBegin

-- A note on every survey answer (FSD FR-PLT-05 amended). Owner's
-- instruction after his third run, 2026-11-09: each question needs a
-- notes box for whatever the participant wants to say while answering,
-- read by the owner, stored on the answer's own row. The per-item flag
-- was the cheap cognitive-interview signal; this is the rich one.
ALTER TABLE pilot_answers
  ADD COLUMN note text NOT NULL DEFAULT '';

COMMENT ON COLUMN pilot_answers.note IS
  'What the participant wrote beside the answer, for the owner to read (FSD FR-PLT-05).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE pilot_answers DROP COLUMN IF EXISTS note;

-- +goose StatementEnd
