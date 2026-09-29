-- +goose Up
-- +goose StatementBegin

-- Where the time went inside a run.
--
-- jd_runs already records queued_ms and duration_ms, which separate
-- waiting from working. Neither says what the work was. A posting that
-- takes fifty minutes and one that takes four hours have the same
-- shape in this table, and telling them apart has meant reading
-- llm_usage timestamps and subtracting by hand.
--
-- That has two holes, and both were hit this week. Retrieval makes no
-- model call, so it leaves no llm_usage row and is invisible to that
-- method entirely. And a run that failed is the run most worth
-- understanding, which is exactly when the reconstruction is hardest:
-- Blue Origin was lost twice, on runs 13 and 14, and answering "where
-- did those four hours go" took a manual join both times.
--
-- Recorded as jsonb rather than a column per phase because the phases
-- follow the pipeline and the pipeline changes; `prompts` in this same
-- table is stored the same way for the same reason. Keys are the
-- phases in services/api/internal/jd/phases.go: setup, posting_check,
-- extract, retrieval, judge, score, resume, render, and `other` for a
-- stage whose wording drifted out of the mapping. Values are
-- milliseconds.
--
-- Empty for every row written before this migration, and empty is
-- honest: those runs were not measured. No backfill is possible and
-- none is attempted.
ALTER TABLE jd_runs ADD COLUMN IF NOT EXISTS phase_ms jsonb NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN jd_runs.phase_ms IS
  'Milliseconds per pipeline phase, keyed by the phases in internal/jd/phases.go. Empty means the run predates the measurement, not that it took no time. An "other" key means a reported stage no longer matches the mapping.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE jd_runs DROP COLUMN IF EXISTS phase_ms;
-- +goose StatementEnd
