-- +goose Up
-- +goose StatementBegin

-- Pipeline progress the submitter can watch: percent and a short stage
-- label, written by the scorer as it moves through requirements,
-- judging, the résumé and the PDF.
ALTER TABLE jd_submissions
  ADD COLUMN progress_pct integer NOT NULL DEFAULT 0,
  ADD COLUMN progress_stage text NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE jd_submissions DROP COLUMN progress_pct, DROP COLUMN progress_stage;

-- +goose StatementEnd
