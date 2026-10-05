-- +goose Up
-- +goose StatementBegin

-- Which sitting this is for this person, recorded as the observations
-- rather than as a conclusion.
--
-- Asked for by the owner on 2026-10-05: people may take the test as
-- often as they like, and every sitting must be labelled with its place
-- in the sequence. `is_repeat` is a boolean, so attempt 2 and attempt 7
-- were indistinguishable, and a practice effect is a curve rather than a
-- flag.
--
-- **Three identities can each see a different number of prior sittings,
-- and all three counts are stored.** An account is exact. An email is
-- near-exact. A cookie is defeated by a private window, another device
-- or cleared storage. They disagree in both directions: somebody who sat
-- it anonymously and later signed up has 0 priors on their account and 1
-- on their cookie; somebody who cleared their cookie has the reverse.
--
-- The first version of this migration stored a single attempt_no taken
-- from whichever identity reported the most, with a label saying which
-- one that was. The owner rejected it, correctly, and for a better
-- reason than the one I would have given:
--
--   * It collapsed three observations into one at write time and threw
--     the components away, so nobody downstream could see that the
--     account said 0 and the cookie said 1. The combination rule became
--     a property of the record instead of a choice the analysis makes.
--   * Taking the maximum biases the number upward whenever the sources
--     disagree, and that bias is not random: it tracks how much identity
--     a participant handed over, so people who gave an email would read
--     as repeaters more often than people who did not. A systematically
--     biased label is worse than a noisy one, because averaging does not
--     remove it.
--
-- So the counts are the data and the sequence is derived in
-- v_dt_answers, where the rule is one line the owner can change without
-- a migration and without losing anything.
--
-- The counts must be stored rather than recomputed later: they are
-- point-in-time. Running the same query next month would include
-- sittings that had not happened yet.
--
-- NULL and 0 are different facts. NULL means there is no such link at
-- all, so that identity has nothing to say. 0 means the link exists and
-- reports no prior sitting, which is positive evidence of a first
-- attempt.
--
-- Two decisions from the owner shape the rest:
--
--   * **The participant is never told.** Somebody who knows they are
--     tracked across attempts behaves differently on the test.
--   * **Nothing is filtered by attempt.** "I will make that call when I
--     review it." The claim-making views report how many repeats are
--     inside a figure; they do not remove them.

ALTER TABLE dt_sessions
  ADD COLUMN IF NOT EXISTS prior_by_account int,
  ADD COLUMN IF NOT EXISTS prior_by_email   int,
  ADD COLUMN IF NOT EXISTS prior_by_cookie  int;

ALTER TABLE dt_sessions
  DROP CONSTRAINT IF EXISTS dt_sessions_prior_counts_ck,
  ADD  CONSTRAINT dt_sessions_prior_counts_ck
       CHECK (coalesce(prior_by_account, 0) >= 0
          AND coalesce(prior_by_email, 0) >= 0
          AND coalesce(prior_by_cookie, 0) >= 0);

COMMENT ON COLUMN dt_sessions.prior_by_account IS
  'Prior sittings visible to this participant''s account when this session started. NULL means there was no account to ask, which is different from 0: the latter is positive evidence of a first attempt. Point-in-time and not recomputable later.';
COMMENT ON COLUMN dt_sessions.prior_by_email IS
  'Prior sittings visible to this participant''s email address when this session started. NULL means no address was given. Near-exact where present.';
COMMENT ON COLUMN dt_sessions.prior_by_cookie IS
  'Prior sittings visible to the first-party anonymous id when this session started. NULL means no cookie. Defeated by a private window, another device or cleared storage, so a 0 here is weak evidence of a first attempt.';

COMMENT ON COLUMN dt_sessions.is_repeat IS
  'True when ANY identity reported a prior sitting. An OR across the three prior_by_* counts, not a selection among them, so it can be true while the view''s derived attempt_no reads 1: that happens when the account saw no prior sitting and the cookie saw one, which means somebody sat the test anonymously and later signed up. The counts are the data; this is a summary kept because it is already written and read.';

-- is_repeat stays and is still written, so the index that already
-- serves it keeps working. The derived attempt number lives in the view
-- and is not indexed: with the volumes this instrument will ever see,
-- an index on a derived column would be ceremony.

-- Backfill: what each identity would have reported at the time, for
-- the sessions that already exist.
--
-- Computed per source independently, with a window counting only
-- earlier sessions, which is what makes it point-in-time rather than a
-- count of everything now. NULL where the row has no such link, so the
-- backfilled rows carry the same NULL-versus-zero distinction as new
-- ones rather than pretending to knowledge they never had.
--
-- Partitioned on is_synthetic as well, because an agent run must not
-- bump a real participant's count and a real run must not inherit one
-- from an agent that happened to share a browser. The previous
-- detection did not separate them: a latent fault that had not fired
-- only because no synthetic run shared a visitor_key with a real one.
WITH counted AS (
  SELECT s.id,
         CASE WHEN p.user_id IS NOT NULL THEN
           count(*) FILTER (WHERE true) OVER (
             PARTITION BY s.is_synthetic, p.user_id
             ORDER BY s.id ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING)
         END AS by_account,
         CASE WHEN coalesce(p.email_key, '') <> '' THEN
           count(*) FILTER (WHERE true) OVER (
             PARTITION BY s.is_synthetic, p.email_key
             ORDER BY s.id ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING)
         END AS by_email,
         CASE WHEN s.visitor_key <> '' THEN
           count(*) FILTER (WHERE true) OVER (
             PARTITION BY s.is_synthetic, s.visitor_key
             ORDER BY s.id ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING)
         END AS by_cookie
    FROM dt_sessions s
    LEFT JOIN dt_participants p ON p.id = s.participant_id
)
UPDATE dt_sessions s
   SET prior_by_account = counted.by_account,
       prior_by_email   = counted.by_email,
       prior_by_cookie  = counted.by_cookie
  FROM counted
 WHERE counted.id = s.id;

-- +goose StatementEnd

-- +goose StatementBegin

-- The views carry the attempt so it reaches the export and the console,
-- and the curves report how many repeats are inside a figure rather
-- than removing them. Rebuilt deepest-last, as 00054 established; no
-- CASCADE, so a view added after this file cannot be taken silently.
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
  -- The three observations, so no consumer has to accept somebody
  -- else's combination of them.
  s.prior_by_account,
  s.prior_by_email,
  s.prior_by_cookie,
  -- A derived convenience, not an authority. It reads the most
  -- RELIABLE source that has anything to say, in an order declared in
  -- advance (account, then email, then cookie), rather than whichever
  -- source happens to report the largest number.
  --
  -- That distinction is the whole point. Choosing by a fixed
  -- reliability ordering is a method. Choosing by which figure is
  -- biggest is selection on the outcome, it biases the result upward
  -- whenever the sources disagree, and the bias tracks how much
  -- identity a participant handed over.
  --
  -- NULL means no identity link at all, so the sequence is genuinely
  -- unknown rather than one. Consumers must not coalesce that to 1.
  coalesce(s.prior_by_account, s.prior_by_email, s.prior_by_cookie) + 1
                            AS attempt_no,
  CASE
    WHEN s.prior_by_account IS NOT NULL THEN 'account'
    WHEN s.prior_by_email   IS NOT NULL THEN 'email'
    WHEN s.prior_by_cookie  IS NOT NULL THEN 'cookie'
    ELSE 'none'
  END                       AS attempt_source,
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
  'One row per decision test question presented. The grain, the export, and the base of every other dt view. Carries the attempt number with the basis that number rests on, the curation status, and the single `usable` definition every claim-making view filters on. No name, no email, no chosen_index and no curator note.';

-- +goose StatementEnd

-- +goose StatementBegin

CREATE VIEW v_dt_blocks AS
SELECT
  session_key,
  is_synthetic,
  status,
  attempt_no,
  attempt_source,
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
GROUP BY session_key, is_synthetic, status, attempt_no, attempt_source,
         prior_by_account, prior_by_email, prior_by_cookie,
         session_review_status, block_review_status, instrument_version,
         item_set_version, block_no, block_load, load_rank;

COMMENT ON VIEW v_dt_blocks IS
  'One row per decision test session per block, with its attempt number and curation status. The review surface, so nothing is filtered out: an excluded block still appears here, labelled.';

-- +goose StatementEnd

-- +goose StatementBegin

CREATE VIEW v_dt_sessions AS
WITH blocks AS (SELECT * FROM v_dt_blocks)
SELECT
  b.session_key,
  b.is_synthetic,
  b.status,
  b.attempt_no,
  b.attempt_source,
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
GROUP BY b.session_key, b.is_synthetic, b.status, b.attempt_no, b.attempt_source,
         b.prior_by_account, b.prior_by_email, b.prior_by_cookie,
         b.session_review_status, b.instrument_version, b.item_set_version;

COMMENT ON VIEW v_dt_sessions IS
  'One row per decision test run, with its attempt number, curation status and how many of its blocks are excluded. The review surface: includes synthetic, unfinished and excluded runs, labelled rather than filtered.';

-- +goose StatementEnd

-- +goose StatementBegin

-- Repeats are reported, not removed. The owner will decide per
-- analysis whether to cut to first attempts; the view's job is to make
-- sure he cannot quote a figure without seeing what is in it.
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
  -- Neither, because no identity could place the sitting in a
  -- sequence. Counted rather than dropped: a figure built largely from
  -- runs whose history is unknown is a different claim from one built
  -- from known first attempts.
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
  'Accuracy and confidence by load level: the instrument''s result. Filters on `usable` only. Repeats are counted in sessions_repeat rather than excluded, because a practice effect is the reviewer''s call and a figure that hides its own composition cannot be quoted safely.';

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
  'Per item: accuracy, how often the lure pulled, expiry rate and the confidence gap. Filters on `usable`. A lure_pct near zero is an item measuring nothing. sessions_repeat warns that an item somebody has seen before is not measuring the same thing.';

CREATE VIEW v_dt_threshold AS
SELECT
  s.session_key,
  s.is_synthetic,
  s.status,
  s.attempt_no,
  s.attempt_source,
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
  'Per run, the lowest load on the ramp (blocks 1 to 4) at which this participant took the intended wrong answer while still reporting high confidence. Carries attempt_no, because a threshold from somebody''s fourth sitting is not the same measurement as one from their first.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- A no-op, for the reason 00056 gives: the views join tables this
-- migration does not create, reverting them would mean a second copy of
-- forty columns that would drift, and production only ever goes up.
SELECT 1;
-- +goose StatementEnd
