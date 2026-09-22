-- +goose Up
-- +goose StatementBegin

-- Optional link to apply for the position, given by the submitter on
-- /jd-upload alongside the role, employer and contact hints.
ALTER TABLE jd_submissions ADD COLUMN apply_url text;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE jd_submissions DROP COLUMN apply_url;

-- +goose StatementEnd
