-- +goose Up
-- +goose StatementBegin

-- Remember that the three-day expiry warning was sent.
--
-- It was tracked in a map in the api process, with a comment saying the
-- worst case was "a duplicate warning email after a process restart,
-- which is graceful". That was true when restarts were rare. Between
-- 2026-09-23 and 2026-09-25 the container was recreated dozens of times
-- for deploys and diagnosis, and every restart re-sent: 30 emails to
-- one member and 26 to another, all identical, all saying access ends
-- in about three days.
--
-- A marker that lives in a process is not a record of whether a person
-- was emailed. It is a record of whether this process emailed them.
--
-- Nullable rather than defaulted, because NULL means "not warned in the
-- current access period" and every existing row is genuinely in that
-- state for the period that follows their next extension.
ALTER TABLE users ADD COLUMN expiry_warning_sent_at timestamptz;

COMMENT ON COLUMN users.expiry_warning_sent_at IS
  'When the three-day access-ending warning was sent for the current expires_at. Cleared whenever expires_at changes, so a renewed account is warned again in its next window.';

-- Backfill from what was actually sent, so the fix does not start by
-- sending everyone one more. The notification log knows who received
-- the warning and when.
UPDATE users u
   SET expiry_warning_sent_at = n.last_sent
  FROM (
    SELECT user_id, max(created_at) AS last_sent
      FROM notification_deliveries
     WHERE kind = 'expiry_warn'
     GROUP BY user_id
  ) n
 WHERE n.user_id = u.id
   AND u.expires_at IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS expiry_warning_sent_at;
-- +goose StatementEnd
