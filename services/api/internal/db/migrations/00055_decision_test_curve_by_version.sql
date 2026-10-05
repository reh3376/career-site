-- +goose Up
-- +goose StatementBegin

-- The load curve and the calibration curve must not pool across
-- instruments.
--
-- Found by reading the first real output of v_dt_load_curve on
-- production, 2026-10-05, where it reported this:
--
--   d3          rank 1   accuracy  33%
--   d4          rank 2   accuracy  58%
--   d4_plus1    rank 3   accuracy  33%
--   d4_plus3    rank 4   accuracy 100%
--   d3_control  rank 1   accuracy 100%
--
-- Accuracy rising to 100% at the hardest load. The view was right and
-- the data was real: both runs were taken on item set items-2026-10-04,
-- where the blocks were family-blocked rather than interleaved. Block 1
-- was six arithmetic items and block 4 was conjunctions and syllogisms,
-- so item family was confounded with load and swamped it. Migration
-- 00052 fixed the composition; those two runs predate it and are
-- correctly tagged as a different item set.
--
-- The flaw is mine, in 00054. v_dt_load_curve grouped by block_load
-- alone, which means it averages every run ever taken regardless of
-- which items they saw or how long they had. That is exactly what
-- instrument_version and item_set_version were put on dt_sessions to
-- prevent, and the view making the strongest claim in the whole set was
-- the one ignoring them. The next run taken on the current composition
-- would have been silently averaged with these two, and the result
-- would have been meaningless while looking like a result.
--
-- So both curves now group by item_set_version and instrument_version.
--
-- This fragments the output, and the fragmentation is the point. Rows
-- with a tiny `sessions` count are the view saying the instrument is
-- still moving and there is nothing to read yet. A single pooled row
-- that averages four instruments is worse than five honest rows that
-- each say "two runs". It also prices a change: every timing tweak
-- starts a new row, which is the cost of changing the instrument made
-- visible instead of hidden.
--
-- No second pooled view is provided. Two definitions of the headline
-- number is how a dashboard starts disagreeing with itself, and the
-- pooled one would be the one people quoted.

DROP VIEW IF EXISTS v_dt_calibration;
DROP VIEW IF EXISTS v_dt_load_curve;

CREATE VIEW v_dt_load_curve AS
SELECT
  item_set_version,
  instrument_version,
  block_load,
  load_rank,
  count(DISTINCT session_key)                     AS sessions,
  count(*)                                        AS answers,
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))        AS accuracy_pct,
  round(100.0 * count(*) FILTER (WHERE is_lure) / count(*))           AS lure_pct,
  round(100.0 * count(*) FILTER (WHERE is_expired) / count(*))        AS expiry_pct,
  round(avg(confidence))                          AS mean_confidence,
  round(avg(confidence) - 100.0 * count(*) FILTER (WHERE is_correct) / count(*))
                                                  AS gap_pct,
  round(100.0 * count(*) FILTER (WHERE confidently_wrong) / count(*)) AS confidently_wrong_pct,
  round(100.0 * count(*) FILTER (WHERE confidently_lured) / count(*)) AS confidently_lured_pct,
  round(avg(brier), 4)                            AS mean_brier,
  round(avg(latency_ms))                          AS mean_latency_ms,
  round(avg(latency_vs_baseline), 2)              AS mean_latency_vs_baseline,
  round(avg(memory_failure), 2)                   AS mean_memory_failure,
  -- Which families sit in this block under this item set. One value is
  -- a family-blocked composition, which means load and family are
  -- confounded and this row cannot support a claim about load at all.
  -- It is on the row rather than in a note because the note would be
  -- read after the number.
  (SELECT string_agg(DISTINCT f.item_family, '+' ORDER BY f.item_family)
     FROM v_dt_answers f
    WHERE f.item_set_version = v.item_set_version
      AND f.block_load = v.block_load
      AND NOT f.is_synthetic)                     AS families
FROM v_dt_answers v
WHERE NOT is_synthetic
GROUP BY item_set_version, instrument_version, block_load, load_rank
ORDER BY item_set_version, instrument_version, load_rank, block_load;

COMMENT ON VIEW v_dt_load_curve IS
  'Accuracy and confidence by load level: the instrument''s result. Grouped by item_set_version AND instrument_version, so runs taken under different items or different timings never pool into one claim. Many rows with few sessions each means the instrument is still moving. A families column holding one value means load is confounded with item family in that row.';

CREATE VIEW v_dt_calibration AS
SELECT
  item_set_version,
  instrument_version,
  block_load,
  load_rank,
  (confidence / 10) * 10                          AS confidence_band,
  count(*)                                        AS answers,
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))        AS accuracy_pct,
  ((confidence / 10) * 10 + 5)
    - round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))    AS overclaim_pct,
  round(avg(brier), 4)                            AS mean_brier
FROM v_dt_answers
WHERE NOT is_synthetic AND confidence IS NOT NULL
GROUP BY item_set_version, instrument_version, block_load, load_rank, (confidence / 10) * 10
ORDER BY item_set_version, instrument_version, load_rank, block_load, confidence_band;

COMMENT ON VIEW v_dt_calibration IS
  'Confidence bands of ten against the accuracy achieved in each, split by load and by instrument. overclaim_pct compares the band midpoint to its accuracy, so positive is overconfident.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Leaves the versioned views in place. Reverting would reinstate the
-- pooling this migration exists to stop, and a down migration here is a
-- development convenience rather than a rollback path.
SELECT 1;
-- +goose StatementEnd
