-- +goose Up
-- +goose StatementBegin

-- Per-member activity stream. Client-side page views / downloads /
-- searches / saves land here via ActivityService.RecordEvents; the
-- server writes login / logout / chat / escalation itself.
--
-- kind is a text column, not an enum, so a new proto Kind value
-- doesn't need a schema migration — we validate against the enum
-- at the handler edge.
--
-- (user_id, client_event_id) is UNIQUE so a client's retried batch
-- de-dupes at the DB layer instead of the handler; NULLs never
-- collide, so events without a client id (server-side auth events)
-- always insert cleanly.

CREATE TABLE activity_events (
  id                bigserial PRIMARY KEY,
  user_id           bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind              text   NOT NULL,
  content_id        text,
  conversation_id   text,
  session_id        bigint,   -- nullable so server-events without a session still fit
  query             text,
  variant           text,
  dwell_ms          integer,
  client_event_id   text,     -- from the client for de-dup; nullable
  payload           jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at       timestamptz NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now()
);

-- Reads are always scoped by user_id + newest first.
CREATE INDEX idx_activity_events_user_time
  ON activity_events (user_id, occurred_at DESC);

-- Aggregate reads (counts by kind) benefit from a (user_id, kind) index.
CREATE INDEX idx_activity_events_user_kind
  ON activity_events (user_id, kind);

-- Client retry de-dup. Partial index so NULL client_event_ids don't
-- consume space or collide.
CREATE UNIQUE INDEX ux_activity_events_user_client
  ON activity_events (user_id, client_event_id)
  WHERE client_event_id IS NOT NULL;

COMMENT ON TABLE  activity_events IS 'Per-member activity stream (page views, downloads, chat, auth events).';
COMMENT ON COLUMN activity_events.kind IS 'Slug matching the proto Kind enum (login, logout, view, download, chat, escalate, ...).';
COMMENT ON COLUMN activity_events.session_id IS 'sessions.id when the event happened in a browser session; NULL otherwise.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS activity_events;

-- +goose StatementEnd
