# Roadmap: the decision test

Sequencing for `docs/fsd-decision-test.md`. The design is settled; this
is the order to build it in and what has to be true before each step
ends.

**Not a sprint plan.** Milestones and exit criteria, no task breakdown.
The sprint plan comes after this is agreed.

---

## The constraint that shapes everything

**Volunteers are scarce, one-shot, and cannot be re-asked.** Recruitment
is a LinkedIn post asking people for fifteen minutes of real effort. A
person who gives that once and finds the test broken, or whose data is
discarded because the instrument was wrong, is not coming back. There is
no second ask.

Everything else follows from that. The plan spends cheap resources,
build time and the owner's own attention, to protect the expensive one.

Four assumptions are unproven and all of them are cheaper to test than
to be wrong about:

| Risk | If it is wrong | Found by |
|---|---|---|
| **The lures do not lure** | there is no effect to measure and the whole dataset is empty of signal | pilot |
| **20.5 s per question is too short** | expiries swamp blocks 3 and 4 and bury the real result | pilot |
| **The load does not bite** | blocks look alike and the research question goes unanswered | pilot |
| Tap-check thresholds are wrong | good participants fail at the door, or anyone passes | pilot |

The first is the one to worry about. Writing an item whose wrong answer
arrives fast and feels right, in two or three lines, is the hardest
content work in this project, and nothing in the build tells you whether
it worked.

---

## M1. The data layer and the item bank

Everything else writes into this, so it is first and it is built to
§7b rather than to whatever is quickest.

**Build:** `dt_participants`, `dt_sessions`, `dt_answers`, `dt_recalls`,
`dt_items` with the key, as migrations. The flat analysis view. Stable
identifiers, named relationships, one row per answer, identity separable
from measurement.

**Also:** `instrument_version`, `item_set_version`, `key_version` on
every session from the first row written. These are what let pilot data
and real data live in one table without contaminating each other, which
is what makes the rest of this plan possible.

**Exit:** a session can be written and read back through the flat view,
with provenance attached. The restore drill still passes.

## M2. The items

**The long pole. Start it at M1 and run it in parallel with
everything.** Thirty scored items plus practice items: base-rate
neglect, conjunction, and CRT-style arithmetic traps, generic, each
readable and answerable in about twenty seconds, none of them the
circulated classics.

Drafting can be assisted; **accepting them is the owner's**, item by
item. An item whose lure does not pull is a wasted question out of only
thirty.

**Exit:** thirty items in `dt_items`, each with its correct answer and
its intended lure, reviewed and accepted.

## M3. A runnable slice

Thin and vertical, deliberately. Enough to take the test end to end and
store a real session, not a polished product.

**Build:** the `(site)`-free run screen, Web Audio tick scheduled ahead,
the block and timing engine, server-side grading, answers posting as
they complete, the recall step.

**Exit:** the owner can take the full test start to finish and the
resulting rows are correct in the database.

## M4. Intake, completion, and the tap-check

**Build:** the effort warning first, before any data is collected; what
it involves; the sound requirement and the can-you-use-sound question;
the tap-along check; the intake form with its email note; the thank-you
and redirect.

**Exit:** a stranger can go from a cold link to a completed session
without help, and someone with no audio is caught at the tap-check
rather than at question nine.

## M5. Pilot and calibrate

**The milestone that protects the volunteers, and the one most likely to
send work backwards.** Expect to return to M2.

Small and known: the owner plus a handful of people who can be asked
twice because they are helping test the test, not providing data.

**Measure:** do the lures pull, under load and without it. Is 20.5
seconds enough. Do blocks 3 and 4 actually differ from blocks 1 and 2.
Does block 5 come back to block 1. What tap-check thresholds separate
real tracking from mashing.

**Exit:** thresholds set from data rather than guessed, items with dead
lures replaced, and timing confirmed achievable. Pilot sessions stay in
the database under their own `instrument_version`, excluded by flag and
never deleted.

## M5b. The live pass, and why automated tests cannot stand in for it

**DECIDED by the owner: the test is not operational until it has been
fully exercised through the UI on a deployed build.** Not a smoke check,
not a component test, not a scripted run against the API. The whole
thing, taken by people, in a browser, start to finish.

This is a stronger bar than the site's usual rule that UI work is unfinished
until it is deployed and looked at, and the reason is specific to this
build: **most of what makes this instrument valid is unobservable to a
test runner.**

| What has to be true | Why only a person can confirm it |
|---|---|
| The tick is audible at one per second | No automated check hears anything. A muted device, a wrong sample rate or audible drift all pass a unit test |
| The visual heartbeat is perceivable | Same, and it is the condition deaf participants run in |
| 20.5 seconds is enough to read, decide and rate | It is a claim about human reading speed. The arithmetic says it fits; only a person says it does |
| Two seconds is enough to memorise four digits | Same |
| The load actually loads | A machine holding four digits is not loaded by it |
| The instructions are understood | Particularly the syllogism framing. A participant who misreads it produces data indistinguishable from belief bias |

A green test suite here would mean the code does what it was told. It
would say nothing about whether the instrument measures what it claims
to, which is the only question that matters before volunteers are asked
for fifteen minutes each.

### What a full pass covers

Every path, not the happy one:

- **Both signal modes.** Sound, and the visual heartbeat, taken end to
  end separately. They are different conditions, not a setting.
- **Desktop and phone.** Phones are allowed and the device class is
  recorded; a four-digit entry on a touch keyboard eats into the same
  20.5 seconds.
- **All five blocks, in one sitting.** A fatigue control cannot be
  validated by testing block 1. The run has to be long enough to be
  tiring, which is the point.
- **A question allowed to expire**, confirming it moves on cleanly and
  lands as `expired` rather than as an error or a dropped row.
- **An abandoned run**, confirming the partial session is kept and
  marked rather than lost.
- **A repeat**, confirming it is flagged and not blocked.
- **The tap-check failing**, by muting the device deliberately, and the
  retry and the switch to visual both working.
- **No-email and with-email**, since they are different downstream
  commitments.

### And then the data is read, not assumed

A run that felt right and wrote the wrong rows has failed. After each
pass, from `v_dt_answers` alone:

- the session carries the right provenance, conditions and baseline
- thirty answers at the right grain, with latency and confidence on each
- recalls scored by failure type, with untransformed distinguished from
  wrong digits
- the headline comparison computes, and block 5 is near block 1

### Live passes do not pollute the dataset

This is what the provenance in M1 was for. Test runs carry their own
`instrument_version`, so they sit in the same tables as real data and
are excluded by flag rather than by deletion. Nothing has to be cleaned
up afterwards and nothing has to be run against a separate database to
keep the real set honest.

**Exit:** the owner has taken the full test on a deployed build, in both
signal modes, and the resulting rows have been read back and are
correct. Until that has happened the test is not operational, whatever
CI says.

## M6. Analysis mechanisms

Collection without analysis is a pile of rows. This is the half of the
deliverable that makes the other half worth having.

**Build:** metric views under the one-definition rule, the CSV export
over the flat view, the admin surface for reading sessions and spotting
a collection fault early.

**Exit:** the confidence gap, accuracy by block, expiry rate by block
and recall failure by type each come from one view with one definition.
A fault is visible after one bad session, not forty.

**Views done, 2026-10-05** (migration 00054, `docs/metrics.md`). Seven
of them: the flat grain, per block, per run, the load curve, the
calibration curve, the per-person threshold and item quality. The two
judgement calls, what counts as confident and how the loads order, are
functions rather than repeated expressions.

Checked in CI against a four-run fixture rather than only on an empty
database, which is what caught the one real bug: the per-person
threshold took its minimum over all five blocks, so a participant who
held through the ramp and then came apart on the fatigue control was
reported as having a threshold at the *easiest* load. It parsed, it
replayed, and it would have read as a strong result. The fixture carries
a calibrated participant as a negative control and a synthetic run every
aggregate must exclude.

**Export done, 2026-10-05.** `ExportDecisionTestData` renders
`v_dt_answers` as CSV from the admin console, with agent-driven runs
excluded unless asked for. The column list is read off the view at query
time rather than written out in Go: a hand-kept list is a second
definition of the dataset, and the first column added to one and not the
other ships an export narrower than the console while both still look
right.

**Still open in M6:** surfacing the load curve, the calibration curve
and the per-person threshold as console pages. The query console at
`/admin/db` reads all seven views today and `docs/metrics.md` carries
the queries, so the analysis is possible now and what is left is
convenience rather than capability.

## M7. Privacy, and opening to volunteers

**The privacy policy ships in the same PR as collection, not after it.**
This is the one rule that has already been broken once on this site: the
meeting scheduler ran for a week ahead of its disclosure. The policy
needs the four uses of the email address, the demographic fields, the
retention story, and the fact that an address is used to identify repeat
participants.

**Build:** policy section, events in `docs/events/README.md`, routes in
`PUBLIC_PATHS`, the live pass on a deployed build.

**Exit:** deployed, reviewed on the real site, policy accurate, and only
then the LinkedIn post.

## M8. Later: the graph projection

Deferred by [ADR 0030](adr/0030-relational-collection-graph-analysis.md)
and expected rather than precluded. Starts once there is data worth
exploring and the cross-surface questions are real. The projection
boundary is already specified, so this is an export rather than a
redesign.

---

## Order, and what can overlap

    M1 data layer  ──┬── M3 slice ── M4 intake ── M5 pilot ──┬── M6 analysis ── M7 open
    M2 items ────────┘                               ↑       │
                                                     └───────┘  expect to loop

M2 runs alongside M1, M3 and M4 and gates M5. M6 can start during M5,
since the pilot is the first thing that needs reading. M7 is last and is
the only irreversible step: once the post goes out, the instrument is
whatever it is.

**M5b gates M7 absolutely.** No volunteer is asked for fifteen minutes
until the owner has taken the whole thing himself on a deployed build
and the rows have been read back.

## M9. Curation and review (opened 2026-10-05)

**Asked for by the owner:** review all the data associated with each
test, add notes, mark a run `good` / `incomplete` / `do not use`, and
whatever else is needed to administer the data properly so that the
curated dataset from each test is fit to feed improvement algorithms.

The reason this is a milestone rather than a form: **the dataset is the
deliverable (§7b), and a dataset nobody has vouched for is collected,
not curated.** Thirty answers from somebody who was interrupted at
question twelve are indistinguishable in SQL from thirty answers given
under the conditions the instrument assumes. Only the owner knows which
is which, because the participants tell him and not the database.

### What exists already

`/admin/decision-test` lists every real run newest first, including
unfinished ones, with score, confidence, expiries, duration, device,
education and the instrument version. Each row opens a detail page with
the accuracy-against-confidence figures, the fatigue control, a
per-block table (correct, lure, expired, confidence, mean time, the
number shown, the recall outcome, memory lost) and every answer with
its outcome, confidence and latency.

All of it is read-only. There is nowhere to record a judgement.

### The gaps, specifically

**1. No judgement can be recorded at all.** No status, no note, no
reviewer, no date. The only lever is `is_synthetic`, which means
something else entirely and would be a lie.

**2. Curation would not reach the numbers.** `v_dt_load_curve`,
`v_dt_calibration` and `v_dt_items` make claims about people. A run
marked unusable that still feeds them is worse than no curation,
because the mark creates a false belief that the data has been cleaned.
The CSV export has the same problem and is the thing that actually
leaves for analysis.

**3. Fields that exist are not shown.** `recall_strategy` is the
debrief answer about whether the participant converted the number at
encoding or carried it and transformed at recall. §3.1 records it
precisely to turn an uncontrolled variable into a recorded one, and it
appears nowhere in the UI. Also absent: `age_range`, the tap-check
result, `baseline_rt_ms` and its standard deviation, and
`repeat_matched_by`. A reviewer deciding whether a run is sound cannot
see the conditions it ran under.

**4. Nothing records what an export contained.** If an analysis is run
today and a run is marked `do_not_use` next week, the analysis is no
longer reproducible and nothing says so. This project has already
learned this lesson twice on the reviewer side: the corpus fingerprint
captured identity but not content (closed by migration 00040), and an
evaluation that could not say what it was configured with cost an hour
the next time two scores disagreed. Retroactive curation without a
record is the same failure in a new place.

### Decisions

**DECIDED: curation is per session, not per answer.** That is the ask,
and a run is the unit a participant can report on. Block-level
exclusion is the thing most likely to be wanted next, and it is
deliberately not built: see the open question below.

**DECIDED: an unreviewed run counts as usable.** The alternative makes
the dataset empty until the owner has worked through a queue, and he
reviews asynchronously. The honest form of that trade is that **every
aggregate reports its own review composition**, so a figure computed
mostly from unreviewed runs says so on the same row. The precedent is
`/admin/analytics`: a criterion with no data says so rather than
showing a zero that reads like a failure.

**DECIDED: `do_not_use` is excluded from the three claim-making views
and from the export by default**, with an explicit opt-in on the export
exactly as `include_synthetic` works. Excluded, never deleted.

**DECIDED: an exclusion carries a reason.** `do_not_use` alone cannot
answer "how much data are we losing, and to what". The distinction that
matters is between *the instrument failed* and *this run was invalid*,
because they point at opposite fixes: the first is a bug to go and
repair, the second is nothing to fix and simply costs a data point.
Proposed values, adjustable: `instrument_fault`, `participant_reported`,
`duplicate`, `other`. The same split the decision log already makes
between disagreement and ungradeable.

**DECIDED: the vocabulary lives in Go and an unknown value is refused,
not stored.** Same rule as `ReviewVocabulary`, for the same reason: a
status nobody can interpret still lands in a count.

**DECIDED: review decisions are appended, not only overwritten.** The
current verdict sits on the session row so every query can filter on it
cheaply, and each change also appends to a history table. "Nothing is
ever deleted" has applied to the measurements since migration 00049;
there is no reason the judgements about them should be the one mutable
thing. It also answers "why is this excluded, and since when", which is
the question somebody will ask about a published finding.

**DECIDED: an export records what it contained.** Row count, session
count, the filters used, and a fingerprint over the session keys, so a
later curation change is visible as a difference rather than invisible.

### Answered by the owner, 2026-10-05

1. **`incomplete` does not exclude anything.** "Incomplete tests are not
   automatically blocked. When I review them I will make that
   determination." So the status describes the run and a single
   value decides its fate: **only `do_not_use` excludes.** An
   incomplete run he judges usable stays marked `incomplete` and counts;
   one he judges unusable becomes `do_not_use`. This replaces my
   proposal to split `incomplete` between the per-answer and
   session-level views, which would have been two rules where one does.

2. **Block-level exclusion, now.** "The individual tests need to have a
   block exclusion setting so that if I decide it should not be present
   I can mark the block as 'do not use'." A phone ringing during block
   three spoils that block and not the other four, and marking the whole
   run would discard twenty-four good answers. Same vocabulary as the
   session so there is one definition of the words, and again only
   `do_not_use` excludes.

3. **A fourth status: yes.** `hold`, for "something is odd and I have
   not decided". Without it an uncertain run is either left unreviewed,
   which is indistinguishable from not looked at, or marked with a
   certainty the reviewer does not have.

4. **Admin only.** Confirmed.

**The vocabulary, settled:** `good`, `incomplete`, `hold`,
`do_not_use`, and the empty string for not yet reviewed. It applies to a
session and to a block. Exactly one value excludes, which is what makes
"is this row usable" answerable in one place rather than per consumer.

**Sprint plan:** [`sprint-decision-test-curation.md`](sprint-decision-test-curation.md).
This document is milestones and exit criteria, per its own second
paragraph; the task breakdown lives there.

### What would make me stop and re-plan

- **The owner wants per-block curation after all.** S1 changes shape and
  is better done once than migrated twice.
- **Review turns out to be the bottleneck rather than volunteers.** If
  reviewing a run takes longer than taking one, the surface is wrong and
  the answer is fewer fields, not more.

## What would make me stop and re-plan

- **Pilot shows the lures do not pull.** Back to M2, and the question is
  item design rather than anything built.
- **Twenty seconds proves too short.** Either items get shorter or the
  question count drops, and the 15-minute ceiling is the owner's
  constraint rather than mine to trade away.
- **Blocks 3 and 4 look like block 2.** The transformation is loading
  encoding and recall but not the decisions, which §3.1 anticipated. The
  debrief question about strategy is what would tell us, and the fix
  would be a design change rather than a bug fix.

---

## Item pools and parallel forms (planned 2026-10-07)

A repeat sitting currently measures item recall: 27 of the 30 items are
shared between the two item sets, and a third sitting scored 30/30 at
half the answer time of a first-time participant.

The plan is
[`sprint-decision-test-item-pools.md`](sprint-decision-test-item-pools.md):
category-balanced parallel forms, a repeat served a form it has not
seen, and the per-block balance made enforced rather than implicit in
hand-ordered `position` values. **No code written.** Four decisions in
that document are the owner's, including whether the categories are
four or three and who writes the sixty new items.
