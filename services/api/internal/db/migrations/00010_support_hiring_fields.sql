-- +goose Up
-- +goose StatementBegin

-- Hiring-inquiry fields on support_messages. Populated only when a
-- contact-form submission's category is `hiring_inquiry`; other
-- categories leave these NULL. Kept as separate columns (not a JSON
-- blob) so the admin console can render them without a payload
-- accessor and so a future admin filter ("show me all hiring
-- inquiries with a JD URL") is a plain WHERE clause.

ALTER TABLE support_messages
  ADD COLUMN hiring_role         text,
  ADD COLUMN hiring_jd_url       text,
  ADD COLUMN hiring_target_start text;

ALTER TYPE support_category ADD VALUE IF NOT EXISTS 'hiring_inquiry';

COMMENT ON COLUMN support_messages.hiring_role IS
  'Role the sender is considering Roger for. Free text, from the form field.';
COMMENT ON COLUMN support_messages.hiring_jd_url IS
  'URL of the job posting. Not validated beyond length so ATS-token URLs paste cleanly.';
COMMENT ON COLUMN support_messages.hiring_target_start IS
  'Free-text target start, e.g. "ASAP" / "Q1 2027" / "flexible".';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE support_messages
  DROP COLUMN hiring_role,
  DROP COLUMN hiring_jd_url,
  DROP COLUMN hiring_target_start;

-- Postgres cannot DROP a value from an enum type; leaving
-- 'hiring_inquiry' as a permitted value has no cost.

-- +goose StatementEnd
