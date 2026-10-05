-- What the decision test's metric views must say about dt_fixture.sql.
--
-- Run by CI straight after loading the fixture. Each block raises on
-- failure, so psql with ON_ERROR_STOP turns a wrong number into a red
-- build. The messages name the measurement rather than the expression,
-- because the person reading the failure needs to know which claim
-- stopped being true, not which line of SQL moved.
--
-- These are the assertions that catch a view which parses and lies.
-- Every one of them is here because getting it wrong would be either
-- invisible or actively misleading, and most of them would read as a
-- successful result.

-- +----------------------------------------------------------------+
-- | Synthetic runs stay out of the aggregates.                     |
-- +----------------------------------------------------------------+
-- The agent run is perfect at maximum confidence. Including it raises
-- accuracy, lowers the gap and shrinks the Brier score: it moves every
-- headline number in the direction that looks like the instrument
-- working. A caveat in a comment would not be enough.
DO $$
DECLARE n int;
BEGIN
  SELECT max(sessions) INTO n FROM v_dt_load_curve;
  IF n <> 3 THEN
    RAISE EXCEPTION 'v_dt_load_curve counts % sessions per load, expected 3: a synthetic run is being averaged into the result', n;
  END IF;

  -- Per item, not per load: an item is served once per run, so four
  -- sessions on any item means the agent is in there. Accuracy cannot
  -- be used for this, since an easy item legitimately reaches 100%
  -- across the three real runs.
  SELECT max(sessions) INTO n FROM v_dt_items;
  IF n <> 3 THEN
    RAISE EXCEPTION 'v_dt_items counts % sessions on an item, expected 3: a synthetic run is in the item statistics', n;
  END IF;

  SELECT count(*) INTO n FROM v_dt_calibration WHERE confidence_band >= 90;
  IF n > 0 THEN
    RAISE EXCEPTION 'v_dt_calibration has a 90+ confidence band, which only the synthetic run produces';
  END IF;
END $$;

-- +----------------------------------------------------------------+
-- | The load curve goes the way the instrument claims.             |
-- +----------------------------------------------------------------+
-- Accuracy down and the gap up as the ramp rises. If this ever fails it
-- is either the view or the composition, and on 2026-10-04 it was the
-- composition: migration 00051's comment said the families were
-- interleaved and its insert order did the opposite, which made
-- accuracy appear to RISE with load.
DO $$
DECLARE a1 numeric; a4 numeric; g1 numeric; g4 numeric;
BEGIN
  SELECT accuracy_pct, gap_pct INTO a1, g1 FROM v_dt_load_curve WHERE block_load = 'd3';
  SELECT accuracy_pct, gap_pct INTO a4, g4 FROM v_dt_load_curve WHERE block_load = 'd4_plus3';
  IF a4 >= a1 THEN
    RAISE EXCEPTION 'accuracy at the hardest load (%) is not below the easiest (%)', a4, a1;
  END IF;
  IF g4 <= g1 THEN
    RAISE EXCEPTION 'the confidence gap at the hardest load (%) is not above the easiest (%)', g4, g1;
  END IF;
END $$;

-- +----------------------------------------------------------------+
-- | Recall severity is scored against retention, not correctness.  |
-- +----------------------------------------------------------------+
-- Alice's block 3: shown 3777, wanted 4888, gave 3777. Memory was
-- flawless and the transformation never happened. Scored against the
-- expected number alone this is a total memory failure, 1.00, which is
-- the opposite of the truth and would bury the single sharpest signal
-- the instrument produces.
DO $$
DECLARE mf numeric; ex boolean;
BEGIN
  SELECT memory_failure, recall_executive_failure INTO mf, ex
    FROM v_dt_blocks WHERE session_key = (SELECT public_id FROM dt_sessions WHERE id = 201)
     AND block_no = 3;
  IF mf <> 0.00 THEN
    RAISE EXCEPTION 'an untransformed recall scored memory_failure %, expected 0.00: retention was perfect and only the transformation failed', mf;
  END IF;
  IF NOT ex THEN
    RAISE EXCEPTION 'an untransformed recall did not set recall_executive_failure';
  END IF;
END $$;

-- An expired recall is unscored, not scored 1.00. Nothing was
-- attempted, and putting "ran out of time" in the same bucket as
-- "forgot completely" would make the measure mean two things.
DO $$
DECLARE mf numeric;
BEGIN
  SELECT memory_failure INTO mf
    FROM v_dt_blocks WHERE session_key = (SELECT public_id FROM dt_sessions WHERE id = 203)
     AND block_no = 5;
  IF mf IS NOT NULL THEN
    RAISE EXCEPTION 'an expired recall scored memory_failure %, expected NULL', mf;
  END IF;
END $$;

-- +----------------------------------------------------------------+
-- | The threshold is taken over the ramp, not the control block.   |
-- +----------------------------------------------------------------+
-- This is the one a parsing check cannot reach, and the one that was
-- actually wrong. Cara's ramp and Alice's are identical: both first go
-- confidently wrong at d4_plus1, rank 3. Cara then collapses on block
-- 5, which is block 1's difficulty and therefore ranks 1. A minimum
-- taken over all five blocks reports her threshold as rank 1, which
-- says the easiest load defeated her when fifteen minutes did.
--
-- It is worst for exactly the participants the control exists to
-- detect, and it would have shown up as a strong result.
DO $$
DECLARE alice int; cara int; ctrl boolean; bob int;
BEGIN
  SELECT lured_at_rank INTO alice FROM v_dt_threshold
   WHERE session_key = (SELECT public_id FROM dt_sessions WHERE id = 201);
  SELECT lured_at_rank, lured_in_control INTO cara, ctrl FROM v_dt_threshold
   WHERE session_key = (SELECT public_id FROM dt_sessions WHERE id = 203);
  SELECT lured_at_rank INTO bob FROM v_dt_threshold
   WHERE session_key = (SELECT public_id FROM dt_sessions WHERE id = 202);

  IF alice <> 3 THEN
    RAISE EXCEPTION 'a clean ramp gave a threshold of rank %, expected 3 (d4_plus1)', alice;
  END IF;
  IF cara <> 3 THEN
    RAISE EXCEPTION 'a fatigued run gave a threshold of rank %, expected 3: block 5 is leaking into the minimum and reporting the control as the threshold', cara;
  END IF;
  IF NOT ctrl THEN
    RAISE EXCEPTION 'a fatigued run did not set lured_in_control, so its threshold reads as a clean load effect';
  END IF;
  IF bob IS NOT NULL THEN
    RAISE EXCEPTION 'a calibrated run was given a threshold of rank %, expected none', bob;
  END IF;
END $$;

-- +----------------------------------------------------------------+
-- | A calibrated person does not look like a result.               |
-- +----------------------------------------------------------------+
-- Bob is the negative control. His accuracy falls as much as Alice's
-- and his confidence falls with it, so the gap stays near zero. A view
-- that finds an effect in Bob is finding it in the ramp itself rather
-- than in the person, and would find one in everybody.
DO $$
DECLARE gap numeric; lured int;
BEGIN
  SELECT gap_pct, confidently_lured INTO gap, lured FROM v_dt_sessions
   WHERE session_key = (SELECT public_id FROM dt_sessions WHERE id = 202);
  IF gap > 5 THEN
    RAISE EXCEPTION 'the calibrated run shows a confidence gap of % points, expected near zero', gap;
  END IF;
  IF lured <> 0 THEN
    RAISE EXCEPTION 'the calibrated run took % lures confidently, expected 0', lured;
  END IF;
END $$;

-- +----------------------------------------------------------------+
-- | The fatigue control is readable.                               |
-- +----------------------------------------------------------------+
-- Block 5 against block 1 is the only thing separating load from
-- time-on-task. Alice holds, Cara does not, and the sign has to be the
-- right way round: negative means block 5 came in below block 1.
DO $$
DECLARE alice numeric; cara numeric;
BEGIN
  SELECT fatigue_delta_pct INTO alice FROM v_dt_sessions
   WHERE session_key = (SELECT public_id FROM dt_sessions WHERE id = 201);
  SELECT fatigue_delta_pct INTO cara FROM v_dt_sessions
   WHERE session_key = (SELECT public_id FROM dt_sessions WHERE id = 203);
  IF alice <> 0 THEN
    RAISE EXCEPTION 'a run whose block 5 matched block 1 reported a fatigue delta of %, expected 0', alice;
  END IF;
  IF cara >= -20 THEN
    RAISE EXCEPTION 'a run whose block 5 collapsed reported a fatigue delta of %, expected a large negative', cara;
  END IF;
END $$;

-- +----------------------------------------------------------------+
-- | The export carries no key and no identity.                     |
-- +----------------------------------------------------------------+
-- v_dt_answers is what leaves as CSV. A name, an email address or a
-- chosen index appearing on it is a leak rather than a bug, and the
-- answer key only has to escape once.
DO $$
DECLARE leaked text;
BEGIN
  SELECT string_agg(column_name, ', ') INTO leaked
    FROM information_schema.columns
   WHERE table_name = 'v_dt_answers'
     AND column_name IN ('email', 'email_key', 'display_name', 'chosen_index',
                         'correct_index', 'lure_index', 'presented_digits',
                         'expected_digits', 'response_digits');
  IF leaked IS NOT NULL THEN
    RAISE EXCEPTION 'v_dt_answers exposes %, which must not leave the server in an export', leaked;
  END IF;
END $$;

SELECT 'decision test metric views: all assertions hold' AS result;
