-- +goose Up
-- +goose StatementBegin

-- Re-grade answers that were scored against the wrong question.
--
-- **What happened.** From migration 00059/00060, every run draws its own
-- thirty questions and `GetBlock` serves them from `dt_session_items`.
-- `SubmitAnswer` was not changed with it: it kept resolving the item as
-- `DTScoredItems()[pos-1]`, the bank's position rather than the run's.
--
-- That was correct for as long as every run was served the bank in its
-- stored order, and became silently wrong the moment the draw landed. A
-- participant answered the question they were shown and it was graded
-- against a different item's key. Not approximately wrong: on the one
-- real run taken afterwards, 0 of 30 answers were compared with the
-- question that had actually been on screen. The score that came out,
-- 9 of 30, was noise. Re-graded against what was shown it is 19 of 30,
-- and block 1 goes from 0/6 to 6/6.
--
-- Nothing caught it. The draw tests checked composition and
-- distinctness; the write tests called DTSaveAnswer directly with the
-- right item, bypassing the lookup; the synthetic UI runs checked that
-- answers were stored, never that the stored answer and the drawn
-- question were the same thing. It was found because the participant
-- said the number looked too low.
--
-- **Why this is repairable at all.** `chosen_index` is the index of the
-- option the participant clicked in the list they were shown, and the
-- list they were shown came from the draw. So the recorded choice is
-- valid against the drawn item; only the grading used the wrong key.
-- The answers are intact and the verdicts were wrong.
--
-- Scoped to answers whose item disagrees with the draw, so this is a
-- no-op on every run taken before the pools existed (those have no
-- dt_session_items rows at all) and on every run graded correctly.

-- block_load is deliberately untouched: it is the BLOCK's load level
-- (d3, d4, d4_plus1 ...), not a property of the item, and it was
-- always recorded correctly. An earlier draft of this migration set it
-- from the item's family, which would have replaced a correct value
-- with a meaningless one across every repaired row.
UPDATE dt_answers a
   SET item_id      = d.id,
       item_code    = d.code,
       item_version = d.version,
       outcome      = CASE
                        -- An expiry stays an expiry: nothing was chosen,
                        -- so there is nothing to re-grade.
                        WHEN a.outcome = 'expired' OR a.chosen_index IS NULL
                          THEN 'expired'
                        WHEN a.chosen_index = d.correct_index THEN 'correct'
                        WHEN a.chosen_index = d.lure_index    THEN 'lure'
                        ELSE 'other'
                      END
  FROM dt_session_items si
  JOIN dt_items d ON d.id = si.item_id
 WHERE si.session_id = a.session_id
   AND si.position_overall = a.position_overall
   AND a.item_id IS DISTINCT FROM si.item_id;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Deliberately not reversible. The previous values were answers graded
-- against questions the participant never saw; restoring them would be
-- restoring a mistake, and the correct grading is recomputable from the
-- draw at any time.
SELECT 1;
-- +goose StatementEnd
