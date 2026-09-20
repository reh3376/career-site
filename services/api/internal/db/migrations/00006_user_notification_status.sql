-- +goose Up
-- +goose StatementBegin

-- Delivery status of the last notification email sent to a user.
-- Today the code sends but only logs on failure — an admin looking
-- at a member has no way to know whether the approval / decline /
-- expiry-warning email actually reached them, or bounced silently
-- at the provider. Persist it so /admin/registrations/[id] can show
-- a "last email" pill instead of hoping the log grep works.

ALTER TABLE users
  ADD COLUMN last_notification_kind  text,
  ADD COLUMN last_notification_at    timestamptz,
  ADD COLUMN last_notification_error text;

COMMENT ON COLUMN users.last_notification_kind IS
  'e.g. "user_approved", "user_declined", "expiry_warn", "expired"; NULL until first notification.';
COMMENT ON COLUMN users.last_notification_at IS
  'Wall-clock time we handed the message to the mail provider (success OR failure attempt).';
COMMENT ON COLUMN users.last_notification_error IS
  'NULL on success. On failure, the truncated error string from the provider.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users
  DROP COLUMN last_notification_kind,
  DROP COLUMN last_notification_at,
  DROP COLUMN last_notification_error;

-- +goose StatementEnd
