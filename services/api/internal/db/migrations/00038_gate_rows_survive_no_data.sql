-- +goose Up
-- +goose StatementBegin

-- A criterion with no data should say so, not vanish.
--
-- v_gate is a UNION ALL of seven sub-views, and six of them selected
-- straight from a view that groups by tenant. An empty source table
-- produces no group, so the sub-view produced no row, so the criterion
-- was absent from v_gate rather than present with pass = NULL. On a
-- database with no data the gate rendered one row instead of seven.
--
-- That inverts what 00031 set out to do. Its own header says a null
-- means "there is not enough data yet" and that collapsing it into
-- false "would make the gate read as broken when it is merely young".
-- A criterion that disappears does not get to be young: the reader is
-- not told it is waiting, they are not told it exists.
--
-- The shape that works was already in that migration.
-- v_gate_prediction selects FROM tenants and left joins its source, so
-- it always yields a row per tenant and reads NULL when there is
-- nothing to say. It is the only one of the seven that survived an
-- empty database, which is how the cause was found. The other six now
-- do the same.
--
-- Two things this has to get right beyond the join:
--
--   * `'a' || NULL` is NULL in SQL, and several of these build their
--     `value` and `detail` strings by concatenation. Every column from
--     the left-joined side is coalesced before it reaches a string, or
--     the row would appear with a null value, which is a different lie
--     from the one being fixed.
--   * the thresholds that decide NULL versus true/false are counts, so
--     a missing row must read as zero rather than as null, or the
--     comparison itself goes null and the CASE falls through.
--
-- Verified on a scratch database built from migrations 00001 to 00037
-- with no rows: seven criteria before and after, one row before, and
-- byte-identical output against production, which has data.

DROP VIEW IF EXISTS v_gate;

-- 1. Reliability.
CREATE OR REPLACE VIEW v_gate_reliability AS
SELECT t.id AS tenant_id,
       1 AS ord,
       'Reliability' AS criterion,
       CASE WHEN COALESCE(r.runs, 0) < 20 THEN NULL
            ELSE (r.failed = 0 AND r.stuck = 0)
       END AS pass,
       COALESCE(r.completed, 0) || ' of the last ' || COALESCE(r.runs, 0)
         || ' finished without intervention' AS value,
       'Twenty consecutive runs with no intervention.' AS target,
       r.newest AS as_of,
       CASE WHEN COALESCE(r.runs, 0) < 20
            THEN 'Only ' || COALESCE(r.runs, 0) || ' runs recorded; twenty are needed before this can pass or fail.'
            WHEN r.failed > 0 OR r.stuck > 0
            THEN r.failed || ' failed, ' || r.stuck || ' still running.'
            ELSE ''
       END AS detail,
       true AS gated
  FROM tenants t
  LEFT JOIN v_reliability r ON r.tenant_id = t.id;

-- 2. Agreement.
CREATE OR REPLACE VIEW v_gate_agreement AS
SELECT t.id AS tenant_id,
       2 AS ord,
       'Agreement' AS criterion,
       CASE WHEN COALESCE(a.reviewed, 0) < 20 THEN NULL
            ELSE (a.agreement_pct >= 85 AND a.hard_disagreements = 0)
       END AS pass,
       COALESCE(a.agreement_pct::text, '0') || '% of ' || COALESCE(a.reviewed, 0)
         || ' reviewed verdicts' AS value,
       'At least 85 percent, with disagreements landing on partial rather than flipping met and unmet.' AS target,
       now() AS as_of,
       CASE WHEN COALESCE(a.reviewed, 0) < 20
            THEN 'Only ' || COALESCE(a.reviewed, 0) || ' verdicts reviewed; twenty are needed before this means anything.'
            WHEN a.hard_disagreements > 0
            THEN a.hard_disagreements || ' hard disagreements (met against unmet), ' || a.soft_disagreements || ' soft.'
            ELSE a.soft_disagreements || ' soft disagreements, none hard.'
       END AS detail,
       true AS gated
  FROM tenants t
  LEFT JOIN v_judge_agreement_summary a ON a.tenant_id = t.id;

-- 3. Time to a result.
CREATE OR REPLACE VIEW v_gate_latency AS
SELECT t.id AS tenant_id,
       3 AS ord,
       'Time to a result' AS criterion,
       CASE WHEN COALESCE(l.finished_runs, 0) < 5 THEN NULL
            ELSE (l.median_minutes < 20 AND l.p95_minutes < 45)
       END AS pass,
       'median ' || COALESCE(l.median_minutes::text, 'n/a') || ' min, 95th '
         || COALESCE(l.p95_minutes::text, 'n/a') || ' min' AS value,
       'Median under 20 minutes, 95th percentile under 45. Queue time counted separately.' AS target,
       now() AS as_of,
       CASE WHEN COALESCE(l.finished_runs, 0) < 5
            THEN 'Only ' || COALESCE(l.finished_runs, 0) || ' finished runs.'
            ELSE 'Slowest ' || l.slowest_minutes || ' min, median queue ' || l.median_queued_minutes
                 || ' min, over ' || l.finished_runs || ' runs.'
       END AS detail,
       true AS gated
  FROM tenants t
  LEFT JOIN v_jd_latency l ON l.tenant_id = t.id;

-- 4. Calibration. The latest finished evaluation, per tenant. The old
-- version took LIMIT 1 across all tenants, which was also wrong the
-- moment there were two.
CREATE OR REPLACE VIEW v_gate_calibration AS
SELECT t.id AS tenant_id,
       4 AS ord,
       'Calibration' AS criterion,
       CASE WHEN e.scored IS NULL THEN NULL
            ELSE (e.gate_correct = e.scored AND e.order_violations = 0)
       END AS pass,
       COALESCE(e.gate_correct::text, '0') || ' of ' || COALESCE(e.scored::text, '0')
         || ' on the expected side' AS value,
       'Every posting on its expected side of the gate, no inversions, holding across a change.' AS target,
       e.started_at AS as_of,
       CASE WHEN e.scored IS NULL
            THEN 'No finished evaluation yet.'
            ELSE e.order_violations || ' inversions, margin '
                 || COALESCE(round(e.margin::numeric, 3)::text, 'n/a')
                 || ', model ' || COALESCE(e.model, 'unknown')
       END AS detail,
       true AS gated
  FROM tenants t
  LEFT JOIN LATERAL (
    SELECT * FROM v_eval_history h
     WHERE h.tenant_id = t.id AND h.status = 'done' AND h.scored > 0
     ORDER BY h.started_at DESC
     LIMIT 1
  ) e ON true;

-- 6. Reach. Measured, never gated.
CREATE OR REPLACE VIEW v_gate_reach AS
SELECT t.id AS tenant_id,
       6 AS ord,
       'Reach, last 30 days' AS criterion,
       NULL::boolean AS pass,
       COALESCE(f.landed, 0) || ' landed, ' || COALESCE(f.registered, 0)
         || ' registered, ' || COALESCE(f.submitted, 0) || ' submitted a posting' AS value,
       'Measured, not gated. Demand is not a property of the reviewer.' AS target,
       now() AS as_of,
       COALESCE(f.read_writing, 0) || ' read the writing, ' || COALESCE(f.clicked, 0)
         || ' clicked through.' AS detail,
       false AS gated
  FROM tenants t
  LEFT JOIN v_funnel_30d_summary f ON f.tenant_id = t.id;

-- 7. Model load.
CREATE OR REPLACE VIEW v_gate_model_load AS
SELECT t.id AS tenant_id,
       7 AS ord,
       'Model load, last 30 days' AS criterion,
       CASE WHEN COALESCE(m.calls, 0) < 50 THEN NULL
            ELSE (100.0 * m.failures / m.calls) < 2.0
       END AS pass,
       COALESCE(m.calls, 0) || ' calls, ' || COALESCE(m.failures, 0) || ' failed ('
         || COALESCE(round(100.0 * m.failures / NULLIF(m.calls, 0), 1)::text, '0') || '%)' AS value,
       'Under 2 percent of calls fail.' AS target,
       m.newest AS as_of,
       CASE WHEN COALESCE(m.calls, 0) < 50
            THEN 'Only ' || COALESCE(m.calls, 0) || ' calls; fifty are needed before a rate means anything.'
            ELSE COALESCE(m.tokens, 0) || ' tokens over ' || m.days
                 || ' days, average ' || round(m.avg_seconds, 1) || ' s a call.'
       END AS detail,
       true AS gated
  FROM tenants t
  LEFT JOIN (
    SELECT tenant_id,
           sum(calls) AS calls,
           sum(failures) AS failures,
           sum(prompt_tokens + completion_tokens) AS tokens,
           count(*) AS days,
           avg(avg_seconds) AS avg_seconds,
           max(day)::timestamptz AS newest
      FROM v_llm_usage_daily
     WHERE day >= current_date - 30
     GROUP BY tenant_id
  ) m ON m.tenant_id = t.id;

-- Rebuilt unchanged; it only needed dropping so the columns underneath
-- could be replaced.
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
-- No down path that is worth having: reverting would restore views that
-- silently drop criteria, and 00031 holds the original text if anyone
-- genuinely wants it back.
-- +goose StatementEnd
