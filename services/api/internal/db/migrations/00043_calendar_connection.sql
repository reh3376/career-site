-- +goose Up
-- +goose StatementBegin

-- The owner's calendar connection: one row, holding the credential the
-- booking flow needs and nothing about what is on the calendar.
--
-- Single row by construction. This application books against exactly
-- one calendar, the owner's own, and a table that can hold two invites
-- the question of which one is live at a moment when the answer has to
-- be obvious.
--
-- refresh_token_sealed is AES-256-GCM (internal/secrets), not a hash:
-- unlike a password this value has to come back out in plaintext to be
-- sent to Google. It is sealed with the context "google_refresh_token",
-- which is authenticated, so a value moved into this column from
-- somewhere else fails to open rather than being quietly accepted.
--
-- The access token is deliberately NOT stored. It lives about an hour,
-- is cheap to mint from the refresh token, and a copy on disk is a
-- credential to protect for no benefit.
CREATE TABLE IF NOT EXISTS calendar_connection (
  -- Always 1. The check is what makes "one row" a property of the
  -- schema rather than a convention someone has to remember.
  id                    integer     PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  provider              text        NOT NULL DEFAULT 'google',
  -- The calendar written to. "primary" is Google's alias for the
  -- account's own calendar, which is what the owner connects.
  calendar_id           text        NOT NULL DEFAULT 'primary',
  -- The account that authorised, shown in the admin surface so the
  -- owner can see which Google account is connected without opening
  -- Google.
  account_email         text        NOT NULL DEFAULT '',
  -- Sealed refresh token. Empty means connected-then-revoked, which is
  -- different from no row at all (never connected).
  refresh_token_sealed  text        NOT NULL DEFAULT '',
  -- Scopes actually granted, so a consent screen that returned less
  -- than was asked for is visible rather than surfacing later as a
  -- puzzling 403.
  scopes                text        NOT NULL DEFAULT '',
  connected_at          timestamptz NOT NULL DEFAULT now(),
  connected_by          bigint      REFERENCES users(id) ON DELETE SET NULL,
  -- When Google last answered successfully, and what it said if it did
  -- not. Lets the admin surface distinguish "never worked" from
  -- "worked until Tuesday", which need different responses.
  last_ok_at            timestamptz,
  last_error            text        NOT NULL DEFAULT '',
  updated_at            timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE calendar_connection IS
  'The single calendar this application books against. Holds a sealed refresh token and health, never anything about the calendar contents: free/busy is read live and never stored (FR-CNT-26).';

COMMENT ON COLUMN calendar_connection.refresh_token_sealed IS
  'AES-256-GCM sealed (internal/secrets), context "google_refresh_token". Encrypted rather than hashed because it must be replayed to Google. Empty means the connection was revoked.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS calendar_connection;
-- +goose StatementEnd
