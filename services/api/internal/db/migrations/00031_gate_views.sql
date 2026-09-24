-- +goose Up
-- +goose StatementBegin

-- The criteria as a gate, not a dashboard (data layer D5, RR-13/14).
--
-- /admin/analytics already shows these numbers arranged by criterion.
-- A number invites interpretation; a gate does not. The difference
-- matters when the question is "is this good enough yet", because a
-- dashboard can be read favourably on a bad day and a pass cannot.
--
-- Every row answers with the same four things: pass, value, target,
-- as_of. Three states, and the third is the important one:
--
--   true   the target is met on the data there is
--   false  the target is not met
--   null   there is not enough data, or no target has been set
--
-- A null is never rendered as a failure. A criterion nobody has set a
-- target for is not failing, it is unanswered, and a criterion with
-- four data points has not earned a verdict either way. Collapsing
-- those into "false" would make the gate read as broken when it is
-- merely young, and then nobody would look at it.
--
-- Same rule as every other metric: defined once here, never recomputed
-- in Go or in a page.

-- 1. Reliability. Twenty consecutive runs that finished without anyone
-- stepping in. Fewer than twenty is not a failure, it is not yet an
-- answer.
CREATE VIEW v_gate_reliability AS
SELECT tenant_id,
       1 AS ord,
       'Reliability' AS criterion,
       CASE WHEN runs < 20 THEN NULL
            ELSE (failed = 0 AND stuck = 0)
       END AS pass,
       completed || ' of the last ' || runs || ' finished without intervention' AS value,
       'Twenty consecutive runs with no intervention.' AS target,
       newest AS as_of,
       CASE WHEN runs < 20
            THEN 'Only ' || runs || ' runs recorded; twenty are needed before this can pass or fail.'
            WHEN failed > 0 OR stuck > 0
            THEN failed || ' failed, ' || stuck || ' still running.'
            ELSE ''
       END AS detail
  FROM v_reliability;

-- 2. Agreement. The owner's verdict beside the model's, on the
-- requirements he has actually reviewed. A hard disagreement is met
-- against unmet, which is the judge being wrong rather than unsure, so
-- the target requires none of those rather than merely a good average.
CREATE VIEW v_gate_agreement AS
SELECT tenant_id,
       2 AS ord,
       'Agreement' AS criterion,
       CASE WHEN reviewed < 20 THEN NULL
            ELSE (agreement_pct >= 85 AND hard_disagreements = 0)
       END AS pass,
       COALESCE(agreement_pct::text, '0') || '% of ' || reviewed || ' reviewed verdicts' AS value,
       'At least 85 percent, with disagreements landing on partial rather than flipping met and unmet.' AS target,
       now() AS as_of,
       CASE WHEN reviewed < 20
            THEN 'Only ' || reviewed || ' verdicts reviewed; twenty are needed before this means anything.'
            WHEN hard_disagreements > 0
            THEN hard_disagreements || ' hard disagreements (met against unmet), ' || soft_disagreements || ' soft.'
            ELSE soft_disagreements || ' soft disagreements, none hard.'
       END AS detail
  FROM v_judge_agreement_summary;

-- 3. Time to a result.
CREATE VIEW v_gate_latency AS
SELECT tenant_id,
       3 AS ord,
       'Time to a result' AS criterion,
       CASE WHEN finished_runs < 5 THEN NULL
            ELSE (median_minutes < 20 AND p95_minutes < 45)
       END AS pass,
       'median ' || median_minutes || ' min, 95th ' || p95_minutes || ' min' AS value,
       'Median under 20 minutes, 95th percentile under 45. Queue time counted separately.' AS target,
       now() AS as_of,
       CASE WHEN finished_runs < 5
            THEN 'Only ' || finished_runs || ' finished runs.'
            ELSE 'Slowest ' || slowest_minutes || ' min, median queue ' || median_queued_minutes || ' min, over ' || finished_runs || ' runs.'
       END AS detail
  FROM v_jd_latency;

-- 4. Calibration. The most recent finished evaluation: every posting on
-- its expected side of the gate, and no inversion. An inversion is
-- worse than a miss because moving the gate cannot fix it.
CREATE VIEW v_gate_calibration AS
WITH latest AS (
  SELECT * FROM v_eval_history
   WHERE status = 'done' AND scored > 0
   ORDER BY started_at DESC LIMIT 1
)
SELECT tenant_id,
       4 AS ord,
       'Calibration' AS criterion,
       (gate_correct = scored AND order_violations = 0) AS pass,
       gate_correct || ' of ' || scored || ' on the expected side' AS value,
       'Every posting on its expected side of the gate, no inversions, holding across a change.' AS target,
       started_at AS as_of,
       order_violations || ' inversions, margin ' || COALESCE(round(margin::numeric, 3)::text, 'n/a')
         || ', model ' || COALESCE(model, 'unknown') AS detail
  FROM latest;

-- 5. Whether the score predicts anything. This is the question the
-- whole data layer exists to answer and it cannot be answered without
-- recorded outcomes, so it stays null rather than showing a zero that
-- reads like a failure.
CREATE VIEW v_gate_prediction AS
SELECT t.id AS tenant_id,
       5 AS ord,
       'Does the score predict anything' AS criterion,
       NULL::boolean AS pass,
       COALESCE((SELECT sum(submissions)::text FROM v_outcome_by_fit o WHERE o.tenant_id = t.id), '0')
         || ' runs with a recorded outcome' AS value,
       'Needs recorded outcomes. Until postings have results attached, this is unanswered rather than failing.' AS target,
       now() AS as_of,
       'Record outcomes on /admin/jd/[id] as they happen; the view is v_outcome_by_fit.' AS detail
  FROM tenants t;

-- 6. Reach, and 7. Model load. Both are real measurements with no
-- target set, so both report null rather than inventing a number to
-- pass against. Setting those targets is the owner's call, and a gate
-- that quietly makes one up on his behalf is worse than one that says
-- the target is missing.
CREATE VIEW v_gate_reach AS
SELECT tenant_id,
       6 AS ord,
       'Reach, last 30 days' AS criterion,
       NULL::boolean AS pass,
       landed || ' landed, ' || registered || ' registered, ' || submitted || ' submitted a posting' AS value,
       'No target set. Anonymous visitors through to a submitted posting.' AS target,
       now() AS as_of,
       'Set a target here and this becomes a gate rather than a count.' AS detail
  FROM v_funnel_30d_summary;

CREATE VIEW v_gate_model_load AS
SELECT tenant_id,
       7 AS ord,
       'Model load, last 30 days' AS criterion,
       NULL::boolean AS pass,
       sum(calls) || ' calls, ' || sum(failures) || ' failed' AS value,
       'No target set. What the box actually did.' AS target,
       max(day)::timestamptz AS as_of,
       COALESCE(sum(prompt_tokens + completion_tokens), 0) || ' tokens over ' || count(*) || ' days.' AS detail
  FROM v_llm_usage_daily
 WHERE day >= current_date - 30
 GROUP BY tenant_id;

-- The gate itself: seven rows, in the order the owner wrote them.
-- Ordering lives here rather than in the page for the same reason the
-- numbers do.
CREATE VIEW v_gate AS
SELECT * FROM v_gate_reliability
UNION ALL SELECT * FROM v_gate_agreement
UNION ALL SELECT * FROM v_gate_latency
UNION ALL SELECT * FROM v_gate_calibration
UNION ALL SELECT * FROM v_gate_prediction
UNION ALL SELECT * FROM v_gate_reach
UNION ALL SELECT * FROM v_gate_model_load;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS v_gate;
DROP VIEW IF EXISTS v_gate_model_load;
DROP VIEW IF EXISTS v_gate_reach;
DROP VIEW IF EXISTS v_gate_prediction;
DROP VIEW IF EXISTS v_gate_calibration;
DROP VIEW IF EXISTS v_gate_latency;
DROP VIEW IF EXISTS v_gate_agreement;
DROP VIEW IF EXISTS v_gate_reliability;
-- +goose StatementEnd
