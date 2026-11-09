-- +goose Up
-- +goose StatementBegin

-- The equivalence catalog (FSD §5.15 FR-HF-15, FR-HF-16;
-- docs/framework-research/05 §1 and 14 §2.3): directional, conditional,
-- scoped assertions of the form "X satisfies a requirement for Y under
-- conditions C". Not a synonym list. Created and seeded at Phase 0
-- step 3; nothing reads it until step 5 renders active entries into a
-- requirement's equivalence_notes, and nothing writes to it until step
-- 8's override write-back. Tenant 1 is the shared base catalog in
-- Phase 0; inheritance between tenants is Phase 1 work.

-- Concept refs are `kind:slug` strings in v1 (05 §1.2). A row per
-- distinct ref gives the catalog something to point at and leaves the
-- external seed question (D-36) open: external_ref is where an O*NET
-- or ESCO id would go.
CREATE TABLE concepts (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  kind         text        NOT NULL,   -- tool | title | credential | domain | skill | method | vocabulary
  slug         text        NOT NULL,
  label        text        NOT NULL,
  external_ref jsonb       NOT NULL DEFAULT '{}'::jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, kind, slug)
);
COMMENT ON TABLE concepts IS
  'The things catalog entries relate: a tool, a title, a credential, a domain, a phrase (FSD FR-HF-15).';

-- confirmations and rejections are not columns here, against 05 §1.2's
-- field list and with docs/metrics.md: they are counted from
-- catalog_events by v_catalog_counts (00069), so a count can never
-- disagree with the events that produced it.
CREATE TABLE equivalence_catalog (
  id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  schema_version text        NOT NULL DEFAULT '1.0',
  vocab_version  text        NOT NULL,
  kind           text        NOT NULL,   -- vocab catalog_kind
  from_concept   uuid        NOT NULL REFERENCES concepts(id) ON DELETE RESTRICT,
  to_concept     uuid        NOT NULL REFERENCES concepts(id) ON DELETE RESTRICT,
  direction      text        NOT NULL,   -- one_way | symmetric
  strength       text        NOT NULL,   -- vocab catalog_strength
  conditions     text[]      NOT NULL DEFAULT '{}',
  domain_scope   text[]      NOT NULL DEFAULT '{}',
  rationale      text        NOT NULL DEFAULT '',
  source         text        NOT NULL,   -- seed | override_confirmed | candidate_asserted | external
  status         text        NOT NULL DEFAULT 'proposed',   -- proposed | active | deprecated
  introduced_in  text        NOT NULL,
  retired_in     text,
  created_by     bigint      REFERENCES users(id) ON DELETE SET NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, kind, from_concept, to_concept)
);
COMMENT ON TABLE equivalence_catalog IS
  'Directional, conditional, scoped equivalence assertions; the reusable knowledge judgments write back to (FSD FR-HF-15).';
CREATE INDEX idx_equivalence_catalog_to ON equivalence_catalog (tenant_id, to_concept, status);
CREATE TRIGGER equivalence_catalog_set_updated_at
BEFORE UPDATE ON equivalence_catalog
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Every confirmation, rejection and proposal, with the judgment that
-- caused it (FR-HF-16). The promotion rule reads v_catalog_counts.
CREATE TABLE catalog_events (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  entry_id    uuid        NOT NULL REFERENCES equivalence_catalog(id) ON DELETE CASCADE,
  judgment_id uuid        REFERENCES judgments(id) ON DELETE SET NULL,
  kind        text        NOT NULL,   -- confirm | reject | propose
  actor       bigint      REFERENCES users(id) ON DELETE SET NULL,
  reason      text        NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE catalog_events IS
  'What happened to a catalog entry and which judgment caused it; counts derive from here (FSD FR-HF-16).';
CREATE INDEX idx_catalog_events_entry ON catalog_events (tenant_id, entry_id, kind);

-- The starter entries of 05 §1.4, first domain, illustrative and
-- expected to be wrong in places: that is what the events table is for.
-- Concepts first, then the entries that join them by (kind, slug). The
-- FEL row of 05 is two entries here: the capital-projects phrase is a
-- full equivalent of front-end loading, and explicitly none of the
-- software phrase, which is the prototype's known misreading recorded
-- as knowledge rather than as a note.
INSERT INTO concepts (tenant_id, kind, slug, label) VALUES
  (1, 'tool', 'siemens_tia_portal', 'Siemens TIA Portal'),
  (1, 'tool', 'rockwell_studio5000', 'Rockwell Studio 5000'),
  (1, 'tool', 'ignition_scada', 'Ignition SCADA'),
  (1, 'tool', 'aveva_system_platform', 'Wonderware / AVEVA System Platform'),
  (1, 'tool', 'scada_platform', 'SCADA platform experience (generic)'),
  (1, 'tool', 'grafana', 'Grafana'),
  (1, 'tool', 'power_bi', 'Power BI'),
  (1, 'tool', 'postgresql', 'PostgreSQL'),
  (1, 'tool', 'relational_database', 'Relational database experience (generic)'),
  (1, 'tool', 'go', 'Go'),
  (1, 'tool', 'compiled_backend_language', 'Backend development in a compiled language (generic)'),
  (1, 'tool', 'python_ml_stack', 'Python machine-learning stack'),
  (1, 'tool', 'machine_learning_frameworks', 'Machine learning frameworks (generic)'),
  (1, 'vocabulary', 'front_end_loading_fel', 'Front-end loading (FEL)'),
  (1, 'vocabulary', 'front_end_development_capital_projects', 'Front-end development (capital projects)'),
  (1, 'vocabulary', 'front_end_development_software', 'Front-end development (software)'),
  (1, 'vocabulary', 'co_manufacturing', 'Co-manufacturing'),
  (1, 'vocabulary', 'contract_manufacturing', 'Contract manufacturing'),
  (1, 'vocabulary', 'mes', 'MES'),
  (1, 'vocabulary', 'manufacturing_operations_management', 'Manufacturing operations management (MOM)'),
  (1, 'vocabulary', 'unified_namespace_mqtt', 'Unified Namespace / MQTT broker architecture'),
  (1, 'vocabulary', 'iiot_data_architecture', 'IIoT data architecture'),
  (1, 'title', 'controls_engineer', 'Controls engineer'),
  (1, 'title', 'automation_engineer', 'Automation engineer'),
  (1, 'title', 'ai_principal', 'AI Principal'),
  (1, 'title', 'director_head_of_ai', 'Director / Head of AI'),
  (1, 'title', 'lead_engineer', 'Lead engineer'),
  (1, 'title', 'engineering_manager', 'Engineering manager'),
  (1, 'skill', 'engineering_practice_8y_in_field', 'Eight or more years of engineering practice in the field'),
  (1, 'skill', 'bs_plus_5y_in_field', 'B.S. plus five or more years in the field'),
  (1, 'credential', 'bs_engineering_field', 'B.S. in the engineering field'),
  (1, 'credential', 'ms_engineering_field', 'M.S. in the field'),
  (1, 'credential', 'vendor_plc_certification', 'Vendor PLC certification (Rockwell, Siemens)'),
  (1, 'credential', 'formal_plc_training', 'Formal training in PLC programming'),
  (1, 'domain', 'distilled_spirits_manufacturing', 'Distilled spirits manufacturing'),
  (1, 'domain', 'food_beverage_manufacturing', 'Food and beverage manufacturing'),
  (1, 'domain', 'aerospace_batch_manufacturing', 'Aerospace batch manufacturing'),
  (1, 'domain', 'regulated_manufacturing', 'Regulated manufacturing (generic)'),
  (1, 'domain', 'pharmaceutical_manufacturing_gmp', 'Pharmaceutical manufacturing (GMP)'),
  (1, 'method', 'integrating_vendor_api', 'Integrating a vendor''s API'),
  (1, 'method', 'partnership_with_vendor', 'Partnership or collaboration with the vendor'),
  (1, 'method', 'supervising_system', 'Supervising a system'),
  (1, 'method', 'designing_system', 'Designing a system'),
  (1, 'method', 'capital_project_ownership_50m', 'Capital project ownership at or above $50M'),
  (1, 'method', 'large_capital_project_experience', 'Large capital project experience (generic)');

INSERT INTO equivalence_catalog
  (tenant_id, vocab_version, kind, from_concept, to_concept, direction, strength, conditions, domain_scope, rationale, source, status, introduced_in)
SELECT 1, '1.0', v.kind, f.id, t.id, v.direction, v.strength, v.conditions, v.domain_scope, v.rationale, 'seed', 'active', '1.0'
FROM (VALUES
  ('tool', 'tool', 'siemens_tia_portal', 'tool', 'rockwell_studio5000', 'symmetric', 'partial',
     ARRAY['Ladder and structured-text concepts transfer; vendor instruction sets and networking (Profinet vs EtherNet/IP) do not; clarify depth'], ARRAY['manufacturing'],
     'Both are PLC programming environments; the concepts carry over, the vendor specifics do not.'),
  ('tool', 'tool', 'ignition_scada', 'tool', 'aveva_system_platform', 'symmetric', 'partial',
     ARRAY['HMI and SCADA concepts transfer; scripting and the tag model differ'], ARRAY['manufacturing'],
     'Both are SCADA platforms; the design concepts carry over, the implementation does not.'),
  ('tool', 'tool', 'ignition_scada', 'tool', 'scada_platform', 'one_way', 'full',
     ARRAY['The requirement is generic'], ARRAY[]::text[],
     'A named SCADA platform satisfies a generic SCADA requirement.'),
  ('tool', 'tool', 'grafana', 'tool', 'power_bi', 'one_way', 'partial',
     ARRAY['Visualisation transfers; the data model and DAX do not'], ARRAY[]::text[],
     'Operational dashboards in one tool evidence the visualisation skill, not the other tool''s modelling language.'),
  ('tool', 'tool', 'postgresql', 'tool', 'relational_database', 'one_way', 'full',
     ARRAY['The requirement is generic'], ARRAY[]::text[],
     'A named relational database satisfies a generic relational database requirement.'),
  ('tool', 'tool', 'go', 'tool', 'compiled_backend_language', 'one_way', 'full',
     ARRAY[]::text[], ARRAY[]::text[],
     'Go is a compiled language used for backend services.'),
  ('tool', 'tool', 'python_ml_stack', 'tool', 'machine_learning_frameworks', 'one_way', 'full',
     ARRAY[]::text[], ARRAY[]::text[],
     'The Python machine-learning stack is what the generic phrase names.'),
  ('vocabulary', 'vocabulary', 'front_end_loading_fel', 'vocabulary', 'front_end_development_capital_projects', 'symmetric', 'full',
     ARRAY['Capital-projects domain only'], ARRAY['capital_projects'],
     'In capital projects, front-end development and front-end loading name the same early engineering phase.'),
  ('vocabulary', 'vocabulary', 'front_end_development_capital_projects', 'vocabulary', 'front_end_development_software', 'one_way', 'none',
     ARRAY['Different domains; the words coincide and the work does not'], ARRAY['software'],
     'The prototype read a capital-projects requirement as web work. Front-end development in a capital project is engineering definition, not user-interface programming.'),
  ('vocabulary', 'title', 'controls_engineer', 'title', 'automation_engineer', 'symmetric', 'full',
     ARRAY['Manufacturing'], ARRAY['manufacturing'],
     'The two titles name the same role in manufacturing.'),
  ('vocabulary', 'vocabulary', 'co_manufacturing', 'vocabulary', 'contract_manufacturing', 'symmetric', 'full',
     ARRAY[]::text[], ARRAY[]::text[],
     'Two names for making a product under contract for another company.'),
  ('vocabulary', 'vocabulary', 'mes', 'vocabulary', 'manufacturing_operations_management', 'symmetric', 'full',
     ARRAY[]::text[], ARRAY['manufacturing'],
     'MES and MOM name the same layer of plant software.'),
  ('vocabulary', 'vocabulary', 'unified_namespace_mqtt', 'vocabulary', 'iiot_data_architecture', 'one_way', 'full',
     ARRAY[]::text[], ARRAY['manufacturing'],
     'A Unified Namespace over an MQTT broker is an IIoT data architecture.'),
  ('title', 'title', 'ai_principal', 'title', 'director_head_of_ai', 'one_way', 'partial',
     ARRAY['Depends on reports and budget; clarify scope'], ARRAY[]::text[],
     'A principal may or may not carry the people and budget scope the director title implies.'),
  ('title', 'title', 'lead_engineer', 'title', 'engineering_manager', 'one_way', 'partial',
     ARRAY['Requires evidence of people management'], ARRAY[]::text[],
     'Technical leadership does not by itself evidence line management.'),
  ('credential_experience', 'skill', 'engineering_practice_8y_in_field', 'credential', 'bs_engineering_field', 'one_way', 'partial',
     ARRAY['Only when the requirement offers "or equivalent experience" or the role is not licensure-gated', 'None where PE or licensure is required'], ARRAY[]::text[],
     'Long practice in the field shows the work a degree certifies, except where a licence is the point.'),
  ('credential_experience', 'skill', 'bs_plus_5y_in_field', 'credential', 'ms_engineering_field', 'one_way', 'partial',
     ARRAY['Same conditions as practice for a B.S.'], ARRAY[]::text[],
     'A first degree plus years in the field shows what a second degree would.'),
  ('credential_experience', 'credential', 'vendor_plc_certification', 'credential', 'formal_plc_training', 'one_way', 'full',
     ARRAY[]::text[], ARRAY['manufacturing'],
     'A vendor certification is formal training in that vendor''s PLCs.'),
  ('domain_transfer', 'domain', 'distilled_spirits_manufacturing', 'domain', 'food_beverage_manufacturing', 'one_way', 'full',
     ARRAY['Regulated batch processing in common'], ARRAY[]::text[],
     'Distilled spirits is a regulated batch food-and-beverage process.'),
  ('domain_transfer', 'domain', 'aerospace_batch_manufacturing', 'domain', 'regulated_manufacturing', 'one_way', 'full',
     ARRAY[]::text[], ARRAY[]::text[],
     'Aerospace manufacturing is regulated manufacturing.'),
  ('domain_transfer', 'domain', 'food_beverage_manufacturing', 'domain', 'pharmaceutical_manufacturing_gmp', 'one_way', 'partial',
     ARRAY['cGMP validation discipline (CQV, 21 CFR Part 11) must be separately evidenced'], ARRAY[]::text[],
     'The process discipline transfers; the pharmaceutical validation regime does not follow from it.'),
  ('method', 'method', 'integrating_vendor_api', 'method', 'partnership_with_vendor', 'one_way', 'none',
     ARRAY[]::text[], ARRAY[]::text[],
     'From the prototype''s relationship rule: calling a vendor''s API is not a working relationship with the vendor.'),
  ('method', 'method', 'supervising_system', 'method', 'designing_system', 'one_way', 'none',
     ARRAY['Depth ladder: used or configured does not satisfy designed'], ARRAY[]::text[],
     'Operating or supervising a system does not show that one designed it.'),
  ('method', 'method', 'capital_project_ownership_50m', 'method', 'large_capital_project_experience', 'one_way', 'full',
     ARRAY[]::text[], ARRAY[]::text[],
     'Owning a project at or above fifty million dollars is large capital project experience.')
) AS v(kind, from_kind, from_slug, to_kind, to_slug, direction, strength, conditions, domain_scope, rationale)
JOIN concepts f ON f.tenant_id = 1 AND f.kind = v.from_kind AND f.slug = v.from_slug
JOIN concepts t ON t.tenant_id = 1 AND t.kind = v.to_kind AND t.slug = v.to_slug;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS catalog_events;
DROP TABLE IF EXISTS equivalence_catalog;
DROP TABLE IF EXISTS concepts;

-- +goose StatementEnd
