-- +goose Up
-- +goose StatementBegin

-- How the meeting actually happens, which the booking never recorded.
--
-- A time on a calendar is not a meeting. Until now a booking said when
-- and what about, and left both parties to work out how they would
-- reach each other, which is a second exchange of emails to arrange
-- the thing the booking was supposed to have arranged.
--
-- Deliberately not an integration. The owner's decision is that the
-- member brings their own video room: they name the provider and are
-- sent to their own calendar to create the link. This application
-- holds no Meet, Teams or Zoom credentials and creates no rooms, which
-- is three fewer OAuth grants and three fewer things to break.
ALTER TABLE meeting_bookings
  ADD COLUMN IF NOT EXISTS meeting_type   text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS video_provider text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS phone_number   text NOT NULL DEFAULT '';

-- Empty is what every existing row has and is a real state: bookings
-- made before this migration genuinely did not record a type, and
-- saying so is better than inventing one.
COMMENT ON COLUMN meeting_bookings.meeting_type IS
  'video | phone, or empty for a booking made before this was asked.';

COMMENT ON COLUMN meeting_bookings.video_provider IS
  'google_meet | teams | zoom, set only when meeting_type = video. The member creates the room in their own calendar; this application never does.';

COMMENT ON COLUMN meeting_bookings.phone_number IS
  'The number the member will call FROM, set only when meeting_type = phone. Empty with meeting_type = phone means they said the number is in the meeting comments instead.';

-- The check is the honest one: it constrains the vocabulary without
-- pretending older rows had a type. A row is valid if it is one of the
-- two kinds, or predates the question.
ALTER TABLE meeting_bookings
  DROP CONSTRAINT IF EXISTS meeting_bookings_type_known;
ALTER TABLE meeting_bookings
  ADD CONSTRAINT meeting_bookings_type_known
  CHECK (meeting_type IN ('', 'video', 'phone'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE meeting_bookings DROP CONSTRAINT IF EXISTS meeting_bookings_type_known;
ALTER TABLE meeting_bookings
  DROP COLUMN IF EXISTS meeting_type,
  DROP COLUMN IF EXISTS video_provider,
  DROP COLUMN IF EXISTS phone_number;
-- +goose StatementEnd
