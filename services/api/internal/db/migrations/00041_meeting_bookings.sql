-- +goose Up
-- +goose StatementBegin

-- Meetings members have booked on the owner's calendar.
--
-- The exclusion constraint is the load-bearing part. Two members can
-- submit the same slot in the same second, and Google cannot settle
-- that race: no event exists until a claim wins, so there is nothing
-- for a free/busy check to see. The database is the only place that
-- can decide, and it decides by refusing the second insert rather than
-- by the application checking first and hoping.
--
-- The range is the meeting plus its clearance, so the constraint also
-- enforces the fifteen minutes between meetings rather than trusting
-- every caller to remember. A 15-minute meeting occupies 30 minutes of
-- the day here, a 30 occupies 45, a 45 occupies 60, which is what the
-- owner asked for stated as a database rule.
--
-- Cancelled bookings keep their row and stop holding time, so a
-- cancellation frees the slot without losing the fact that it happened.
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE meeting_bookings (
    id           bigserial PRIMARY KEY,
    tenant_id    bigint      NOT NULL DEFAULT 1 REFERENCES tenants (id),
    user_id      bigint      REFERENCES users (id) ON DELETE SET NULL,
    name         text        NOT NULL,
    email        text        NOT NULL,
    note         text        NOT NULL DEFAULT '',
    starts_at    timestamptz NOT NULL,
    duration_min integer     NOT NULL CHECK (duration_min > 0),
    gap_min      integer     NOT NULL DEFAULT 15 CHECK (gap_min >= 0),
    event_id     text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    cancelled_at timestamptz,
    -- The time this booking actually holds. Derived by a trigger rather
    -- than supplied, so no caller can get it wrong and no two callers
    -- can disagree about it. A generated column would be the obvious
    -- home for it, but timestamptz + interval is only STABLE, not
    -- IMMUTABLE, because adding an interval depends on the session
    -- timezone, and Postgres refuses it.
    held         tstzrange NOT NULL
);

-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION meeting_bookings_set_held() RETURNS trigger AS $$
BEGIN
    NEW.held := tstzrange(
        NEW.starts_at,
        NEW.starts_at + make_interval(mins => NEW.duration_min + NEW.gap_min),
        '[)');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin

CREATE TRIGGER meeting_bookings_held
    BEFORE INSERT OR UPDATE OF starts_at, duration_min, gap_min
    ON meeting_bookings
    FOR EACH ROW EXECUTE FUNCTION meeting_bookings_set_held();

ALTER TABLE meeting_bookings
  ADD CONSTRAINT meeting_bookings_no_overlap
  EXCLUDE USING gist (tenant_id WITH =, held WITH &&)
  WHERE (cancelled_at IS NULL);

CREATE INDEX idx_meeting_bookings_starts ON meeting_bookings (starts_at)
    WHERE cancelled_at IS NULL;
CREATE INDEX idx_meeting_bookings_user ON meeting_bookings (user_id, starts_at DESC);

COMMENT ON COLUMN meeting_bookings.held IS
  'The meeting plus its clearance. The exclusion constraint on this is what makes two simultaneous claims on one slot impossible, which no amount of checking before inserting can achieve.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS meeting_bookings;
-- +goose StatementEnd
