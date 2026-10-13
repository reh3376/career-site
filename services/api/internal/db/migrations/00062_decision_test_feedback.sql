-- +goose Up
-- +goose StatementBegin

-- Feedback written from the decision test's thanks page, while the
-- fifteen minutes are still fresh.
--
-- It lands in support_messages rather than in its own table because it
-- is the same thing the contact form already handles: prose from a
-- person, needing a reply, belonging in one inbox. What makes it
-- different is who may send it and what it can be joined to, and those
-- are the two things added here.
--
-- Why not LinkedIn comments, which was the alternative: a participant
-- describing the test in public primes every later reader, which is
-- the one thing the instrument cannot survive. A private box is not
-- merely more convenient, it protects the measurement.

ALTER TABLE support_messages
  ADD COLUMN dt_session_public_id uuid;

ALTER TYPE support_category ADD VALUE IF NOT EXISTS 'decision_test_feedback';

COMMENT ON COLUMN support_messages.dt_session_public_id IS
  'The dt_sessions.public_id of the run this feedback came from, when the '
  'sender wrote from the thanks page. NULL for every other message. This is '
  'what lets the admin console show that a note came from someone who '
  'actually finished the test, and what lets a remark be read beside that '
  'run''s own numbers. Deliberately not a foreign key: feedback must '
  'outlive a purged session, and a note whose run has gone is still worth '
  'reading.';

-- Partial, because the column is NULL for all but a small fraction of
-- rows and the only query that wants it is "feedback for this run".
CREATE INDEX IF NOT EXISTS idx_support_messages_dt_session
  ON support_messages (dt_session_public_id)
  WHERE dt_session_public_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_support_messages_dt_session;

ALTER TABLE support_messages
  DROP COLUMN dt_session_public_id;

-- Postgres cannot DROP a value from an enum type; leaving
-- 'decision_test_feedback' as a permitted value has no cost.

-- +goose StatementEnd
