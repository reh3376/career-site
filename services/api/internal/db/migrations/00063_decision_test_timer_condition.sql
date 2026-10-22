-- +goose Up
-- The visible countdown, as a randomised condition rather than a change
-- to the test.
--
-- Proposed 2026-10-13 and deferred with the reasoning written down,
-- because the idea is good and the objection is specific: a visible
-- countdown gives a participant a legitimate reason not to re-check an
-- answer that already feels right. Somebody with three seconds left who
-- takes the fluent answer is doing rational triage under a deadline,
-- which is sensible, and is a different phenomenon from the bias this
-- instrument exists to measure. Shown to everybody, a wrong answer
-- stops being evidence of the bias, and nothing in the data separates
-- the two afterwards.
--
-- Shown to a random half it is a finding instead of a guess: whether
-- visible time pressure raises the lure rate, and what it does to
-- calibration. That is why this is a column and not a constant.
--
-- The column sits beside audio_mode and device_class, which is the
-- idiom already here for a condition that splits the sample. NOT NULL
-- with a default of false, so every run recorded before today reads as
-- what it was: nobody saw a timer.
--
-- Deliberately NOT threaded into v_dt_answers here. That view is
-- redefined across several later migrations and carries the one
-- definition of `usable`; rewriting it to add a column belongs with the
-- analysis work that will actually query the split, with the whole
-- current definition in hand. Until then the condition is on
-- dt_sessions, which every analysis query can join, and the admin
-- surfaces read it from there as they already do for audio_mode.
ALTER TABLE dt_sessions
  ADD COLUMN timer_visible boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN dt_sessions.timer_visible IS
  'Randomised condition, assigned at session start: was a per-question countdown shown? It splits the sample, so it is recorded rather than inferred. False for every run before 2026-10-22, when nobody saw one.';

-- +goose Down
ALTER TABLE dt_sessions DROP COLUMN IF EXISTS timer_visible;
