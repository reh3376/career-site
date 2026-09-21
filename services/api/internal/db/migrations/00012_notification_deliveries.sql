-- +goose Up
-- +goose StatementBegin

-- One row per outbound email attempt. users.last_notification_* only
-- remembers the most recent send, so an admin could never tell whether
-- the *approval* email reached someone once a later expiry warning
-- overwrote it. This table keeps every attempt so /admin/registrations/[id]
-- can show a history and offer a resend.
--
-- user_id is NULL for owner-bound mail (contact form, approval request
-- to Roger) — those are still audited, they just don't belong to a member.

CREATE TABLE notification_deliveries (
  id           bigserial PRIMARY KEY,
  user_id      bigint REFERENCES users(id) ON DELETE SET NULL,
  kind         text NOT NULL,
  recipient    text NOT NULL,
  provider     text NOT NULL,
  triggered_by text NOT NULL DEFAULT 'system',
  duration_ms  integer NOT NULL DEFAULT 0,
  error        text,
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notification_deliveries_user_idx
  ON notification_deliveries (user_id, created_at DESC);

COMMENT ON TABLE notification_deliveries IS
  'Audit of every transactional email attempt. error IS NULL means the provider accepted it.';
COMMENT ON COLUMN notification_deliveries.kind IS
  'Slug: verify_email, welcome_whitelist, user_approved, user_declined, user_auto_declined, expiry_warn, expired, password_reset, approval_request, contact_owner.';
COMMENT ON COLUMN notification_deliveries.triggered_by IS
  '"system" for automatic sends; "admin:<user_id>" for console-initiated resends.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE notification_deliveries;

-- +goose StatementEnd
