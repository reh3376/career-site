# Metrics

Every number about the reviewer is defined once, as a SQL view, in
migration `00028_metric_views.sql`. The api selects from those views and
the admin pages render what it returns. Nothing recomputes a metric in
Go or in a page.

That rule exists because two definitions of the same number is how a
dashboard starts disagreeing with itself, and then nobody can say which
one is lying. If a number here looks wrong, there is exactly one place
to go and read what it means.

## The views

| View | The question it answers |
| --- | --- |
| `v_jd_runs` | Every real pipeline run, with minutes and gate side derived. Evaluation runs are excluded: they are a test of the reviewer, not use of it, and mixing them flatters both latency and volume. |
| `v_reliability` | Of the last twenty real attempts to review a posting, how many finished without anyone stepping in. Runs the gatekeeper refused (`not_a_posting`) are excluded from the window: the pipeline read the input, decided it was not a posting and stopped, which is the system working rather than a run that failed to complete. |
| `v_judge_agreement` | For each reviewed requirement, the model's verdict beside the owner's. |
| `v_judge_agreement_summary` | The agreement rate, split into soft disagreements (one side said partial, the judge was unsure) and hard ones (met against unmet, the judge was wrong). Counting them together would hide which is happening. |
| `v_jd_latency` | Median and 95th percentile time to a result, with queue time reported apart from work time. |
| `v_llm_usage_daily` | Model calls, tokens, failures and average latency per day. Cost stays null on a self-hosted model; tokens are the real measure of what the box did. |
| `v_funnel_30d` | One row per anonymous visitor in the last 30 days, with the steps they reached. |
| `v_funnel_30d_summary` | That funnel as counts. |
| `v_eval_history` | Golden-set evaluations with their pass rate, for before-and-after comparison. |
| `v_outcome_by_fit` | The fit band a run assigned against what actually happened afterwards. |
| `v_gate` | The seven criteria as pass, not met, or unanswered, with the target each is judged against. Defined in migration 00031, one view per criterion. |

## Where the time went

`v_jd_latency` answers how long a review took. It does not answer where
that time went; `jd_runs.phase_ms` (migration 00042) does, per run:

```sql
SELECT p.key AS phase,
       round(avg(p.value::bigint)/1000.0) AS avg_seconds,
       count(*) AS runs
  FROM jd_runs r, jsonb_each_text(r.phase_ms) p
 WHERE r.finished_at > now() - interval '30 days'
 GROUP BY 1 ORDER BY 2 DESC;
```

There is no view over it yet, and three caveats apply. It is empty for
runs before 00042. It covers only the span between the first and last
progress report, so it does not sum to `duration_ms`. And an `other` key
means a stage was reworded and no longer matches the mapping in
`services/api/internal/jd/phases.go`: treat `other` as a bug in the
mapping rather than as a phase.

## The decision test

Defined in migration `00054_decision_test_metrics.sql`. Same rule: the
numbers exist once, in SQL, and the console renders what it is given.

The grain is `dt_answers`, one row per question presented. Everything
below counts from it and nothing is stored.

| View | The question it answers |
| --- | --- |
| `v_dt_answers` | The flat view, one row per question, with the session and participant context on it. This is also the CSV export. It carries no name, no email address and no `chosen_index`: `is_lure` says which kind of wrong an answer was, which is the measurement, while the raw index across enough runs would let somebody reconstruct the answer key from an export. |
| `v_dt_blocks` | One row per run per block: accuracy, confidence, the gap between them, expiry rate, and that block's recall. |
| `v_dt_sessions` | One row per run, including the early-against-hardest comparison the result email makes and the block 5 against block 1 fatigue control. |
| `v_dt_load_curve` | **The result.** Accuracy and confidence by load level, pooled across participants. Excludes synthetic runs outright rather than flagging them, because this view makes a claim about people. |
| `v_dt_calibration` | Confidence bands of ten against the accuracy actually achieved in each, split by load. The standard reliability curve. `overclaim_pct` is positive where a band is more sure than right. |
| `v_dt_threshold` | **The per-person answer.** The lowest load on the ramp at which this participant took the intended wrong answer while still reporting high confidence. |
| `v_dt_items` | Per item: accuracy, how often the lure pulled, the gap. A `lure_pct` near zero is an item measuring nothing. |

Two definitions had to be made rather than found, and both live in a
function so that changing one is a single line:

- **`dt_confident_threshold()` returns 70.** The research question is
  about being wrong *and* sure, so a number for "sure" is unavoidable.
  The FSD states the measure and deliberately leaves the cut open; this
  is a calibration to revisit once there are enough runs to see the
  distribution, and it is revisited **there**, not in a query.
- **`dt_load_rank(load)`** orders the loads, because `d4_plus1` sorts
  after `d4` for a reason rather than by accident of the alphabet.
  `d3_control` ranks **1**, the same as `d3`, because it is `d3`: block
  5 returns to block 1's difficulty and is the fatigue control.

### Two traps in these views

**The control block must not enter the per-person threshold.** Block 5
ranks as the easiest load, so a minimum taken over all five blocks
reports a participant who held through the ramp and then came apart at
the end as having a threshold at the *easiest* load. That is the
time-on-task confound the control exists to separate, readmitted through
a `min()`, and it is worst for precisely the participants the control is
there to find. `v_dt_threshold` takes `block_no < 5` and reports
`lured_in_control` beside it; a threshold with that flag set is a weaker
claim about load and should be read as one.

**An empty database proves a view parses and nothing else.** These views
are checked in CI against `testdata/dt_fixture.sql`, four runs with
deliberately different shapes, by `testdata/dt_metric_assertions.sql`.
The fixture includes a calibrated participant as a negative control and
a synthetic run that every aggregate must exclude. Both files exist
because the threshold bug above passed review, passed the replay, and
would have read as a strong result.

## What is not here

These views cover the reviewer. **Ask Roger has no views at all.** Its
measurements live in the raw event stream and in `decision_log`, and
`docs/events/README.md` carries the queries, including the near-miss
ranking that shows which questions almost matched the Q&A bank.

That is a deliberate wait rather than an oversight. A view is a
commitment to a definition, and the useful chat definitions are not
settled yet: 15 `chat_answer` rows is not enough to say what a good
answer rate is. When the shape of the question stops moving, the numbers
belong here under the same one-definition rule as everything else.

Migration `00031_gate_views.sql` turns those numbers into an answer.
One view per criterion (`v_gate_reliability` and the rest), unioned as
`v_gate`, each returning the same four things: `pass`, `value`,
`target`, `as_of`, plus a `detail` that qualifies the answer.

`pass` has three states and the third is the point:

| | meaning |
| --- | --- |
| `true` | the target is met on the data there is |
| `false` | the target is not met |
| `null` | not enough data yet |

A null is never rendered as a failure: a criterion with four data points
has not earned a verdict either way, and collapsing that into a failure
would make a young system read as broken.

`gated` is separate and says whether a row is a criterion at all. Reach
is measured and deliberately not gated (owner, 2026-09-24): whether
strangers find the site is demand, not quality, and a gate that cannot
be moved by doing good work teaches nothing. Model load is gated on the
failure rate, under 2 percent, which is the part of it that is a
property of the box rather than of how busy it was. Latency is not
repeated there because time to a result already gates it.

Agreement counts only submissions the posting-check gatekeeper accepted
(migration 00032). A metric that holds the judge accountable for input
the pipeline would now refuse measures the wrong thing. The gatekeeper
decides, not the verdict, so a refused posting is excluded whether its
verdicts were flattering or not.

It does count evaluation runs (migration 00034). `v_jd_runs` leaves
them out because it measures use, where a test run would flatter both
latency and volume. Agreement measures judgment, and a golden-set run
is the same pipeline reading the same corpus about a real posting, so
its verdicts are as good a sample as a live submission's. Excluding
them once threw away twenty-nine reviewed verdicts in an afternoon,
while `/admin/decisions` went on offering those same rows for review.

`/admin/gate` renders it. `/admin/analytics` shows the same measurements
arranged for reading.

## Reading them

`/admin/analytics` shows the headline numbers, arranged by the criteria
in the owner's response to the go-to-market plan: reliability,
agreement, time to a result, calibration, whether the score predicts
anything, reach, and model load. A criterion with no data says so rather
than showing a zero that reads like a failure.

For anything else, the read-only query console at `/admin/db` can select
from these views directly. The read-only role picks up new views
automatically through the default privileges set in migration 00005, so
adding a view needs no grant.

```sql
-- has agreement moved since the prompt change?
SELECT prompt_version, count(*), round(100.0 * count(*) FILTER (WHERE agreed) / count(*), 1) AS pct
  FROM v_judge_agreement GROUP BY prompt_version ORDER BY prompt_version;

-- did the last evaluation hold up against the one before it?
SELECT id, note, gate_pct, order_violations, margin, model, corpus_fingerprint
  FROM v_eval_history WHERE status = 'done' ORDER BY started_at DESC LIMIT 2;

-- where does time actually go?
SELECT status, count(*), round(avg(minutes), 1) AS avg_min, round(avg(queued_minutes), 1) AS avg_queued
  FROM v_jd_runs GROUP BY status;

-- does the decision test work? accuracy should fall as the rank rises
-- while confidence does not, so gap_pct grows.
SELECT block_load, load_rank, sessions, accuracy_pct, mean_confidence, gap_pct, confidently_lured_pct
  FROM v_dt_load_curve ORDER BY load_rank, block_load;

-- the per-person answer, with the control beside it. lured_in_control
-- true means read the threshold as a weaker claim about load.
SELECT session_key, accuracy_pct, gap_pct, lured_at_load, lured_in_control, fatigue_delta_pct
  FROM v_dt_threshold WHERE NOT is_synthetic ORDER BY lured_at_rank NULLS LAST;

-- is any item measuring nothing? a lure that never pulls is an item
-- everybody either knows or guesses.
SELECT item_code, item_family, answers, accuracy_pct, lure_pct, gap_pct
  FROM v_dt_items WHERE lure_pct < 10 ORDER BY lure_pct;
```

## Adding one

Add the view to a migration, add a field to `GetMetrics` if it belongs
on the dashboard, and add a row to the table above. Do not compute it in
the handler or the page, however small it seems: the first exception is
what makes the rule stop working.

A metric that only makes sense for one question does not need to reach
the dashboard at all. The query console reads the views directly, and a
number nobody looks at weekly is better left as a query in this file
than as a tile someone has to interpret.
