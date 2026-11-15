-- +goose Up
-- +goose StatementBegin

-- Phase 0 step 7 (docs/framework-research/13 §2; FSD §5.15 FR-HF-13,
-- 14, 22 and 24): the clarification loop. An unresolved judgment is a
-- question the system could not answer from the documents it had, and
-- the candidate is usually the cheapest and most accurate source of the
-- answer. The tables exist since 00067; this is what the loop needs
-- that they did not foresee.

-- One evidence item per answered question. An answer saved again
-- replaces its item rather than adding a second, so a judgment never
-- cites two versions of one statement.
CREATE UNIQUE INDEX uq_evidence_items_clarification_item
  ON evidence_items (clarification_item_id) WHERE clarification_item_id IS NOT NULL;

-- A request learns which posting it belongs to (so a candidate's list
-- of open questions is one read), whether the run's fit is withheld
-- while it is open (FR-HF-22), when the candidate was reminded, and how
-- and by which run it was closed. `held` is false for a request the
-- candidate asked for on a run that had already been given its fit.
ALTER TABLE clarification_requests
  ADD COLUMN submission_id  bigint REFERENCES jd_submissions(id) ON DELETE CASCADE,
  ADD COLUMN held           boolean     NOT NULL DEFAULT false,
  ADD COLUMN reminded_at    timestamptz,
  ADD COLUMN closed_at      timestamptz,
  ADD COLUMN closed_reason  text        NOT NULL DEFAULT '',   -- answered | expired | superseded
  ADD COLUMN closing_run_id uuid REFERENCES jd_runs(run_id) ON DELETE SET NULL;
COMMENT ON COLUMN clarification_requests.held IS
  'Whether the run carries no fit while this request is open (FSD FR-HF-22).';
CREATE INDEX idx_clarification_requests_open ON clarification_requests (tenant_id, status, expires_at) WHERE status = 'sent';
CREATE INDEX idx_clarification_requests_submission ON clarification_requests (tenant_id, submission_id);

-- An item keeps the requirement's id within the role for display, and
-- the fields it offered as they were offered: `options` is a list of
-- fields, each with its label, its kind and the choices drawn for it,
-- every choice naming where it came from (own_corpus or generic_list).
ALTER TABLE clarification_items
  ADD COLUMN requirement_local_id text NOT NULL DEFAULT '';
CREATE INDEX idx_clarification_items_requirement ON clarification_items (tenant_id, requirement_id, answer_status);

-- Vocabulary 1.2: the activity classes a disambiguation answer chooses
-- from (04 §4.1). A generic, requirement-independent list, so choosing
-- from it tells the candidate nothing about what a requirement is
-- scored on. Additive, so a minor version; the judge's lists are
-- untouched and its fingerprint does not move.
INSERT INTO vocab_codes (vocab, code, label, description, introduced_in, sort_order) VALUES
  ('activity_class', 'engineering_design',   'Engineering design',   'Designing or specifying a physical system, plant or equipment.', '1.2', 1),
  ('activity_class', 'software_development', 'Software development', 'Writing or architecting software.', '1.2', 2),
  ('activity_class', 'project_delivery',     'Project delivery',     'Planning, estimating or managing a project through to completion.', '1.2', 3),
  ('activity_class', 'commissioning',        'Commissioning',        'Bringing a system into service: start-up, testing, qualification.', '1.2', 4),
  ('activity_class', 'operations',           'Operations',           'Running or maintaining a system or plant day to day.', '1.2', 5),
  ('activity_class', 'leadership',           'Leadership',           'Leading people, a team or an organisation.', '1.2', 6),
  ('activity_class', 'commercial',           'Commercial',           'Budget, procurement, vendor or customer work.', '1.2', 7),
  ('activity_class', 'research_teaching',    'Research or teaching', 'Research, writing or teaching others.', '1.2', 8),
  ('activity_class', 'other',                'Other',                'A kind of work not on this list.', '1.2', 9);

-- The judgment view learns which clarification items a judgment
-- consumed and which judgment it supersedes, so the record can say a
-- verdict rests on the candidate's own statement and what it was
-- before.
DROP VIEW IF EXISTS v_judgments_effective;
CREATE VIEW v_judgments_effective AS
SELECT j.id, j.tenant_id, j.run_id, j.candidate_id, j.requirement_id, j.requirement_local_id,
       q.text AS requirement_text, q.version AS requirement_version, q.type AS requirement_type,
       q.category, q.weight,
       j.determination, j.basis, j.uncertainty,
       j.effective_determination, j.effective_basis, j.derived_verdict,
       j.override_determination IS NOT NULL AS overridden, j.override_role, j.override_reason,
       j.rationale, j.rule_adjustments, j.provenance_summary, j.stale,
       (SELECT array_agg(DISTINCT e.source_kind ORDER BY e.source_kind)
          FROM evidence_items e WHERE e.id = ANY (j.evidence_ids)) AS evidence_kinds,
       cardinality(j.evidence_ids) AS evidence_count,
       j.prompt_version, j.prompt_fingerprint, j.model, j.created_at,
       q.class AS requirement_class, q.included AS requirement_included,
       q.edited_by IS NOT NULL AS requirement_edited,
       j.evidence_ids, j.reported_facts,
       j.override_determination, j.override_basis, j.override_by, j.override_at,
       COALESCE(ou.name, '') AS override_by_name,
       j.decision_id,
       j.override_carried_from,
       cr.attempt AS override_carried_attempt,
       o.determination AS override_model_determination,
       o.basis AS override_model_basis,
       o.id IS NOT NULL AND (o.determination <> j.determination OR o.basis <> j.basis) AS override_recheck,
       j.clarification_item_ids, j.supersedes
FROM judgments j
JOIN requirements q ON q.id = j.requirement_id
LEFT JOIN users ou ON ou.id = j.override_by
LEFT JOIN jd_runs cr ON cr.run_id = j.override_carried_from
LEFT JOIN judgments o ON o.run_id = j.override_carried_from AND o.requirement_id = j.requirement_id;
COMMENT ON VIEW v_judgments_effective IS
  'Each judgment with the override-or-model value, the whole override and where a carried one came from, the requirement version it judged with its class and inclusion, the evidence cited, the facts reported and the clarification items it consumed (FSD FR-HF-03, FR-HF-08, FR-HF-14, FR-HF-17, FR-HF-23).';

-- What the candidate has been asked and has not yet answered, one row
-- per open item, with the posting it belongs to and the time limit.
CREATE VIEW v_clarification_open AS
SELECT i.id AS item_id, i.tenant_id, r.id AS request_id, r.submission_id, r.run_id, r.candidate_id,
       r.held, r.generated_at, r.expires_at, r.reminded_at,
       i.requirement_local_id, i.uncertainty_code, i.answer_type, i.answer_status, i.asked_by
FROM clarification_items i
JOIN clarification_requests r ON r.id = i.request_id
WHERE r.status = 'sent' AND i.answer_status = 'pending';
COMMENT ON VIEW v_clarification_open IS
  'Clarification items waiting on the candidate: which posting, since when, and by when (FSD FR-HF-13, FR-HF-22).';

-- How the loop is doing: per uncertainty code, how many questions were
-- asked, how they were answered and what the judgment became. The
-- signal for whether asking resolves what it was meant to.
CREATE VIEW v_clarification_outcomes AS
SELECT i.tenant_id, i.uncertainty_code, i.answer_type, i.answer_status,
       nj.effective_determination AS determination_after,
       count(*) AS items
FROM clarification_items i
LEFT JOIN judgments nj ON nj.run_id = i.rejudge_run_id AND nj.requirement_id = i.requirement_id
GROUP BY i.tenant_id, i.uncertainty_code, i.answer_type, i.answer_status, nj.effective_determination;
COMMENT ON VIEW v_clarification_outcomes IS
  'Per uncertainty code: questions asked, how they were answered, and what the judgment became after (FSD FR-HF-14).';

-- A run that asked the candidate a question and rests with no fit
-- (status awaiting_candidate, FSD FR-HF-22) finished its work without
-- intervention: it counts as completed in the reliability figure, as a
-- run that reached a fit does. Left out, every review that waited on an
-- answer would read as one that never finished. Replaced in place, with
-- its columns unchanged, because v_gate reads it.
CREATE OR REPLACE VIEW v_reliability AS
WITH recent AS (
  SELECT *
    FROM v_jd_runs
   WHERE status <> 'not_a_posting'
   ORDER BY started_at DESC
   LIMIT 20
)
SELECT tenant_id,
       count(*)                                          AS runs,
       count(*) FILTER (WHERE status IN ('ready', 'below_threshold', 'awaiting_candidate')) AS completed,
       count(*) FILTER (WHERE status = 'failed')         AS failed,
       count(*) FILTER (WHERE status = 'running')        AS stuck,
       min(started_at)                                   AS oldest,
       max(started_at)                                   AS newest
  FROM recent
 GROUP BY tenant_id;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

CREATE OR REPLACE VIEW v_reliability AS
WITH recent AS (
  SELECT *
    FROM v_jd_runs
   WHERE status <> 'not_a_posting'
   ORDER BY started_at DESC
   LIMIT 20
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

DROP VIEW IF EXISTS v_clarification_outcomes;
DROP VIEW IF EXISTS v_clarification_open;

DROP VIEW IF EXISTS v_judgments_effective;
CREATE VIEW v_judgments_effective AS
SELECT j.id, j.tenant_id, j.run_id, j.candidate_id, j.requirement_id, j.requirement_local_id,
       q.text AS requirement_text, q.version AS requirement_version, q.type AS requirement_type,
       q.category, q.weight,
       j.determination, j.basis, j.uncertainty,
       j.effective_determination, j.effective_basis, j.derived_verdict,
       j.override_determination IS NOT NULL AS overridden, j.override_role, j.override_reason,
       j.rationale, j.rule_adjustments, j.provenance_summary, j.stale,
       (SELECT array_agg(DISTINCT e.source_kind ORDER BY e.source_kind)
          FROM evidence_items e WHERE e.id = ANY (j.evidence_ids)) AS evidence_kinds,
       cardinality(j.evidence_ids) AS evidence_count,
       j.prompt_version, j.prompt_fingerprint, j.model, j.created_at,
       q.class AS requirement_class, q.included AS requirement_included,
       q.edited_by IS NOT NULL AS requirement_edited,
       j.evidence_ids, j.reported_facts,
       j.override_determination, j.override_basis, j.override_by, j.override_at,
       COALESCE(ou.name, '') AS override_by_name,
       j.decision_id,
       j.override_carried_from,
       cr.attempt AS override_carried_attempt,
       o.determination AS override_model_determination,
       o.basis AS override_model_basis,
       o.id IS NOT NULL AND (o.determination <> j.determination OR o.basis <> j.basis) AS override_recheck
FROM judgments j
JOIN requirements q ON q.id = j.requirement_id
LEFT JOIN users ou ON ou.id = j.override_by
LEFT JOIN jd_runs cr ON cr.run_id = j.override_carried_from
LEFT JOIN judgments o ON o.run_id = j.override_carried_from AND o.requirement_id = j.requirement_id;
COMMENT ON VIEW v_judgments_effective IS
  'Each judgment with the override-or-model value, the whole override and where a carried one came from, the requirement version it judged with its class and inclusion, the evidence cited and the facts reported (FSD FR-HF-03, FR-HF-08, FR-HF-17, FR-HF-23).';

DELETE FROM vocab_codes WHERE vocab = 'activity_class';

DROP INDEX IF EXISTS idx_clarification_items_requirement;
ALTER TABLE clarification_items DROP COLUMN IF EXISTS requirement_local_id;

DROP INDEX IF EXISTS idx_clarification_requests_submission;
DROP INDEX IF EXISTS idx_clarification_requests_open;
ALTER TABLE clarification_requests
  DROP COLUMN IF EXISTS closing_run_id,
  DROP COLUMN IF EXISTS closed_reason,
  DROP COLUMN IF EXISTS closed_at,
  DROP COLUMN IF EXISTS reminded_at,
  DROP COLUMN IF EXISTS held,
  DROP COLUMN IF EXISTS submission_id;

DROP INDEX IF EXISTS uq_evidence_items_clarification_item;

-- +goose StatementEnd
