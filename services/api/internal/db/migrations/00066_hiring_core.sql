-- +goose Up
-- +goose StatementBegin

-- The hiring framework's core entities (FSD §5.15 FR-HF-01, 04, 09,
-- 10, 11, 12, 19; docs/framework-research/14-schema-rendering.md §2.1).
--
-- Phase 0 step 3 (13-phase-0-build-plan.md §2): data before logic. The
-- JD reviewer keeps writing its jsonb assessment, and from this
-- migration also writes roles, requirements, evidence items and match
-- records as rows. Judgment rows begin at step 4, when the judge emits
-- determination, basis and uncertainty; the table is created now so the
-- views and the agreement matrices exist before the first row does.
--
-- Conventions (14 §0): uuid keys because these rows are quoted outside
-- the database and a serial leaks write order; tenant_id on every table
-- with every index leading on it (ADR 0029); an explicit ON DELETE on
-- every foreign key; updated_at and the trigger on anything updated in
-- place. Vocabulary codes are text with no CHECK, on purpose: 04 §0
-- says a vocabulary change must not be a migration, and the control is
-- the vocab_codes registry below plus TestWrittenCodesAreRegistered
-- (ADR 0031: the control is stated where the constraint would be
-- expected).

-- A role is what requirements belong to. In Phase 0 every role is a
-- submitted posting (C-08); the C-06 authored role is the same row with
-- source = 'authored' and no submission. Created now so a second source
-- is a value, not a migration (14 §6 point 1).
CREATE TABLE roles (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  source        text        NOT NULL DEFAULT 'posting',        -- posting | authored
  submission_id bigint      UNIQUE REFERENCES jd_submissions(id) ON DELETE SET NULL,
  title         text        NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE roles IS
  'What requirements belong to: a submitted posting in Phase 0, an authored role later (FSD FR-HF-01).';
CREATE INDEX idx_roles_tenant ON roles (tenant_id, created_at DESC);
CREATE TRIGGER roles_set_updated_at
BEFORE UPDATE ON roles
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One candidate in Phase 0 (FR-HF-19). The row exists so every evidence
-- item, judgment and match record names its person from birth; the
-- fixed id lets corpus_documents default to it without a lookup. A
-- second candidate is a generated id like any other row.
CREATE TABLE candidates (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  user_id      bigint      REFERENCES users(id) ON DELETE SET NULL,
  display_name text        NOT NULL,
  status       text        NOT NULL DEFAULT 'active',        -- active | withdrawn
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE candidates IS
  'Whose evidence a judgment weighs; one row in Phase 0, the owner (FSD FR-HF-19).';
CREATE TRIGGER candidates_set_updated_at
BEFORE UPDATE ON candidates
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO candidates (id, tenant_id, display_name)
VALUES ('00000000-0000-0000-0000-000000000001', 1, 'Roger Henley');

-- The owner's corpus is the owner candidate's evidence. The boundary
-- FR-HF-19 enforces is this column in the retrieval WHERE clause, not
-- a sentence in the copy.
ALTER TABLE corpus_documents
  ADD COLUMN candidate_id uuid NOT NULL
    DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES candidates(id);
COMMENT ON COLUMN corpus_documents.candidate_id IS
  'The candidate whose evidence this document is; retrieval filters on it (FSD FR-HF-19).';
CREATE INDEX idx_corpus_documents_candidate ON corpus_documents (candidate_id, visibility);

-- The vocabulary registry (04 §0 rule 5, FR-HF-11). Codes are never
-- reused; a retired code keeps its row with retired_in set. The judge
-- prompt renders its allowed codes from here, so a vocabulary change is
-- one INSERT and shows up as a new prompt fingerprint, and a reader
-- meeting a code it does not know keeps the raw value and treats it as
-- other.
CREATE TABLE vocab_codes (
  vocab         text    NOT NULL,   -- determination | basis | uncertainty | requirement_type
                                    -- | answer_type | source_kind | override_role
                                    -- | involvement_level | catalog_kind | catalog_strength
  code          text    NOT NULL,
  label         text    NOT NULL,
  description   text    NOT NULL DEFAULT '',
  introduced_in text    NOT NULL,   -- vocab version, e.g. '1.0'
  retired_in    text,
  replaced_by   text,
  sort_order    integer NOT NULL DEFAULT 0,
  PRIMARY KEY (vocab, code)
);
COMMENT ON TABLE vocab_codes IS
  'The controlled vocabularies as data: what a determination, basis or code may be, by version (FSD FR-HF-11).';

-- Vocabulary 1.0, from 03 §2 and 04 §1, §2, §3, §4 and 05 §1.2.
INSERT INTO vocab_codes (vocab, code, label, description, introduced_in, sort_order) VALUES
  ('determination', 'met',            'Met',                'The evidence satisfies the requirement as written or via one of its own stated alternatives. Full weight.', '1.0', 1),
  ('determination', 'met_equivalent', 'Met by equivalence', 'The evidence satisfies the requirement through an equivalency the system or a reviewer asserted. Full weight, flagged for audit.', '1.0', 2),
  ('determination', 'unresolved',     'Unresolved',         'The evidence is suggestive but a specific, nameable fact is missing. Reported as unresolved weight, never scored as half.', '1.0', 3),
  ('determination', 'not_met',        'Not met',            'The evidence addresses the requirement and falls short, or does not address it and there is no reason to believe it could. Zero.', '1.0', 4),

  ('basis', 'direct',                   'Direct',                   'Same thing, same words or trivially the same.', '1.0', 1),
  ('basis', 'vocabulary',               'Vocabulary',               'Same capability, different vocabulary, title or framing. The lexical gap the project exists for.', '1.0', 2),
  ('basis', 'tool_equivalent',          'Equivalent tool',          'Different named tool, platform or vendor, same function.', '1.0', 3),
  ('basis', 'experience_for_education', 'Experience for education', 'The requirement names a credential; the evidence shows the work the credential certifies.', '1.0', 4),
  ('basis', 'education_for_experience', 'Education for experience', 'The requirement names years or exposure; the evidence shows formal training or a credential covering it.', '1.0', 5),
  ('basis', 'adjacent_domain',          'Adjacent domain',          'Same skill demonstrated in a neighbouring industry or context; transferability is the question.', '1.0', 6),
  ('basis', 'scope_shortfall',          'Scope shortfall',          'Right work at a smaller scale, team, budget or system size than stated.', '1.0', 7),
  ('basis', 'duration_shortfall',       'Duration shortfall',       'Right work, fewer years than stated.', '1.0', 8),
  ('basis', 'recency',                  'Recency',                  'Right work, but old relative to what the requirement implies.', '1.0', 9),
  ('basis', 'relationship_unproven',    'Relationship unproven',    'The requirement names organisations; the evidence uses their products but shows no working relationship.', '1.0', 10),
  ('basis', 'not_addressed',            'Not addressed',            'Nothing in the evidence speaks to it.', '1.0', 11),
  ('basis', 'contradicted',             'Contradicted',             'The evidence affirmatively indicates the requirement is not satisfied.', '1.0', 12),

  ('uncertainty', 'scope_unknown',           'Scope unknown',           'Scale not stated in the evidence.', '1.0', 1),
  ('uncertainty', 'duration_unknown',        'Duration unknown',        'Years not stated.', '1.0', 2),
  ('uncertainty', 'recency_unknown',         'Recency unknown',         'Dates not stated.', '1.0', 3),
  ('uncertainty', 'depth_unknown',           'Depth unknown',           'Used, owned or designed is ambiguous.', '1.0', 4),
  ('uncertainty', 'equivalence_unconfirmed', 'Equivalence unconfirmed', 'A vocabulary or tool equivalence is plausible but asserted by no catalog entry or reviewer.', '1.0', 5),
  ('uncertainty', 'credential_unknown',      'Credential unknown',      'The degree or certificate is neither confirmed nor denied by the evidence.', '1.0', 6),
  ('uncertainty', 'party_unknown',           'Party unknown',           'An organisation named in the requirement is not confirmed as a working relationship.', '1.0', 7),
  ('uncertainty', 'evidence_ambiguous',      'Evidence ambiguous',      'The text can be read more than one way.', '1.0', 8),

  ('requirement_type', 'skill',        'Skill',        'A capability the work needs.', '1.0', 1),
  ('requirement_type', 'outcome',      'Outcome',      'A result the role is expected to have produced before.', '1.0', 2),
  ('requirement_type', 'tool',         'Tool',         'A named tool, platform or vendor.', '1.0', 3),
  ('requirement_type', 'domain',       'Domain',       'An industry or context the work sits in.', '1.0', 4),
  ('requirement_type', 'credential',   'Credential',   'A degree, licence or certificate.', '1.0', 5),
  ('requirement_type', 'experience',   'Experience',   'Years or exposure in a kind of work.', '1.0', 6),
  ('requirement_type', 'relationship', 'Relationship', 'A working relationship with named parties.', '1.0', 7),
  ('requirement_type', 'constraint',   'Constraint',   'A condition of the role: location, clearance, travel, schedule.', '1.0', 8),
  ('requirement_type', 'behavioral',   'Behavioral',   'A personality or working-style adjective. Never judged by the model (FSD FR-HF-18).', '1.0', 9),

  ('answer_type', 'span',             'Span',             'One or more engagements with organisation, role and dates.', '1.0', 1),
  ('answer_type', 'magnitude',        'Magnitude',        'A unit and a value, the unit from the fixed scope list.', '1.0', 2),
  ('answer_type', 'involvement',      'Involvement',      'One rung of the depth ladder and which engagement.', '1.0', 3),
  ('answer_type', 'credential',       'Credential',       'Level, field, institution, year and status, or none.', '1.0', 4),
  ('answer_type', 'relationship',     'Relationship',     'An organisation from the requirement and a relationship kind.', '1.0', 5),
  ('answer_type', 'evidence_pointer', 'Evidence pointer', 'Engagements from the own corpus and a short description.', '1.0', 6),
  ('answer_type', 'disambiguation',   'Disambiguation',   'Which engagement a passage refers to and an activity class.', '1.0', 7),

  ('source_kind', 'corpus_document',    'Corpus document',    'A chunk of a document the candidate placed in the corpus.', '1.0', 1),
  ('source_kind', 'profile_sheet',      'Profile sheet',      'A structured sheet the candidate maintains.', '1.0', 2),
  ('source_kind', 'candidate_response', 'Candidate response', 'An answer to a clarification item. Self-attested.', '1.0', 3),
  ('source_kind', 'verified_external',  'Verified external',  'A record verified by a third party.', '1.0', 4),

  ('override_role', 'hiring_manager',    'Hiring manager',    'The domain expert who owns the role.', '1.0', 1),
  ('override_role', 'recruiter',         'Recruiter',         'The recruiter or HR reader.', '1.0', 2),
  ('override_role', 'candidate_dispute', 'Candidate dispute', 'Reserved for a candidate contesting a judgment (D-38).', '1.0', 3),
  ('override_role', 'system_admin',      'System admin',      'The operator correcting a defect.', '1.0', 4),

  ('involvement_level', 'aware',      'Aware',      'Knows of it.', '1.0', 1),
  ('involvement_level', 'used',       'Used',       'Operated it as a user.', '1.0', 2),
  ('involvement_level', 'configured', 'Configured', 'Set it up or tuned it.', '1.0', 3),
  ('involvement_level', 'designed',   'Designed',   'Specified or architected it.', '1.0', 4),
  ('involvement_level', 'owned',      'Owned',      'Accountable for it end to end.', '1.0', 5),
  ('involvement_level', 'taught',     'Taught',     'Trained others in it.', '1.0', 6),
  ('involvement_level', 'unknown',    'Unknown',    'The evidence does not say.', '1.0', 7),

  ('catalog_kind', 'tool',                  'Tool',                  'One tool for another.', '1.0', 1),
  ('catalog_kind', 'vocabulary',            'Vocabulary',            'One phrase for another.', '1.0', 2),
  ('catalog_kind', 'title',                 'Title',                 'One title for another.', '1.0', 3),
  ('catalog_kind', 'credential_experience', 'Credential and experience', 'Practice for a credential, or the reverse.', '1.0', 4),
  ('catalog_kind', 'domain_transfer',       'Domain transfer',       'One industry for another.', '1.0', 5),
  ('catalog_kind', 'method',                'Method',                'One way of working for another.', '1.0', 6),

  ('catalog_strength', 'full',    'Full',    'Counts as met_equivalent.', '1.0', 1),
  ('catalog_strength', 'partial', 'Partial', 'Counts as unresolved with equivalence_unconfirmed; prompts a clarification.', '1.0', 2),
  ('catalog_strength', 'none',    'None',    'Explicitly not equivalent; the judge stops proposing it.', '1.0', 3);

-- A requirement is a row, versioned per edit (FR-HF-01). An edit
-- inserts a new row with supersedes set and the old row's status moved
-- to retired; text is never rewritten in place, so a judgment's
-- requirement_id names exactly what was judged. Column or jsonb per
-- 14 §1: a column where a view, index, key or WHERE needs it.
CREATE TABLE requirements (
  id                         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id                  bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  role_id                    uuid        NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  local_id                   text        NOT NULL,            -- r1, r2: stable across versions
  version                    integer     NOT NULL DEFAULT 1,
  supersedes                 uuid        REFERENCES requirements(id) ON DELETE SET NULL,
  schema_version             text        NOT NULL DEFAULT '1.0',
  vocab_version              text        NOT NULL,
  text                       text        NOT NULL,
  source_quote               text        NOT NULL DEFAULT '',
  type                       text        NOT NULL,            -- vocab requirement_type
  category                   text        NOT NULL,            -- must | nice
  weight                     smallint    NOT NULL CHECK (weight BETWEEN 1 AND 3),
  evidence_standard          text        NOT NULL DEFAULT '',
  evidence_standard_template text        NOT NULL DEFAULT '', -- type@template_version
  alternatives               jsonb       NOT NULL DEFAULT '[]'::jsonb,
  equivalence_notes          text        NOT NULL DEFAULT '',
  thresholds                 jsonb       NOT NULL DEFAULT '{}'::jsonb,
  named_parties              text[]      NOT NULL DEFAULT '{}',
  extraction                 jsonb,      -- {prompt_id, prompt_version, prompt_fingerprint, model, decision_id}; null when human-authored
  edited_by                  bigint      REFERENCES users(id) ON DELETE SET NULL,
  edited_at                  timestamptz,
  edit_reason                text        NOT NULL DEFAULT '',
  status                     text        NOT NULL DEFAULT 'draft',  -- draft | active | retired
  created_at                 timestamptz NOT NULL DEFAULT now(),
  updated_at                 timestamptz NOT NULL DEFAULT now(),
  UNIQUE (role_id, local_id, version)
);
COMMENT ON TABLE requirements IS
  'One requirement of a role, one row per version; a judgment names the exact version it judged (FSD FR-HF-01).';
CREATE INDEX idx_requirements_role_status ON requirements (tenant_id, role_id, status);
CREATE INDEX idx_requirements_type ON requirements (tenant_id, type);
CREATE TRIGGER requirements_set_updated_at
BEFORE UPDATE ON requirements
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- An evidence item is the attributable claim a judgment cites
-- (FR-HF-09). corpus_chunks stay the retrieval unit. Chunk-backed items
-- are created lazily, one per chunk the first time any judgment cites
-- it, so the table does not duplicate the corpus (14 §6 point 2);
-- answer-backed items are created when the answer is recorded (00067).
-- visibility for chunk-backed items mirrors corpus_documents.visibility
-- (public → public, corpus_only → private); it is stored rather than
-- joined so provenance_summary on a judgment is one read.
CREATE TABLE evidence_items (
  id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id             bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  candidate_id          uuid        NOT NULL REFERENCES candidates(id) ON DELETE CASCADE,
  source_kind           text        NOT NULL,   -- vocab source_kind
  chunk_id              bigint      UNIQUE REFERENCES corpus_chunks(id) ON DELETE CASCADE,
  clarification_item_id uuid,                   -- FK added in 00067, after the table exists
  text                  text        NOT NULL,
  attested_at           timestamptz NOT NULL DEFAULT now(),
  visibility            text        NOT NULL DEFAULT 'private',  -- private | employer_scoped | public
  audience              uuid[]      NOT NULL DEFAULT '{}',
  withdrawn_at          timestamptz,
  schema_version        text        NOT NULL DEFAULT '1.0',
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  CHECK (chunk_id IS NOT NULL OR clarification_item_id IS NOT NULL OR source_kind = 'verified_external')
);
COMMENT ON TABLE evidence_items IS
  'The attributable claims judgments cite: a corpus chunk, a clarification answer or a verified record (FSD FR-HF-09).';
CREATE INDEX idx_evidence_items_candidate ON evidence_items (tenant_id, candidate_id, source_kind);
CREATE TRIGGER evidence_items_set_updated_at
BEFORE UPDATE ON evidence_items
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- The match record extends jd_runs rather than repeating it
-- (FR-HF-10). jd_runs already holds model, num_ctx, embedder, prompt
-- fingerprints and the corpus fingerprint; this row adds what the
-- framework needs and `versions` holds only what jd_runs lacks. During
-- the dual write (14 §4) formula_version is v2-nice-bonus and evidenced
-- is the v2 score; unresolved and unmet stay null until step 4.
CREATE TABLE match_records (
  run_id                    uuid        PRIMARY KEY REFERENCES jd_runs(run_id) ON DELETE CASCADE,
  tenant_id                 bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  role_id                   uuid        NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  candidate_id              uuid        NOT NULL REFERENCES candidates(id) ON DELETE CASCADE,
  attempt                   integer     NOT NULL,
  supersedes_run_id         uuid        REFERENCES jd_runs(run_id) ON DELETE SET NULL,
  requirement_ids           uuid[]      NOT NULL DEFAULT '{}',    -- exact versions judged
  formula_version           text        NOT NULL,                 -- v2-nice-bonus during dual write, v3-three-numbers after
  evidenced                 double precision,
  unresolved                double precision,
  unmet                     double precision,
  coverage_equivalent_share double precision,
  self_attested_share       double precision,
  keyword_baseline          jsonb       NOT NULL DEFAULT '{}'::jsonb,  -- {"rule":"kw1","items":{"r1":{"found":true,"terms":["..."]}}}
  override_count            integer     NOT NULL DEFAULT 0,
  versions                  jsonb       NOT NULL DEFAULT '{}'::jsonb,  -- {requirement_schema, judgment_schema, vocab, formula, rules}
  schema_version            text        NOT NULL DEFAULT '1.0',
  created_at                timestamptz NOT NULL DEFAULT now(),
  updated_at                timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE match_records IS
  'One per pipeline run: the three numbers, the two shares, the keyword baseline and the version snapshot (FSD FR-HF-10, FR-HF-12).';
CREATE INDEX idx_match_records_role ON match_records (tenant_id, role_id, attempt DESC);
CREATE INDEX idx_match_records_candidate ON match_records (tenant_id, candidate_id, created_at DESC);
CREATE TRIGGER match_records_set_updated_at
BEFORE UPDATE ON match_records
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One judgment of one requirement version against one candidate's
-- evidence in one run (FR-HF-04, 05, 07, 08). Immutable except for the
-- override columns and `stale`. effective_determination is generated so
-- every reader, view and formula gets the override-or-model value from
-- one place and cannot pick the wrong column. requirement_id is ON
-- DELETE RESTRICT: a requirement version that has been judged cannot be
-- deleted, which is the point of versioning it. No rows until step 4.
CREATE TABLE judgments (
  id                      uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id               bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  run_id                  uuid        NOT NULL REFERENCES jd_runs(run_id) ON DELETE CASCADE,
  requirement_id          uuid        NOT NULL REFERENCES requirements(id) ON DELETE RESTRICT,
  requirement_local_id    text        NOT NULL,
  candidate_id            uuid        NOT NULL REFERENCES candidates(id) ON DELETE CASCADE,
  schema_version          text        NOT NULL DEFAULT '1.0',
  vocab_version           text        NOT NULL,
  -- model-reported, post-rule
  determination           text        NOT NULL,
  basis                   text        NOT NULL,
  uncertainty             text[]      NOT NULL DEFAULT '{}',
  evidence_ids            uuid[]      NOT NULL DEFAULT '{}',
  rationale               text        NOT NULL,
  reported_facts          jsonb       NOT NULL DEFAULT '{}'::jsonb,
  confidence              real,                                  -- logged, never in arithmetic
  -- code-applied
  rule_adjustments        jsonb       NOT NULL DEFAULT '[]'::jsonb,
  rationale_issues        jsonb       NOT NULL DEFAULT '[]'::jsonb,
  provenance_summary      jsonb       NOT NULL DEFAULT '{}'::jsonb,   -- {"corpus_document": 3, "candidate_response": 1}
  derived_verdict         text        NOT NULL,                       -- met | partial | unmet (FR-HF-07)
  -- override
  override_determination  text,
  override_basis          text,
  override_by             bigint      REFERENCES users(id) ON DELETE SET NULL,
  override_at             timestamptz,
  override_reason         text,
  override_role           text,                                       -- vocab override_role
  effective_determination text        GENERATED ALWAYS AS (COALESCE(override_determination, determination)) STORED,
  effective_basis         text        GENERATED ALWAYS AS (COALESCE(override_basis, basis)) STORED,
  -- provenance
  decision_id             bigint      REFERENCES decision_log(id) ON DELETE SET NULL,
  prompt_id               text        NOT NULL DEFAULT '',
  prompt_version          integer     NOT NULL DEFAULT 0,
  prompt_fingerprint      text        NOT NULL DEFAULT '',
  model                   text        NOT NULL DEFAULT '',
  clarification_item_ids  uuid[]      NOT NULL DEFAULT '{}',
  supersedes              uuid        REFERENCES judgments(id) ON DELETE SET NULL,
  stale                   boolean     NOT NULL DEFAULT false,         -- a cited evidence item was withdrawn
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  CHECK (override_determination IS NULL OR (override_reason IS NOT NULL AND override_by IS NOT NULL AND override_role IS NOT NULL))
);
COMMENT ON TABLE judgments IS
  'One judgment of one requirement version in one run: determination, basis, uncertainty, evidence, and the override read as the effective value (FSD FR-HF-04, FR-HF-08).';
CREATE INDEX idx_judgments_run ON judgments (tenant_id, run_id);
CREATE INDEX idx_judgments_requirement ON judgments (tenant_id, requirement_id);
CREATE INDEX idx_judgments_outcome ON judgments (tenant_id, effective_determination, effective_basis);
CREATE INDEX idx_judgments_uncertainty ON judgments USING gin (uncertainty);
CREATE TRIGGER judgments_set_updated_at
BEFORE UPDATE ON judgments
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS judgments;
DROP TABLE IF EXISTS match_records;
DROP TABLE IF EXISTS evidence_items;
DROP TABLE IF EXISTS requirements;
DROP TABLE IF EXISTS vocab_codes;
DROP INDEX IF EXISTS idx_corpus_documents_candidate;
ALTER TABLE corpus_documents DROP COLUMN IF EXISTS candidate_id;
DROP TABLE IF EXISTS candidates;
DROP TABLE IF EXISTS roles;

-- +goose StatementEnd
