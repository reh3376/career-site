-- +goose Up
-- +goose StatementBegin

-- Whitelist + time-limited access (D-20, ADR-0020). Adds an `access_grants`
-- table for pre-authorized emails, an `expires_at` column on users, an
-- `expired` value in the user_status enum, `whitelist_auto` in the
-- decision-channel enum, and a `granted_ttl` interval on approval_decisions.

-- 1. New enum values (must run outside a transaction on older Postgres, but
--    Postgres 14+ allows this inside a txn — goose runs migrations in a txn
--    by default which is fine here).

ALTER TYPE user_status ADD VALUE IF NOT EXISTS 'expired';
ALTER TYPE approval_channel ADD VALUE IF NOT EXISTS 'whitelist_auto';

-- 2. users.expires_at (nullable = permanent).

ALTER TABLE users
  ADD COLUMN expires_at timestamptz;

CREATE INDEX users_expires_at_idx
  ON users (expires_at)
  WHERE expires_at IS NOT NULL AND status = 'active';

-- 3. approval_decisions.granted_ttl — how long the approval granted, or NULL
--    for `decline`, `auto_decline`, or a permanent approval.

ALTER TABLE approval_decisions
  ADD COLUMN granted_ttl interval;

-- 4. access_grants — the whitelist.

CREATE TYPE access_grant_ttl AS ENUM ('1d', '3d', '7d', '30d', 'permanent');

CREATE TABLE access_grants (
  id                bigserial PRIMARY KEY,
  email             citext NOT NULL UNIQUE,
  default_ttl       access_grant_ttl NOT NULL DEFAULT '7d',
  notes             text NOT NULL DEFAULT '',
  entry_expires_at  timestamptz,
  created_by        bigint REFERENCES users(id) ON DELETE SET NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX access_grants_entry_expires_idx
  ON access_grants (entry_expires_at)
  WHERE entry_expires_at IS NOT NULL;

CREATE TRIGGER access_grants_set_updated_at
BEFORE UPDATE ON access_grants
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS access_grants;
DROP TYPE  IF EXISTS access_grant_ttl;

ALTER TABLE approval_decisions
  DROP COLUMN IF EXISTS granted_ttl;

DROP INDEX IF EXISTS users_expires_at_idx;
ALTER TABLE users
  DROP COLUMN IF EXISTS expires_at;

-- Note: Postgres does not support dropping an enum value. `expired` and
-- `whitelist_auto` remain in the type after a down migration. Recreating
-- the types requires rewriting every dependent column and is intentionally
-- out of scope for a routine rollback.

-- +goose StatementEnd
