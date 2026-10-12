# FR-DT-20: the tap-along check's thresholds

**State as of 2026-10-12.** The check and its arithmetic are built and
verified correct: the FSD's claim that a 150 ms standard-deviation cut
is "unreachable" holds up exactly, because the measured quantity is a
one-sided offset bounded to [0, 300] ms, and the maximum standard
deviation of any such bounded variable, at a perfectly bimodal
distribution sitting on the two edges, is exactly half the range, 150
ms. The real blocker is not that too few sessions exist, though that is
also true; it is that the check gates its own data collection. A real
(non-synthetic) participant cannot reach the rest of the test, and
therefore cannot produce a stored `dt_sessions` row at all, until they
pass the current 80 ms cut. Every failed attempt is scored entirely in
the browser, is never sent to the server in any form, not even as an
analytics event, and leaves no trace anywhere. So the one dataset that
could set this threshold empirically, a labeled mix of genuine tracking
and genuine mashing, cannot exist under the current instrumentation no
matter how many real participants take the test: the only tap-check
outcomes that ever reach the database are ones that already passed the
threshold being evaluated.

## What the FSD requires

`docs/FSD.md:571`, FR-DT-20:

> The tap-along check shall separate real tracking from mashing, on
> thresholds set from pilot data rather than guessed.

Status column: "Should (partially **Shipped**)". Notes column, in
full:

> The check is built and its result and baseline reaction time are
> recorded on every session. The cut is provisional: at least seven of
> ten presses inside 300 ms with an offset standard deviation at or
> under 80 ms. The design document's starting figure of 150 ms is
> unreachable, since the maximum standard deviation inside a 300 ms
> window is 150 ms, so it could never have refused anyone.

## What exists today

**The check, and its exact thresholds.**
`apps/web/src/lib/metronome.ts:134-171` (`scoreTapCheck`): for each tap,
the distance to the nearest metronome tick is kept only if it is `<=
0.3` seconds (`:145`); a participant is scored on three conditions
(`:153-163`):

```ts
const notMashing = tapTimes.length <= tickTimes.length + 2;
const enoughHits = hits >= Math.ceil(tickTimes.length * 0.7);
const steady = hits < 3 ? false : sd <= 80;
```

i.e. not more taps than ticks (plus slack of two), at least 70% of
ticks hit within the 300 ms window, and (once there are at least three
hits) a standard deviation of 80 ms or under among the hit offsets.

**The measurement window is exactly ten ticks**, confirming the FSD's
"seven of ten" is literal, not approximate. The metronome ticks once
per second (`metronome.ts:33`, `TICK_INTERVAL_S = 1.0`), and the scored
window is `MEASURE_MS = 10_000`
(`apps/web/src/app/(site)/decision-test/briefing.tsx:136`), after a
3-second unscored lead-in (`:135`, `LEAD_IN_MS`) explicitly added
because the first few taps of orientation were inflating both the mean
offset and the standard deviation for people doing nothing wrong
(`briefing.tsx:120-135`, comment).

**The 150 ms figure's unreachability is arithmetically correct, and the
80 ms figure's justification is stated in the code, not just in the
FSD.** `metronome.ts:155-162`, the comment directly above `steady`:

> 80 ms, and the number is arithmetic rather than taste. Taps that all
> land inside the 300 ms window can have a standard deviation of at
> most 150 ms (perfectly bimodal at the two edges), so a 150 ms cut is
> close to unreachable and the consistency test would never fire.
> Measured reference points: somebody genuinely tracking a metronome
> lands around 8 ms of jitter, and taps spread uniformly across the
> window land around 96 ms. 80 sits between them with room on both
> sides.

Checking the claim directly: `scoreTapCheck` only retains offsets with
`best <= 0.3` (`:145`), so every retained value is a one-sided distance
in `[0, 300]` ms, never negative. For a distribution confined to an
interval `[0, L]`, variance is maximized by putting all mass at the two
endpoints; a two-point distribution at `0` and `L` with equal weight
has mean `L/2` and variance `(L/2)^2`, so standard deviation `L/2`. At
`L = 300`, that is exactly 150 ms. The FSD's reasoning holds exactly,
not approximately.

**What is actually persisted per session: two summary numbers, not the
underlying taps.** `dt_sessions` has
(`services/api/internal/db/migrations/00049_decision_test.sql:205-215`):

```sql
tap_check_passed    boolean     NOT NULL DEFAULT false,
baseline_rt_ms       int,
baseline_rt_sd_ms    int,
```

The client sends exactly these three values, once, in
`StartSession`'s `conditions`
(`briefing.tsx:217-219`: `tapCheckPassed: tap?.passed ?? false`,
`baselineRtMs: tap?.rt ?? 0`, `baselineRtSdMs: tap?.sd ?? 0`), and the
handler stores them as-is
(`services/api/internal/handlers/decisiontest.go:121-124`). So the
session's final mean offset and standard deviation genuinely are
recorded, which is better than a bare pass/fail flag, but the
individual tap-by-tap offsets, the hit count, and which attempt (first,
second, third) produced the stored numbers are not; only the one
summary from whichever attempt finally passed is kept.

**The check gates its own data: a real participant cannot produce a
failing row.** `briefing.tsx:544-550`:

```tsx
{tapPhase === "idle" && (tap?.passed || synthetic) ? (
  <button type="button" onClick={() => setStep("form")} ...>
    {tap?.passed ? "Continue" : "Continue without passing"}
  </button>
) : null}
```

The Continue button, which leads to the intake form and then to
`StartSession`, only renders when `tap.passed` is true, unless
`synthetic` is set (and synthetic runs are excluded from every
real-participant aggregate by design, per FR-DT-13). A real participant
who fails can retry (`attempts` increments, `:175`), and after two
failed attempts is offered a switch to the visual on-screen beat
(`:522-532`), but there is no path for a real, non-synthetic
participant to proceed with a failing result. The comment directly
above that button confirms this was deliberate, for a different reason
(keeping agents out, not pilot data):

> A synthetic run may continue without passing. Tapping in rhythm is a
> human motor skill: an agent driving the page cannot produce it, and
> Continue only ever appeared once the check passed, so no agent could
> reach the rest of the test at all. ... `tapCheckPassed` is still
> recorded truthfully from whatever the check actually found, so the
> row says the check was not passed rather than pretending it was.

That last sentence is true only in the sense that no row with
`tap_check_passed = false` can exist for a real participant at all,
since reaching `StartSession` already required passing.

**Failed attempts leave no trace anywhere, not even analytics.** A
search for `track(` (the client-side event helper) in `briefing.tsx`
returns nothing. The `attempts` counter is local component state only;
it is never sent to the server in any form, as an activity event, a
product event, or otherwise. So there is no record, anywhere in the
system, of how many participants failed the check at all, how many
attempts they took, or what their failing offsets looked like, even in
aggregate.

**How few real sessions exist: not independently re-verified here, but
documented elsewhere as very few.** `docs/backlog.md:945-951` (dated
through the 2026-10-08 close of that section) states plainly: "Two real
runs exist and neither can speak to load," for an unrelated reason (an
item-family confound fixed by migration 00052). `docs/in-flight.md:75-82`
repeats the same count. Neither source gives a count as of 2026-10-12
specifically.

## What is missing

1. A labeled dataset of genuine mashing. Under the current
   instrumentation, one cannot exist: the gate (`briefing.tsx:544-550`)
   ensures every stored real session already passed.
2. Any server-side or analytics record of failed attempts: no event,
   no row, no count survives past the browser tab that produced it.
3. Per-attempt detail beyond the final passing attempt's summary: which
   attempt number succeeded, and what the failing attempts' offsets and
   hit counts looked like, are not recorded even for the session that
   does get stored.
4. A current count of real (non-synthetic) `dt_sessions` rows as of
   2026-10-12; the two-session figure on record predates this date.

## Dependencies and constraints

- The gate exists for a stated, different, and still-valid reason
  (excluding agents, FR-DT-13a); removing it to collect failing rows
  would reopen that problem unless done carefully (e.g. recording
  failed attempts without letting them substitute for passing the
  real gate).
- `baseline_rt_ms` / `baseline_rt_sd_ms` already double as the
  unloaded-reaction-time baseline that every in-test latency is read
  against (`00049_decision_test.sql:211-214`, comment); any change to
  what triggers a stored session has to preserve that meaning rather
  than, say, storing failing attempts as if they were valid baselines.
- Setting a threshold from data needs both classes represented: real
  tracking and real mashing. The current design can only ever supply
  the first class from genuine participants; the second class would
  have to come from a deliberately different source (staff or testers
  instructed to mash on purpose, recorded separately from the
  participant-facing gate) rather than from waiting for more
  volunteers under the status quo.
- `docs/fsd-open-items.md:150-156` already frames this correctly as
  "blocked on participant data, not on code," but that framing predates
  (or did not consider) the gating mechanism documented here; "more
  participant data" alone does not resolve this unless the gate or the
  recording changes too.

## Decisions the owner must make

1. Whether to record failed attempts at all (as a new, explicit
   signal, not as alternate session rows), which requires either a new
   lightweight event on each `scoreTapCheck` call or a change to what
   `StartSession` is told.
2. Whether to deliberately collect labeled mashing examples from a
   non-participant source (staff, testers) rather than relying on real
   volunteers to ever produce one, given the gate makes the latter
   structurally impossible.
3. Whether the existing 80 ms cut should simply stay as the shipped
   value, documented as "chosen from measured reference points, not
   yet validated against a labeled dataset," rather than treating
   "set from pilot data" as blocking Shipped status indefinitely; this
   is a legitimate option FR-DT-20's own wording ("rather than
   guessed") does not clearly rule out, since 80 ms was not guessed,
   it was derived from the stated reference points.
4. Whether per-attempt detail (not just the final passing summary) is
   worth adding to `dt_sessions` or a side table, given it would make
   any future threshold analysis richer without changing the gate.

## Task checklist

1. Decide whether this item is "close the gap in instrumentation" or
   "accept 80 ms as the shipped value with its current justification"
   (Decision 3) before doing anything else; the remaining steps assume
   the former.
2. If closing the gap: add a lightweight, non-session-creating record
   of each `scoreTapCheck` call (pass/fail, mean offset, sd, attempt
   number), sent regardless of whether the participant ever passes.
3. Decide and, if pursued, execute a deliberate mashing-data collection
   pass (Decision 2), since real participants cannot supply it.
4. Re-run the arithmetic check in this document against whatever
   dataset results, rather than assuming the current 80 ms figure
   transfers once real pass/fail-labeled data exists.
5. Update `docs/FSD.md:571`'s status and notes once a threshold is
   actually set from data, however that data was obtained, rather than
   from the measured reference points cited in the code comment today.

## What could not be verified

- The actual current count of real (non-synthetic), completed
  `dt_sessions` rows as of 2026-10-12. This environment's local
  Postgres container has no `dt_sessions` table at all (`\dt` lists 34
  tables, none of the `dt_*` family; `goose_db_version` is at 48,
  before migration 00049 that creates them), and no production
  database access was available from here. The "two real runs" figure
  is carried from `docs/backlog.md` and `docs/in-flight.md` as
  documented, not re-counted.
- Whether any mechanism outside `briefing.tsx` (e.g., a different entry
  path into the decision test) can produce a `dt_sessions` row with
  `tap_check_passed = false` for a non-synthetic participant. Only the
  one entry path, the briefing screen, was checked.
- Whether the owner's two recorded pilot runs happened to pass by a
  wide or a narrow margin, which would speak to how much headroom the
  current 80 ms cut has; `baseline_rt_sd_ms` for those specific rows
  was not read from a live database.
