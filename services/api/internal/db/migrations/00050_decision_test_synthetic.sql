-- +goose Up
-- +goose StatementBegin

-- Marking a session that no human took.
--
-- The owner's plan is to drive the test UI with subagents once it is
-- built, to confirm the path works and the data pipeline behaves. That
-- is worth doing and it is good at things a human pilot is bad at:
-- every error branch, regression after a change, and volume. Fifty
-- synthetic sessions exercise the views, the aggregates and the export
-- in a way five real ones never will.
--
-- It also writes rows that are indistinguishable in shape from a
-- participant's: thirty answers, latencies, confidence ratings, a
-- recall per block. Left unmarked they would silently enter every
-- analysis, and the first symptom would be a correlation that does not
-- replicate with real people.
--
-- instrument_version is the wrong place to carry this, which is worth
-- saying because it is the obvious place. That column answers "which
-- version of the test was this", and the most useful agent run is one
-- against the exact version production is serving. Overloading it would
-- make testing the real instrument impossible without mislabelling the
-- run as a different instrument.
--
-- So: a separate flag, defaulting to false, and every analysis filters
-- on it. Synthetic rows are kept rather than deleted, like every other
-- excluded category here, because a pipeline regression is easiest to
-- find by comparing a known-good synthetic run against a later one.
ALTER TABLE dt_sessions
  ADD COLUMN IF NOT EXISTS is_synthetic boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN dt_sessions.is_synthetic IS
  'True when the session was driven by an agent rather than taken by a person. Deliberately separate from instrument_version so an agent can exercise the exact version production serves. Excluded from analysis by filter, never by deletion.';

-- A synthetic run is cheap to make and easy to forget about, so make it
-- cheap to exclude and hard to miss.
CREATE INDEX IF NOT EXISTS idx_dt_sessions_synthetic
    ON dt_sessions (tenant_id, started_at DESC) WHERE NOT is_synthetic;

-- The projection boundary carries it too (ADR 0030), or every consumer
-- has to remember the join and the filter. A column nobody can see is a
-- column nobody will filter on.
--
-- DROP then CREATE rather than CREATE OR REPLACE. Postgres will only
-- let a replace APPEND columns, and is_synthetic belongs beside status
-- where a reader will see it rather than tacked on the end. The first
-- version of this migration used CREATE OR REPLACE, which failed, left
-- the column on the table and absent from the view, and would have
-- shipped a flag that nothing could filter on.
DROP VIEW IF EXISTS v_dt_answers;

CREATE VIEW v_dt_answers AS
SELECT
  a.id                      AS answer_id,
  s.public_id               AS session_key,
  p.public_id               AS participant_key,
  s.status,
  s.is_synthetic,
  s.instrument_version,
  s.item_set_version,
  s.key_version,
  s.audio_mode,
  s.device_class,
  s.tap_check_passed,
  s.baseline_rt_ms,
  s.baseline_rt_sd_ms,
  s.is_repeat,
  s.recall_strategy,
  p.age_range,
  p.education,
  p.occupation,
  (p.email_key <> '')       AS gave_email,
  a.block_no,
  a.block_load,
  a.position_in_block,
  a.position_overall,
  a.item_code,
  a.item_version,
  i.family                  AS item_family,
  a.outcome,
  (a.outcome = 'correct')   AS is_correct,
  (a.outcome = 'lure')      AS is_lure,
  (a.outcome = 'expired')   AS is_expired,
  a.latency_ms,
  CASE WHEN s.baseline_rt_ms > 0 THEN a.latency_ms::numeric / s.baseline_rt_ms END
                            AS latency_vs_baseline,
  a.confidence,
  r.outcome                 AS recall_outcome,
  (r.outcome = 'untransformed') AS recall_executive_failure,
  s.started_at
FROM dt_answers a
JOIN dt_sessions s     ON s.id = a.session_id
JOIN dt_items i        ON i.id = a.item_id
LEFT JOIN dt_participants p ON p.id = s.participant_id
LEFT JOIN dt_recalls r ON r.session_id = a.session_id AND r.block_no = a.block_no;

COMMENT ON VIEW v_dt_answers IS
  'The projection boundary (ADR 0030). One row per answer with demographics, conditions, baseline, load and outcome already joined. Carries is_synthetic so agent-driven runs can be excluded at the boundary rather than by every consumer remembering to.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_dt_sessions_synthetic;
ALTER TABLE dt_sessions DROP COLUMN IF EXISTS is_synthetic;
-- +goose StatementEnd
