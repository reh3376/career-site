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

### D1. Keep four categories, not three

The ask suggested three (logic, practical math, common misconceptions).
The recommendation is to keep the four that exist:

| proposed category | existing family | per block | per test |
|---|---|---|---|
| practical math | `arithmetic` | 2 | 10 |
| logic | `syllogism` | 2 | 10 |
| base rates | `base_rate` | 1 | 5 |
| conjunction | `conjunction` | 1 | 5 |

**Why.** Four already divides a six-question block cleanly as 2/1/1/2,
and that quota is live and working. Collapsing base-rate and conjunction
into one "misconceptions" category would force the per-block split to be
re-derived for no measurement gain, and would lose a distinction the
instrument already reports on: `v_dt_items` gives per-family accuracy and
lure rates, and base-rate neglect and the conjunction fallacy are
different errors with different literatures.

The three-way framing still works for **reporting** — base rates and
conjunction roll up to "common misconceptions" in prose whenever that
reads better. The storage stays at four.

**Open to reversal** if the owner wants three for a reason the data does
not show. It costs a re-derived quota, nothing more.

### D2. Fixed parallel forms, not per-session random sampling

Two ways to make items vary:

**Per-session sampling** draws 30 from a larger pool for each
participant, honouring the quota. Maximum variation, and it defeats
practice completely. But every participant then sees a different test:
per-item statistics accumulate far more slowly, and with n in the low
tens `v_dt_items` would never say anything about any individual item.

**Parallel forms** build 2 or 3 fixed sets, each balanced to the same
quota. Two participants on form B are directly comparable, per-item
statistics accumulate, and it is what a published instrument does.

**Recommendation: forms.** The binding constraint on this instrument is
participants, not items. Anything that makes a given item's numbers take
longer to stabilise is the wrong trade at n = 2 independent runs.

### D3. A repeat sitting is served a form the participant has not seen

This is the point of the whole exercise and it needs no new identity
machinery: `prior_by_account`, `prior_by_email` and `prior_by_cookie` are
already recorded per session, and `dt_sessions` already records which
form each sitting used. Assignment rule, in order:

1. A form this participant has no recorded sitting on.
2. If they have seen them all, the one seen longest ago, flagged.
3. For a first sitting with no identity at all, the least-used form, so
   the forms fill evenly rather than form A carrying every anonymous run.

### D4. Forms are not assumed equivalent

Two forms balanced by category are **not** thereby equally hard. Until
there is evidence, a finding must not pool across forms.

`item_set_version` will encode the form (`items-2026-10-05-a`), which
means the existing grouping in `v_dt_load_curve` and `v_dt_calibration`
already keeps them apart. A later step adds a view that reports accuracy
per form so equivalence can be argued rather than assumed, and the
decision to pool stays the owner's.

### D5. Mechanism ships before content

Form A is today's 30 items, renamed. Forms B and C are **item-writing
work**, not code: 60 new items with correct and lure answers, matched to
category and difficulty. The code must therefore work correctly with
only form A present, and must refuse to serve an incomplete form.

---

## The steps

Each step is independently shippable and leaves the test working.

### S1. Make the existing balance enforced, and bump nothing

Add a test that reads the live item bank and asserts the quota: every
block of six is 2/1/1/2, every category total is 10/10/5/5, and there are
exactly 30 active scored items. No schema change, no behaviour change.

*Why first:* it locks in what is true today, so every later step has a
tripwire. It also fails loudly the next time someone edits the bank by
hand, which is the fragility that prompted this.

*Exit:* the test passes against production's bank, and fails if an item
is retired without a replacement in the same category.

### S2. Schema: forms, and the run's record of which one it drew

Migration adding:

- `dt_items.form text NOT NULL DEFAULT 'a'` — existing items become
  form `a` with no data change.
- `dt_sessions.item_form text NOT NULL DEFAULT ''` — which form the run
  drew. Empty for the runs taken before this existed, which is honest:
  they predate the concept and must not be relabelled as form `a`
  retrospectively.
- An index on `(form, kind, active, position)` for the selection query.
- A `v_dt_form_composition` view: per form, the per-block and per-test
  category counts. One definition of "is this form complete and
  balanced", readable from SQL and from the admin console.

*Exit:* migrations replay on an empty database; the existing 30 items
read back as form `a`; `v_dt_form_composition` reports form `a` as
complete and balanced.

### S3. Selection: serve a whole form, refuse a partial one

- `DTScoredItemsForForm(ctx, form)` replaces `DTScoredItems`, returning
  the form's 30 items in presentation order.
- **It returns an error unless the form is complete and balanced.** A
  half-written form B must never reach a participant; this is the
  guard that makes it safe to write items incrementally in production.
- `itemSetVersion` stops being a hand-edited constant and becomes
  derived: the bank version plus the form (`items-2026-10-05-a`). The
  same reasoning as `instrument_version` being derived from the timings,
  which exists precisely so there is no version to forget to bump.

*Exit:* a database test inserts a deliberately unbalanced form and
asserts the selection refuses it; form `a` still serves identically to
today, and a run started under it is byte-identical in its item
sequence to a run started before the change.

### S4. Assignment: pick the form at session start

- `DTPickForm(ctx, priorByAccount, priorByEmail, priorByCookie, visitorKey)`
  implementing D3, returning the form and why it was chosen.
- `StartDecisionTest` records `item_form` and the derived
  `item_set_version`.
- The reason is logged, not stored: which rule fired is useful when a
  run looks odd and is not a measurement.

*Exit:* a database test covering each branch — unseen form preferred,
all-seen falls back to least-recent, no-identity gets the least-used —
and a synthetic run through the real UI confirming the session row
carries the form it was served.

### S5. The form is visible everywhere a run is

- `DecisionTestRun.item_form` on the proto, the admin list and the run
  page, beside `item_set_version`.
- A form filter on the run list.
- `item_form` in both per-run CSVs and in the dataset export.

*Exit:* the live pass. The owner can see which form a run used without
opening `/admin/db`, which is the same bar S3 of the curation sprint was
held to.

### S6. Equivalence, before anything is pooled

- `v_dt_form_equivalence`: per form, per category, accuracy and mean
  confidence with counts, so two forms can be compared.
- `docs/metrics.md` gains the entry, including the plain statement that
  forms must not be pooled until this view says they can, and that at
  current n it cannot say so.

*Exit:* the view exists and reports honestly on one form, which is the
answer "not enough data" rather than a number.

### S7. Content: forms B and C

Sixty items, 30 per form, matched to the quota: 10 practical math, 10
logic, 5 base rate, 5 conjunction per form, each with a correct answer
and a designed lure. **Not code.** Each form is added to the bank
inactive, then activated once `v_dt_form_composition` reports it
complete, at which point S3's guard lets it be served.

*Exit:* `v_dt_form_composition` reports three complete forms and S4
starts assigning them.

---

## Order and what gates what

S1 first and alone: it protects everything after it. S2 and S3 are one
unit of work and should land together, because a schema with no selector
is a column nobody reads. S4 is the behaviour change and is the first
step a participant could notice. S5 is the review surface. S6 before any
finding is quoted across forms. S7 is content and can proceed in
parallel with S5 and S6 once S3's guard is in place.

**S1 to S6 are worth doing even if forms B and C are never written**,
because they convert an implicit hand-ordered convention into an enforced
one and make the version derived rather than remembered.

## Risks

- **A partial form reaching a participant** is the one failure that
  costs real data. S3's refusal is the guard and it is the first test
  written, not the last.
- **Silent pooling across forms** would make a finding wrong rather than
  absent. The existing `item_set_version` grouping prevents it as long as
  the version really does encode the form, which is why S3 derives it
  instead of leaving a constant to be edited.
- **Item quality drifting between forms.** Sixty new items written
  quickly will not match the originals' difficulty. S6 is the check, and
  until it has data the honest position is that forms are separate
  instruments.
- **Participants are the scarce resource.** Nothing in S1 to S6 changes
  what a participant sees, so none of it can cost a run. S4 changes which
  items they see and should land when no one is mid-test.

## Decisions that are the owner's

1. **Four categories or three?** The recommendation is four (D1), with
   base rates and conjunction rolled up to "common misconceptions" in
   prose only.
2. **Two forms or three?** Three gives a participant three clean
   sittings; two halves the writing. The code treats the count as data
   either way.
3. **Who writes forms B and C?** Sixty items with designed lures is the
   largest single piece of work here and it is content, not code.
4. **Should the existing runs be relabelled form `a`?** The
   recommendation is no (S2): they predate the concept, and an empty
   `item_form` is the truthful record.
