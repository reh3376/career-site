-- +goose Up
-- +goose StatementBegin

-- Contact / support form storage (FR-CNT-22). Every submission — from a
-- signed-in member or an anonymous visitor — lands here so the admin
-- console has a persistent list to work through (FR-ADM-14). The email
-- delivery is separate; if the API can't reach Resend, the row still
-- exists.

CREATE TYPE support_category AS ENUM (
  'general_question',
  'bug_report',
  'feature_request',
  'contributor_access',
  'press_inquiry',
  'other'
);

CREATE TYPE support_status AS ENUM (
  'open',
  'resolved'
);

CREATE TABLE support_messages (
  id               bigserial PRIMARY KEY,
  category         support_category NOT NULL,
  subject          text NOT NULL,
  body             text NOT NULL,
  -- For member submissions, user_id is set and name/email are copied
  -- from the user row at insert time (denormalised so the message
  -- survives account deletion). For anonymous submissions, user_id is
  -- NULL and name/email come from the form.
  user_id          bigint REFERENCES users(id) ON DELETE SET NULL,
  sender_name      text NOT NULL,
  sender_email     citext NOT NULL,
  reply_channel    text NOT NULL DEFAULT 'email',
  status           support_status NOT NULL DEFAULT 'open',
  ip_hash          bytea,
  user_agent       text NOT NULL DEFAULT '',
  ticket_id        text NOT NULL UNIQUE,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  resolved_at      timestamptz
);

CREATE INDEX support_messages_status_idx     ON support_messages (status);
CREATE INDEX support_messages_category_idx   ON support_messages (category);
CREATE INDEX support_messages_created_at_idx ON support_messages (created_at DESC);
CREATE INDEX support_messages_user_id_idx    ON support_messages (user_id) WHERE user_id IS NOT NULL;

CREATE TRIGGER support_messages_set_updated_at
BEFORE UPDATE ON support_messages
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS support_messages;
DROP TYPE  IF EXISTS support_status;
DROP TYPE  IF EXISTS support_category;

-- +goose StatementEnd
