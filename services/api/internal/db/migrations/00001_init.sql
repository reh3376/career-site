-- +goose Up
-- +goose StatementBegin

-- Phase 0 baseline: the tables the identity + approval-gated registration
-- flow needs (FSD 0.3.3 FR-AUTH-01..16, FR-ADM-10, §8.4). Domain tables
-- for content, activity, corpus, chat, etc. land in their own migrations
-- in Phases 2..5.

CREATE EXTENSION IF NOT EXISTS citext;

-- Free-form updated_at maintainer used by every mutable table.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- users --------------------------------------------------------------------

CREATE TYPE user_status AS ENUM (
  'unverified',       -- registered, email not yet confirmed
  'pending_approval', -- email confirmed, awaiting admin decision (D-02)
  'active',           -- approved by admin, can sign in
  'declined',         -- admin declined, cannot sign in
  'disabled',         -- admin-suspended active account
  'deleted'           -- self-deleted; personal data purged, row retained for audit
);

CREATE TYPE user_role AS ENUM ('member', 'admin');

CREATE TABLE users (
  id                bigserial PRIMARY KEY,
  email             citext NOT NULL UNIQUE,
  password_hash     text,
  name              text NOT NULL DEFAULT '',
  status            user_status NOT NULL DEFAULT 'unverified',
  role              user_role NOT NULL DEFAULT 'member',
  consent_version   text NOT NULL DEFAULT '',
  consent_at        timestamptz,
  mfa_secret        text,
  last_seen_at      timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX users_status_idx ON users (status);
CREATE INDEX users_role_idx   ON users (role);

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- oauth_accounts -----------------------------------------------------------

CREATE TABLE oauth_accounts (
  id                bigserial PRIMARY KEY,
  user_id           bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider          text NOT NULL,
  provider_subject  text NOT NULL,
  email             citext NOT NULL,
  profile_json      jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_subject)
);

CREATE INDEX oauth_accounts_user_id_idx ON oauth_accounts (user_id);

CREATE TRIGGER oauth_accounts_set_updated_at
BEFORE UPDATE ON oauth_accounts
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- sessions -----------------------------------------------------------------

CREATE TABLE sessions (
  id              bigserial PRIMARY KEY,
  user_id         bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash      bytea NOT NULL UNIQUE,
  expires_at      timestamptz NOT NULL,
  last_active_at  timestamptz NOT NULL DEFAULT now(),
  ip_hash         bytea,
  user_agent      text NOT NULL DEFAULT '',
  revoked_at      timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx  ON sessions (user_id);
CREATE INDEX sessions_expires_idx  ON sessions (expires_at) WHERE revoked_at IS NULL;

-- email_tokens -------------------------------------------------------------

CREATE TYPE email_token_purpose AS ENUM (
  'verify',
  'reset',
  'change_email'
);

CREATE TABLE email_tokens (
  id          bigserial PRIMARY KEY,
  user_id     bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose     email_token_purpose NOT NULL,
  token_hash  bytea NOT NULL UNIQUE,
  code_hash   bytea,
  expires_at  timestamptz NOT NULL,
  used_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX email_tokens_user_id_idx ON email_tokens (user_id);
CREATE INDEX email_tokens_expires_idx ON email_tokens (expires_at) WHERE used_at IS NULL;

-- approval_decisions -------------------------------------------------------

CREATE TYPE approval_decision AS ENUM ('approve', 'decline', 'auto_decline');
CREATE TYPE approval_channel  AS ENUM ('email_link', 'console', 'scheduler');

CREATE TABLE approval_decisions (
  id             bigserial PRIMARY KEY,
  user_id        bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  decision       approval_decision NOT NULL,
  decided_by     bigint REFERENCES users(id) ON DELETE SET NULL,
  decided_via    approval_channel NOT NULL,
  decided_at     timestamptz NOT NULL DEFAULT now(),
  token_hash     bytea,
  ip_hash        bytea,
  user_agent     text NOT NULL DEFAULT '',
  superseded_by  bigint REFERENCES approval_decisions(id) ON DELETE SET NULL
);

CREATE INDEX approval_decisions_user_id_idx    ON approval_decisions (user_id);
CREATE INDEX approval_decisions_decided_at_idx ON approval_decisions (decided_at DESC);

-- audit_log ----------------------------------------------------------------

CREATE TABLE audit_log (
  id             bigserial PRIMARY KEY,
  actor_user_id  bigint REFERENCES users(id) ON DELETE SET NULL,
  action         text NOT NULL,
  target_type    text NOT NULL DEFAULT '',
  target_id      text NOT NULL DEFAULT '',
  payload_json   jsonb NOT NULL DEFAULT '{}'::jsonb,
  ip_hash        bytea,
  occurred_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_actor_idx      ON audit_log (actor_user_id);
CREATE INDEX audit_log_action_idx     ON audit_log (action);
CREATE INDEX audit_log_occurred_idx   ON audit_log (occurred_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS approval_decisions;
DROP TYPE  IF EXISTS approval_channel;
DROP TYPE  IF EXISTS approval_decision;
DROP TABLE IF EXISTS email_tokens;
DROP TYPE  IF EXISTS email_token_purpose;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS oauth_accounts;
DROP TABLE IF EXISTS users;
DROP TYPE  IF EXISTS user_role;
DROP TYPE  IF EXISTS user_status;
DROP FUNCTION IF EXISTS set_updated_at();

-- +goose StatementEnd
