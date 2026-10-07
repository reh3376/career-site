-- +goose Up
-- +goose StatementBegin

-- Item pools: the test's thirty questions drawn per run rather than
-- served in a fixed order.
--
-- S2 of docs/sprint-decision-test-item-pools.md.
--
-- **The problem.** Every participant has been served the same thirty
-- items in the same order. A third sitting on 2026-10-07 scored 30/30
-- at a mean answer time of 5.7s against 12.6s for first-time
-- participants, because 27 of the 30 were items that person had already
-- answered twice. A repeat sitting measures item recall, not reasoning
-- under load.
--
-- **The rule, as the owner specified it.** Questions are random within
-- a category, each used at most once per test, and the exclusion resets
-- after each test rather than depleting the pools over time.
--
-- What must be preserved is the category composition. Every block of
-- six carries 2 arithmetic, 1 base-rate, 1 conjunction and 2 syllogism
-- today, and that is why one block's accuracy can be compared with
-- another's at all: if block 4 held an extra syllogism, it would differ
-- from block 1 for a reason that has nothing to do with load. Varying
-- the items while holding the mix is the whole point.

-- The per-block quota, as a function rather than as a number repeated
-- in a view, in Go and in a test.
--
-- Same reasoning as dt_confident_threshold() and dt_load_rank(): a
-- number that decides what the instrument is should be changeable in
-- one line, and should be impossible to change in one place and miss in
-- another. An unknown family returns 0, so a category nobody has
-- declared a quota for contributes nothing to a draw rather than
-- silently taking a slot.
CREATE OR REPLACE FUNCTION dt_category_quota(family text) RETURNS int
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE family
    WHEN 'arithmetic'  THEN 2
    WHEN 'syllogism'   THEN 2
    WHEN 'base_rate'   THEN 1
    WHEN 'conjunction' THEN 1
    ELSE 0
  END
$$;

COMMENT ON FUNCTION dt_category_quota(text) IS
  'Questions of this category in every block. The six per block are 2 arithmetic, 1 base_rate, 1 conjunction, 2 syllogism; multiply by five blocks for the per-test need. Changing a number here changes the instrument.';

-- +goose StatementEnd

-- +goose StatementBegin

-- The thirty questions one run was given, in order.
--
-- Written when the session is created, in the same transaction, so a
-- session can never exist without its draw and a run cannot become
-- unservable half way through.
--
-- This is also the direct answer to "track the questions used for a
-- given test". dt_answers already records the item per answered
-- position, but only once the question has been answered: the draw has
-- to exist before the participant sees anything, and it has to survive
-- a run that is abandoned at question three.
CREATE TABLE IF NOT EXISTS dt_session_items (
  session_id       bigint NOT NULL REFERENCES dt_sessions(id),
  position_overall int    NOT NULL,
  item_id          bigint NOT NULL REFERENCES dt_items(id),
  created_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, position_overall),
  -- The no-repeat rule, enforced by the database rather than by
  -- whichever code happens to write the draw. "Used once per test" is a
  -- property of the data, so it is a constraint and not a convention.
  CONSTRAINT dt_session_items_once_per_test UNIQUE (session_id, item_id),
  CONSTRAINT dt_session_items_position_ck CHECK (position_overall BETWEEN 1 AND 30)
);

COMMENT ON TABLE dt_session_items IS
  'The thirty items one run was drawn, in presentation order. Written with the session in one transaction. The unique constraint on (session_id, item_id) is the "used once per test" rule.';

CREATE INDEX IF NOT EXISTS idx_dt_session_items_item
  ON dt_session_items (item_id);

-- Drawing needs the active pool of one category, fast.
CREATE INDEX IF NOT EXISTS idx_dt_items_pool
  ON dt_items (family, position) WHERE active AND kind = 'scored';

-- +goose StatementEnd

-- +goose StatementBegin

-- How much room each category actually has to vary.
--
-- **The view exists because the mechanism can look like it is working
-- when it cannot.** Every pool is currently exactly the size of its own
-- per-test quota: ten arithmetic items for the ten a test needs, five
-- base-rate for five. Drawing ten at random from ten is not a draw, so
-- until the bank grows every participant still sees the same thirty
-- questions with the order shuffled, and nothing on any screen would
-- look wrong.
--
-- So the shortfall is a number somebody can see, rather than something
-- discovered when two participants compare notes.
CREATE OR REPLACE VIEW v_dt_item_pools AS
SELECT
  i.family                                   AS category,
  count(*)                                   AS pool,
  dt_category_quota(i.family)                AS per_block,
  dt_category_quota(i.family) * 5            AS per_test,
  count(*) - dt_category_quota(i.family) * 5 AS spare,
  -- A draw is only a draw if there is something to leave behind.
  count(*) > dt_category_quota(i.family) * 5 AS can_vary
FROM dt_items i
WHERE i.active AND i.kind = 'scored'
GROUP BY i.family;

COMMENT ON VIEW v_dt_item_pools IS
  'Per category: how many active scored items exist, how many a single test consumes, and the spare. can_vary is false where the pool is exactly the quota, which means every participant is served the same items however random the selection claims to be.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS v_dt_item_pools;
DROP INDEX IF EXISTS idx_dt_items_pool;
DROP INDEX IF EXISTS idx_dt_session_items_item;
DROP TABLE IF EXISTS dt_session_items;
DROP FUNCTION IF EXISTS dt_category_quota(text);
-- +goose StatementEnd
