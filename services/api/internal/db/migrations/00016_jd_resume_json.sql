-- +goose Up
-- +goose StatementBegin

-- The structured résumé the model produced, after code-side
-- verification (every bullet carries the chunk ids it came from;
-- unsourced bullets were dropped). resume_markdown is rendered from
-- this; the PDF step renders from it too, so the JSON is the record.
ALTER TABLE jd_submissions
  ADD COLUMN resume_json jsonb;

COMMENT ON COLUMN jd_submissions.resume_json IS
  'Verified structured résumé (headline, summary, competencies, experience, education) with source chunk ids per item.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE jd_submissions DROP COLUMN resume_json;

-- +goose StatementEnd
