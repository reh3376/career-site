# Sprint plan: decision test curation and review

The task breakdown for **M9** of
[`roadmap-decision-test.md`](roadmap-decision-test.md), which holds the
milestone, the gap analysis and the decisions. Read that first; this is
only the order of work.

Asked for by the owner on 2026-10-05: review all the data associated
with each test, add notes, mark a run `good` / `incomplete` /
`do not use`, and whatever else properly administers the data so the
curated dataset from each test is fit to feed improvement algorithms.

**Four open questions in M9 are the owner's**, and the first of them
changes S1. They are listed there rather than repeated here.

---

**S1. The data layer and the vocabulary.** Migration: review columns on
`dt_sessions` (`review_status`, `review_reason`, `review_note`,
`reviewed_at`, `reviewed_by`) with a CHECK on the vocabulary, plus
`dt_session_reviews` for the history. Rebuild `v_dt_answers` to carry
`review_status`, and the three claim-making views to exclude
`do_not_use`. Go: the vocabulary, `DTReviewSession`, and the repo reads.
*Exit:* a run can be marked and the mark is visible in SQL; the curves
drop a `do_not_use` run; the fixture and assertions cover both, so a
view that forgets the filter fails the build.

**S2. The review surface.** Proto: `ReviewDecisionTestRun`, review
fields on `DecisionTestRun`, a status filter on the list. Web: the
status control, the reason, the note and a save on the detail page;
status tags and an unreviewed-first filter on the list with a count.
*Exit:* the owner can review a run end to end on a deployed build,
which is the only test that counts for UI here.

**S3. The whole record on one page.** Surface `recall_strategy`,
`age_range`, the tap-check result, `baseline_rt_ms` and its standard
deviation, and `repeat_matched_by`. Add the fields missing from `DTRun`
to the query rather than inventing new ones.
*Exit:* nothing a reviewer needs in order to judge a run requires
`/admin/db`.

**S4. Curation reaches the handoff.** Export excludes `do_not_use` by
default with an opt-in; export records what it contained; aggregates
carry their review composition.
*Exit:* a CSV can be tied to the curation state that produced it, and
no aggregate can be quoted without showing how much of it is
unreviewed.

**Order.** S1 gates everything. S2 is what the owner asked for and is
the point of the milestone. S3 and S4 are independent of each other and
either can follow. S4 before the first finding is quoted anywhere.


---

## Standing constraints

- **Nothing is deleted.** Curation marks and excludes; it never removes
  a row. That has held since migration 00049 and applies to the
  judgements as much as to the measurements.
- **Volunteers are the scarce resource.** No step here blocks a
  participant or changes the participant path, so none of it can cost a
  data point. That is why this milestone can proceed while volunteers
  are mid-test.
- **No UI work is finished until it is deployed and reviewed with the
  owner** (M5b's rule, and the reason S2's exit criterion is a live pass
  rather than a green suite).
