-- +goose Up
-- +goose StatementBegin

-- Named SQL statements the admin has chosen to keep around on the
-- /admin/db page. Personal to each admin (owner_id), so two admins
-- don't overwrite each other's slots.
--
-- Names are unique per owner but not globally, so "top members" can
-- exist under multiple accounts. The SQL body is capped by the RPC
-- validator (4000 chars) to match RunDbQuery's own limit.

CREATE TABLE admin_saved_queries (
  id          bigserial PRIMARY KEY,
  owner_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        text   NOT NULL,
  sql_body    text   NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (owner_id, name)
);

-- Index the owner scan since ListSavedQueries always filters by it.
CREATE INDEX idx_admin_saved_queries_owner
  ON admin_saved_queries (owner_id, name);

COMMENT ON TABLE admin_saved_queries IS
  'Per-admin saved SQL queries surfaced on /admin/db.';
COMMENT ON COLUMN admin_saved_queries.sql_body IS
  'The SELECT text. Enforcement (SELECT-only) is applied at execute time by adminquery.Run, not on save.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS admin_saved_queries;

-- +goose StatementEnd
