-- Fixtures for the decision test metric views (migration 00054).
--
-- Four runs with deliberately different shapes. Loaded by CI after the
-- migration replay, then checked by dt_metric_assertions.sql.
--
-- These exist because an empty database proves a view parses and
-- nothing more. Every mistake worth catching in these views is a
-- mistake about what the numbers MEAN, and that is invisible until real
-- shapes go through them. One already got through: v_dt_threshold
-- originally took its minimum over all five blocks, which reported a
-- participant who came apart on the control block as having a threshold
-- at the easiest load. It parsed perfectly and said the opposite of the
-- truth, for exactly the participants the control exists to detect.
--
--   alice  the effect. Accuracy falls across blocks 1 to 4 while
--          confidence holds; block 5 recovers. Takes the lure
--          confidently from block 3 on. Her block 3 recall is held
--          perfectly and never transformed.
--   bob    calibrated. Accuracy falls and confidence falls with it, so
--          the gap stays near zero and no lure is taken confidently.
--          He is the negative control: a view that finds an effect in
--          bob is finding it in noise.
--   cara   fatigued. Same ramp as alice, but block 5 collapses too.
--          The reading that means time-on-task, not load.
--   agent  synthetic. Perfect accuracy at maximum confidence. Every
--          aggregate view must exclude it; one that does not will be
--          flattered by it in exactly the direction that looks like
--          success.
--
-- No real participant data. The addresses are .test, which is reserved.

INSERT INTO dt_participants (id, display_name, age_range, education, occupation, email, email_key, wants_results)
VALUES (101, 'Alice', '35 to 44', 'Bachelor''s Degree', 'Engineer', 'alice@example.test', 'alice@example.test', true),
       (102, 'Bob',   '45 to 54', 'Master''s Degree',   'Analyst',  '', '', false),
       (103, 'Cara',  '25 to 34', 'High School',        'Operator', 'cara@example.test', 'cara@example.test', true),
       (104, 'Agent', '', '', '', '', '', false);

INSERT INTO dt_sessions (id, participant_id, instrument_version, item_set_version, key_version,
                         status, audio_mode, device_class, tap_check_passed, baseline_rt_ms, baseline_rt_sd_ms,
                         is_repeat, repeat_matched_by, visitor_key, is_synthetic, recall_strategy,
                         prior_by_account, prior_by_email, prior_by_cookie,
                         started_at, finished_at)
VALUES (201, 101, 'v2-m6000-q25000-r20000', 'items-2026-10-05', 'key-1', 'completed', 'sound',  'desktop', true, 300, 40, false, '', 'va', false, 'encode', NULL, 0, 0, now() - interval '2 hours', now() - interval '105 minutes'),
       (202, 102, 'v2-m6000-q25000-r20000', 'items-2026-10-05', 'key-1', 'completed', 'sound',  'desktop', true, 280, 35, false, '', 'vb', false, 'defer',  NULL, NULL, 0, now() - interval '3 hours', now() - interval '165 minutes'),
       (203, 103, 'v2-m6000-q25000-r20000', 'items-2026-10-05', 'key-1', 'completed', 'visual', 'phone',   true, 420, 90, false, '', 'vc', false, 'encode', NULL, 0, 0, now() - interval '4 hours', now() - interval '225 minutes'),
       (204, 104, 'v2-m6000-q25000-r20000', 'items-2026-10-05', 'key-1', 'completed', 'sound',  'desktop', true, 100, 5,  false, '', 'vd', true,  'encode', NULL, NULL, 0, now() - interval '5 hours', now() - interval '290 minutes');

-- Answers: six per block, five blocks, four runs.
--
-- Generated from a per-block target rather than written out, so the
-- intended shape is readable as a table and a change to it is one
-- number. The first n_correct positions in a block are correct; the
-- rest take the lure where the run is meant to be lured and are 'other'
-- otherwise. Items are joined by presentation position, so this follows
-- the real composition (migration 00052) rather than inventing one.
INSERT INTO dt_answers (session_id, item_id, item_code, item_version, block_no, block_load,
                        position_in_block, position_overall, outcome, chosen_index,
                        latency_ms, confidence)
SELECT
  t.session_id,
  i.id,
  i.code,
  i.version,
  t.block_no,
  CASE t.block_no WHEN 1 THEN 'd3' WHEN 2 THEN 'd4' WHEN 3 THEN 'd4_plus1'
                  WHEN 4 THEN 'd4_plus3' ELSE 'd3_control' END,
  pos,
  (t.block_no - 1) * 6 + pos,
  CASE WHEN pos <= t.n_correct THEN 'correct'
       WHEN t.lures            THEN 'lure'
       ELSE 'other' END,
  0,
  t.latency,
  t.confidence
FROM (
  VALUES
    -- session, block, correct of 6, confidence, takes the lure, latency
    (201, 1, 5, 78, false,  9000),
    (201, 2, 4, 77, false, 11000),
    (201, 3, 2, 76, true,  15000),
    (201, 4, 1, 74, true,  18000),
    (201, 5, 5, 75, false, 10000),
    (202, 1, 5, 72, false,  8000),
    (202, 2, 4, 64, false, 10000),
    (202, 3, 3, 52, false, 13000),
    (202, 4, 2, 38, false, 16000),
    (202, 5, 5, 70, false,  9000),
    (203, 1, 5, 80, false, 12000),
    (203, 2, 4, 78, false, 14000),
    (203, 3, 2, 77, true,  19000),
    (203, 4, 1, 76, true,  22000),
    (203, 5, 1, 75, true,  21000),
    (204, 1, 6, 99, false,   500),
    (204, 2, 6, 99, false,   500),
    (204, 3, 6, 99, false,   500),
    (204, 4, 6, 99, false,   500),
    (204, 5, 6, 99, false,   500)
) AS t(session_id, block_no, n_correct, confidence, lures, latency)
CROSS JOIN generate_series(1, 6) AS pos
JOIN LATERAL (
  SELECT id, code, version FROM dt_items
   WHERE kind = 'scored' AND position = (t.block_no - 1) * 6 + pos
   LIMIT 1
) AS i ON true;

-- Recalls: one per block per run. Four cases carry the weight.
--
--   alice block 3  held perfectly, never transformed. memory_failure
--                  must be 0.00 and recall_executive_failure true. This
--                  is the case that proves severity is scored against
--                  retention rather than against correctness.
--   alice block 4  matched neither number. Genuine partial loss.
--   cara  block 5  expired. memory_failure must be NULL, never 1.00:
--                  nothing was attempted, and "ran out of time" is not
--                  "forgot completely".
--   bob   block 3+ transformed correctly under load, which is what a
--                  clean executive looks like.
INSERT INTO dt_recalls (session_id, block_no, block_load, presented_digits, expected_digits,
                        response_digits, outcome, digits_correct, digits_held, latency_ms)
VALUES
  (201, 1, 'd3',         '482',  '482',  '482',  'exact',          3, 3,  4000),
  (201, 2, 'd4',         '7193', '7193', '7183', 'partial',        3, 3,  6000),
  (201, 3, 'd4_plus1',   '3777', '4888', '3777', 'untransformed',  0, 4,  7000),
  (201, 4, 'd4_plus3',   '8762', '1095', '8754', 'wrong_digits',   0, 2,  9000),
  (201, 5, 'd3_control', '106',  '106',  '106',  'exact',          3, 3,  3500),
  (202, 1, 'd3',         '531',  '531',  '531',  'exact',          3, 3,  3800),
  (202, 2, 'd4',         '2048', '2048', '2048', 'exact',          4, 4,  5200),
  (202, 3, 'd4_plus1',   '4216', '5327', '5327', 'exact',          4, 4,  8000),
  (202, 4, 'd4_plus3',   '9031', '2364', '2364', 'exact',          4, 4, 11000),
  (202, 5, 'd3_control', '774',  '774',  '774',  'exact',          3, 3,  3600),
  (203, 1, 'd3',         '615',  '615',  '615',  'exact',          3, 3,  5000),
  (203, 2, 'd4',         '3840', '3840', '3480', 'partial',        2, 2,  7000),
  (203, 3, 'd4_plus1',   '5921', '6032', '5921', 'untransformed',  0, 4,  9000),
  (203, 4, 'd4_plus3',   '1478', '4701', '1000', 'wrong_digits',   0, 1, 12000),
  (203, 5, 'd3_control', '392',  '392',  '',     'expired',        0, 0,  NULL),
  (204, 1, 'd3',         '111',  '111',  '111',  'exact',          3, 3,   800),
  (204, 2, 'd4',         '2222', '2222', '2222', 'exact',          4, 4,   900),
  (204, 3, 'd4_plus1',   '3333', '4444', '4444', 'exact',          4, 4,   900),
  (204, 4, 'd4_plus3',   '5555', '8888', '8888', 'exact',          4, 4,   900),
  (204, 5, 'd3_control', '666',  '666',  '666',  'exact',          3, 3,   800);

-- One more run, on a DIFFERENT item set, to keep the curves from
-- pooling across instruments again.
--
-- This is the shape that produced the first real reading of
-- v_dt_load_curve on production: accuracy RISING to 100% at the hardest
-- load, because the two runs in the database were taken on the
-- family-blocked composition where item family swamped load. The view
-- was correct and the claim it made was worthless, because it grouped
-- by block_load alone and averaged every run regardless of which items
-- it saw.
--
-- So: a run with the ramp inverted, tagged as an older item set. If the
-- curves ever stop grouping by version, this run's 100% at d4_plus3
-- will drag the current item set's figures and the assertions below
-- will catch it.
INSERT INTO dt_participants (id, display_name, age_range, education, occupation, email, email_key, wants_results)
VALUES (105, 'Dev', '55 to 64', 'Associate Degree', 'Technician', '', '', false);

INSERT INTO dt_sessions (id, participant_id, instrument_version, item_set_version, key_version,
                         status, audio_mode, device_class, tap_check_passed, baseline_rt_ms, baseline_rt_sd_ms,
                         is_repeat, repeat_matched_by, visitor_key, is_synthetic, recall_strategy,
                         prior_by_account, prior_by_email, prior_by_cookie,
                         started_at, finished_at)
VALUES (205, 105, 'v1', 'items-2026-10-04', 'key-1', 'completed', 'sound', 'desktop', true, 310, 45,
        false, '', 've', false, 'encode', NULL, NULL, 0,
        now() - interval '8 hours', now() - interval '470 minutes');

INSERT INTO dt_answers (session_id, item_id, item_code, item_version, block_no, block_load,
                        position_in_block, position_overall, outcome, chosen_index,
                        latency_ms, confidence)
SELECT
  t.session_id, i.id, i.code, i.version, t.block_no,
  CASE t.block_no WHEN 1 THEN 'd3' WHEN 2 THEN 'd4' WHEN 3 THEN 'd4_plus1'
                  WHEN 4 THEN 'd4_plus3' ELSE 'd3_control' END,
  pos, (t.block_no - 1) * 6 + pos,
  CASE WHEN pos <= t.n_correct THEN 'correct' ELSE 'other' END,
  0, t.latency, t.confidence
FROM (
  VALUES
    -- The ramp backwards: the old composition made the hardest block
    -- the easiest items.
    (205, 1, 2, 88, 9000),
    (205, 2, 3, 88, 9000),
    (205, 3, 2, 88, 9000),
    (205, 4, 6, 88, 9000),
    (205, 5, 6, 88, 9000)
) AS t(session_id, block_no, n_correct, confidence, latency)
CROSS JOIN generate_series(1, 6) AS pos
JOIN LATERAL (
  SELECT id, code, version FROM dt_items
   WHERE kind = 'scored' AND position = (t.block_no - 1) * 6 + pos
   LIMIT 1
) AS i ON true;

INSERT INTO dt_recalls (session_id, block_no, block_load, presented_digits, expected_digits,
                        response_digits, outcome, digits_correct, digits_held, latency_ms)
VALUES
  (205, 1, 'd3',         '271',  '271',  '271',  'exact', 3, 3, 4100),
  (205, 2, 'd4',         '8305', '8305', '8305', 'exact', 4, 4, 5400),
  (205, 3, 'd4_plus1',   '6194', '7205', '7205', 'exact', 4, 4, 8200),
  (205, 4, 'd4_plus3',   '2748', '5071', '5071', 'exact', 4, 4, 9900),
  (205, 5, 'd3_control', '913',  '913',  '913',  'exact', 3, 3, 3900);

-- Repeat attempts, for the attempt-numbering assertions.
--
-- Alice sits the test twice more. The second is matched by email, the
-- third by cookie with no email given, which is the weaker link and has
-- to say so. The shapes do not matter here; the sequencing does.
INSERT INTO dt_participants (id, display_name, age_range, education, occupation, email, email_key, wants_results)
VALUES (106, 'Alice', '35 to 44', 'Bachelor''s Degree', 'Engineer', 'alice@example.test', 'alice@example.test', false),
       (107, '',      '',         '',                   '',         '', '', false);

INSERT INTO dt_sessions (id, participant_id, instrument_version, item_set_version, key_version,
                         status, audio_mode, device_class, tap_check_passed, baseline_rt_ms, baseline_rt_sd_ms,
                         is_repeat, repeat_matched_by, visitor_key, is_synthetic, recall_strategy,
                         prior_by_account, prior_by_email, prior_by_cookie,
                         started_at, finished_at)
VALUES
  -- Alice's second sitting. Her email saw the first, and so did the
  -- browser. The two agree, which is the easy case.
  (206, 106, 'v2-m6000-q25000-r20000', 'items-2026-10-05', 'key-1', 'completed', 'sound', 'desktop', true, 300, 40,
   true, 'email', 'va', false, 'encode', NULL, 1, 1, now() - interval '90 minutes', now() - interval '75 minutes'),
  -- Her third, taken without giving an address. Only the browser can
  -- place it, and it reports two priors. attempt_no derives to 3 from
  -- the cookie because no stronger source exists, and attempt_source
  -- says 'cookie' so nobody mistakes that for an exact sequence.
  (207, 107, 'v2-m6000-q25000-r20000', 'items-2026-10-05', 'key-1', 'completed', 'sound', 'desktop', true, 300, 40,
   true, 'cookie', 'va', false, 'encode', NULL, NULL, 2, now() - interval '60 minutes', now() - interval '45 minutes');

INSERT INTO dt_answers (session_id, item_id, item_code, item_version, block_no, block_load,
                        position_in_block, position_overall, outcome, chosen_index,
                        latency_ms, confidence)
SELECT t.session_id, i.id, i.code, i.version, t.block_no,
  CASE t.block_no WHEN 1 THEN 'd3' WHEN 2 THEN 'd4' WHEN 3 THEN 'd4_plus1'
                  WHEN 4 THEN 'd4_plus3' ELSE 'd3_control' END,
  pos, (t.block_no - 1) * 6 + pos,
  CASE WHEN pos <= t.n_correct THEN 'correct' ELSE 'other' END,
  0, 9000, t.confidence
FROM (
  VALUES
    -- A practice effect: both repeats do better than the first sitting.
    (206, 1, 6, 70), (206, 2, 5, 70), (206, 3, 4, 70), (206, 4, 3, 70), (206, 5, 6, 70),
    (207, 1, 6, 68), (207, 2, 6, 68), (207, 3, 5, 68), (207, 4, 4, 68), (207, 5, 6, 68)
) AS t(session_id, block_no, n_correct, confidence)
CROSS JOIN generate_series(1, 6) AS pos
JOIN LATERAL (
  SELECT id, code, version FROM dt_items
   WHERE kind = 'scored' AND position = (t.block_no - 1) * 6 + pos
   LIMIT 1
) AS i ON true;

INSERT INTO dt_recalls (session_id, block_no, block_load, presented_digits, expected_digits,
                        response_digits, outcome, digits_correct, digits_held, latency_ms)
SELECT t.session_id, t.block_no,
       CASE t.block_no WHEN 1 THEN 'd3' WHEN 2 THEN 'd4' WHEN 3 THEN 'd4_plus1'
                       WHEN 4 THEN 'd4_plus3' ELSE 'd3_control' END,
       t.d, t.d, t.d, 'exact', length(t.d), length(t.d), 4000
FROM (VALUES
  (206,1,'111'),(206,2,'1111'),(206,3,'1111'),(206,4,'1111'),(206,5,'111'),
  (207,1,'222'),(207,2,'2222'),(207,3,'2222'),(207,4,'2222'),(207,5,'222')
) AS t(session_id, block_no, d);
