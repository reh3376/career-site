-- +goose Up
-- +goose StatementBegin

-- The employer-side pilot, run on the site (FSD §5.16 FR-PLT-03..14;
-- docs/framework-research/15-pilot-on-site.md §3).
--
-- Ten recruiters and hiring managers take the survey instrument, read
-- a mocked match record and the company hiring workflow, and comment
-- on both. What is collected is a research dataset about an instrument
-- and two artefacts, not about people: answers are read for item
-- confusion, comments for what the record and the workflow get wrong.
-- No table here stores a name or an employer beyond what users already
-- holds; exports leave the site only into the owner's private folder.

-- The instrument as it was fielded. The definition lives in the API
-- binary as versioned JSON; the first time a version is used it is
-- copied here, so an edit to the file cannot change what a past
-- participant saw. The version string is the one the change log in
-- docs/framework-research/12-instrument-change-log.md tracks.
CREATE TABLE pilot_instruments (
  version     text        PRIMARY KEY,
  tenant_id   bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  definition  jsonb       NOT NULL,
  fielded_at  timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE pilot_instruments IS
  'Survey definitions as fielded, copied from the embedded JSON on first use (FSD FR-PLT-04).';

-- One per participant per instrument version. Resumable: the pilot is
-- a wording test, not a timed measurement, so leaving and returning
-- costs nothing. Status records which of the three parts are done; the
-- participant may take them in any order after consenting.
CREATE TABLE pilot_sessions (
  id                     uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id              bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  user_id                bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  instrument_version     text        NOT NULL REFERENCES pilot_instruments(version),
  -- recruiter | hm | both | '' until A1 is answered; '' also for "none
  -- of these", which ends the survey.
  role_branch            text        NOT NULL DEFAULT '',
  -- consented | finished. The three parts are tracked by their own
  -- timestamps below so the order they were taken in is visible.
  status                 text        NOT NULL DEFAULT 'consented',
  consent_at             timestamptz NOT NULL DEFAULT now(),
  recording_ok           boolean     NOT NULL DEFAULT false,
  survey_finished_at     timestamptz,
  record_finished_at     timestamptz,
  workflow_finished_at   timestamptz,
  finished_at            timestamptz,
  self_reported_minutes  integer,
  most_wanted_to_skip    text        NOT NULL DEFAULT '',
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, instrument_version)
);

COMMENT ON TABLE pilot_sessions IS
  'One pilot sitting per participant per instrument version; resumable (FSD FR-PLT-10).';

CREATE INDEX idx_pilot_sessions_status ON pilot_sessions (tenant_id, status, created_at DESC);

CREATE TRIGGER pilot_sessions_set_updated_at
BEFORE UPDATE ON pilot_sessions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One row per item answered. The flag rides the answer: "this was
-- unclear or didn't fit me" is the cheapest cognitive-interview data
-- there is, collected while the confusion is fresh (08 §2.2).
CREATE TABLE pilot_answers (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  session_id  uuid        NOT NULL REFERENCES pilot_sessions(id) ON DELETE CASCADE,
  section     text        NOT NULL,
  item_id     text        NOT NULL,                 -- A1, C1, D4, ...
  value       jsonb       NOT NULL,                 -- shape per item type (internal/pilot)
  flagged     boolean     NOT NULL DEFAULT false,
  answered_at timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (session_id, item_id)
);

COMMENT ON TABLE pilot_answers IS
  'One row per survey item answered, with the per-item flag (FSD FR-PLT-05).';

CREATE INDEX idx_pilot_answers_session ON pilot_answers (tenant_id, session_id, section);

CREATE TRIGGER pilot_answers_set_updated_at
BEFORE UPDATE ON pilot_answers
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Section open and submit instants, so the self-reported time at the
-- end can be read against a measurement (07 §1's time check).
CREATE TABLE pilot_section_events (
  id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  session_id uuid        NOT NULL REFERENCES pilot_sessions(id) ON DELETE CASCADE,
  section    text        NOT NULL,
  event      text        NOT NULL,                  -- opened | submitted
  at         timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE pilot_section_events IS
  'When each survey section was opened and submitted (FSD FR-PLT-07).';

CREATE INDEX idx_pilot_section_events_session ON pilot_section_events (tenant_id, session_id, at);

-- Comments on the match-record rows and the workflow nodes, one per
-- target per session, upserted. Nothing here ever changes the record.
CREATE TABLE pilot_comments (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  session_id  uuid        NOT NULL REFERENCES pilot_sessions(id) ON DELETE CASCADE,
  -- record_row | record_general | workflow_node | workflow_missing | workflow_general
  target_kind text        NOT NULL,
  -- r1..r10, C-01..C-26, a sequence number for missing steps, or ''.
  target_id   text        NOT NULL DEFAULT '',
  body        text        NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (session_id, target_kind, target_id)
);

COMMENT ON TABLE pilot_comments IS
  'Participant comments on the mocked match record and the company workflow (FSD FR-PLT-08/09).';

CREATE INDEX idx_pilot_comments_session ON pilot_comments (tenant_id, session_id, target_kind);

CREATE TRIGGER pilot_comments_set_updated_at
BEFORE UPDATE ON pilot_comments
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS pilot_comments;
DROP TABLE IF EXISTS pilot_section_events;
DROP TABLE IF EXISTS pilot_answers;
DROP TABLE IF EXISTS pilot_sessions;
DROP TABLE IF EXISTS pilot_instruments;

-- +goose StatementEnd
