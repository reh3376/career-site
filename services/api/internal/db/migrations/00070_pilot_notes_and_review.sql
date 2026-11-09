-- +goose Up
-- +goose StatementBegin

-- The owner's side of the pilot, in the database (FSD FR-PLT-16,
-- FR-PLT-17; docs/framework-research/15-pilot-on-site.md §12).
--
-- Owner's instruction, 2026-11-09, during the live review: what a pilot
-- participant generates is to be collected, governed, stored and
-- curated in the database, not in files. Until now the owner's notes
-- on a sitting were to be a markdown file per participant in the
-- private folder (11 §2) and the exports were the record. From this
-- migration the notes live beside the data they are about, and every
-- session carries the owner's curation judgement, as the decision
-- test's review table (00056) does for its runs.

-- One note per target per session, upserted. target_kind mirrors the
-- participant's comment kinds plus 'session' (the sitting as a whole)
-- and 'item' (one survey answer), so the call's Parts A, B and C land
-- on the answers, rows and steps they discuss.
CREATE TABLE pilot_notes (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  session_id  uuid        NOT NULL REFERENCES pilot_sessions(id) ON DELETE CASCADE,
  -- session | item | record_row | record_general | workflow_node | workflow_missing | workflow_general
  target_kind text        NOT NULL,
  -- A1..F3, r1..r10, C-01..C-26, a missing-step sequence number, or ''.
  target_id   text        NOT NULL DEFAULT '',
  body        text        NOT NULL,
  author_id   bigint      REFERENCES users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (session_id, target_kind, target_id)
);

COMMENT ON TABLE pilot_notes IS
  'The owner''s notes on a pilot sitting and on its answers, rows and steps; the call''s record, kept with the data (FSD FR-PLT-16).';

CREATE INDEX idx_pilot_notes_session ON pilot_notes (tenant_id, session_id, target_kind);

CREATE TRIGGER pilot_notes_set_updated_at
BEFORE UPDATE ON pilot_notes
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- The curation judgement on a sitting. 'reviewed' is the owner
-- vouching for it; 'excluded' is a sitting that stays in the database
-- and out of every figure, with the reason; 'unreviewed' is the
-- default and the honest state of a sitting nobody has read.
ALTER TABLE pilot_sessions
  ADD COLUMN review_status text        NOT NULL DEFAULT 'unreviewed',
  ADD COLUMN review_note   text        NOT NULL DEFAULT '',
  ADD COLUMN reviewed_at   timestamptz,
  ADD COLUMN reviewed_by   bigint      REFERENCES users(id) ON DELETE SET NULL;

COMMENT ON COLUMN pilot_sessions.review_status IS
  'The owner''s curation judgement: unreviewed, reviewed or excluded (FSD FR-PLT-17).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE pilot_sessions
  DROP COLUMN IF EXISTS reviewed_by,
  DROP COLUMN IF EXISTS reviewed_at,
  DROP COLUMN IF EXISTS review_note,
  DROP COLUMN IF EXISTS review_status;
DROP TABLE IF EXISTS pilot_notes;

-- +goose StatementEnd
