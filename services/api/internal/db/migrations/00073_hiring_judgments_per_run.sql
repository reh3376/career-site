-- +goose Up
-- +goose StatementBegin

-- Phase 0 step 4 (docs/framework-research/13 §2; 14 §4). Judgment rows
-- begin here, one per requirement per run; the unique index makes a
-- retried persist of the same run a replacement, never a duplicate.
CREATE UNIQUE INDEX uq_judgments_run_requirement ON judgments (run_id, requirement_id);

-- The dual-write check learns the second formula. Under v3 the record's
-- `evidenced` is coverage, not the v2 score, so comparing it with
-- jd_runs.match_score would fail by construction. Instead the v2 score
-- is recomputed from the judgment rows' derived verdicts (FR-HF-07):
-- every must requirement in the denominator, a nice one only when it
-- earned something, met 1, partial 0.5, unmet 0, every requirement
-- when the posting has no must lines, behavioral rows excluded
-- (FR-HF-18). If that equals what the pipeline stored, the rows and the
-- jsonb agree judgment by judgment, which is what the hand-over at the
-- end of step 4 is decided from.
DROP VIEW IF EXISTS v_dual_write_agreement;
CREATE VIEW v_dual_write_agreement AS
WITH per_run AS (
  SELECT j.run_id,
         bool_or(q.category = 'must') AS has_must,
         sum(q.weight * CASE j.derived_verdict WHEN 'met' THEN 1.0 WHEN 'partial' THEN 0.5 ELSE 0.0 END) AS weighted,
         sum(q.weight) FILTER (WHERE q.category = 'must' OR j.derived_verdict IN ('met', 'partial')) AS denom_must,
         sum(q.weight) AS denom_all,
         count(*) AS judgment_rows
  FROM judgments j
  JOIN requirements q ON q.id = j.requirement_id
  WHERE q.type <> 'behavioral'
  GROUP BY j.run_id
), v2 AS (
  SELECT run_id, judgment_rows,
         CASE WHEN has_must THEN weighted / NULLIF(denom_must, 0) ELSE weighted / NULLIF(denom_all, 0) END AS score
  FROM per_run
)
SELECT r.run_id, r.tenant_id, r.submission_id, r.attempt, r.status, r.finished_at,
       r.match_score, m.evidenced, m.formula_version,
       v2.score AS judgments_v2_score, v2.judgment_rows,
       r.requirement_count, cardinality(m.requirement_ids) AS requirement_rows,
       m.run_id IS NOT NULL AS record_written,
       m.run_id IS NOT NULL
         AND r.requirement_count = cardinality(m.requirement_ids)
         AND CASE
               WHEN m.formula_version = 'v3-three-numbers'
                 THEN v2.score IS NOT NULL AND abs(COALESCE(r.match_score, 0) - v2.score) < 1e-9
               ELSE (r.match_score IS NULL AND m.evidenced IS NULL
                     OR abs(COALESCE(r.match_score, 0) - COALESCE(m.evidenced, 0)) < 1e-9)
             END AS agrees
FROM jd_runs r
LEFT JOIN match_records m ON m.run_id = r.run_id
LEFT JOIN v2 ON v2.run_id = r.run_id
JOIN jd_submissions s ON s.id = r.submission_id
WHERE r.status IN ('generating', 'below_threshold', 'ready')
   OR m.run_id IS NOT NULL;
COMMENT ON VIEW v_dual_write_agreement IS
  'Per run: does the match record agree with jd_runs on the requirement count and, under v2 on the score or under v3 on the v2 score recomputed from the judgment rows (14 §4; FSD FR-HF-07).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP VIEW IF EXISTS v_dual_write_agreement;
CREATE VIEW v_dual_write_agreement AS
SELECT r.run_id, r.tenant_id, r.submission_id, r.attempt, r.status, r.finished_at,
       r.match_score, m.evidenced,
       r.requirement_count, cardinality(m.requirement_ids) AS requirement_rows,
       m.run_id IS NOT NULL AS record_written,
       m.run_id IS NOT NULL
         AND r.requirement_count = cardinality(m.requirement_ids)
         AND (r.match_score IS NULL AND m.evidenced IS NULL
              OR abs(COALESCE(r.match_score, 0) - COALESCE(m.evidenced, 0)) < 1e-9) AS agrees
FROM jd_runs r
LEFT JOIN match_records m ON m.run_id = r.run_id
JOIN jd_submissions s ON s.id = r.submission_id
WHERE r.status IN ('generating', 'below_threshold', 'ready')
   OR m.run_id IS NOT NULL;
DROP INDEX IF EXISTS uq_judgments_run_requirement;

-- +goose StatementEnd
