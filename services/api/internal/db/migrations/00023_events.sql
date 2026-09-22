-- +goose Up
-- +goose StatementBegin

-- One append-only event stream for product behaviour, anonymous and
-- member alike (docs/events/README.md is the registry). Replaces the
-- member-only activity_events for new writes; the old rows are copied
-- in below so the funnel is one query from day one.
CREATE TABLE events (
  id             bigserial PRIMARY KEY,
  event_id       uuid        NOT NULL UNIQUE,   -- client-minted for browser events, server-minted otherwise; idempotent inserts
  name           text        NOT NULL,          -- dotted, from the registry (page.view, jd.submitted, ...)
  occurred_at    timestamptz NOT NULL,          -- server clock at receipt
  client_ts      timestamptz,                   -- as the client reported it; skew analysis only
  anon_id        uuid,                          -- first-party cookie, 13 months, set by the web proxy on first visit
  user_id        bigint REFERENCES users(id) ON DELETE SET NULL,
  session_id     bigint,
  ui_mode        text,                          -- it | ot
  path           text,
  referrer_host  text,
  utm            jsonb,
  device         text,                          -- phone | tablet | desktop, classified from the user agent
  ip_hash        text,                          -- salted SHA-256 of the client address (EVENT_IP_SALT)
  app_commit     text        NOT NULL,
  props          jsonb       NOT NULL DEFAULT '{}'::jsonb,
  schema_version smallint    NOT NULL DEFAULT 1
);

CREATE INDEX idx_events_name_time ON events (name, occurred_at DESC);
CREATE INDEX idx_events_anon_time ON events (anon_id, occurred_at) WHERE anon_id IS NOT NULL;
CREATE INDEX idx_events_user_time ON events (user_id, occurred_at) WHERE user_id IS NOT NULL;

-- Backfill: the member activity stream becomes the first events.
INSERT INTO events (event_id, name, occurred_at, user_id, session_id, path, props, app_commit)
SELECT gen_random_uuid(),
       CASE kind WHEN 'view' THEN 'page.view' WHEN 'login' THEN 'login.success' ELSE 'activity.' || kind END,
       occurred_at, user_id, session_id, content_id,
       COALESCE(payload, '{}'::jsonb) || jsonb_strip_nulls(jsonb_build_object(
         'dwell_ms', dwell_ms, 'conversation_id', conversation_id, 'query', query, 'variant', variant,
         'backfilled_from', 'activity_events', 'activity_event_id', id)),
       'backfill'
FROM activity_events;

COMMENT ON TABLE events IS 'Append-only product event stream (docs/events/README.md).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS events;

-- +goose StatementEnd
