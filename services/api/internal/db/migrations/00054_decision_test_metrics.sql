-- +goose Up
-- +goose StatementBegin

-- The decision test's metric views (docs/metrics.md, roadmap M6).
--
-- Collection without analysis is a pile of rows. These views are the
-- other half of the deliverable, and they follow the same rule as every
-- other number on this site: **defined once, in SQL, and never
-- recomputed in Go or in a page.** Two definitions of the same number
-- is how a dashboard starts disagreeing with itself and then nobody can
-- say which one is lying.
--
-- The grain stays dt_answers: one row per question presented. Every
-- view here counts from it. Nothing below is stored.
--
-- Two definitions had to be made rather than found, and both are
-- isolated into functions so that changing them is one line in one
-- place rather than a search across seven views.

-- What counts as confident.
--
-- The research question is about being wrong *and sure*, so a number
-- for "sure" is unavoidable. The FSD states the measure (§2.2: the load
-- level at which this participant starts answering with the lure while
-- still reporting high confidence) and deliberately does not fix the
-- threshold, for the same reason the tap-check thresholds are left open
-- in §4b: it is a calibration, and guessing it in a document is how a
-- cut becomes either a nuisance or a formality.
--
-- 70 is the starting point to pilot against. On a 0 to 100 slider it is
-- a clear claim to being more right than wrong, and with four options
-- chance is 25, so it sits well clear of a shrug. Expect to move it
-- once there are enough runs to see where the distribution actually
-- sits, and move it HERE.
CREATE OR REPLACE FUNCTION dt_confident_threshold() RETURNS int
LANGUAGE sql IMMUTABLE AS $$ SELECT 70 $$;

-- How the loads order.
--
-- Needed because the headline measure is "the lowest load at which this
-- person went confidently wrong", and 'd4_plus1' does not sort after
-- 'd4' by accident of the alphabet, it sorts after it because it is
-- harder.
--
-- d3_control ranks the same as d3 because it IS d3: block 5 returns to
-- block 1's difficulty and is the fatigue control (FSD §3.3). Ranking
-- it 5 would make the control look like the hardest block in every
-- ordered query, which is precisely backwards.
CREATE OR REPLACE FUNCTION dt_load_rank(load text) RETURNS int
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE load
           WHEN 'd3'         THEN 1
           WHEN 'd3_control' THEN 1
           WHEN 'd4'         THEN 2
           WHEN 'd4_plus1'   THEN 3
           WHEN 'd4_plus3'   THEN 4
         END
$$;

-- +goose StatementEnd

-- +goose StatementBegin

-- ------------------------------------------------------- the flat view
--
-- Rebuilt rather than replaced: CREATE OR REPLACE VIEW can only append
-- columns, and appending silently does nothing useful when what you
-- wanted was a column in the middle. That exact failure put
-- is_synthetic on the table and left it off the view on 2026-10-04,
-- with the replay script swallowing the error. DROP then CREATE, every
-- time.
--
-- This is also the export view (CSV, admin console). What it does NOT
-- carry is deliberate:
--
--   * No name and no email address. participant_key is the opaque uuid,
--     so a repeat participant's runs join to each other without the
--     export carrying who they are.
--   * No chosen_index. is_lure says which kind of wrong an answer was,
--     which is the measurement; the raw index across enough sessions
--     would let somebody reconstruct the answer key from an export, and
--     the key only has to escape once.
--
-- Dependents are dropped first, deepest last, before anything is
-- created. On a fresh replay none of them exist and every statement is
-- a no-op. On any later rebuild of the flat view they do exist, and a
-- bare DROP of it fails with "other objects depend on it" rather than
-- doing the wrong thing quietly. Written in this order so the next
-- migration that has to touch v_dt_answers has the pattern in front of
-- it; CASCADE is deliberately not used, because CASCADE would also
-- silently take a view added after this file was written.
DROP VIEW IF EXISTS v_dt_items;
DROP VIEW IF EXISTS v_dt_threshold;
DROP VIEW IF EXISTS v_dt_calibration;
DROP VIEW IF EXISTS v_dt_load_curve;
DROP VIEW IF EXISTS v_dt_sessions;
DROP VIEW IF EXISTS v_dt_blocks;
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
  dt_load_rank(a.block_load) AS load_rank,
  a.position_in_block,
  a.position_overall,
  a.item_code,
  a.item_version,
  i.family                  AS item_family,
  a.outcome,
  (a.outcome = 'correct')   AS is_correct,
  (a.outcome = 'lure')      AS is_lure,
  (a.outcome = 'expired')   AS is_expired,
  -- Wrong and sure, which is the thing this instrument exists to
  -- measure. An expiry is not confidently wrong: nothing was claimed,
  -- so it is false here rather than null.
  (a.outcome NOT IN ('correct', 'expired')
     AND a.confidence >= dt_confident_threshold())
                            AS confidently_wrong,
  -- The FSD's strict form of the same thing (§2.2): the intended
  -- intuitive answer, taken with high confidence. Separate from
  -- confidently_wrong because choosing the lure is a different event
  -- from choosing at random, and pooling them would blunt the one
  -- measure the item design was built for.
  (a.outcome = 'lure' AND a.confidence >= dt_confident_threshold())
                            AS confidently_lured,
  a.latency_ms,
  CASE WHEN s.baseline_rt_ms > 0 THEN a.latency_ms::numeric / s.baseline_rt_ms END
                            AS latency_vs_baseline,
  a.confidence,
  -- Brier score for this answer: the squared distance between the
  -- confidence claimed and what happened. 0 is perfect, 1 is maximally
  -- wrong and sure. Null on an expiry, where there is no claim to
  -- score. Computed, never shown to a participant (FSD §2.1): it is the
  -- more rigorous statement of the same thing the plain comparison
  -- already makes, and it needs a paragraph most readers would skip.
  CASE WHEN a.confidence IS NULL THEN NULL
       ELSE round(power(a.confidence / 100.0
                        - CASE WHEN a.outcome = 'correct' THEN 1 ELSE 0 END, 2), 4)
  END                       AS brier,
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
  'One row per decision test question presented, with the session and participant context flattened onto it. The grain, the export, and the base of every other dt view. Carries no name, no email and no chosen_index: the answer key must not be reconstructable from an export.';

-- +goose StatementEnd

-- +goose StatementBegin

-- --------------------------------------------------------- per block
--
-- One row per session per block. This is where the instrument either
-- works or does not: accuracy should fall across blocks 1 to 4 while
-- confidence does not, and block 5 should come back up.
CREATE VIEW v_dt_blocks AS
SELECT
  session_key,
  is_synthetic,
  status,
  instrument_version,
  item_set_version,
  block_no,
  block_load,
  load_rank,
  count(*)                                        AS answered,
  count(*) FILTER (WHERE is_correct)              AS correct,
  count(*) FILTER (WHERE is_lure)                 AS lure,
  count(*) FILTER (WHERE is_expired)              AS expired,
  count(*) FILTER (WHERE confidently_wrong)       AS confidently_wrong,
  count(*) FILTER (WHERE confidently_lured)       AS confidently_lured,
  -- Accuracy counts an expiry as not correct, because under load it is
  -- not correct. The expiry rate is reported beside it so the two
  -- readings of a low block are distinguishable: not knowing, and not
  -- finishing.
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))  AS accuracy_pct,
  round(100.0 * count(*) FILTER (WHERE is_expired) / count(*))  AS expiry_pct,
  round(avg(confidence))                          AS mean_confidence,
  -- THE HEADLINE. Confidence minus accuracy, in points. Positive means
  -- more sure than right.
  round(avg(confidence) - 100.0 * count(*) FILTER (WHERE is_correct) / count(*))
                                                  AS gap_pct,
  round(avg(brier), 4)                            AS mean_brier,
  round(avg(latency_ms))                          AS mean_latency_ms,
  round(avg(latency_vs_baseline), 2)              AS mean_latency_vs_baseline,
  max(recall_outcome)                             AS recall_outcome,
  max(digits_held)                                AS digits_held,
  max(memory_failure)                             AS memory_failure,
  bool_or(recall_executive_failure)               AS recall_executive_failure,
  min(started_at)                                 AS started_at
FROM v_dt_answers
GROUP BY session_key, is_synthetic, status, instrument_version,
         item_set_version, block_no, block_load, load_rank;

COMMENT ON VIEW v_dt_blocks IS
  'One row per decision test session per block: accuracy, confidence, the gap between them, expiry rate and the recall. Where the instrument is read as working or not. Recall columns are per block so max() over the group is the block''s single recall row, not an aggregate.';

-- +goose StatementEnd

-- +goose StatementBegin

-- ------------------------------------------------------- per session
--
-- One row per run. Carries the two comparisons the result email makes,
-- so that what a participant is told and what the console shows come
-- from the same definition.
CREATE VIEW v_dt_sessions AS
WITH blocks AS (SELECT * FROM v_dt_blocks)
SELECT
  b.session_key,
  b.is_synthetic,
  b.status,
  b.instrument_version,
  b.item_set_version,
  min(b.started_at)                               AS started_at,
  count(DISTINCT b.block_no)                      AS blocks_reached,
  sum(b.answered)                                 AS answered,
  sum(b.correct)                                  AS correct,
  sum(b.expired)                                  AS expired,
  sum(b.confidently_wrong)                        AS confidently_wrong,
  sum(b.confidently_lured)                        AS confidently_lured,
  round(100.0 * sum(b.correct) / sum(b.answered))       AS accuracy_pct,
  round(avg(b.mean_confidence))                   AS mean_confidence,
  round(avg(b.mean_confidence) - 100.0 * sum(b.correct) / sum(b.answered))
                                                  AS gap_pct,
  round(avg(b.mean_brier), 4)                     AS mean_brier,
  -- The early-against-hardest comparison the result email leads with.
  -- Blocks 1 and 2 against block 4, which is the top of the ramp.
  round(avg(b.accuracy_pct)      FILTER (WHERE b.block_no IN (1, 2))) AS early_accuracy_pct,
  round(avg(b.mean_confidence)   FILTER (WHERE b.block_no IN (1, 2))) AS early_confidence_pct,
  round(avg(b.accuracy_pct)      FILTER (WHERE b.block_no = 4))       AS hard_accuracy_pct,
  round(avg(b.mean_confidence)   FILTER (WHERE b.block_no = 4))       AS hard_confidence_pct,
  -- The fatigue control. Block 5 is block 1's difficulty at the end of
  -- the test, so block 5 holding up is what lets the decline across
  -- blocks 2 to 4 be attributed to load rather than to fifteen minutes
  -- of elapsed effort (FSD §2.3). Without this pair the headline is
  -- uninterpretable, which is why it sits on the session row rather
  -- than being left for someone to work out.
  round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 1))            AS block1_accuracy_pct,
  round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 5))            AS block5_accuracy_pct,
  round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 5))
    - round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 1))        AS fatigue_delta_pct,
  -- Recall across the run: how much of the number survived on average,
  -- and how many blocks were handed back untransformed. The second is
  -- storage holding while the executive fails, which is the sharpest
  -- single signal this instrument produces.
  round(avg(b.memory_failure), 2)                 AS mean_memory_failure,
  count(*) FILTER (WHERE b.recall_executive_failure) AS untransformed_blocks
FROM blocks b
GROUP BY b.session_key, b.is_synthetic, b.status,
         b.instrument_version, b.item_set_version;

COMMENT ON VIEW v_dt_sessions IS
  'One row per decision test run: accuracy, confidence, the gap, the early-against-hardest comparison the result email makes, and the block 5 against block 1 fatigue control. Includes synthetic and unfinished runs, flagged rather than filtered.';

-- +goose StatementEnd

-- +goose StatementBegin

-- ------------------------------------------- the result, across people
--
-- The load curve: what happens to accuracy and to confidence as the
-- load rises, pooled across participants. If this instrument works,
-- accuracy_pct falls from rank 1 to rank 4 while mean_confidence does
-- not, and gap_pct grows.
--
-- Synthetic runs are EXCLUDED here, not flagged. Every other view keeps
-- them visible because the console needs to show them; this one makes a
-- claim about people, and an agent's answers silently averaged into it
-- would be a lie rather than a caveat.
CREATE VIEW v_dt_load_curve AS
SELECT
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
  round(avg(memory_failure), 2)                   AS mean_memory_failure
FROM v_dt_answers
WHERE NOT is_synthetic
GROUP BY block_load, load_rank
ORDER BY load_rank, block_load;

COMMENT ON VIEW v_dt_load_curve IS
  'Accuracy and confidence by load level across all real participants: the instrument''s result. Excludes synthetic runs outright rather than flagging them, because this view makes a claim about people.';

-- +goose StatementEnd

-- +goose StatementBegin

-- --------------------------------------------------- the reliability curve
--
-- Confidence claimed against accuracy achieved, in bands of ten. The
-- standard way to show overconfidence: a well-calibrated person's
-- accuracy_pct tracks their band, and the diagonal is where honesty
-- would put them.
--
-- Reported by load as well as overall, because the question is not
-- whether people are overconfident in general. It is whether load makes
-- them more so, which only a split can answer.
CREATE VIEW v_dt_calibration AS
SELECT
  block_load,
  load_rank,
  (confidence / 10) * 10                          AS confidence_band,
  count(*)                                        AS answers,
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))        AS accuracy_pct,
  -- Positive means the band overclaims: more confident than correct.
  ((confidence / 10) * 10 + 5)
    - round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))    AS overclaim_pct,
  round(avg(brier), 4)                            AS mean_brier
FROM v_dt_answers
WHERE NOT is_synthetic AND confidence IS NOT NULL
GROUP BY block_load, load_rank, (confidence / 10) * 10
ORDER BY load_rank, block_load, confidence_band;

COMMENT ON VIEW v_dt_calibration IS
  'Confidence bands of ten against the accuracy actually achieved in each, split by load. overclaim_pct compares the band midpoint to its accuracy, so positive is overconfident.';

-- +goose StatementEnd

-- +goose StatementBegin

-- ------------------------------------------------ the per-person answer
--
-- The research question is about *a given person*: the load level at
-- which this participant starts answering with the lure while still
-- reporting high confidence (FSD §2.2). This view is that number, one
-- row per run.
--
-- Null threshold means it never happened in this run, which is a real
-- answer and not missing data: either the load never got far enough for
-- this person, or they stayed calibrated, and the two are told apart by
-- reading accuracy_pct beside it.
--
-- THE THRESHOLD IS TAKEN OVER THE RAMP ONLY, blocks 1 to 4.
--
-- Block 5 is excluded because it is block 1's difficulty served at the
-- end of the test, and it ranks 1 for exactly that reason. A
-- participant who holds up through the ramp and then comes apart on the
-- control would otherwise be reported as having a threshold of rank 1,
-- which says the easiest load defeated them when what actually
-- defeated them was fifteen minutes. That is the confound FSD §2.3
-- exists to separate, and leaving block 5 in the min() would reintroduce
-- it through the back door, worst for the participants the control was
-- built to detect.
--
-- The control is not discarded: lured_in_control reports it alongside,
-- so a threshold can be read together with whether this person's
-- block 5 held. A threshold with lured_in_control true is a weaker
-- claim about load and should be read as such.
CREATE VIEW v_dt_threshold AS
SELECT
  s.session_key,
  s.is_synthetic,
  s.status,
  s.accuracy_pct,
  s.gap_pct,
  s.block5_accuracy_pct,
  s.block1_accuracy_pct,
  s.fatigue_delta_pct,
  -- Lowest load on the ramp at which a lure was taken with high
  -- confidence. The strict form, per the FSD.
  (SELECT min(a.load_rank) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.confidently_lured
      AND a.block_no < 5)                                             AS lured_at_rank,
  (SELECT min(a.block_load) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.confidently_lured
      AND a.block_no < 5
      AND a.load_rank = (SELECT min(b.load_rank) FROM v_dt_answers b
                          WHERE b.session_key = s.session_key AND b.confidently_lured
                            AND b.block_no < 5))                      AS lured_at_load,
  -- The looser form: any wrong answer held with high confidence.
  (SELECT min(a.load_rank) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.confidently_wrong
      AND a.block_no < 5)                                             AS wrong_at_rank,
  -- Did the control block go the same way. True weakens every reading
  -- above it.
  (SELECT bool_or(a.confidently_lured) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.block_no = 5)           AS lured_in_control,
  s.confidently_lured,
  s.confidently_wrong,
  s.started_at
FROM v_dt_sessions s;

COMMENT ON VIEW v_dt_threshold IS
  'Per run, the lowest load on the ramp (blocks 1 to 4) at which this participant took the intended wrong answer while still reporting high confidence. The per-person form of the research question. Block 5 is excluded from the min() because it ranks as block 1''s difficulty and would report a fatigued participant''s threshold as the easiest load; it is reported beside as lured_in_control. A null threshold is an answer, not missing data.';

-- +goose StatementEnd

-- +goose StatementBegin

-- ------------------------------------------------------- item quality
--
-- Whether each item is doing its job, which is the thing that needs to
-- be visible after one bad session rather than after forty.
--
-- Two failure modes to watch, and they look nothing alike:
--
--   * lure_pct near zero means the lure does not pull. The item is
--     measuring nothing: everybody either knows it or guesses, and the
--     item design is what needs revisiting, not the instrument.
--   * accuracy_pct near zero with lure_pct high at rank 1 means the
--     item is simply too hard or badly worded, and it will look like a
--     load effect at every load.
--
-- Practice items are absent because they never enter dt_answers.
CREATE VIEW v_dt_items AS
SELECT
  item_code,
  item_version,
  item_family,
  count(*)                                        AS answers,
  count(DISTINCT session_key)                     AS sessions,
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))        AS accuracy_pct,
  round(100.0 * count(*) FILTER (WHERE is_lure) / count(*))           AS lure_pct,
  round(100.0 * count(*) FILTER (WHERE is_expired) / count(*))        AS expiry_pct,
  round(avg(confidence))                          AS mean_confidence,
  round(avg(confidence) - 100.0 * count(*) FILTER (WHERE is_correct) / count(*))
                                                  AS gap_pct,
  round(avg(brier), 4)                            AS mean_brier,
  round(avg(latency_ms))                          AS mean_latency_ms,
  -- Which blocks this item has been served in. The set should be one
  -- value: the item order is fixed, so more than one means the
  -- composition changed and these rows span two instruments.
  array_agg(DISTINCT block_no ORDER BY block_no)  AS block_nos,
  array_agg(DISTINCT item_set_version)            AS item_set_versions
FROM v_dt_answers
WHERE NOT is_synthetic
GROUP BY item_code, item_version, item_family
ORDER BY item_code;

COMMENT ON VIEW v_dt_items IS
  'Per item: accuracy, how often the lure pulled, expiry rate and the confidence gap. A lure_pct near zero is an item measuring nothing. block_nos holding more than one value means the composition changed under these rows.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drops the aggregates and leaves v_dt_answers and the two functions in
-- place, on purpose.
--
-- Reversing the flat view would mean carrying a second copy of its forty
-- columns in this file, and the two would drift. The columns this
-- migration added to it are additions: the admin console and the CSV
-- export read it by name, so leaving them costs nothing and dropping
-- the view would break both. The functions stay because the view calls
-- them.
--
-- A down migration here is a development convenience, not a rollback
-- path. Production only ever goes up, and a rollback rolls the image
-- back rather than the schema (deploy/README.md).
DROP VIEW IF EXISTS v_dt_items;
DROP VIEW IF EXISTS v_dt_threshold;
DROP VIEW IF EXISTS v_dt_calibration;
DROP VIEW IF EXISTS v_dt_load_curve;
DROP VIEW IF EXISTS v_dt_sessions;
DROP VIEW IF EXISTS v_dt_blocks;
-- +goose StatementEnd
