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

## The gate

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
