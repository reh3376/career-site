-- +goose Up
-- +goose StatementBegin

-- Pilot access and the agent account (FSD §5.16 FR-PLT-01/02,
-- FR-AUTH-19, FR-AUTH-20; docs/framework-research/15-pilot-on-site.md).
--
-- Three small changes in one migration because they ship in one PR and
-- the pilot cannot be fielded without all of them: who is a pilot
-- participant, how a forgotten password is recovered without a link,
-- and how an agent gets in for a live pass without a standing password.

-- A pilot participant is a member whose flag is set. The flag is
-- copied from the whitelist entry when registration auto-approves,
-- and the owner can flip it on the member page. Admins see the pilot
-- area whatever this says; the gate reads the role first.
ALTER TABLE users
  ADD COLUMN pilot_approved boolean NOT NULL DEFAULT false;
CREATE INDEX users_pilot_idx ON users (pilot_approved) WHERE pilot_approved;

-- The whitelist entry is where the owner identifies the ten, after
-- confirming the address with each one (15 §0 decision 1).
ALTER TABLE access_grants
  ADD COLUMN pilot boolean NOT NULL DEFAULT false;

-- Forgot-password by emailed 5-digit code (FR-AUTH-19). email_tokens
-- already carries code_hash from the verify flow; what it lacked was a
-- counter, because a 5-digit code has 100,000 values and must be
-- burned after a few misses. Five, then the token is marked used.
ALTER TABLE email_tokens
  ADD COLUMN attempts integer NOT NULL DEFAULT 0;

-- One-time passwords for the agent account (FR-AUTH-20).
--
-- The agent signs in with a fixed username and a password the owner
-- creates from the admin console. A password is good for one sign-in
-- and for fifteen minutes from creation, whichever ends first; creating
-- a new one revokes any live one, so there is never more than one that
-- works. Only the argon2 hash is stored; the plaintext is shown to the
-- owner once, in the response that created it.
--
-- Every row is kept: used, expired and revoked alike. The table is the
-- record of every time an agent was let in, which is the point of
-- having a separate account for it rather than lending the owner's.
CREATE TABLE agent_passwords (
  id            bigserial   PRIMARY KEY,
  tenant_id     bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  password_hash text        NOT NULL,
  -- Why this agent session: free text from the owner, kept with the row.
  reason        text        NOT NULL DEFAULT '',
  created_by    bigint      REFERENCES users(id) ON DELETE SET NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  expires_at    timestamptz NOT NULL,
  used_at       timestamptz,
  used_ip_hash  bytea,
  -- The session the sign-in minted, so the console can show and revoke it.
  session_id    bigint      REFERENCES sessions(id) ON DELETE SET NULL,
  revoked_at    timestamptz,
  revoked_by    bigint      REFERENCES users(id) ON DELETE SET NULL
);

COMMENT ON TABLE agent_passwords IS
  'One-time passwords for the agent account: 15 minutes, single use, one live at a time (FSD FR-AUTH-20).';

CREATE INDEX agent_passwords_live_idx
  ON agent_passwords (tenant_id, expires_at DESC)
  WHERE used_at IS NULL AND revoked_at IS NULL;
CREATE INDEX agent_passwords_created_idx
  ON agent_passwords (tenant_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS agent_passwords_created_idx;
DROP INDEX IF EXISTS agent_passwords_live_idx;
DROP TABLE IF EXISTS agent_passwords;
ALTER TABLE email_tokens DROP COLUMN IF EXISTS attempts;
ALTER TABLE access_grants DROP COLUMN IF EXISTS pilot;
DROP INDEX IF EXISTS users_pilot_idx;
ALTER TABLE users DROP COLUMN IF EXISTS pilot_approved;

-- +goose StatementEnd
