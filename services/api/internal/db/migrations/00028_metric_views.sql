-- +goose Up
-- +goose StatementBegin

-- Every metric, defined once (data layer D5).
--
-- The rule, adopted from the go-to-market plan and worth keeping
-- whatever happens to that plan: a metric is defined here, in SQL, and
-- the application reads it. It is never recomputed in Go or in a page,
-- because two definitions of the same number is how a dashboard starts
-- disagreeing with itself and nobody can say which one is lying.
--
-- These views answer the criteria the owner set for "runs very well"
-- (docs/personal/review/GTM-response.md §4). One view per question.

-- Every real pipeline run, with the derived fields a reader wants.
-- Evaluation runs are excluded: they are a test of the reviewer, not
-- use of it, and mixing them would flatter both the latency and the
-- volume. v_eval_history covers them separately.
CREATE VIEW v_jd_runs AS
SELECT r.tenant_id,
       r.run_id,
       r.submission_id,
       r.attempt,
       r.trigger,
       r.status,
       r.error,
       r.model,
       r.host,
       r.num_ctx,
       r.app_commit,
       r.corpus_fingerprint,
       r.prompts,
       r.match_score,
       r.retrieval_score,
       r.threshold,
       r.fit,
       r.requirement_count,
       r.met_count,
       r.partial_count,
       r.unmet_count,
       r.resume_generated,
       r.queued_ms,
       r.duration_ms,
       round(r.duration_ms / 60000.0, 2) AS minutes,
       round(r.queued_ms / 60000.0, 2)   AS queued_minutes,
       CASE WHEN r.match_score IS NULL OR r.threshold IS NULL THEN NULL
            WHEN r.match_score >= r.threshold THEN 'above' ELSE 'below' END AS gate_side,
       r.started_at,
       r.finished_at
  FROM jd_runs r
  JOIN jd_submissions s ON s.id = r.submission_id
 WHERE NOT s.is_eval;

COMMENT ON VIEW v_jd_runs IS 'Real pipeline runs (evaluations excluded), with derived minutes and gate side.';

-- Reliability: the owner's criterion is twenty consecutive runs with no
-- manual intervention. "Completed" means it reached a terminal state
-- the submitter can act on; `failed` and a run still `running` long
-- after the fact are both intervention.
CREATE VIEW v_reliability AS
WITH recent AS (
  SELECT * FROM v_jd_runs ORDER BY started_at DESC LIMIT 20
)
SELECT tenant_id,
       count(*)                                          AS runs,
       count(*) FILTER (WHERE status IN ('ready', 'below_threshold')) AS completed,
       count(*) FILTER (WHERE status = 'failed')         AS failed,
       count(*) FILTER (WHERE status = 'running')        AS stuck,
       min(started_at)                                   AS oldest,
       max(started_at)                                   AS newest
  FROM recent
 GROUP BY tenant_id;

COMMENT ON VIEW v_reliability IS 'The last 20 real runs: how many finished without intervention.';

-- Agreement: where the owner reviewed a model verdict, did they agree?
-- Only requirement verdicts count. The gate rows are a different kind
-- of judgment and averaging them in would make the number mean nothing.
CREATE VIEW v_judge_agreement AS
SELECT d.tenant_id,
       d.id,
       d.run_id,
       d.ref_id       AS submission_id,
       d.key          AS requirement_id,
       d.model,
       d.prompt_id,
       d.prompt_version,
       d.output->>'verdict' AS model_verdict,
       d.human_verdict,
       (d.output->>'verdict') = d.human_verdict AS agreed,
       d.reviewed_at
  FROM decision_log d
 WHERE d.kind = 'jd_requirement_verdict'
   AND d.reviewed_at IS NOT NULL
   AND d.human_verdict IS NOT NULL;

COMMENT ON VIEW v_judge_agreement IS 'Reviewed requirement verdicts: the model''s call beside the owner''s.';

CREATE VIEW v_judge_agreement_summary AS
SELECT tenant_id,
       count(*)                          AS reviewed,
       count(*) FILTER (WHERE agreed)    AS agreed,
       round(100.0 * count(*) FILTER (WHERE agreed) / NULLIF(count(*), 0), 1) AS agreement_pct,
       -- A disagreement between met and unmet is a different problem
       -- from one that lands on partial: the first is the judge being
       -- wrong, the second is it being unsure. Counting them together
       -- would hide which one is happening.
       count(*) FILTER (WHERE NOT agreed AND 'partial' IN (model_verdict, human_verdict)) AS soft_disagreements,
       count(*) FILTER (WHERE NOT agreed AND 'partial' NOT IN (model_verdict, human_verdict)) AS hard_disagreements
  FROM v_judge_agreement
 GROUP BY tenant_id;

COMMENT ON VIEW v_judge_agreement_summary IS 'Agreement rate, split into soft (partial) and hard (met vs unmet) disagreements.';

-- Latency: the owner's criterion is a median under 20 minutes and a
-- 95th percentile under 45. Queue time is reported apart from work
-- time, because one says the box is busy and the other says the
-- pipeline is slow.
CREATE VIEW v_jd_latency AS
SELECT tenant_id,
       count(*) AS finished_runs,
       round((percentile_cont(0.5) WITHIN GROUP (ORDER BY duration_ms) / 60000.0)::numeric, 2)  AS median_minutes,
       round((percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms) / 60000.0)::numeric, 2) AS p95_minutes,
       round((percentile_cont(0.5) WITHIN GROUP (ORDER BY queued_ms) / 60000.0)::numeric, 2)    AS median_queued_minutes,
       round((max(duration_ms) / 60000.0)::numeric, 2) AS slowest_minutes
  FROM v_jd_runs
 WHERE finished_at IS NOT NULL AND status <> 'failed'
 GROUP BY tenant_id;

COMMENT ON VIEW v_jd_latency IS 'Median and 95th percentile time to a result, queue time kept separate.';

-- Model cost and volume by day. cost_usd stays null on a self-hosted
-- model, which is the current state; the tokens are the real measure
-- of what the box did.
CREATE VIEW v_llm_usage_daily AS
SELECT tenant_id,
       date_trunc('day', created_at)::date AS day,
       count(*)                            AS calls,
       count(*) FILTER (WHERE NOT ok)      AS failures,
       sum(prompt_tokens)                  AS prompt_tokens,
       sum(completion_tokens)              AS completion_tokens,
       round(avg(latency_ms) / 1000.0, 1)  AS avg_seconds,
       sum(cost_usd)                       AS cost_usd
  FROM llm_usage
 GROUP BY tenant_id, date_trunc('day', created_at)::date;

COMMENT ON VIEW v_llm_usage_daily IS 'Model calls, tokens, failures and latency per day.';

-- The landing funnel over 30 days, one row per visitor. The counts a
-- reader wants are sums of these booleans, which keeps the definition
-- of each step in one place.
CREATE VIEW v_funnel_30d AS
SELECT tenant_id,
       anon_id,
       bool_or(name = 'page.view' AND path = '/')  AS landed,
       bool_or(name = 'landing.cta_click')         AS clicked,
       bool_or(name = 'page.view' AND path LIKE '/articles%') AS read_writing,
       bool_or(name = 'register.submit')           AS registered,
       bool_or(name = 'verify.success')            AS verified,
       bool_or(name = 'login.success')             AS signed_in,
       bool_or(name = 'jd.submitted')              AS submitted,
       min(occurred_at)                            AS first_seen
  FROM events
 WHERE occurred_at > now() - interval '30 days'
   AND anon_id IS NOT NULL
 GROUP BY tenant_id, anon_id;

COMMENT ON VIEW v_funnel_30d IS 'One row per anonymous visitor in the last 30 days, with the steps they reached.';

CREATE VIEW v_funnel_30d_summary AS
SELECT tenant_id,
       count(*) FILTER (WHERE landed)       AS landed,
       count(*) FILTER (WHERE read_writing) AS read_writing,
       count(*) FILTER (WHERE clicked)      AS clicked,
       count(*) FILTER (WHERE registered)   AS registered,
       count(*) FILTER (WHERE verified)     AS verified,
       count(*) FILTER (WHERE signed_in)    AS signed_in,
       count(*) FILTER (WHERE submitted)    AS submitted
  FROM v_funnel_30d
 GROUP BY tenant_id;

COMMENT ON VIEW v_funnel_30d_summary IS 'The 30-day landing funnel as counts.';

-- Evaluations over time: the before-and-after record.
CREATE VIEW v_eval_history AS
SELECT tenant_id,
       id,
       eval_id,
       note,
       status,
       model,
       app_commit,
       corpus_fingerprint,
       prompts,
       threshold,
       total,
       scored,
       gate_correct,
       round(100.0 * gate_correct / NULLIF(scored, 0), 1) AS gate_pct,
       order_violations,
       margin,
       errors,
       started_at,
       finished_at
  FROM eval_runs;

COMMENT ON VIEW v_eval_history IS 'Golden-set evaluations with their pass rate, for before-and-after comparison.';

-- Does the score predict anything? This is the view the whole data
-- layer was built to make possible: the fit band a run assigned,
-- against what actually happened afterwards. It will say nothing until
-- outcomes are recorded, which is the honest state of it.
CREATE VIEW v_outcome_by_fit AS
SELECT r.tenant_id,
       r.fit,
       o.status AS outcome,
       count(*) AS submissions
  FROM v_jd_runs r
  JOIN jd_outcomes o ON o.submission_id = r.submission_id
 WHERE r.attempt = (SELECT max(attempt) FROM jd_runs x WHERE x.submission_id = r.submission_id)
 GROUP BY r.tenant_id, r.fit, o.status;

COMMENT ON VIEW v_outcome_by_fit IS 'Fit band against what happened in the world: whether the score predicts anything.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP VIEW IF EXISTS v_outcome_by_fit;
DROP VIEW IF EXISTS v_eval_history;
DROP VIEW IF EXISTS v_funnel_30d_summary;
DROP VIEW IF EXISTS v_funnel_30d;
DROP VIEW IF EXISTS v_llm_usage_daily;
DROP VIEW IF EXISTS v_jd_latency;
DROP VIEW IF EXISTS v_judge_agreement_summary;
DROP VIEW IF EXISTS v_judge_agreement;
DROP VIEW IF EXISTS v_reliability;
DROP VIEW IF EXISTS v_jd_runs;

-- +goose StatementEnd
