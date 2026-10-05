-- +goose Up
-- +goose StatementBegin

-- Scoring a missed recall by severity rather than by cause.
--
-- The owner's call, and he is right that the alternative is not
-- available: there is no honest way to tell a typo from a transposition
-- from a genuine memory lapse at the moment somebody types four digits.
-- An incorrect number is a memory failure, and what can be measured is
-- how bad it was. Miss one digit of four and 0.25 of the number was
-- lost; miss all four and it was 1.0.
--
-- # Why it is not scored against the expected number alone
--
-- The obvious formula, one minus (digits matching what was wanted over
-- digits total), gets the most interesting case exactly backwards.
--
-- Block 3 of the owner's first run: shown 3777, wanted 4888 after the
-- +1, and he typed 3777. Against the expected number that is zero
-- digits right and scores 1.0, a total memory failure. It was the
-- opposite. He held the number perfectly and did not apply the
-- transformation, which is the executive failing while storage holds,
-- and it is the single sharpest signal this instrument produces.
--
-- So memory is scored against WHICHEVER of the two the response is
-- closer to. Giving back the raw number proves it was held. Giving back
-- the transformed number proves it was held and operated on. Giving
-- back neither is the only case where memory actually failed.
--
-- digits_held carries that count, and the view derives the severity, so
-- there is one definition of it (docs/metrics.md) rather than one per
-- consumer.

ALTER TABLE dt_recalls ADD COLUMN IF NOT EXISTS digits_held int NOT NULL DEFAULT 0;

COMMENT ON COLUMN dt_recalls.digits_held IS
  'Positional digit matches against whichever of presented_digits or expected_digits the response is closer to. Measures retention, not correctness: returning the untransformed number scores full marks here, because the number survived and only the operation did not.';

-- Backfill from the strings already stored, so the ten existing rows
-- carry the measure too rather than starting at zero and looking like
-- total failures.
WITH scored AS (
  SELECT r.id,
         greatest(
           (SELECT count(*) FROM generate_series(1, length(r.presented_digits)) i
             WHERE substr(r.presented_digits, i, 1) = substr(r.response_digits, i, 1)),
           (SELECT count(*) FROM generate_series(1, length(r.expected_digits)) i
             WHERE substr(r.expected_digits, i, 1) = substr(r.response_digits, i, 1))
         ) AS held
    FROM dt_recalls r
   WHERE r.response_digits <> ''
)
UPDATE dt_recalls SET digits_held = scored.held
  FROM scored WHERE dt_recalls.id = scored.id;

-- The severity, defined once, at the projection boundary (ADR 0030).
--
-- 0.0 is a number held intact, 1.0 is one lost entirely. An expired
-- recall is not scored: nothing was attempted, so there is nothing to
-- measure, and calling it 1.0 would put "ran out of time" and "forgot
-- completely" in the same bucket.
DROP VIEW IF EXISTS v_dt_answers;

CREATE VIEW v_dt_answers AS
SELECT
  a.id                      AS answer_id,
  s.public_id               AS session_key,
  p.public_id               AS participant_key,
  s.status,
  s.is_synthetic,
  s.instrument_version,
  s.item_set_version,
  s.key_version,
  s.audio_mode,
  s.device_class,
  s.tap_check_passed,
  s.baseline_rt_ms,
  s.baseline_rt_sd_ms,
  s.is_repeat,
  s.recall_strategy,
  p.age_range,
  p.education,
  p.occupation,
  (p.email_key <> '')       AS gave_email,
  a.block_no,
  a.block_load,
  a.position_in_block,
  a.position_overall,
  a.item_code,
  a.item_version,
  i.family                  AS item_family,
  a.outcome,
  (a.outcome = 'correct')   AS is_correct,
  (a.outcome = 'lure')      AS is_lure,
  (a.outcome = 'expired')   AS is_expired,
  a.latency_ms,
  CASE WHEN s.baseline_rt_ms > 0 THEN a.latency_ms::numeric / s.baseline_rt_ms END
                            AS latency_vs_baseline,
  a.confidence,
  r.outcome                 AS recall_outcome,
  (r.outcome = 'untransformed') AS recall_executive_failure,
  r.digits_held,
  -- How much of the number was lost, 0.0 to 1.0. Null where the recall
  -- expired, because nothing was attempted.
  CASE
    WHEN r.outcome IS NULL OR r.outcome = 'expired' THEN NULL
    WHEN length(r.presented_digits) = 0 THEN NULL
    ELSE round(1 - (r.digits_held::numeric / length(r.presented_digits)), 2)
  END                       AS memory_failure,
  s.started_at
FROM dt_answers a
JOIN dt_sessions s     ON s.id = a.session_id
JOIN dt_items i        ON i.id = a.item_id
LEFT JOIN dt_participants p ON p.id = s.participant_id
LEFT JOIN dt_recalls r ON r.session_id = a.session_id AND r.block_no = a.block_no;

COMMENT ON VIEW v_dt_answers IS
  'The projection boundary (ADR 0030). One row per answer with demographics, conditions, baseline, load, outcome and recall severity already joined. memory_failure is 0.0 for a number held intact and 1.0 for one lost entirely, scored against whichever of the presented or expected number the response is closer to so that an untransformed answer reads as perfect retention rather than total loss.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE dt_recalls DROP COLUMN IF EXISTS digits_held;
-- +goose StatementEnd
