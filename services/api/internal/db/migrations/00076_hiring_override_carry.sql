-- +goose Up
-- +goose StatementBegin

-- The owner's decisions of 2026-11-13 on step 6's open questions
-- (docs/framework-research/14 §4; FSD FR-HF-08). One of them needs a
-- column: an override is carried to the next run of the same posting
-- for as long as the requirement it was set on is unchanged, so a
-- person does not decide the same row again after every re-score or
-- release. A carried override names the run it was set on. It is never
-- carried across an edit of the requirement: a new requirement version
-- is a new question.
ALTER TABLE judgments
  ADD COLUMN override_carried_from uuid REFERENCES jd_runs(run_id) ON DELETE SET NULL;
COMMENT ON COLUMN judgments.override_carried_from IS
  'The run a carried override was set on; NULL for an override set on this run and for a judgment with none (FSD FR-HF-08).';

-- The judgment view learns where a carried override came from and
-- whether it needs a second look. "Second look" is derived, never
-- stored: the model's verdict on this run differs from its verdict on
-- the run where the person decided, so what the person overrode is no
-- longer what the model says. Saving the override again on this run
-- makes it this run's own and clears the mark.
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

-- One decision by a person is one row of agreement, however many runs
-- it is carried to. Without this a verdict set once and carried across
-- five re-scores would count five times, and against five model
-- answers the person never looked at. A carried override saved again
-- on a later run is that run's own and counts there, because then the
-- person did look.
CREATE OR REPLACE VIEW v_determination_agreement AS
SELECT tenant_id, prompt_version, model,
       determination AS model_determination,
       override_determination,
       determination = override_determination AS agreed,
       count(*) AS judgments
FROM judgments
WHERE override_determination IS NOT NULL AND override_carried_from IS NULL
GROUP BY tenant_id, prompt_version, model, determination, override_determination;
COMMENT ON VIEW v_determination_agreement IS
  'A person''s override versus the model''s determination, by prompt version and model, counting each decision once, on the run it was made (FSD FR-HF-08; successor to v_judge_agreement).';

CREATE OR REPLACE VIEW v_basis_agreement AS
SELECT tenant_id, prompt_version, model,
       basis AS model_basis,
       override_basis,
       basis = override_basis AS agreed,
       count(*) AS judgments
FROM judgments
WHERE override_basis IS NOT NULL AND override_carried_from IS NULL
GROUP BY tenant_id, prompt_version, model, basis, override_basis;
COMMENT ON VIEW v_basis_agreement IS
  'A person''s override basis versus the model''s basis, by basis code, counting each decision once: which kind of equivalency is misjudged (FSD FR-HF-08).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

CREATE OR REPLACE VIEW v_determination_agreement AS
SELECT tenant_id, prompt_version, model,
       determination AS model_determination,
       override_determination,
       determination = override_determination AS agreed,
       count(*) AS judgments
FROM judgments
WHERE override_determination IS NOT NULL
GROUP BY tenant_id, prompt_version, model, determination, override_determination;
COMMENT ON VIEW v_determination_agreement IS
  'Owner override versus model determination, by prompt version and model (FSD FR-HF-08; successor to v_judge_agreement).';

CREATE OR REPLACE VIEW v_basis_agreement AS
SELECT tenant_id, prompt_version, model,
       basis AS model_basis,
       override_basis,
       basis = override_basis AS agreed,
       count(*) AS judgments
FROM judgments
WHERE override_basis IS NOT NULL
GROUP BY tenant_id, prompt_version, model, basis, override_basis;
COMMENT ON VIEW v_basis_agreement IS
  'Owner override basis versus model basis, by basis code: which kind of equivalency is misjudged (FSD FR-HF-08).';

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
       j.decision_id
FROM judgments j
JOIN requirements q ON q.id = j.requirement_id
LEFT JOIN users ou ON ou.id = j.override_by;
COMMENT ON VIEW v_judgments_effective IS
  'Each judgment with the override-or-model value, the whole override, the requirement version it judged with its class and inclusion, the evidence cited and the facts reported (FSD FR-HF-03, FR-HF-08, FR-HF-17, FR-HF-23).';

ALTER TABLE judgments DROP COLUMN IF EXISTS override_carried_from;

-- +goose StatementEnd
