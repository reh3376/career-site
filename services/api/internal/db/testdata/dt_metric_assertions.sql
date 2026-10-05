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

  -- Per item, not per load. An item is served once per run and
  -- v_dt_items pools across item sets deliberately (it carries
  -- item_set_versions as an array so mixing is visible), so every item
  -- should show the four real runs and not the fifth synthetic one.
  -- Accuracy cannot be used for this, since an easy item legitimately
  -- reaches 100% across the real runs.
  SELECT max(sessions) INTO n FROM v_dt_items;
  IF n <> 4 THEN
    RAISE EXCEPTION 'v_dt_items counts % sessions on an item, expected 4: a synthetic run is in the item statistics', n;
  END IF;

  SELECT count(*) INTO n FROM v_dt_calibration WHERE confidence_band >= 90;
  IF n > 0 THEN
    RAISE EXCEPTION 'v_dt_calibration has a 90+ confidence band, which only the synthetic run produces';
  END IF;
END $$;

-- +----------------------------------------------------------------+
-- | The load curve goes the way the instrument claims.             |
-- +----------------------------------------------------------------+
-- Accuracy down and the gap up as the ramp rises, WITHIN one item set.
-- If this ever fails it is either the view or the composition, and on
-- 2026-10-04 it was the composition: migration 00051's comment said the
-- families were interleaved and its insert order did the opposite,
-- which made accuracy appear to RISE with load.
DO $$
DECLARE a1 numeric; a4 numeric; g1 numeric; g4 numeric;
BEGIN
  SELECT accuracy_pct, gap_pct INTO a1, g1 FROM v_dt_load_curve
   WHERE block_load = 'd3' AND item_set_version = 'items-2026-10-05';
  SELECT accuracy_pct, gap_pct INTO a4, g4 FROM v_dt_load_curve
   WHERE block_load = 'd4_plus3' AND item_set_version = 'items-2026-10-05';
  IF a4 >= a1 THEN
    RAISE EXCEPTION 'accuracy at the hardest load (%) is not below the easiest (%)', a4, a1;
  END IF;
  IF g4 <= g1 THEN
    RAISE EXCEPTION 'the confidence gap at the hardest load (%) is not above the easiest (%)', g4, g1;
  END IF;
END $$;

-- +----------------------------------------------------------------+
-- | The curves do not pool across instruments.                     |
-- +----------------------------------------------------------------+
-- The first real reading of v_dt_load_curve on production reported
-- accuracy RISING to 100% at the hardest load. The view was right and
-- the data was real: both runs had been taken on the family-blocked
-- composition, where item family swamped load. The view grouped by
-- block_load alone, so it averaged every run regardless of which items
-- it saw or how long it had, which is exactly what instrument_version
-- and item_set_version exist on dt_sessions to prevent.
--
-- The fixture now carries a run on an older item set with the ramp
-- inverted. Pooling would drag the current set's numbers toward it, and
-- the assertion above would start passing or failing for reasons
-- nothing to do with the view being correct.
DO $$
DECLARE n int; sets int;
BEGIN
  SELECT count(DISTINCT item_set_version) INTO sets FROM v_dt_load_curve;
  IF sets < 2 THEN
    RAISE EXCEPTION 'v_dt_load_curve reports % item set(s): the fixture has two, so the view is pooling them', sets;
  END IF;

  -- The current set has three real runs; the older one has exactly one.
  SELECT sessions INTO n FROM v_dt_load_curve
   WHERE block_load = 'd3' AND item_set_version = 'items-2026-10-04';
  IF n <> 1 THEN
    RAISE EXCEPTION 'the older item set reports % sessions at d3, expected 1', n;
  END IF;
  SELECT sessions INTO n FROM v_dt_load_curve
   WHERE block_load = 'd3' AND item_set_version = 'items-2026-10-05';
  IF n <> 3 THEN
    RAISE EXCEPTION 'the current item set reports % sessions at d3, expected 3', n;
  END IF;

  SELECT count(DISTINCT item_set_version) INTO sets FROM v_dt_calibration;
  IF sets < 2 THEN
    RAISE EXCEPTION 'v_dt_calibration reports % item set(s), so it is pooling instruments too', sets;
  END IF;
END $$;

-- A row whose families column holds a single value is one where load
-- and item family are confounded, and it cannot support a claim about
-- load. The old item set's rows must say so on the row rather than in a
-- note somebody reads after the number.
DO $$
DECLARE fams text;
BEGIN
  SELECT families INTO fams FROM v_dt_load_curve
   WHERE item_set_version = 'items-2026-10-05' AND block_load = 'd4_plus3';
  IF fams IS NULL OR position('+' in fams) = 0 THEN
    RAISE EXCEPTION 'the current item set reports families "%" at the hardest load: one family per block means load is confounded with family', fams;
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

-- +----------------------------------------------------------------+
-- | Curation reaches the numbers.                                  |
-- +----------------------------------------------------------------+
-- A mark that does not exclude is worse than no mark, because it
-- creates the belief that the data has been cleaned. These assertions
-- apply the marks, check the effect and then revert, so the rest of
-- this file does not depend on the order it is read in.
DO $$
DECLARE before_sessions int; after_sessions int; after_block int; answers_kept int;
BEGIN
  SELECT sessions INTO before_sessions FROM v_dt_load_curve
   WHERE item_set_version = 'items-2026-10-05' AND block_load = 'd3';
  IF before_sessions <> 3 THEN
    RAISE EXCEPTION 'expected 3 usable sessions before curation, found %', before_sessions;
  END IF;

  -- Exclude a whole run, and one block of a different run.
  UPDATE dt_sessions SET review_status = 'do_not_use', review_reason = 'participant_reported'
   WHERE id = 201;
  INSERT INTO dt_block_reviews (session_id, block_no, status, reason)
  VALUES (202, 3, 'do_not_use', 'instrument_fault');

  -- The excluded run is gone from every load.
  SELECT sessions INTO after_sessions FROM v_dt_load_curve
   WHERE item_set_version = 'items-2026-10-05' AND block_load = 'd3';
  IF after_sessions <> 2 THEN
    RAISE EXCEPTION 'a run marked do_not_use still feeds the load curve: % sessions at d3, expected 2', after_sessions;
  END IF;

  -- The excluded BLOCK costs only its own load. d4_plus1 is block 3,
  -- so it loses one more session than the others; if block exclusion
  -- were ignored it would read 2 like everything else, and if it took
  -- the whole run with it the other loads would read 1.
  SELECT sessions INTO after_block FROM v_dt_load_curve
   WHERE item_set_version = 'items-2026-10-05' AND block_load = 'd4_plus1';
  IF after_block <> 1 THEN
    RAISE EXCEPTION 'block-level exclusion is wrong: % sessions at d4_plus1, expected 1', after_block;
  END IF;
  SELECT sessions INTO after_sessions FROM v_dt_load_curve
   WHERE item_set_version = 'items-2026-10-05' AND block_load = 'd4_plus3';
  IF after_sessions <> 2 THEN
    RAISE EXCEPTION 'excluding one block removed the whole run: % sessions at d4_plus3, expected 2', after_sessions;
  END IF;

  -- Nothing was deleted. Exclusion is a filter, and the measurements
  -- survive it: "nothing is ever deleted" has held since 00049 and
  -- curation must not become the exception.
  SELECT count(*) INTO answers_kept FROM dt_answers WHERE session_id = 201;
  IF answers_kept <> 30 THEN
    RAISE EXCEPTION 'an excluded run lost rows: % answers remain for session 201, expected 30', answers_kept;
  END IF;

  -- The review surfaces still show it, labelled rather than hidden,
  -- because a reviewer has to be able to find what they excluded.
  IF NOT EXISTS (SELECT 1 FROM v_dt_sessions
                  WHERE session_review_status = 'do_not_use') THEN
    RAISE EXCEPTION 'v_dt_sessions hides an excluded run, so it cannot be un-excluded';
  END IF;

  DELETE FROM dt_block_reviews WHERE session_id = 202 AND block_no = 3;
  UPDATE dt_sessions SET review_status = '', review_reason = '' WHERE id = 201;
END $$;

-- The vocabulary is enforced by the table, not only by Go. A status
-- nobody can interpret still lands in a count.
DO $$
BEGIN
  BEGIN
    UPDATE dt_sessions SET review_status = 'probably_fine' WHERE id = 201;
    RAISE EXCEPTION 'the review_status CHECK accepted a word outside the vocabulary';
  EXCEPTION WHEN check_violation THEN
    NULL;
  END;
  BEGIN
    UPDATE dt_sessions SET review_reason = 'because' WHERE id = 201;
    RAISE EXCEPTION 'the review_reason CHECK accepted a word outside the vocabulary';
  EXCEPTION WHEN check_violation THEN
    NULL;
  END;
END $$;

SELECT 'decision test metric views: all assertions hold' AS result;
