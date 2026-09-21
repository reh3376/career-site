-- +goose Up
-- +goose StatementBegin

-- JD upload is members-only from here on; record who submitted so the
-- admin triage view shows the person, not just an IP hash. Existing
-- rows (pre-gate) keep NULL.
ALTER TABLE jd_submissions
  ADD COLUMN user_id bigint REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_jd_submissions_user ON jd_submissions (user_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_jd_submissions_user;
ALTER TABLE jd_submissions DROP COLUMN user_id;

-- +goose StatementEnd
