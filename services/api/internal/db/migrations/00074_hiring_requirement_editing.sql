-- +goose Up
-- +goose StatementBegin

-- Phase 0 step 5 (docs/framework-research/13 §2; FSD §5.15 FR-HF-01,
-- 02, 03 and 23): the requirement becomes an editable entity. The
-- extractor writes draft rows with a type, the posting's own
-- alternatives, thresholds, an evidence standard rendered from the
-- type's template and equivalence notes from the catalog; a person with
-- the admin role edits a draft before judging; an edit is a new
-- version; judging runs against released versions and every judgment
-- names the version it judged.

-- Class is how much a requirement counts, set by the hiring manager
-- (FR-HF-23): required at full weight, preferred at a fraction of its
-- weight, ideal as bonus credit only. `category` stays what the posting
-- said (must, nice); class starts from it and is what the arithmetic
-- reads from here on.
ALTER TABLE requirements
  ADD COLUMN class text NOT NULL DEFAULT 'required';
UPDATE requirements SET class = CASE WHEN category = 'must' THEN 'required' ELSE 'preferred' END;
COMMENT ON COLUMN requirements.class IS
  'required | preferred | ideal (vocab requirement_class): how the item counts, set by the hiring manager; starts from category (FSD FR-HF-23).';

-- The hiring manager's own statement of what is accepted in place of
-- the requirement as worded ("a graduate engineering degree counts as
-- two of the ten years"). Apart from equivalence_notes, which are the
-- catalog's known equivalents: this one is the definition FR-HF-23
-- asks for before a years or credential item counts.
ALTER TABLE requirements
  ADD COLUMN equivalency text NOT NULL DEFAULT '';
COMMENT ON COLUMN requirements.equivalency IS
  'What the hiring manager accepts in place of the requirement as worded; a years or credential item counts only when this or a posting alternative exists (FSD FR-HF-23).';

-- Whether this version counts in the numbers, decided in code when the
-- version is written (hiring.Inclusion, rule incl-v1) and frozen on the
-- row, so a later change to the rule does not rewrite what an earlier
-- run was computed from. A years or credential item with no equivalency
-- and no alternative is shown on the record and left out of the numbers.
ALTER TABLE requirements
  ADD COLUMN included boolean NOT NULL DEFAULT true;
COMMENT ON COLUMN requirements.included IS
  'Counts in the numbers; false for a years or credential item with no equivalency defined (FSD FR-HF-23).';

-- When a draft was released for judging and by whom. A null released_by
-- on a released row is the pipeline releasing its own extraction, which
-- is every row before this migration and every run that is not held for
-- review.
ALTER TABLE requirements
  ADD COLUMN released_at timestamptz,
  ADD COLUMN released_by bigint REFERENCES users(id) ON DELETE SET NULL;
UPDATE requirements SET released_at = created_at WHERE status IN ('active', 'retired');
COMMENT ON COLUMN requirements.released_at IS
  'When this version was released for judging; null while it is a draft (FSD FR-HF-03).';

-- One released version and at most one pending draft per requirement.
-- An edit of a released requirement is a draft beside it until the
-- drafts are released together, so a run in between judges what was
-- released and never a half-edited list.
CREATE UNIQUE INDEX uq_requirements_one_active ON requirements (role_id, local_id) WHERE status = 'active';
CREATE UNIQUE INDEX uq_requirements_one_draft  ON requirements (role_id, local_id) WHERE status = 'draft';

-- Vocabulary 1.1: the requirement classes, and the three small
-- vocabularies thresholds are written in (04 §1.2). Additive, so a
-- minor version (04 §0.2 rule 1); the judge's four lists are untouched
-- and its fingerprint does not move.
INSERT INTO vocab_codes (vocab, code, label, description, introduced_in, sort_order) VALUES
  ('requirement_class', 'required',  'Required',  'The posting requires it. Counts at its full weight.', '1.1', 1),
  ('requirement_class', 'preferred', 'Preferred', 'The posting prefers it. Counts at a fraction of its weight, a quarter unless the setting says otherwise.', '1.1', 2),
  ('requirement_class', 'ideal',     'Ideal',     'Good to have. Earns bonus credit only and never counts against a candidate.', '1.1', 3),

  ('scope_unit', 'team_size',   'Team size',   'People led or managed.', '1.1', 1),
  ('scope_unit', 'budget_usd',  'Budget',      'Money owned or managed, in US dollars.', '1.1', 2),
  ('scope_unit', 'users',       'Users',       'People served by the system.', '1.1', 3),
  ('scope_unit', 'sites',       'Sites',       'Plants, facilities or locations.', '1.1', 4),
  ('scope_unit', 'io_points',   'I/O points',  'Input and output points of a control system.', '1.1', 5),
  ('scope_unit', 'systems',     'Systems',     'Systems owned or integrated.', '1.1', 6),
  ('scope_unit', 'revenue_usd', 'Revenue',     'Revenue responsible for, in US dollars.', '1.1', 7),
  ('scope_unit', 'other',       'Other',       'A measure not on this list.', '1.1', 8),

  ('credential_level', 'certificate', 'Certificate', 'A certification or certificate.', '1.1', 1),
  ('credential_level', 'associate',   'Associate',   'An associate degree.', '1.1', 2),
  ('credential_level', 'bachelor',    'Bachelor',    'A bachelor''s degree.', '1.1', 3),
  ('credential_level', 'master',      'Master',      'A master''s degree.', '1.1', 4),
  ('credential_level', 'doctorate',   'Doctorate',   'A doctorate.', '1.1', 5),
  ('credential_level', 'license',     'Licence',     'A professional licence.', '1.1', 6),

  ('relationship_kind', 'employed_by',   'Employed by',   'Worked for the organisation.', '1.1', 1),
  ('relationship_kind', 'contracted_to', 'Contracted to', 'Worked for it under contract.', '1.1', 2),
  ('relationship_kind', 'customer_of',   'Customer of',   'Bought from it and worked with its people.', '1.1', 3),
  ('relationship_kind', 'partner_of',    'Partner of',    'Worked with it as a partner.', '1.1', 4),
  ('relationship_kind', 'any_working',   'Any working relationship', 'Any of the above.', '1.1', 5);

-- How a requirement finds the catalog entries that speak to it (05
-- §1.3 step 1). A concept matches a requirement when one of its terms
-- occurs in the requirement's text or source quote, on word boundaries;
-- a term ending in * matches as a prefix. Only concepts something can be
-- equivalent to need terms: the `to` side of a one-way entry and both
-- sides of a symmetric one. Phrases, not single common words, so a
-- note does not attach itself to every requirement that says "design".
ALTER TABLE concepts
  ADD COLUMN match_terms text[] NOT NULL DEFAULT '{}';
COMMENT ON COLUMN concepts.match_terms IS
  'Lower-case phrases whose presence in a requirement makes this concept apply; a trailing * is a prefix match (05 §1.3).';

UPDATE concepts c SET match_terms = v.terms
FROM (VALUES
  ('tool', 'siemens_tia_portal',  ARRAY['tia portal', 'siemens', 'step 7', 'simatic']),
  ('tool', 'rockwell_studio5000', ARRAY['studio 5000', 'rockwell', 'allen-bradley', 'allen bradley', 'controllogix', 'compactlogix', 'rslogix']),
  ('tool', 'ignition_scada',      ARRAY['ignition']),
  ('tool', 'aveva_system_platform', ARRAY['wonderware', 'aveva', 'system platform']),
  ('tool', 'scada_platform',      ARRAY['scada']),
  ('tool', 'power_bi',            ARRAY['power bi', 'powerbi']),
  ('tool', 'relational_database', ARRAY['relational database*', 'rdbms', 'sql database*']),
  ('tool', 'compiled_backend_language', ARRAY['compiled language*', 'statically typed language*']),
  ('tool', 'machine_learning_frameworks', ARRAY['machine learning framework*', 'ml framework*', 'deep learning framework*']),
  ('vocabulary', 'front_end_loading_fel', ARRAY['front-end loading', 'front end loading', 'fel']),
  ('vocabulary', 'front_end_development_capital_projects', ARRAY['front-end development', 'front end development', 'front-end engineering', 'front end engineering']),
  ('vocabulary', 'front_end_development_software', ARRAY['front-end development', 'front end development', 'frontend development']),
  ('vocabulary', 'co_manufacturing',       ARRAY['co-manufactur*', 'co manufactur*', 'comanufactur*', 'co-man', 'co-packer*', 'copacker*']),
  ('vocabulary', 'contract_manufacturing', ARRAY['contract manufactur*']),
  ('vocabulary', 'mes', ARRAY['mes', 'manufacturing execution']),
  ('vocabulary', 'manufacturing_operations_management', ARRAY['mom', 'manufacturing operations management']),
  ('vocabulary', 'iiot_data_architecture', ARRAY['iiot', 'industrial iot', 'industrial internet of things']),
  ('title', 'controls_engineer',   ARRAY['controls engineer*', 'control engineer*']),
  ('title', 'automation_engineer', ARRAY['automation engineer*']),
  ('title', 'director_head_of_ai', ARRAY['director of ai', 'head of ai', 'ai director', 'vp of ai']),
  ('title', 'engineering_manager', ARRAY['engineering manager*', 'manager of engineering', 'engineering management']),
  ('credential', 'bs_engineering_field', ARRAY['bachelor*', 'b.s.', 'bs in', 'bsc', 'undergraduate degree', 'engineering degree']),
  ('credential', 'ms_engineering_field', ARRAY['master''s', 'masters', 'master of', 'm.s.', 'ms in', 'msc', 'graduate degree']),
  ('credential', 'formal_plc_training',  ARRAY['plc training', 'training in plc*', 'plc programming training']),
  ('domain', 'food_beverage_manufacturing', ARRAY['food and beverage', 'food & beverage', 'f&b', 'beverage manufactur*', 'food manufactur*']),
  ('domain', 'regulated_manufacturing',     ARRAY['regulated manufactur*', 'regulated industr*', 'regulated environment*']),
  ('domain', 'pharmaceutical_manufacturing_gmp', ARRAY['pharmaceutical*', 'pharma', 'gmp', 'cgmp', 'biotech*']),
  ('method', 'partnership_with_vendor', ARRAY['partner with', 'partnership with', 'partnerships with', 'strategic partner*', 'technology partner*', 'vendor relationship*']),
  ('method', 'designing_system',        ARRAY['system design', 'systems design', 'designing', 'architect*']),
  ('method', 'large_capital_project_experience', ARRAY['large capital project*', 'major capital project*', 'large-scale capital', 'major investments'])
) AS v(kind, slug, terms)
WHERE c.tenant_id = 1 AND c.kind = v.kind AND c.slug = v.slug;

-- The dual-write check reads the class. The v2 score the gate reads
-- counts a required item always and any other only when it earned
-- something; before this migration that was category must and nice,
-- and for a requirement nobody has edited the two say the same thing.
-- The recompute also applies under formula v4, which writes coverage,
-- not the v2 score, to `evidenced` exactly as v3 does.
DROP VIEW IF EXISTS v_dual_write_agreement;
CREATE VIEW v_dual_write_agreement AS
WITH per_run AS (
  SELECT j.run_id,
         bool_or(q.class = 'required') AS has_must,
         sum(q.weight * CASE j.derived_verdict WHEN 'met' THEN 1.0 WHEN 'partial' THEN 0.5 ELSE 0.0 END) AS weighted,
         sum(q.weight) FILTER (WHERE q.class = 'required' OR j.derived_verdict IN ('met', 'partial')) AS denom_must,
         sum(q.weight) AS denom_all,
         count(*) AS judgment_rows
  FROM judgments j
  JOIN requirements q ON q.id = j.requirement_id
  WHERE q.type <> 'behavioral'
  GROUP BY j.run_id
), v2 AS (
  SELECT run_id, judgment_rows,
         CASE WHEN has_must THEN weighted / NULLIF(denom_must, 0) ELSE weighted / NULLIF(denom_all, 0) END AS score
  FROM per_run
)
SELECT r.run_id, r.tenant_id, r.submission_id, r.attempt, r.status, r.finished_at,
       r.match_score, m.evidenced, m.formula_version,
       v2.score AS judgments_v2_score, v2.judgment_rows,
       r.requirement_count, cardinality(m.requirement_ids) AS requirement_rows,
       m.run_id IS NOT NULL AS record_written,
       m.run_id IS NOT NULL
         AND r.requirement_count = cardinality(m.requirement_ids)
         AND CASE
               WHEN m.formula_version IN ('v3-three-numbers', 'v4-classes')
                 THEN v2.score IS NOT NULL AND abs(COALESCE(r.match_score, 0) - v2.score) < 1e-9
               ELSE (r.match_score IS NULL AND m.evidenced IS NULL
                     OR abs(COALESCE(r.match_score, 0) - COALESCE(m.evidenced, 0)) < 1e-9)
             END AS agrees
FROM jd_runs r
LEFT JOIN match_records m ON m.run_id = r.run_id
LEFT JOIN v2 ON v2.run_id = r.run_id
JOIN jd_submissions s ON s.id = r.submission_id
WHERE r.status IN ('generating', 'below_threshold', 'ready')
   OR m.run_id IS NOT NULL;
COMMENT ON VIEW v_dual_write_agreement IS
  'Per run: does the match record agree with jd_runs on the requirement count and, under v2 on the score or under v3 and v4 on the v2 score recomputed from the judgment rows (14 §4; FSD FR-HF-07).';

-- The judgment view learns the class and whether the item counted, so
-- the viewer of step 6 and any export read them from one place.
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

-- What a person edited, and from what: one row per human edit with the
-- version it replaced beside it. The edit reason is the signal for
-- extractor calibration (04 §1.1), and recurring edits to the evidence
-- standard of one type are the signal to revise that type's template
-- (05 §2.4).
CREATE VIEW v_requirement_edits AS
SELECT n.id, n.tenant_id, n.role_id, n.local_id, n.version, n.edited_by, n.edited_at, n.edit_reason,
       n.status, n.released_at,
       p.id AS previous_id, p.version AS previous_version,
       p.text <> n.text AS text_changed,
       p.type <> n.type AS type_changed,
       p.class <> n.class AS class_changed,
       p.weight <> n.weight AS weight_changed,
       p.thresholds <> n.thresholds AS thresholds_changed,
       p.evidence_standard <> n.evidence_standard AS evidence_standard_changed,
       p.equivalence_notes <> n.equivalence_notes AS equivalence_notes_changed,
       p.equivalency <> n.equivalency AS equivalency_changed,
       p.alternatives <> n.alternatives AS alternatives_changed,
       p.edited_by IS NULL AS previous_was_extraction,
       n.type AS type, p.type AS previous_type,
       n.evidence_standard_template
FROM requirements n
JOIN requirements p ON p.id = n.supersedes
WHERE n.edited_by IS NOT NULL;
COMMENT ON VIEW v_requirement_edits IS
  'One row per human edit of a requirement with what changed against the version it replaced; the edit reason feeds extractor calibration (FSD FR-HF-03).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP VIEW IF EXISTS v_requirement_edits;

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
       j.prompt_version, j.prompt_fingerprint, j.model, j.created_at
FROM judgments j
JOIN requirements q ON q.id = j.requirement_id;
COMMENT ON VIEW v_judgments_effective IS
  'Each judgment with the override-or-model value, the requirement version it judged and the evidence kinds cited (FSD FR-HF-08, FR-HF-17).';

DROP VIEW IF EXISTS v_dual_write_agreement;
CREATE VIEW v_dual_write_agreement AS
WITH per_run AS (
  SELECT j.run_id,
         bool_or(q.category = 'must') AS has_must,
         sum(q.weight * CASE j.derived_verdict WHEN 'met' THEN 1.0 WHEN 'partial' THEN 0.5 ELSE 0.0 END) AS weighted,
         sum(q.weight) FILTER (WHERE q.category = 'must' OR j.derived_verdict IN ('met', 'partial')) AS denom_must,
         sum(q.weight) AS denom_all,
         count(*) AS judgment_rows
  FROM judgments j
  JOIN requirements q ON q.id = j.requirement_id
  WHERE q.type <> 'behavioral'
  GROUP BY j.run_id
), v2 AS (
  SELECT run_id, judgment_rows,
         CASE WHEN has_must THEN weighted / NULLIF(denom_must, 0) ELSE weighted / NULLIF(denom_all, 0) END AS score
  FROM per_run
)
SELECT r.run_id, r.tenant_id, r.submission_id, r.attempt, r.status, r.finished_at,
       r.match_score, m.evidenced, m.formula_version,
       v2.score AS judgments_v2_score, v2.judgment_rows,
       r.requirement_count, cardinality(m.requirement_ids) AS requirement_rows,
       m.run_id IS NOT NULL AS record_written,
       m.run_id IS NOT NULL
         AND r.requirement_count = cardinality(m.requirement_ids)
         AND CASE
               WHEN m.formula_version = 'v3-three-numbers'
                 THEN v2.score IS NOT NULL AND abs(COALESCE(r.match_score, 0) - v2.score) < 1e-9
               ELSE (r.match_score IS NULL AND m.evidenced IS NULL
                     OR abs(COALESCE(r.match_score, 0) - COALESCE(m.evidenced, 0)) < 1e-9)
             END AS agrees
FROM jd_runs r
LEFT JOIN match_records m ON m.run_id = r.run_id
LEFT JOIN v2 ON v2.run_id = r.run_id
JOIN jd_submissions s ON s.id = r.submission_id
WHERE r.status IN ('generating', 'below_threshold', 'ready')
   OR m.run_id IS NOT NULL;
COMMENT ON VIEW v_dual_write_agreement IS
  'Per run: does the match record agree with jd_runs on the requirement count and, under v2 on the score or under v3 on the v2 score recomputed from the judgment rows (14 §4; FSD FR-HF-07).';

ALTER TABLE concepts DROP COLUMN IF EXISTS match_terms;

DELETE FROM vocab_codes WHERE vocab IN ('requirement_class', 'scope_unit', 'credential_level', 'relationship_kind');

DROP INDEX IF EXISTS uq_requirements_one_draft;
DROP INDEX IF EXISTS uq_requirements_one_active;

ALTER TABLE requirements
  DROP COLUMN IF EXISTS released_by,
  DROP COLUMN IF EXISTS released_at,
  DROP COLUMN IF EXISTS included,
  DROP COLUMN IF EXISTS equivalency,
  DROP COLUMN IF EXISTS class;

-- +goose StatementEnd
