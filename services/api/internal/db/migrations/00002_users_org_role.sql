-- +goose Up
-- +goose StatementBegin

-- Registration collects organization + stated role up front (they appear in
-- the admin approval email). member_profiles will hold the richer profile
-- state Phase 3 introduces (tracks, seniority, priorities). Keeping the
-- registration-time values on users keeps the approval flow's read path
-- trivial and lets member_profiles remain optional until Phase 3.

ALTER TABLE users
  ADD COLUMN organization text NOT NULL DEFAULT '',
  ADD COLUMN stated_role  text NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users
  DROP COLUMN IF EXISTS stated_role,
  DROP COLUMN IF EXISTS organization;

-- +goose StatementEnd
