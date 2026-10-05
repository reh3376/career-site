-- +goose Up
-- +goose StatementBegin

-- Fixing the block composition, after the owner's second live run.
--
-- The run came back with accuracy RISING as load rose: 50, 50, 33, 100,
-- 100 percent. That is the opposite of the hypothesis, and it was an
-- artefact rather than a finding.
--
-- The items were served in insert order, which was family order, so the
-- blocks came out:
--
--   block 1  A1-A6      all arithmetic traps
--   block 2  A7-A10 B1-B2
--   block 3  B3-B8      all base-rate
--   block 4  C1-C4 D1-D2
--   block 5  D3-D8      all syllogisms
--
-- Block difficulty was therefore confounded with load, and the owner is
-- better at syllogisms than at arithmetic traps, so the "hardest" blocks
-- handed him the items he finds easiest.
--
-- Worse, THE FATIGUE CONTROL WAS INVALID. Block 5 is supposed to mirror
-- block 1 so that a difference between them is fatigue and nothing else.
-- Six syllogisms against six arithmetic traps compares nothing.
--
-- Migration 00051's own comment claimed "families are interleaved rather
-- than blocked so no single block is all arithmetic or all syllogisms".
-- That was simply false: the comment described an intention and the
-- insert order did the opposite.
--
-- # What changes
--
-- Presentation order becomes an explicit column rather than an accident
-- of insert order, because the order is a property of the instrument and
-- deserves to be stated.
--
-- The family counts are adjusted so six divides evenly across five
-- blocks: arithmetic 10, base_rate 5, conjunction 5, syllogism 10. Every
-- block then gets exactly 2 arithmetic, 1 base-rate, 1 conjunction and 2
-- syllogisms, and block 5 mirrors block 1 by construction rather than by
-- hope.
--
-- Three base-rate items are retired and three new items take their place
-- in the other families. Retired rather than deleted: they are kept
-- inactive so a run recorded against item_set_version items-2026-10-04
-- can still be read.
--
-- B2 is one of the three, which also settles a question the owner raised
-- about it. It asked whether somebody "decisive who enjoys pressure" at
-- a conference of 80 nurses and 20 surgeons is more likely a surgeon,
-- and it depended on a stereotype being weak enough for the base rate to
-- win. That is a cultural judgement that can age badly, and it is no
-- loss to drop it. B4 and B5 go because they duplicate the structures of
-- B3 and B1 respectively.

ALTER TABLE dt_items ADD COLUMN IF NOT EXISTS position int NOT NULL DEFAULT 0;

COMMENT ON COLUMN dt_items.position IS
  'Presentation order, 1-based, for scored items. The block an item lands in determines the load it is answered under, so this is part of the instrument rather than a display detail. Insert order was used before 00052 and produced family-blocked blocks.';

-- Retire, do not delete.
UPDATE dt_items SET active = false WHERE code IN ('B2','B4','B5');

-- One conjunction and two syllogisms, to make the families divide.
INSERT INTO dt_items (code, family, kind, prompt, reminder, options, correct_index, lure_index, rationale)
VALUES
  ('C5','conjunction','scored',
   'Priya is 29, runs marathons, and tracks her diet carefully. Which is more likely?',
   '',
   '["Equally likely","Priya works in insurance","Priya works in insurance and competes at weekends"]'::jsonb,
   1, 2,
   'The detailed version fits the description and is contained by the plain one.'),
  ('D9','syllogism','scored',
   'All engineers can read drawings. Some people who read drawings work offshore. Therefore some engineers work offshore. Assuming both statements are true, does the conclusion follow?',
   'Assume both statements are true.',
   '["Yes","Cannot tell","No"]'::jsonb,
   2, 0,
   'Believable and invalid. The offshore workers need not be the engineers.'),
  ('D10','syllogism','scored',
   'All rivers flow uphill. The Severn is a river. Therefore the Severn flows uphill. Assuming both statements are true, does the conclusion follow?',
   'Assume both statements are true.',
   '["Yes","No","Cannot tell"]'::jsonb,
   0, 1,
   'Valid, with a false premise and an absurd conclusion. Pairs with D4.')
ON CONFLICT (tenant_id, code, version) DO NOTHING;

-- Interleave. Each block: 2 arithmetic, 1 base-rate, 1 conjunction,
-- 2 syllogisms. Block 5 mirrors block 1 by construction.
--
--   block 1  A1  A2   B1  C1  D1  D2
--   block 2  A3  A4   B3  C2  D3  D4
--   block 3  A5  A6   B6  C3  D5  D6
--   block 4  A7  A8   B7  C4  D7  D8
--   block 5  A9  A10  B8  C5  D9  D10
UPDATE dt_items SET position = v.pos
  FROM (VALUES
    ('A1',1),('A2',2),('B1',3),('C1',4),('D1',5),('D2',6),
    ('A3',7),('A4',8),('B3',9),('C2',10),('D3',11),('D4',12),
    ('A5',13),('A6',14),('B6',15),('C3',16),('D5',17),('D6',18),
    ('A7',19),('A8',20),('B7',21),('C4',22),('D7',23),('D8',24),
    ('A9',25),('A10',26),('B8',27),('C5',28),('D9',29),('D10',30)
  ) AS v(code, pos)
 WHERE dt_items.code = v.code AND dt_items.kind = 'scored';

-- Practice keeps its own order, after the scored ones so it cannot
-- collide.
UPDATE dt_items SET position = 100 + id WHERE kind = 'practice';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE dt_items SET active = true WHERE code IN ('B2','B4','B5');
DELETE FROM dt_items WHERE code IN ('C5','D9','D10');
ALTER TABLE dt_items DROP COLUMN IF EXISTS position;
-- +goose StatementEnd
