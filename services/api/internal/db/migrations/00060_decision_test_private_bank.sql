-- +goose Up
-- +goose StatementBegin

-- The item bank leaves this repository.
--
-- S8 of docs/sprint-decision-test-item-pools.md.
--
-- **Why.** The thirty scored items were seeded in migration 00051 on
-- 2026-10-04, in a public repository. Every prompt, correct answer and
-- lure has been readable on GitHub ever since, while the run page tells
-- a reviewer that "an answer key that escapes contaminates a
-- standardized instrument permanently and it only has to escape once".
-- The page was right and the repository made it false.
--
-- **Why they are retired rather than deleted.** Removing the migration
-- would not un-publish them: the commit is in public history, and
-- rewriting history on a public repo is neither reliable nor compatible
-- with a repository whose whole claim is that the diffs are in the
-- open. So the published items are retired and replaced by a bank that
-- is never committed. Exposure was three days with zero forks, which is
-- what made replacing them worth the effort rather than academic.
--
-- The rows stay. Four runs point at them through dt_answers and nothing
-- here is ever deleted; they are simply no longer drawn.
UPDATE dt_items SET active = false, updated_at = now()
 WHERE kind = 'scored';

COMMENT ON TABLE dt_items IS
  'The question bank. Scored items seeded before 2026-10-07 are retired: their key was published in migration 00051 and cannot be unpublished. The live bank is synced from a private source by deploy/items-sync.sh and is never committed. Rows marked is_fixture are structurally valid placeholders for CI and local development, never served in production.';

-- +goose StatementEnd

-- +goose StatementBegin

-- Fixtures: a bank shaped like the real one, obviously not it.
--
-- CI, `go test` on a laptop and a local compose stack all need
-- something to draw from, and none of them can have the real items. So
-- the public repository seeds placeholders with the right categories in
-- the right quantities, and every existing test keeps working, because
-- all of them check shape and mechanics rather than content: the
-- category quota, the draw composition, the no-repeat constraint, the
-- refusal when a category runs short.
--
-- The flag is what keeps them out of production. DTDrawItems excludes
-- fixtures unless DT_ALLOW_FIXTURE_BANK is set, which production never
-- sets, so a database that never received the real bank refuses to
-- start a run rather than asking a volunteer what two plus two is.
ALTER TABLE dt_items
  ADD COLUMN IF NOT EXISTS is_fixture boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN dt_items.is_fixture IS
  'A placeholder seeded by this repository so CI and local development have a bank. Never drawn unless DT_ALLOW_FIXTURE_BANK=1, which production does not set.';

-- Three times the per-test need in every category, matching the real
-- bank's shape, so a fixture-backed draw exercises the same code paths
-- including the variation a spare makes possible.
INSERT INTO dt_items
  (tenant_id, code, version, family, kind, prompt, reminder, options,
   correct_index, lure_index, rationale, active, position, is_fixture)
SELECT 1,
       'FX-' || fam.prefix || n,
       1,
       fam.family,
       'scored',
       'Fixture ' || fam.family || ' ' || n ||
         '. This is not the instrument. Which answer is the number four?',
       '',
       '["four", "five", "six", "seven"]'::jsonb,
       0,
       1,
       'Placeholder seeded by migration 00060 so CI and local development have a bank to draw from. The real items are synced privately.',
       true,
       fam.base + n,
       true
  FROM (VALUES
          ('A', 'arithmetic',  30,   0),
          ('D', 'syllogism',   30,  30),
          ('B', 'base_rate',   15,  60),
          ('C', 'conjunction', 15,  75)
       ) AS fam(prefix, family, cnt, base),
       generate_series(1, 30) AS n
 WHERE n <= fam.cnt;

-- +goose StatementEnd

-- +goose StatementBegin

-- Pool status has to distinguish the real bank from the placeholders,
-- or "can_vary" would read as true on a database that cannot serve a
-- single real question.
-- Dropped and recreated rather than replaced. CREATE OR REPLACE VIEW
-- cannot rename or reorder columns, and this adds real_items and
-- fixtures in the middle:
--   cannot change name of view column "per_block" to "real_items"
-- Nothing else selects from it, so a drop is safe here.
DROP VIEW IF EXISTS v_dt_item_pools;

CREATE VIEW v_dt_item_pools AS
SELECT
  i.family                                   AS category,
  count(*)                                   AS pool,
  count(*) FILTER (WHERE NOT i.is_fixture)   AS real_items,
  count(*) FILTER (WHERE i.is_fixture)       AS fixtures,
  dt_category_quota(i.family)                AS per_block,
  dt_category_quota(i.family) * 5            AS per_test,
  count(*) FILTER (WHERE NOT i.is_fixture)
    - dt_category_quota(i.family) * 5        AS spare,
  count(*) FILTER (WHERE NOT i.is_fixture)
    > dt_category_quota(i.family) * 5        AS can_vary
FROM dt_items i
WHERE i.active AND i.kind = 'scored'
GROUP BY i.family;

COMMENT ON VIEW v_dt_item_pools IS
  'Per category: the active scored pool split into real items and fixtures, what one test consumes, and the spare. spare and can_vary count REAL items only, so a database holding nothing but placeholders reports a negative spare rather than claiming it can vary.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS v_dt_item_pools;
DELETE FROM dt_items WHERE is_fixture;
ALTER TABLE dt_items DROP COLUMN IF EXISTS is_fixture;
UPDATE dt_items SET active = true WHERE kind = 'scored';
-- +goose StatementEnd
