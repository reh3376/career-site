-- +goose Up
-- +goose StatementBegin

-- Phase 0 step 6 (docs/framework-research/13 §2; FSD §5.15 FR-HF-08
-- and FR-HF-17): the match record is read by a person, and a reviewer
-- may override a judgment. The override columns, the CHECK that an
-- override carries a reason, a role and a person, and the generated
-- effective columns exist since 00066; nothing about the tables changes
-- here. What changes is what the two reading views carry, because the
-- viewer and any export read the record through them (14 §2.4) and they
-- stopped short of what a reader needs.

-- The record view learns which requirement versions were judged, in the
-- order they were judged, and how the run was started. requirement_ids
-- is the reading order of the record; trigger says whether this run
-- extracted the posting or judged a released list.
DROP VIEW IF EXISTS v_match_record;
CREATE VIEW v_match_record AS
SELECT m.run_id, m.tenant_id, m.role_id, m.candidate_id, m.attempt, m.supersedes_run_id,
       r.submission_id, r.status AS run_status,
       m.formula_version, m.evidenced, m.unresolved, m.unmet,
       m.coverage_equivalent_share, m.self_attested_share, m.override_count,
       cardinality(m.requirement_ids) AS requirement_count,
       m.keyword_baseline, m.versions, m.schema_version,
       r.model, r.num_ctx, r.embedder_model, r.prompts, r.corpus_fingerprint,
       r.score_formula AS run_score_formula, r.match_score AS run_match_score, r.fit,
       r.started_at, r.finished_at, m.created_at,
       m.requirement_ids, r.trigger AS run_trigger, r.corpus_documents, m.updated_at
FROM match_records m
JOIN jd_runs r ON r.run_id = m.run_id;
COMMENT ON VIEW v_match_record IS
  'One row per run: the three numbers, the two shares, override count, the requirement versions judged and provenance; what the viewer and exports read (FSD FR-HF-10, FR-HF-17).';

-- The judgment view learns what the viewer shows beside a verdict: the
-- evidence cited, the facts the model reported (the numbers the code
-- rules compared), the whole override (who, when, as what) and the
-- decision_log row that holds the prompt and the response, which is
-- what "audit" opens.
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

-- An override is found by the judgment it changed. The partial index
-- keeps "which judgments on this run carry an override" and the
-- agreement views off a scan of every judgment.
CREATE INDEX idx_judgments_overridden ON judgments (tenant_id, run_id) WHERE override_determination IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_judgments_overridden;

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
       q.edited_by IS NOT NULL AS requirement_edited
FROM judgments j
JOIN requirements q ON q.id = j.requirement_id;
COMMENT ON VIEW v_judgments_effective IS
  'Each judgment with the override-or-model value, the requirement version it judged, its class and inclusion, and the evidence kinds cited (FSD FR-HF-03, FR-HF-08, FR-HF-17, FR-HF-23).';

DROP VIEW IF EXISTS v_match_record;
CREATE VIEW v_match_record AS
SELECT m.run_id, m.tenant_id, m.role_id, m.candidate_id, m.attempt, m.supersedes_run_id,
       r.submission_id, r.status AS run_status,
       m.formula_version, m.evidenced, m.unresolved, m.unmet,
       m.coverage_equivalent_share, m.self_attested_share, m.override_count,
       cardinality(m.requirement_ids) AS requirement_count,
       m.keyword_baseline, m.versions, m.schema_version,
       r.model, r.num_ctx, r.embedder_model, r.prompts, r.corpus_fingerprint,
       r.score_formula AS run_score_formula, r.match_score AS run_match_score, r.fit,
       r.started_at, r.finished_at, m.created_at
FROM match_records m
JOIN jd_runs r ON r.run_id = m.run_id;
COMMENT ON VIEW v_match_record IS
  'One row per run: the three numbers, the two shares, override count and provenance; what the viewer and exports read (FSD FR-HF-10).';

-- +goose StatementEnd
