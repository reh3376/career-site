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

## M6. Analysis mechanisms

Collection without analysis is a pile of rows. This is the half of the
deliverable that makes the other half worth having.

**Build:** metric views under the one-definition rule, the CSV export
over the flat view, the admin surface for reading sessions and spotting
a collection fault early.

**Exit:** the confidence gap, accuracy by block, expiry rate by block
and recall failure by type each come from one view with one definition.
A fault is visible after one bad session, not forty.

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
