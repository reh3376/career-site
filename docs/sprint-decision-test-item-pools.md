# Sprint plan: item pools and parallel forms

**No code has been written for this.** The owner asked for a complete
step-by-step plan first, on 2026-10-07. This is that plan. It ends with
the decisions that are his rather than mine.

## The ask, in his words

> Create 2 or 3 question pools with categories like: logic, practical
> math, common misconceptions. Each test and block within the test will
> be made up of the same number of questions from each category. This
> will allow the actual questions to vary but not the general category,
> which should help maintain consistency. We will need to track the
> questions used for a given test and the answers, not just the question
> category.

The problem behind it is concrete. On 2026-10-07 a third sitting scored
30/30 at a mean answer time of 5.7s, against 12.6s for first-time
participants, because 27 of the 30 items were ones that participant had
already seen twice. A repeat sitting currently measures item recall, not
reasoning under load.

---

## What already exists, checked rather than assumed

Three things are already true, and they make this smaller than it looks.

**The per-block category balance is already exactly what was asked
for.** Every block is 2 arithmetic, 1 base-rate, 1 conjunction, 2
syllogism:

| block | arithmetic | base_rate | conjunction | syllogism |
|---|---|---|---|---|
| 1–5 | 2 | 1 | 1 | 2 |

**Which items a run used is already recorded.** `dt_answers` carries
`item_id`, `item_code` and `item_version` on every row, so "track the
questions used, not just the category" is done. What is missing is the
run-level record of *which pool was drawn from*.

**Runs under different item sets already cannot pool.**
`item_set_version` is on every session and `v_dt_load_curve` and
`v_dt_calibration` group by it (migration 00055, FR-DT-15). A new form
that bumps that version inherits the protection for free.

So the real work is: make the balance **enforced instead of implicit**,
and add **pools** so the items vary while it holds.

### What is fragile today

The balance is a property of hand-ordered `position` values 1 to 30.
Nothing checks it. Someone retiring one base-rate item and adding one
syllogism would silently change block 3's composition, and the first
sign would be a load curve that moved for a reason nobody could name.
`itemSetVersion` is a hand-edited Go constant, so the bump is also
manual and forgettable.

---

## Design decisions, with the reasoning

### D1. Four categories, and they are the families that already exist

**Settled 2026-10-07.** The owner's suggested names (logic, practical
math, common misconceptions) were offered as examples, and he confirmed
that existing names should be used and that more than three categories
is fine. So there is no new taxonomy and no renaming:

| category (`dt_items.family`) | per block | per test |
|---|---|---|
| `arithmetic` | 2 | 10 |
| `syllogism` | 2 | 10 |
| `base_rate` | 1 | 5 |
| `conjunction` | 1 | 5 |

**Why these four.** They already divide a six-question block cleanly as
2/1/1/2, and that quota is live and working today. Collapsing base-rate
and conjunction into one category would force the per-block split to be
re-derived for no measurement gain, and would lose a distinction the
instrument already reports on: `v_dt_items` gives per-family accuracy and
lure rates, and base-rate neglect and the conjunction fallacy are
different errors with different literatures.

**No column is added for this.** `family` is already the category, is
already on every item, and is already carried through `v_dt_answers` to
the export. The three-way framing still works in prose, where base-rate
and conjunction read naturally as "common misconceptions"; it is a
reporting choice, not a storage one.

**Room to grow.** A fifth category is possible but changes the
arithmetic: six questions per block divides by 2, 3 and 6, so a
five-category split cannot be equal per block. It would need either an
unequal per-block quota that is still equal per test, or a block size
other than six. Worth knowing before anyone proposes one; not a problem
today.

### D2. A random draw per test, without replacement

**Settled by the owner on 2026-10-07**, and it overrides the earlier
recommendation in this document, which was for two or three fixed
parallel forms. His words:

> The questions used are random within a given category and must only be
> used once per test, therefore you will need a mechanism to exclude the
> used questions from the pool after it is used in a given test. This
> will all need to reset for each test.

So: each test draws its 30 items at random from the category pools,
honouring the per-block quota in D1, with no item appearing twice in one
test, and the exclusion resetting at the start of every test rather than
carrying across tests.

**What this costs, stated once so it is not a surprise later.** Every
participant sees a different 30. Per-item statistics therefore
accumulate far more slowly than under fixed forms: `v_dt_items` reports
accuracy and lure rate per item, and at n in the tens most items will
have been seen once or twice. Item-level claims will take much longer to
support. What is preserved is the thing the owner is protecting, which is
the comparison that actually matters: category composition is identical
across every test and every block, so block 4 against block 1 and person
against person remain fair.

### D2a. The draw happens once, at session start, and is stored

**Confirmed by the owner on 2026-10-07**, who said the whole test may be
generated before it begins and left the choice here. Drawing up front is
the right one, and it is close to forced by how the test is already
served: the client fetches blocks one at a time through
`GetBlock(blockNo)`. Drawing per block could not guarantee that block 4
avoids an item block 2 already used, because each call would know nothing
of the others.

So the whole 30 are drawn when the session starts and written to a new
`dt_session_items` table, one row per position. `GetBlock(n)` then serves
positions `(n-1)*6 + 1` through `n*6` from that stored draw.

Drawing up front has a second benefit worth naming: the draw is one
transaction at the start, so a session can never exist without its items,
and a network failure part way through a run cannot leave later blocks
undrawable.

This table is also the direct answer to "track the questions used for a
given test". `dt_answers` already records the item per answered position,
but only once the question has been answered; the draw has to exist
before that, and it has to survive a run that is abandoned half way.

### D2c. Randomness is zero until the bank grows, and that must not be silent

This is the constraint that decides the order of the work.

| category | needed per test | in the bank today | spare |
|---|---|---|---|
| `arithmetic` | 10 | 10 | 0 |
| `syllogism` | 10 | 10 | 0 |
| `base_rate` | 5 | 5 | 0 |
| `conjunction` | 5 | 5 | 0 |

Every pool is exactly the size of its own quota, so "draw 10 at random
from 10" is not a draw. **Shipped against today's bank, this mechanism
serves the same 30 items to everybody**, exactly as now, with the order
shuffled.

That is not a reason to delay the mechanism, and it is a reason to make
the shortfall visible rather than letting it look like it is working. The
selection reports the pool size against the quota, and the admin console
shows it, so "the pools are too small to vary" is a number on a screen
rather than something discovered when two participants compare notes.

Meaningful variation needs roughly double: 20 arithmetic, 20 syllogism,
10 base-rate, 10 conjunction, so 60 items against today's 30. Three
distinct sittings' worth needs 90.

### D3. Exclusion is per test, and resets after each one

Stated twice by the owner, and worth being exact about because "reset"
could mean two different things.

Within one test, an item that has been drawn is removed from the pool for
the rest of that test: no question appears twice in the same sitting.
When the test ends the exclusion **resets**, so the next test draws from
the full pool again and is not narrowed by what earlier tests happened to
use. The pools do not deplete over time and item 11 is not reserved for
participant 2.

The one deliberate exception is a repeat sitting by a participant the
system recognises (D3a), where the point is precisely to avoid what that
person has already seen.

### D3a. A repeat sitting draws from what the participant has not seen

This is the original problem: a third sitting scored 30/30 because 27 of
its 30 items were ones that participant had already answered twice.

For a participant the system can recognise, the draw first excludes items
already served to them in an earlier sitting, read from
`dt_session_items` and joined on the identity observations already
recorded per session. If a category cannot be filled from unseen items
alone, it falls back to the least recently seen and the run is flagged,
because a partly repeated sitting is a different measurement and the
analysis has to be able to tell.

This is a per-participant exclusion, not a global one: it narrows that
person's draw and nobody else's.

### D4. Items within a category are not assumed equally hard

Two tests balanced by category are **not** thereby equally hard, because
the items inside a category differ. Under fixed forms that difference
would be a systematic offset between forms; under a random draw it
becomes variance between participants, which is the better failure of the
two but is not nothing.

`item_set_version` continues to identify the **pool**, not the draw,
which is correct: every run drawn from the same pool under the same
timings is comparable in the way the instrument claims. The per-run draw
is recorded in `dt_session_items`, so a difficulty effect can be looked
for later rather than assumed away.

A later step adds a view reporting per-item accuracy and lure rate with
counts, so that "this item is much harder than its category" is
answerable once there is enough data. At current n it will honestly
report that there is not.

### D5. Mechanism ships before content

The 30 new items are **item-writing work**, not code: each needs a
correct answer and a designed lure, matched to its category. The code
must therefore work correctly against today's bank, where every pool is
exactly its quota and the draw is forced, and must keep working as items
are added one at a time rather than requiring a complete new set.

---

## The steps

Each step is independently shippable and leaves the test working.

### S1. Make the existing balance enforced. DONE 2026-10-07.

Two database-backed tests read the live bank and assert its shape:
every block of six is 2 arithmetic / 1 base-rate / 1 conjunction / 2
syllogism, positions run 1 to 30 with no gap or duplicate, there are 30
active scored items and at least one practice item, and every scored item
has a lure distinct from its correct answer.

Verified to fail as well as pass: moving one item from `base_rate` to
`syllogism` in a test database produced "block 1 has 3 syllogism items,
want 2" and "block 1 has 0 base_rate items, want 1".

*Why first:* it locks in what is true today, so every later step has a
tripwire, and it fails loudly the next time the bank is edited by hand.
No schema change, no behaviour change.

### S2. Schema: the pools, the draw, and a view over both. DONE 2026-10-07.

Migration 00059. `dt_session_items` with the no-repeat rule as a unique
constraint, `dt_category_quota(family)` as the single definition of the
per-block mix, and `v_dt_item_pools`, which reports the honest state of
the bank today: a spare of zero and `can_vary = false` in every
category.

One migration:

- `dt_session_items (session_id, position_overall, item_id)`, primary key
  `(session_id, position_overall)`, unique on `(session_id, item_id)`.
  **The unique constraint is the no-repeat rule**, enforced by the
  database rather than by the code that happens to write it.
- `dt_category_quota(family text) RETURNS int`, the per-block quota as a
  function, following `dt_confident_threshold()` and `dt_load_rank()`.
  One definition, read by the view, the selection and the S1 test, so the
  quota cannot be changed in one place and missed in another.
- `v_dt_item_pools`: per category, the pool size, the per-test quota and
  the spare. This is what makes D2c visible instead of silent.
- No `form` column anywhere. That was for the superseded design.

*Exit:* migrations replay on an empty database; `v_dt_item_pools`
reports four categories with a spare of zero, which is the true and
uncomfortable answer for today's bank.

### S3. Selection: draw 30, balanced, distinct, and record it. DONE 2026-10-07.

`DTDrawItems`, `DTDrawnBlock`, `DTSeenItems` and `DTItemPools`. Four
database-backed tests: twenty-five draws each checked for composition and
distinctness, the unique constraint refusing a repeat directly, a
depleted category refusing rather than serving a short block and leaving
no rows behind, and a repeat sitting sharing nothing with the first.

The last of those **skips today** and says why: no category has a spare,
so a second sitting cannot avoid the first. Verified by doubling the bank
in a test database, where it runs and passes. That skip is the plan's
D2c made executable.

Two faults found by the tests rather than by reading: a nil exclusion
list arrives as NULL, and `id = ANY(NULL)` is NULL rather than false, so
a first sitting drew nothing at all; and `DTSeenItems` deliberately
ignores synthetic runs, which is a judgement now written down in the
code rather than implied.

- `DTDrawItems(ctx, sessionID, exclude []int64)` picks the test's 30:
  for each block, `dt_category_quota(family)` items per category, chosen
  at random from the category's active pool minus everything already
  drawn for this session, and writes them to `dt_session_items`.
- It **refuses rather than improvises** if a category cannot be filled.
  A short draw must never silently produce a five-question block.
- `DTBlockItems(ctx, sessionID, blockNo)` reads back positions
  `(n-1)*6+1` to `n*6`, replacing the current fixed slice of the bank.
- Randomness comes from the database (`ORDER BY random()`), so there is
  no seed to thread through Go and no second source of truth about what
  was drawn: the rows in `dt_session_items` are the record.

*Exit:* a database test drawing many times over, asserting every draw has
the right composition per block, no duplicate within a test, and a
different composition is impossible; plus a test that a deliberately
depleted category causes a refusal rather than a short block.

### S4. Wire the draw into the run. DONE 2026-10-07.

The draw runs inside the transaction that creates the session, so a
session can never exist without its items. `GetBlock` serves from
`dt_session_items` instead of slicing the bank by position.
`item_set_version` becomes `pool-2026-10-07`: it now names the pool a
run drew from rather than an order it was served in, which is the right
grain once two runs from one pool share no questions.

- `StartDecisionTest` calls `DTDrawItems` inside the same transaction
  that creates the session, so a session can never exist without its
  draw.
- `GetBlock` serves from `dt_session_items` rather than from the bank's
  fixed order.
- The practice block is unchanged: it is its own `kind` and is not drawn
  from the scored pools.

*Exit:* a synthetic run through the real UI, then reading
`dt_session_items` for that session and confirming the 30 rows match the
30 answers, in order.

### S5. Repeat sittings avoid what the participant has seen. DONE 2026-10-07.

Folded into S4: the exclusion set comes from `DTSeenItems` and is passed
to the draw. A lookup failure is not fatal, because failing to remember
what somebody saw last time costs a less varied draw while refusing the
run costs the run.

- The exclusion set for `DTDrawItems` is built from `dt_session_items`
  for the participant's earlier sessions, found through the identity
  observations already recorded.
- When a category cannot be filled from unseen items, fall back to the
  least recently seen and record on the session that it happened, so a
  partly repeated sitting is visible in the data rather than inferred.

*Exit:* a database test running two sittings for one visitor and
asserting the second draws nothing the first used, and that when the pool
is too small the fallback is recorded rather than silent.

### S6. The draw is visible where a run is

- The run page lists the 30 items it drew, in order, with their
  categories, behind the same collapsed toggle the questions already sit
  behind.
- `v_dt_item_pools` on the admin decision-test page, so pool shortfall is
  a number the owner sees rather than something that surfaces as two
  participants comparing notes.
- The per-run CSVs already carry `item_code` per answer and need nothing.

*Exit:* the live pass, which is the only test that counts for UI here.

### S7. Content: 90 private items. DRAFTED 2026-10-07, awaiting review.

All 90 are drafted in `docs/personal/decision-test-item-drafts.json`,
with a generated review copy beside it. Thirty per category for
arithmetic and syllogism, fifteen each for base-rate and conjunction,
which is three times the per-test need everywhere.

Every number was recomputed independently rather than trusted from the
prose. Option positions are shuffled with a fixed seed, because the
first draft had the correct answer in position 0 on all sixty and would
have become a test of noticing that.

*Exit:* the owner vets the items, principally that each lure is
genuinely the tempting answer, which is the part no script can check.

### S8. Make the bank private, and keep the tests honest. DONE 2026-10-07.

Migration 00060 retires every published scored item, adds
`dt_items.is_fixture`, and seeds 90 placeholders. `DT_ALLOW_FIXTURE_BANK`
gates them: CI and the dev compose set it, production does not, so a box
that never received the real bank refuses to start a run rather than
asking a volunteer which answer is the number four.
`deploy/items-sync.sh` pushes the real bank from `docs/personal/`,
validates it first, and refuses to finish if any category still cannot
vary.

Rehearsed against a local database: 90 real items in, fixtures
deactivated, every category `can_vary`. Two independent draws then
shared 10.1 of 30 questions on average, which is the third expected at
three times quota.

S1's test was rewritten here rather than deleted. It asserted a fixed
ordered bank of thirty; position no longer decides composition, so it
now asserts that every pool can fill a test and that the quotas sum to a
block.

**The decision, taken on 2026-10-07.** The existing 30 items were seeded
in a public migration on 2026-10-04. Deleting it would not un-publish
them, and rewriting public history is neither reliable nor compatible
with a repository whose pitch is that the diffs are in the open. So all
30 are retired and replaced by the 90, which never enter the repository.
The claim on the run page stays exactly as written, because once the
live bank is private it is simply true.

This is not only a data move, and the awkward part is the tests. The
balance test, the draw tests and the CI database all need *a* bank, and
they cannot have the real one.

- **A fixture bank, public.** A migration seeds 90 structurally valid
  placeholder items: right categories, right counts, right positions,
  and prompts that are obviously not the instrument ("Fixture A11:
  what is 2 + 2?"). CI and a laptop run against these. Every existing
  test keeps working, because all of them check shape and mechanics
  rather than content: the balance quota, the draw composition, the
  no-repeat constraint, the refusal on a depleted category.
- **The real bank, private.** `deploy/items-sync.sh`, following
  `corpus-sync.sh`, reads the JSON from `docs/personal/` and upserts it
  into production, deactivating the fixtures in the same transaction. A
  production database therefore holds the real 90 and no fixtures; a CI
  database holds the fixtures and no real items.
- **A guard against the obvious disaster:** a check that refuses to
  serve a run whose drawn items are fixtures, so a failed or forgotten
  sync shows up as a refusal rather than as a participant being asked
  what two plus two is.
- **Retire the old 30** in the same public migration that seeds the
  fixtures. They stay in the table, inactive, because nothing is ever
  deleted and the four runs already taken point at them.

*Exit:* CI green against fixtures; production serving the real bank;
`v_dt_item_pools` reporting `can_vary = true` in all four categories for
the first time; and a deliberate check that a fixture-backed run is
refused.

---

## Order and what gates what

S1 is done. S2 and S3 are one unit and should land together, because a
schema with no selector is a table nobody writes to. S4 is the first step
a participant could notice and is the point of no return: after it, the
served items come from the draw. S5 needs S4. S6 is the review surface.
S7 is content and can proceed at any time once S2 has landed, since the
pools are just items with a category.

**S2 to S6 are worth doing even if no new items are ever written**,
because they replace a hand-ordered convention with an enforced one and
make the no-repeat rule a database constraint. They will not, on their
own, make two participants' tests differ. Only S7 does that, and the plan
says so rather than letting the mechanism look like it is working.

## Risks

- **A short or unbalanced draw reaching a participant** is the failure
  that costs real data. S3 refuses rather than improvises, and that
  refusal is the first test written.
- **The mechanism looking like it works when it cannot.** With every
  pool exactly its quota, S4 will serve the same 30 items to everybody
  and nothing will look wrong. `v_dt_item_pools` exists so the shortfall
  is stated rather than discovered.
- **Per-item statistics thinning out.** A random draw spreads
  observations across more items, so item-level claims need more
  participants than before. Accepted deliberately by the owner in D2; the
  comparison he is protecting is category composition, and that is
  preserved exactly.
- **Participants are the scarce resource.** S1 to S3 change nothing a
  participant sees. S4 does, and should land when nobody is mid-test,
  which `deploy/rollout.sh` now refuses to do anyway.

## Decisions that are the owner's

*(D1, the categories, and D2, a random per-test draw rather than fixed
forms, were both settled on 2026-10-07.)*

1. **How big should the pools get?** Doubling the quota (60 items in
   total) makes the draw genuinely random. More is better and each item
   is real work. S7 is the only step that makes two participants' tests
   differ, so this is the decision that determines whether any of the
   rest changes what a participant sees.
2. **Who writes the new items?** Each needs a correct answer and a
   designed lure in an existing category. This is the largest single
   piece of work in the plan and it is content, not code.
3. **What should a repeat sitting do when the pool is too small to avoid
   repeats?** The plan falls back to the least recently seen and flags
   the run (D3a). The alternative is to refuse the sitting, which costs a
   willing volunteer. The recommendation is to flag rather than refuse.
