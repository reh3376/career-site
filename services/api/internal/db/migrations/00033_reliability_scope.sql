-- +goose Up
-- +goose StatementBegin

-- What reliability is allowed to count against itself.
--
-- The criterion is twenty consecutive runs that finished without anyone
-- stepping in. A run the gatekeeper refused, `not_a_posting`, did
-- finish without anyone stepping in: the pipeline read the input,
-- decided it was not a job posting, said so, and stopped. That is the
-- system working, and counting it as a run that failed to complete
-- makes the reviewer look unreliable for doing exactly what it should.
--
-- Two of the last eight runs were such refusals, which dropped the row
-- to "6 of the last 8" for no fault at all. They are excluded from the
-- window entirely rather than counted as completed, so the twenty are
-- twenty real attempts to review a posting.
--
-- The narrower question this does not answer: whether the gatekeeper
-- refuses the right things. That belongs to agreement and to the
-- decision log, not here, and hiding a bad refusal inside a reliability
-- number would be the wrong place to find it.
--
-- Replaced in place, and written from pg_get_viewdef rather than from
-- the migration that created it. On 2026-09-24 a view replacement
-- written from the original definition dropped two columns a later
-- migration had added, and a drop-and-recreate took the api down
-- because a newer view depended on it.

CREATE OR REPLACE VIEW v_reliability AS
WITH recent AS (
  SELECT *
    FROM v_jd_runs
   WHERE status <> 'not_a_posting'
   ORDER BY started_at DESC
   LIMIT 20
)
SELECT tenant_id,
       count(*)                                          AS runs,
       count(*) FILTER (WHERE status IN ('ready', 'below_threshold')) AS completed,
       count(*) FILTER (WHERE status = 'failed')         AS failed,
       count(*) FILTER (WHERE status = 'running')        AS stuck,
       min(started_at)                                   AS oldest,
       max(started_at)                                   AS newest
  FROM recent
 GROUP BY tenant_id;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE VIEW v_reliability AS
WITH recent AS (
  SELECT * FROM v_jd_runs ORDER BY started_at DESC LIMIT 20
)
SELECT tenant_id,
       count(*)                                          AS runs,
       count(*) FILTER (WHERE status IN ('ready', 'below_threshold')) AS completed,
       count(*) FILTER (WHERE status = 'failed')         AS failed,
       count(*) FILTER (WHERE status = 'running')        AS stuck,
       min(started_at)                                   AS oldest,
       max(started_at)                                   AS newest
  FROM recent
 GROUP BY tenant_id;
-- +goose StatementEnd
