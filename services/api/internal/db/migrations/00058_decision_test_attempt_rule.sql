-- +goose Up
-- +goose StatementBegin

-- The attempt sequence, derived consistently and in one place.
--
-- **The bug, found in production data rather than reasoned about.** The
-- owner's own session 5 read:
--
--   attempt_no=1  source=account  acct=0  eml=0  cky=1  is_repeat=TRUE
--
-- He sat the test anonymously (session 3), then signed in and sat it
-- again. His account had seen no prior sitting; his browser had seen
-- one. So the run was his second and the view reported his first.
--
-- **Two rules were disagreeing, and I had documented the disagreement
-- as a signal instead of fixing it.**
--
--   is_repeat  = ANY source saw a prior sitting            (an OR)
--   attempt_no = the MOST RELIABLE source that had an answer (a coalesce)
--
-- Those cannot both be right. A row that is a repeat and also attempt
-- one is not a subtle finding, it is two derivations of the same fact
-- contradicting each other, and whichever a reader happens to use
-- decides what they conclude.
--
-- **The rule, and why it is not the thing the owner rejected.** He
-- rejected storing a single attempt number taken from whichever
-- identity reported the most, because that collapsed three
-- observations into one at write time and discarded the components.
-- That objection stands and the components are still stored.
--
-- What is correct is a UNION over the observations: if the cookie saw a
-- prior sitting then a prior sitting exists, whatever the account did
-- or did not see. GREATEST over the three counts is set union, not
-- selection on the value, and it is exactly the arithmetic that
-- `is_repeat` already does with booleans. The two now agree by
-- construction: attempt_no > 1 if and only if some source saw a prior.
--
-- It is also a LOWER BOUND on the truth and can only err downward. We
-- cannot observe a sitting that did not happen; we can easily miss one,
-- which is what a private window or a second device does.
--
-- The one way it can overcount is a browser shared by two people,
-- where the cookie's count includes somebody else's sittings. That is
-- an identity-resolution limit rather than an arithmetic bias, it is
-- visible in the stored components, and `attempt_source` says when the
-- number rests on a cookie.
--
-- Both readings are published rather than one chosen, so nothing is
-- hidden: attempt_no is the union, attempt_no_strongest is what the
-- single most reliable source says, and attempt_sources_disagree marks
-- every row where they differ. Session 5 becomes attempt_no 2,
-- strongest 1, disagree true, which is the whole situation on one row.

-- The rule lives in functions, so changing it never again requires
-- rebuilding seven views. v_dt_answers has now been rebuilt four times
-- in a day, three of those because a derivation was written inline, and
-- that is the lesson rather than the migration.
CREATE OR REPLACE FUNCTION dt_attempt_no(by_account int, by_email int, by_cookie int)
RETURNS int LANGUAGE sql IMMUTABLE AS $$
  -- GREATEST ignores NULLs in Postgres and is NULL only when every
  -- argument is NULL, which is precisely the semantics wanted: an
  -- identity that does not exist has nothing to say, and a run with no
  -- identity at all has an unknown sequence rather than a first one.
  SELECT greatest(by_account, by_email, by_cookie) + 1
$$;

COMMENT ON FUNCTION dt_attempt_no(int, int, int) IS
  'Sitting number as a union over the three identity observations: if any source saw a prior sitting, one existed. A lower bound on the truth, consistent with is_repeat by construction, NULL when no identity could place the run.';

CREATE OR REPLACE FUNCTION dt_attempt_strongest(by_account int, by_email int, by_cookie int)
RETURNS int LANGUAGE sql IMMUTABLE AS $$
  -- What the single most reliable available source says, in an order
  -- fixed in advance. Published beside the union so a reader can see
  -- both and pick, which is the whole point of storing the components.
  SELECT coalesce(by_account, by_email, by_cookie) + 1
$$;

COMMENT ON FUNCTION dt_attempt_strongest(int, int, int) IS
  'Sitting number from the most reliable identity that has an answer (account, then email, then cookie). Can under-report: an account newer than the participant''s activity sees none of it.';

CREATE OR REPLACE FUNCTION dt_attempt_source(by_account int, by_email int, by_cookie int)
RETURNS text LANGUAGE sql IMMUTABLE AS $$
  -- Which source the UNION came from, so a number resting on a cookie
  -- is never mistaken for one resting on an account. Reliability order
  -- on a tie, so the best evidence for the figure is what gets named.
  SELECT CASE
    WHEN by_account IS NOT NULL
     AND by_account = greatest(by_account, by_email, by_cookie) THEN 'account'
    WHEN by_email IS NOT NULL
     AND by_email = greatest(by_account, by_email, by_cookie)   THEN 'email'
    WHEN by_cookie IS NOT NULL
     AND by_cookie = greatest(by_account, by_email, by_cookie)  THEN 'cookie'
    ELSE 'none'
  END
$$;

COMMENT ON FUNCTION dt_attempt_source(int, int, int) IS
  'Which identity produced the attempt_no union, preferring the more reliable on a tie. "none" means no identity could place the run at all.';

-- +goose StatementEnd

-- +goose StatementBegin

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
  s.prior_by_account,
  s.prior_by_email,
  s.prior_by_cookie,
  dt_attempt_no(s.prior_by_account, s.prior_by_email, s.prior_by_cookie)
                            AS attempt_no,
  dt_attempt_strongest(s.prior_by_account, s.prior_by_email, s.prior_by_cookie)
                            AS attempt_no_strongest,
  dt_attempt_source(s.prior_by_account, s.prior_by_email, s.prior_by_cookie)
                            AS attempt_source,
  -- Where the two readings differ, which is the case worth seeing: an
  -- account newer than the participant's own activity.
  (dt_attempt_no(s.prior_by_account, s.prior_by_email, s.prior_by_cookie)
     IS DISTINCT FROM
   dt_attempt_strongest(s.prior_by_account, s.prior_by_email, s.prior_by_cookie))
                            AS attempt_sources_disagree,
  s.recall_strategy,
  s.review_status           AS session_review_status,
  coalesce(br.status, '')   AS block_review_status,
  (NOT s.is_synthetic
     AND s.review_status <> 'do_not_use'
     AND coalesce(br.status, '') <> 'do_not_use')
                            AS usable,
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
  (a.outcome NOT IN ('correct', 'expired')
     AND a.confidence >= dt_confident_threshold())
                            AS confidently_wrong,
  (a.outcome = 'lure' AND a.confidence >= dt_confident_threshold())
                            AS confidently_lured,
  a.latency_ms,
  CASE WHEN s.baseline_rt_ms > 0 THEN a.latency_ms::numeric / s.baseline_rt_ms END
                            AS latency_vs_baseline,
  a.confidence,
  CASE WHEN a.confidence IS NULL THEN NULL
       ELSE round(power(a.confidence / 100.0
                        - CASE WHEN a.outcome = 'correct' THEN 1 ELSE 0 END, 2), 4)
  END                       AS brier,
  r.outcome                 AS recall_outcome,
  (r.outcome = 'untransformed') AS recall_executive_failure,
  r.digits_held,
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
LEFT JOIN dt_recalls r ON r.session_id = a.session_id AND r.block_no = a.block_no
LEFT JOIN dt_block_reviews br ON br.session_id = a.session_id AND br.block_no = a.block_no;

COMMENT ON VIEW v_dt_answers IS
  'One row per decision test question presented. The grain, the export, and the base of every other dt view. Carries all three identity observations, both readings of the sequence, the curation status and the single `usable` definition. No name, no email, no chosen_index and no curator note.';

-- +goose StatementEnd

-- +goose StatementBegin

CREATE VIEW v_dt_blocks AS
SELECT
  session_key,
  is_synthetic,
  status,
  attempt_no,
  attempt_no_strongest,
  attempt_source,
  attempt_sources_disagree,
  prior_by_account,
  prior_by_email,
  prior_by_cookie,
  session_review_status,
  block_review_status,
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
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))  AS accuracy_pct,
  round(100.0 * count(*) FILTER (WHERE is_expired) / count(*))  AS expiry_pct,
  round(avg(confidence))                          AS mean_confidence,
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
GROUP BY session_key, is_synthetic, status, attempt_no, attempt_no_strongest,
         attempt_source, attempt_sources_disagree, prior_by_account,
         prior_by_email, prior_by_cookie, session_review_status,
         block_review_status, instrument_version, item_set_version,
         block_no, block_load, load_rank;

COMMENT ON VIEW v_dt_blocks IS
  'One row per decision test session per block, with its attempt sequence and curation status. The review surface, so nothing is filtered out: an excluded block still appears, labelled.';

-- +goose StatementEnd

-- +goose StatementBegin

CREATE VIEW v_dt_sessions AS
WITH blocks AS (SELECT * FROM v_dt_blocks)
SELECT
  b.session_key,
  b.is_synthetic,
  b.status,
  b.attempt_no,
  b.attempt_no_strongest,
  b.attempt_source,
  b.attempt_sources_disagree,
  b.prior_by_account,
  b.prior_by_email,
  b.prior_by_cookie,
  b.session_review_status,
  b.instrument_version,
  b.item_set_version,
  min(b.started_at)                               AS started_at,
  count(DISTINCT b.block_no)                      AS blocks_reached,
  count(DISTINCT b.block_no) FILTER (WHERE b.block_review_status = 'do_not_use')
                                                  AS blocks_excluded,
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
  round(avg(b.accuracy_pct)      FILTER (WHERE b.block_no IN (1, 2))) AS early_accuracy_pct,
  round(avg(b.mean_confidence)   FILTER (WHERE b.block_no IN (1, 2))) AS early_confidence_pct,
  round(avg(b.accuracy_pct)      FILTER (WHERE b.block_no = 4))       AS hard_accuracy_pct,
  round(avg(b.mean_confidence)   FILTER (WHERE b.block_no = 4))       AS hard_confidence_pct,
  round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 1))            AS block1_accuracy_pct,
  round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 5))            AS block5_accuracy_pct,
  round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 5))
    - round(avg(b.accuracy_pct) FILTER (WHERE b.block_no = 1))        AS fatigue_delta_pct,
  round(avg(b.memory_failure), 2)                 AS mean_memory_failure,
  count(*) FILTER (WHERE b.recall_executive_failure) AS untransformed_blocks
FROM blocks b
GROUP BY b.session_key, b.is_synthetic, b.status, b.attempt_no,
         b.attempt_no_strongest, b.attempt_source, b.attempt_sources_disagree,
         b.prior_by_account, b.prior_by_email, b.prior_by_cookie,
         b.session_review_status, b.instrument_version, b.item_set_version;

COMMENT ON VIEW v_dt_sessions IS
  'One row per decision test run, with both readings of its attempt sequence, its curation status and how many blocks are excluded. The review surface: includes synthetic, unfinished and excluded runs, labelled rather than filtered.';

-- +goose StatementEnd

-- +goose StatementBegin

CREATE VIEW v_dt_load_curve AS
SELECT
  item_set_version,
  instrument_version,
  block_load,
  load_rank,
  count(DISTINCT session_key)                     AS sessions,
  count(DISTINCT session_key) FILTER (WHERE session_review_status <> '')
                                                  AS sessions_reviewed,
  count(DISTINCT session_key) FILTER (WHERE session_review_status = '')
                                                  AS sessions_unreviewed,
  count(DISTINCT session_key) FILTER (WHERE attempt_no = 1)
                                                  AS sessions_first_attempt,
  count(DISTINCT session_key) FILTER (WHERE attempt_no > 1)
                                                  AS sessions_repeat,
  count(DISTINCT session_key) FILTER (WHERE attempt_no IS NULL)
                                                  AS sessions_attempt_unknown,
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
  (SELECT string_agg(DISTINCT f.item_family, '+' ORDER BY f.item_family)
     FROM v_dt_answers f
    WHERE f.item_set_version = v.item_set_version
      AND f.block_load = v.block_load
      AND f.usable)                               AS families
FROM v_dt_answers v
WHERE usable
GROUP BY item_set_version, instrument_version, block_load, load_rank
ORDER BY item_set_version, instrument_version, load_rank, block_load;

COMMENT ON VIEW v_dt_load_curve IS
  'Accuracy and confidence by load level: the instrument''s result. Filters on `usable` only. Repeats are counted rather than excluded, because a practice effect is the reviewer''s call and a figure that hides its own composition cannot be quoted safely.';

CREATE VIEW v_dt_calibration AS
SELECT
  item_set_version,
  instrument_version,
  block_load,
  load_rank,
  (confidence / 10) * 10                          AS confidence_band,
  count(*)                                        AS answers,
  count(DISTINCT session_key) FILTER (WHERE session_review_status = '')
                                                  AS sessions_unreviewed,
  count(DISTINCT session_key) FILTER (WHERE attempt_no > 1)
                                                  AS sessions_repeat,
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))        AS accuracy_pct,
  ((confidence / 10) * 10 + 5)
    - round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))    AS overclaim_pct,
  round(avg(brier), 4)                            AS mean_brier
FROM v_dt_answers
WHERE usable AND confidence IS NOT NULL
GROUP BY item_set_version, instrument_version, block_load, load_rank, (confidence / 10) * 10
ORDER BY item_set_version, instrument_version, load_rank, block_load, confidence_band;

COMMENT ON VIEW v_dt_calibration IS
  'Confidence bands of ten against the accuracy achieved in each, split by load and instrument. Filters on `usable`; repeats counted, not removed.';

CREATE VIEW v_dt_items AS
SELECT
  item_code,
  item_version,
  item_family,
  count(*)                                        AS answers,
  count(DISTINCT session_key)                     AS sessions,
  count(DISTINCT session_key) FILTER (WHERE session_review_status = '')
                                                  AS sessions_unreviewed,
  count(DISTINCT session_key) FILTER (WHERE attempt_no > 1)
                                                  AS sessions_repeat,
  round(100.0 * count(*) FILTER (WHERE is_correct) / count(*))        AS accuracy_pct,
  round(100.0 * count(*) FILTER (WHERE is_lure) / count(*))           AS lure_pct,
  round(100.0 * count(*) FILTER (WHERE is_expired) / count(*))        AS expiry_pct,
  round(avg(confidence))                          AS mean_confidence,
  round(avg(confidence) - 100.0 * count(*) FILTER (WHERE is_correct) / count(*))
                                                  AS gap_pct,
  round(avg(brier), 4)                            AS mean_brier,
  round(avg(latency_ms))                          AS mean_latency_ms,
  array_agg(DISTINCT block_no ORDER BY block_no)  AS block_nos,
  array_agg(DISTINCT item_set_version)            AS item_set_versions
FROM v_dt_answers
WHERE usable
GROUP BY item_code, item_version, item_family
ORDER BY item_code;

COMMENT ON VIEW v_dt_items IS
  'Per item: accuracy, how often the lure pulled, expiry rate and the confidence gap. Filters on `usable`. A lure_pct near zero is an item measuring nothing.';

CREATE VIEW v_dt_threshold AS
SELECT
  s.session_key,
  s.is_synthetic,
  s.status,
  s.attempt_no,
  s.attempt_no_strongest,
  s.attempt_source,
  s.attempt_sources_disagree,
  s.session_review_status,
  s.accuracy_pct,
  s.gap_pct,
  s.block5_accuracy_pct,
  s.block1_accuracy_pct,
  s.fatigue_delta_pct,
  s.blocks_excluded,
  (SELECT min(a.load_rank) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.confidently_lured
      AND a.block_no < 5 AND a.usable)                            AS lured_at_rank,
  (SELECT min(a.block_load) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.confidently_lured
      AND a.block_no < 5 AND a.usable
      AND a.load_rank = (SELECT min(b.load_rank) FROM v_dt_answers b
                          WHERE b.session_key = s.session_key AND b.confidently_lured
                            AND b.block_no < 5 AND b.usable))     AS lured_at_load,
  (SELECT min(a.load_rank) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.confidently_wrong
      AND a.block_no < 5 AND a.usable)                            AS wrong_at_rank,
  (SELECT bool_or(a.confidently_lured) FROM v_dt_answers a
    WHERE a.session_key = s.session_key AND a.block_no = 5 AND a.usable)
                                                                  AS lured_in_control,
  s.confidently_lured,
  s.confidently_wrong,
  s.started_at
FROM v_dt_sessions s;

COMMENT ON VIEW v_dt_threshold IS
  'Per run, the lowest load on the ramp (blocks 1 to 4) at which this participant took the intended wrong answer while still reporting high confidence. Carries both readings of attempt_no, because a threshold from a fourth sitting is not the measurement a first sitting gives.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- A no-op, as 00056 and 00057 explain: the views would need a second
-- copy of forty columns that would then drift, and production only ever
-- goes up. The functions are additive and cost nothing left in place.
SELECT 1;
-- +goose StatementEnd
