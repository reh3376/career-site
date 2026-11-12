# Schema Rendering: SQL and Proto for the Phase 0 Entities

**Status.** Written 2026-11-08 as step 2 of `13-phase-0-build-plan.md` §2. Renders the v1 schemas in `04-schemas-v1-requirement-and-judgment.md` and the catalog in `05-equivalence-catalog-and-evidence-standards.md` into the forms this repository uses: goose migrations for Postgres and `career.v1` protobuf. Nothing here is code yet; step 3 turns §2 into migration files and step 5 to 8 turn §3 into RPCs. For the owner's asynchronous review before step 3 begins. Repository copy ahead of project (2026-11-08): new document. **Step 3 built 2026-11-09:** §2's four migrations landed as 00066 to 00069 (00064 and 00065 went to the pilot); §5's package exists as `services/api/internal/hiring`, without `rules.go`, which belongs with the judge at step 4; §4 records how the agreement test was shaped and the production view that stands beside it. The six points of §6 drew no objection and stand.

Requirements rendered: FR-HF-01 to FR-HF-16 and FR-HF-19 (FSD §5.15). Decisions applied: `13` §0 items 4, 5, 7, 9, 10.

---

## 0. Conventions this inherits

From `AGENTS.md`, `proto/README.md` and the migrations already in `services/api/internal/db/migrations/`:

- **goose** files, `-- +goose StatementBegin/End`, an `Up` and a `Down`, a comment block that says why the table exists. Replayed from empty on every pull request, so order matters and the `Down` has to work.
- **`tenant_id bigint NOT NULL DEFAULT 1 REFERENCES tenants(id)`** on every table, and every index leads with it (ADR 0029). `DEFAULT 1` keeps an un-updated writer correct rather than broken; Go writers pass `tenant.FromContext(ctx)` explicitly.
- **Every foreign key has an explicit `ON DELETE`.** Every table a row of which is ever updated in place has `updated_at` and the `set_updated_at()` trigger from `00001_init.sql`.
- **Identifiers.** New entities use `uuid` primary keys (`gen_random_uuid()`, already used by `00023_events.sql` and `00049_decision_test.sql`), because match records, judgments and clarification items will be exported and quoted outside the database and a serial leaks write order. Existing tables keep their `bigserial`; references into them use their existing keys (`jd_runs.run_id`, `decision_log.id`, `corpus_chunks.id`, `users.id`).
- **Proto** is `career.v1`, additive only, every RPC carries `option (career.v1.auth)` (which the interceptor enforces), every request field carries `buf.validate` rules (which document the contract; the handler validates), every element has a comment because the comments are the API reference.
- **Nothing recomputes a metric outside a view** (`docs/metrics.md`). Counts are derived, never stored as a source of truth.

## 1. Column or jsonb

The rule from `04` §7, made specific. **A column** when a view, an index, a foreign key or a `WHERE` clause needs it. **jsonb** when the value is a shape the model emits or code writes whole and reads whole. **`text[]`** for short code lists that queries filter on (`uncertainty`, `named_parties`), because `@>` on an array is cheaper to write and read than the jsonb equivalent and GIN indexes both.

| Column | jsonb |
|---|---|
| ids, `tenant_id`, `candidate_id`, `role_id`, `run_id`, `requirement_id` | `thresholds` (sparse, six optional keys) |
| `determination`, `basis`, `derived_verdict`, `effective_determination` | `reported_facts` (model output, compared by code) |
| `type`, `category`, `weight`, `status`, `source_kind`, `visibility`, `answer_type`, `answer_status`, `asked_by` | `rule_adjustments[]`, `rationale_issues[]` |
| every version column (`schema_version`, `vocab_version`, `formula_version`, `prompt_fingerprint`, `rule_version`) | `extraction` provenance, `versions` snapshot |
| the three numbers and two shares | `options[]` and `answer` on a clarification item |
| `created_at`, `updated_at`, `*_at` | `keyword_baseline` per requirement |

**Vocabulary codes are `text` with no `CHECK` constraint.** `04` §0 says a vocabulary change must not be a migration, and a `CHECK (basis IN (...))` is one. The control is the `vocab_codes` registry (§2.3): the Go writer refuses a code the registry does not know at the record's `vocab_version`, and `TestWrittenCodesAreRegistered` reads every distinct code in the judgment and requirement tables of the test database against the registry. Per ADR 0031 that is stated here, where the constraint would otherwise be expected, so nobody reads a bare `text` column as an oversight.

## 2. SQL

Four migrations, numbered from the next free slot (00064 at the time of writing; step 3 renumbers if something lands first). Comments in the DDL below are abbreviated; the migration files carry the full reasoning in the repository's style.

### 2.1 `00064_hiring_core.sql`: roles, candidates, vocabulary, requirements, evidence, match records

```sql
-- A role is what requirements belong to. In Phase 0 every role is a
-- submitted posting (C-08, the rendering); the C-06 authored role
-- document is the same row with source = 'authored' and no submission.
-- Created now so a second source is a value, not a migration.
CREATE TABLE roles (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  source        text        NOT NULL DEFAULT 'posting',        -- posting | authored
  submission_id bigint      UNIQUE REFERENCES jd_submissions(id) ON DELETE SET NULL,
  title         text        NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_roles_tenant ON roles (tenant_id, created_at DESC);

-- One candidate in Phase 0 (FR-HF-19). The row exists so every evidence
-- item, judgment and match record names its person from birth; the
-- fixed id lets corpus_documents default to it without a lookup.
CREATE TABLE candidates (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  user_id      bigint      REFERENCES users(id) ON DELETE SET NULL,
  display_name text        NOT NULL,
  status       text        NOT NULL DEFAULT 'active',        -- active | withdrawn
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
INSERT INTO candidates (id, tenant_id, display_name)
VALUES ('00000000-0000-0000-0000-000000000001', 1, 'Roger Henley');

-- The owner's corpus is the owner candidate's evidence. The boundary
-- FR-HF-19 enforces is this column in the retrieval WHERE clause.
ALTER TABLE corpus_documents
  ADD COLUMN candidate_id uuid NOT NULL
    DEFAULT '00000000-0000-0000-0000-000000000001' REFERENCES candidates(id);
CREATE INDEX idx_corpus_documents_candidate ON corpus_documents (candidate_id, visibility);

-- The vocabulary registry (04 §0 rule 5). Codes are never reused; a
-- retired code keeps its row. The judge prompt renders its allowed
-- codes from here, so a vocabulary change is one INSERT and shows up
-- as a new prompt fingerprint.
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
-- Seeded with vocab 1.0: 4 determinations, 12 bases, 8 uncertainty
-- codes, 9 requirement types, 7 answer types, 4 source kinds, 4
-- override roles, 7 involvement levels, 6 catalog kinds, 3 strengths.

-- A requirement is a row, versioned per edit (FR-HF-01). An edit
-- inserts a new row with supersedes set and the old row's status
-- moved to retired; text is never rewritten in place, so a judgment's
-- requirement_id names exactly what was judged.
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
  type                       text        NOT NULL,
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
CREATE INDEX idx_requirements_role_status ON requirements (tenant_id, role_id, status);
CREATE INDEX idx_requirements_type ON requirements (tenant_id, type);

-- An evidence item is the attributable claim a judgment cites
-- (FR-HF-09). corpus_chunks stay the retrieval unit. Chunk-backed
-- items are created lazily, one per chunk the first time any judgment
-- cites it, so the table does not duplicate the corpus; answer-backed
-- items are created when the answer is recorded (00065).
CREATE TABLE evidence_items (
  id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id             bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  candidate_id          uuid        NOT NULL REFERENCES candidates(id) ON DELETE CASCADE,
  source_kind           text        NOT NULL,   -- corpus_document | profile_sheet | candidate_response | verified_external
  chunk_id              bigint      UNIQUE REFERENCES corpus_chunks(id) ON DELETE CASCADE,
  clarification_item_id uuid,                   -- FK added in 00065, after the table exists
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
CREATE INDEX idx_evidence_items_candidate ON evidence_items (tenant_id, candidate_id, source_kind);
-- visibility for chunk-backed items mirrors corpus_documents.visibility
-- (public → public, corpus_only → private) and is refreshed by the
-- reindex job in one UPDATE ... FROM; it is stored rather than joined
-- so provenance_summary on a judgment is one read.

-- The match record extends jd_runs rather than repeating it
-- (FR-HF-10). jd_runs already holds model, num_ctx, embedder, prompt
-- fingerprints and the corpus fingerprint; this row adds what the
-- framework needs and `versions` holds only what jd_runs lacks.
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
CREATE INDEX idx_match_records_role ON match_records (tenant_id, role_id, attempt DESC);
CREATE INDEX idx_match_records_candidate ON match_records (tenant_id, candidate_id, created_at DESC);
```

`judgments` is in the same migration and gets its own block because it carries the most decisions:

```sql
-- One judgment of one requirement version against one candidate's
-- evidence in one run (FR-HF-04, 05, 07, 08). Immutable except for the
-- override columns and `stale`. effective_determination is generated so
-- every reader, view and formula gets the override-or-model value from
-- one place and cannot pick the wrong column.
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
  override_role           text,                                       -- hiring_manager | recruiter | candidate_dispute | system_admin
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
CREATE INDEX idx_judgments_run ON judgments (tenant_id, run_id);
CREATE INDEX idx_judgments_requirement ON judgments (tenant_id, requirement_id);
CREATE INDEX idx_judgments_outcome ON judgments (tenant_id, effective_determination, effective_basis);
CREATE INDEX idx_judgments_uncertainty ON judgments USING gin (uncertainty);
```

`requirement_id ... ON DELETE RESTRICT`: a requirement version that has been judged cannot be deleted, which is the point of versioning it.

### 2.2 `00065_hiring_clarification.sql`: requests and items

```sql
-- A questionnaire generated from one run's unresolved judgments
-- (FR-HF-13). One round in Phase 0; `round` exists so a second does
-- not need a migration (D-35).
CREATE TABLE clarification_requests (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  run_id       uuid        NOT NULL REFERENCES jd_runs(run_id) ON DELETE CASCADE,
  role_id      uuid        NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  candidate_id uuid        NOT NULL REFERENCES candidates(id) ON DELETE CASCADE,
  round        integer     NOT NULL DEFAULT 1,
  status       text        NOT NULL DEFAULT 'draft',   -- draft | sent | answered | expired | declined
  asked_by     text        NOT NULL DEFAULT 'system',  -- system | recruiter (09 Q3)
  visibility   text        NOT NULL DEFAULT 'private',
  audience     uuid[]      NOT NULL DEFAULT '{}',
  generated_at timestamptz NOT NULL DEFAULT now(),
  sent_at      timestamptz,
  answered_at  timestamptz,
  expires_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (run_id, round)
);
CREATE INDEX idx_clarification_requests_candidate ON clarification_requests (tenant_id, candidate_id, status);

-- One question, generated deterministically from one unresolved
-- judgment. The options offered are stored with their provenance so
-- the answer can be read later even after the corpus changes.
CREATE TABLE clarification_items (
  id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id         bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  request_id        uuid        NOT NULL REFERENCES clarification_requests(id) ON DELETE CASCADE,
  judgment_id       uuid        NOT NULL REFERENCES judgments(id) ON DELETE CASCADE,
  requirement_id    uuid        NOT NULL REFERENCES requirements(id) ON DELETE RESTRICT,
  uncertainty_code  text        NOT NULL,
  answer_type       text        NOT NULL,   -- span | magnitude | involvement | credential | relationship | evidence_pointer | disambiguation
  question_text     text        NOT NULL,
  template_version  text        NOT NULL DEFAULT '',
  options           jsonb       NOT NULL DEFAULT '[]'::jsonb,   -- [{"value":..., "label":..., "source":"own_corpus|generic_list", "ref":...}]
  answer            jsonb,                                       -- shape per answer_type; null until answered
  answer_free_text  text        NOT NULL DEFAULT '',             -- ≤ 600 chars; becomes evidence, never parsed
  answer_status     text        NOT NULL DEFAULT 'pending',      -- pending | answered | not_applicable | declined | withdrawn
  asked_by          text        NOT NULL DEFAULT 'system',
  answered_at       timestamptz,
  withdrawn_at      timestamptz,
  evidence_item_id  uuid        REFERENCES evidence_items(id) ON DELETE SET NULL,
  rejudge_run_id    uuid        REFERENCES jd_runs(run_id) ON DELETE SET NULL,
  schema_version    text        NOT NULL DEFAULT '1.0',
  vocab_version     text        NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (request_id, judgment_id)
);
CREATE INDEX idx_clarification_items_request ON clarification_items (tenant_id, request_id, answer_status);

ALTER TABLE evidence_items
  ADD CONSTRAINT fk_evidence_items_clarification_item
  FOREIGN KEY (clarification_item_id) REFERENCES clarification_items(id) ON DELETE SET NULL;
```

### 2.3 `00066_hiring_catalog.sql`: concepts, catalog, catalog events

```sql
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

-- Directional, conditional, scoped equivalence assertions (FR-HF-15).
-- Tenant 1 is the shared base catalog in Phase 0; inheritance between
-- tenants is Phase 1 work.
CREATE TABLE equivalence_catalog (
  id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  schema_version text        NOT NULL DEFAULT '1.0',
  vocab_version  text        NOT NULL,
  kind           text        NOT NULL,   -- tool | vocabulary | title | credential_experience | domain_transfer | method
  from_concept   uuid        NOT NULL REFERENCES concepts(id) ON DELETE RESTRICT,
  to_concept     uuid        NOT NULL REFERENCES concepts(id) ON DELETE RESTRICT,
  direction      text        NOT NULL,   -- one_way | symmetric
  strength       text        NOT NULL,   -- full | partial | none
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
CREATE INDEX idx_equivalence_catalog_to ON equivalence_catalog (tenant_id, to_concept, status);

-- Every confirmation, rejection and proposal, with the judgment that
-- caused it (FR-HF-16). The counts in 05 §1.2 (`confirmations`,
-- `rejections`) are derived from this table by a view, never stored,
-- per docs/metrics.md; the promotion rule reads the view.
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
CREATE INDEX idx_catalog_events_entry ON catalog_events (tenant_id, entry_id, kind);
```

The starter entries in `05` §1.4 are seeded by this migration with `source = 'seed'`, `status = 'active'`, `introduced_in = '1.0'`, and the concepts they reference are inserted first.

### 2.4 `00067_hiring_views.sql`: what reads the rows

| View | Answers | Notes |
|---|---|---|
| `v_match_record` | One row per run with the three numbers, the two shares, override count, and the provenance from `jd_runs` (model, prompts, corpus fingerprint) joined in | The flat projection the result RPC and any export read; the boundary ADR 0030 asks for |
| `v_judgments_effective` | Every judgment with effective determination and basis, the requirement's current text and version, and the evidence kinds cited | What the viewer renders |
| `v_catalog_counts` | `confirmations` and `rejections` per catalog entry from `catalog_events` | Feeds the promotion rule; the counts `05` §1.2 listed as columns live here |
| `v_determination_agreement` | Owner override vs model determination, by prompt version and model | Successor to `v_judge_agreement` for the new vocabulary; the old view keeps reading `decision_log.output.verdict`, which FR-HF-07 continues to write |
| `v_basis_agreement` | Owner override basis vs model basis, by basis code | The more useful matrix (`03` §2.4): it says which kind of equivalency the model misjudges |
| `v_keyword_baseline` | Per run, per requirement: judge found evidence / keyword filter found it | The false-negative comparison (FR-HF-12) |

Every view carries a `COMMENT ON VIEW` naming the question it answers, as `00028` does.

**Widened 2026-11-12 (00075), step 6.** The viewer reads the record through `v_match_record` and `v_judgments_effective` and found both short of what a reader is shown. `v_match_record` gained `requirement_ids` (the versions judged, in reading order), `run_trigger`, `corpus_documents` and `updated_at`; `v_judgments_effective` gained `evidence_ids`, `reported_facts`, the whole override (`override_determination`, `override_basis`, `override_by`, `override_by_name`, `override_at`) and `decision_id`. Columns were appended, none renamed, so a reader written against the earlier shape still works. A partial index, `idx_judgments_overridden`, finds the judgments that carry an override.

### 2.5 What this changes in the existing tables

- `corpus_documents.candidate_id` (00064). Nothing else on existing tables.
- `decision_log` is unchanged; `judgments.decision_id` points at the row that holds the prompt and response, and `output.verdict` keeps being written as the derived three-way verdict.
- `jd_runs` is unchanged; `match_records` keys on its `run_id`.
- `jd_submissions.assessment` keeps being written until the step 4 golden run passes (§4); it is never dropped, because it is the history of every run before the change.

`SERVICES.md` gains four migration rows and `docs/FSD.md` §8.4 gains the new tables at step 3, when they exist.

## 3. Proto

### 3.1 Vocabulary codes travel as strings

`04` §0 makes the vocabulary data and asks readers to tolerate unknown codes. A protobuf enum would make every new basis code a proto change, a regeneration and a deploy, which is a migration by another name. So `determination`, `basis`, `uncertainty`, `type`, `answer_type`, `source_kind` and `override_role` are `string` fields in the API, validated at the edge against `vocab_codes` at the record's `vocab_version`, with the v1.0 codes listed in the field comment and the registry named as the source. Enums are used only for lifecycle states that are structural (`RequirementStatus`, `ClarificationStatus`), which change with the schema, not the vocabulary.

### 3.2 New file `proto/career/v1/hiring.proto`

Messages, in dependency order. Field numbers are assigned at step 5 when the file is written; this lists shape and comment intent. **Written 2026-11-10** as `proto/career/v1/hiring.proto` with the step 5 messages (`Thresholds`, `ScopeThreshold`, `Requirement`, `RequirementPatch`, `StringList`, `VocabCode`, `HiringSettings`) and five methods; the rest of the table lands with the steps that use it. `Requirement` gained `requirement_class`, `equivalency`, `included`, `inclusion_reason`, `released_at`, `released_by_person`, `type_source` and `evidence_standard_template` beyond the list below.

| Message | Fields | Mirrors |
|---|---|---|
| `Thresholds` | `min_years`, `recency_years`, `min_scope {unit, value}`, `credential_level`, `credential_field`, `relationship_kind`, all optional | `04` §1.2 |
| `Requirement` | `id`, `role_id`, `local_id`, `version`, `supersedes`, `schema_version`, `vocab_version`, `text`, `source_quote`, `type`, `category`, `weight`, `evidence_standard`, `alternatives[]`, `equivalence_notes`, `thresholds`, `named_parties[]`, `edited_by_display`, `edited_at`, `edit_reason`, `status` (enum) | `requirements` |
| `ReportedFacts` | `stated_span_years`, `most_recent_year`, `stated_scope`, `credential_found {level, field, status}`, `parties_evidenced[]`, `alternative_satisfied`, `involvement_level`, `catalog_entry_id` | `04` §2.2 plus `05` §1.3 |
| `RuleAdjustment` | `rule`, `rule_version`, `from_determination`, `to_determination`, `from_basis`, `to_basis`, `added_uncertainty[]`, `reason` | `04` §2.1 |
| `EvidenceRef` | `id`, `source_kind`, `title`, `excerpt`, `attested_at`, `withdrawn` | what a reader needs to see a citation; never the private text of a `corpus_only` chunk (FR-JD rules carry over) |
| `Judgment` | `id`, `run_id`, `requirement_id`, `requirement_local_id`, `determination`, `basis`, `uncertainty[]`, `evidence[]` (EvidenceRef), `rationale`, `reported_facts`, `rule_adjustments[]`, `provenance_summary` (map), `derived_verdict`, `override {determination, basis, by_display, at, reason, role}`, `effective_determination`, `effective_basis`, `gap_class` (lexical / substantive / unaddressed, derived from basis), `stale` | `judgments` |
| `Coverage` | `evidenced`, `unresolved`, `unmet`, `equivalent_share`, `self_attested_share`, `formula_version` | `match_records` |
| `KeywordBaselineItem` | `requirement_local_id`, `found`, `terms[]` | FR-HF-12 |
| `MatchRecord` | `run_id`, `role_id`, `candidate_id`, `attempt`, `supersedes_run_id`, `coverage`, `judgments[]`, `requirements[]` (the versions judged), `keyword_baseline[]`, `override_count`, `versions` (map), `created_at` | `v_match_record` |
| `ClarificationOption` | `value`, `label`, `source` (own_corpus / generic_list), `ref` | `clarification_items.options` |
| `ClarificationItem` | `id`, `request_id`, `requirement_local_id`, `source_quote`, `uncertainty_code`, `answer_type`, `question_text`, `options[]`, `answer_status`, `answered_at` | what the candidate sees: never thresholds, evidence standard, determination or score (FR-HF-13) |
| `ClarificationAnswer` | `oneof`: `span {org, role, start, end, ongoing}`, `magnitude {unit, value}`, `involvement {level, engagement_ref}`, `credential {level, field, institution, year, status}`, `relationship {org, kind}`, `evidence_pointer {engagement_refs[], description}`, `disambiguation {engagement_ref, activity_class}`; plus `not_applicable`, `decline`; `free_text` ≤ 600 | `04` §4.1 |
| `ClarificationRequest` | `id`, `run_id`, `round`, `status` (enum), `asked_by`, `items[]`, `generated_at`, `expires_at` | `clarification_requests` |
| `CatalogEntry`, `Concept` | per `05` §1.2 with `confirmations` and `rejections` read from `v_catalog_counts` | step 8 |

### 3.3 `HiringService`

New service rather than more methods on `JdService`, because the auth story differs by method and the file would otherwise mix the member's submit-and-poll flow with the admin's editing flow. `JdService` is untouched; `GetJdResultResponse` gains additive fields in step 6 (`match_record_run_id`, `coverage`, and on `RequirementVerdict`: `determination`, `basis`, `uncertainty[]`, `gap_class`, `keyword_found`, `self_attested`; the existing `verdict` field keeps carrying the derived value).

| RPC | Auth | Lands in | Notes |
|---|---|---|---|
| `ListRequirements(role_id)` | ADMIN | step 5 | Current versions, with draft and active status; retired versions on request |
| `UpdateRequirement(requirement_id, patch, edit_reason)` | ADMIN | step 5 | Creates a new version; `edit_reason` required (`min_len: 1`); returns the new row and marks the old `retired` |
| `ReleaseRequirements(role_id)` | ADMIN | step 5 | Moves drafts to `active` and resumes the paused pipeline (J8 step 4) |
| `GetMatchRecord(run_id)` | MEMBER | step 6 | The submitting member or an admin; private evidence text is never returned, only `EvidenceRef` |
| `OverrideJudgment(judgment_id, determination, basis, reason, role)` | ADMIN | step 6 | `reason` required; writes the override columns and a `decision_log` row of kind `hf_override`; recomputes the match record's numbers |
| `RequestClarification(run_id)` | MEMBER | step 7 | The candidate asks; generates round 1 from unresolved judgments; refused if a request for that run and round exists |
| `AnswerClarificationItem(item_id, answer)` | MEMBER | step 7 | Validates the answer shape against `answer_type`; creates the `candidate_response` evidence item; queues the scoped re-judge |
| `WithdrawClarificationAnswer(item_id)` | MEMBER | step 7 | Soft-deletes; marks the judgments that cited it `stale` |
| `ListCatalogEntries(filter)`, `UpsertCatalogEntry(entry)` | ADMIN | step 8 | Curation; `UpsertCatalogEntry` on an existing entry writes a `propose` or status event, never rewrites history |
| `ListVocab(vocab)` | MEMBER | step 6 | The registry, so the viewer labels codes from data rather than from copy |

Every request field carries `buf.validate` rules in the file (string lengths matching the column limits in §2, `repeated` sizes matching the schema caps in `04`, uuid format on ids). The handler validates the same things; the rules are the contract's documentation, as `proto/README.md` §3 says.

**As written at step 6 (2026-11-12).** `GetMatchRecord` takes the submission and, optionally, one run of it, since a posting is what both readers know by number; with no run it is the newest run that has a record. `OverrideJudgment` gained `withdraw`: without it a mis-set override could only be corrected by overriding again to the model's own value, which the agreement views would count as a confirmation. `ListVocab` was not written as a method of its own: the vocabularies a viewer needs (determination, basis, uncertainty, requirement type and class, source kind, override role) travel with the record, as the editor's travel with `ListRequirements`, so a page makes one call. `GetJdResultResponse` gained `match_record_run_id` and nothing else: the per-verdict fields listed above would have been a second, thinner rendering of the judgment for the same page, so the member's page reads `GetMatchRecord` and the verdict list keeps its old shape for postings that have no record. Messages added beyond §3.2's list: `OverrideChoice` (a determination and the bases it may carry, so the form offers only pairs the server accepts), `EarlierOverride`, and on `Judgment` `not_assessed`, `keyword_known`, `keyword_found`, `keyword_terms` and `decision_id` in place of a separate `KeywordBaselineItem` list and a `provenance_summary` map. `ReportedFacts` carries the four facts a reader is shown (span, most recent year, scope, parties); the rest stay in the row.

## 4. Dual write and the hand-over

Refines `13` §2 steps 3 and 4.

**Step 3 writes:** `roles` (one per submission, created at submit), `candidates` (seeded), `vocab_codes` (seeded), `requirements` (from the extractor's output, status `active` straight away since editing is step 5), `evidence_items` (lazily, for every chunk the judge cites), `match_records` (with `formula_version = 'v2-nice-bonus'`, `evidenced` = the v2 score, `unresolved` and `unmet` null, `keyword_baseline` populated). `concepts`, `equivalence_catalog` and `catalog_events` are created and seeded but nothing reads them until step 5. **Judgment rows begin at step 4**, when the judge emits the fields they need; a judgment row cannot be faked from a three-way verdict without inventing a basis.

**The test that holds the window:** `TestDualWriteAgrees` runs every golden posting through the pipeline in the test database and asserts that `requirements` rows match the `assessment` jsonb requirement by requirement (text, source quote, category, weight), that `match_records.evidenced` equals `jd_runs.match_score`, and from step 4 that `judgments.derived_verdict` equals `decision_log.output.verdict` for every row. It is a CI test, not a production check, because the dual write exists to be removed.

**As built (2026-11-09).** The golden set stores which side of the gate each posting should land on and no per-requirement verdicts, and running the pipeline needs the model host, so the CI test runs `hiring.Writer.Record` over the assessment shape the pipeline produces, against the test database: three requirements, two cited chunks, a second run that rewords one requirement. It asserts the rows requirement by requirement (text, source quote, category, weight), `match_records.evidenced` against `jd_runs.match_score`, the lazily created evidence items for cited chunks only, the keyword baseline (found for the Ignition row, not found for the capital-projects front-end row against FEL wording), the version chain (`supersedes` set, the old row retired, the new record citing version 2) and `v_dual_write_agreement`. That view (00069) is the production half: one row per finished run saying whether a match record exists and agrees with `jd_runs` on the score and the requirement count, so the hand-over at step 4 is decided from data rather than from the test alone. Requirement `type` is set by `hiring.InferType` (`type-heuristic-v0`, named in each row's `extraction`) until the extractor types it at step 5; the type is excluded from what makes two extractions the same version, so a change to the heuristic does not mint a new version of every requirement. `TestWrittenCodesAreRegistered` reads every code written to the requirement, judgment and catalog tables against `vocab_codes`, as §1 said it would.

**First real run, local, 2026-11-09.** A controls-engineer posting through the full pipeline on the Mac (sidecar on the host's `qwen3:4b-q8_0`, embeddings stubbed, so the score itself means nothing): twelve requirements extracted and written as rows with heuristic types that read right (tool, skill, relationship, credential, experience, domain, behavioral, constraint), one match record on `v2-nice-bonus`, `v_dual_write_agreement` true on both the score and the count, four evidence items for the cited chunks, and the keyword baseline finding seven of twelve. Two things to carry forward: `kw1` lets a few function words through (`its`, `own`, `through`) and stems "power" to "pow", so a tightened `kw2` is expected and the record will say which rule produced it; and the extractor gave the behavioral row weight 3 as a must, which step 4 and 5 will set to weight 0 and not judge (FR-HF-18). Production's first run is step 21 of the owner's review (`15` §11).

**Step 4 as built (2026-11-10).** Judgment rows begin: one per requirement per run (a unique index on `(run_id, requirement_id)`, migration 00073, and the writer replaces a run's rows rather than appending), carrying the post-rule `determination`, `basis` and `uncertainty`, the evidence item ids the cited chunks resolve to, `reported_facts`, `rule_adjustments`, `rationale_issues`, a `provenance_summary` counted by source kind, the `derived_verdict` of FR-HF-07, the `decision_log` id holding the prompt and raw response, and the judge's fingerprint. The match record is written under `v3-three-numbers` with `evidenced`, `unresolved`, `unmet` and the two shares from `hiring.CoverageV3` over the rows, `versions.rules = rules-v1`; a run with no determinations (pre-v13) still writes the v2 record. Because `evidenced` is now coverage rather than the v2 score, `v_dual_write_agreement` recomputes the v2 score from the judgment rows (every must in the denominator, a nice one only when it earned something, met 1, partial 0.5, behavioral excluded, every row when there is no must) and agrees when that equals `jd_runs.match_score`, which is the row-by-row check `TestDualWriteAgrees` makes on its fixture and the hand-over is decided from. The gate still reads the v2 score (D-32); the three numbers ride beside it on the `jd_gate` decision row so the decision can be made from stored data.

**Step 5 as built (2026-11-10).** The requirement rows are written before judging rather than after it. `jd.Assessor.Extract` returns the model's output (text, verified quote, category, weight, named parties and, since `jd_requirements` v5, a type and the posting's alternatives); `hiring.Writer.Prepare` drafts a row from each in code (`hiring.Draft`: thresholds read from the text, class from the category, evidence standard from the type's template, equivalence notes from the catalog, inclusion) and writes the list with `SyncRequirementVersions`; `jd.Assessor.Judge` is then handed the rows, so the judge (v14) is shown the row and the code rules compare against the thresholds it stores. What makes two extractions the same version is now every field the judge sees or the arithmetic reads, the type included, so a judgment's `requirement_id` is enough to say exactly what was judged; the extraction provenance is not part of it. If the rows cannot be written the drafts are judged as they are and written afterwards, and the review is not lost.

Migration 00074 adds what §2 did not foresee: `class`, `equivalency`, `included`, `released_at`, `released_by` on `requirements`; two partial unique indexes holding one released version and one pending draft per requirement; `concepts.match_terms`; vocabulary 1.1. Editing follows a model slightly different from §3.3's one line for `UpdateRequirement`. An edit of the released version inserts a draft with `supersedes` set and leaves the released version released, because "judging runs against the released versions" has to stay true between the save and the release; `ReleaseRequirements` activates every draft and retires what each replaces in one transaction, then starts a run with trigger `release` that reads the released rows and does not extract. An edit of a draft replaces it and retires the earlier draft. From the first edit on, any run on the role, a plain re-score included, judges the released rows: an extraction must never overwrite an edit, and the local ids of two extractions do not line up well enough to merge them. `SyncRequirementVersions` refuses with `ErrHumanEdited` rather than trusting every caller to have checked.

One thing `05` §2.4 asks for was built, measured and narrowed: the judge is shown a requirement's evidence standard only when a person has rewritten it (`evidence_standard_template` beginning `edited:`). The first build showed the template's instance on every requirement, and on the golden set the template's "unresolved when" clauses moved the judge from `not_met` to `unresolved` on weak requirements, enough to carry one below-gate posting across the gate. The template stays on the row for the reader it was written for; a rewritten standard is an instruction for that requirement and is shown. The alternatives, the accepted equivalent and the catalog's notes are shown whenever the row carries them.

Three things §3 and `05` describe are not in this step, each for a stated reason. The pipeline does not pause before a submission's first judging (`ReleaseRequirements` "resumes the paused pipeline" in §3.3, J8 step 1): it releases its own extraction and judges at once, because a pause would hold every member's review for a person; it is a setting to add once the owner has decided what a member should see while waiting. `reported_facts.catalog_entry_id` (`05` §1.3) is not emitted: naming the entry is step 8's exit, and until then the entries that seeded a requirement's notes are listed in its `extraction`. And the constraint template says "not met when the documents say nothing about it", where `05` §2.3 has "unresolved": an unresolved judgment has to cite evidence (the evidence rule), so a constraint nobody addressed cannot be one until the clarification loop of step 7 gives it somewhere to go.

The match record is written under `v4-classes` (FR-HF-23): `hiring.CoverageV4` over the rows' class and `included`, with `versions` carrying `preferred_fraction`, `inclusion_rule` (`incl-v1`), `thresholds_rule`, `templates`, `rules = rules-v2` and `requirements_source` (`extraction` or `rows`). `v_dual_write_agreement` reads the class where it read the category and applies the v2 recompute to v4 records as it did to v3; the score the gate reads is unchanged for a posting nobody has edited.

**Step 6 as built (2026-11-12).** The record is read by `hiring.Repo.GetRecord` through the two views and written to by one thing, `hiring.Repo.OverrideJudgment`. An override is one transaction: the judgment row is locked; a working-style requirement and a record a later run has superseded are refused; the override columns are written (or cleared, for a withdrawal); a `decision_log` row of kind `hf_override` is inserted with `model = 'human'`, the judgment, the model's values and any override it replaced as input, the action, the values, the role and the reason as output, and `reviewed_by` and `reviewed_at` set, because a person's decision is not queued to be graded by a person; then the record's three numbers and two shares are computed again by `hiring.RecordCoverage` over effective determinations, under the formula the record was written with and the preferred fraction in its `versions`, and `override_count` is set to the number of judgments carrying an override. Withdrawing every override returns the numbers exactly, which `TestOverrideChangesTheEffectiveValue` holds. Nothing an override does touches `determination`, `basis` or `derived_verdict`, so `v_dual_write_agreement` cannot be moved by one and `v_determination_agreement` and `v_basis_agreement` have something to compare.

The pairs a person may set are the consistency rules of `04` §2.3 without the evidence-count clauses (`hiring.OverrideViolation`): a reviewer may know what the documents do not say. The gap class of FR-HF-17 is `hiring.GapClass`, derived from the effective basis from the lists `codes.go` already held (the equivalency bases are lexical, the shortfall bases and `contradicted` substantive, `not_addressed` unaddressed), and a test fails when a basis code is added without a place in it.

What a reader is sent is decided in one function per message in the handler, by audience. No reader is sent the text of a passage. A member is sent a document's title only when the evidence item is public, never a passage number, never the reviewer's name or reason, never the editor's name or reason on a requirement, and no decision-log id. The display model the page draws is built in the web layer (`lib/match-record.ts`) in the shape of the pilot's stimulus, so the stimulus can be regenerated from a real record by writing that function's output to a file.

Four things are stated, not built. An override belongs to one judgment of one run: a new run judges every requirement again and starts without overrides, and `GetRecord` marks each such row with the newest override an earlier run carried so the reviewer is told; whether an override should instead be carried forward to an unchanged requirement version is the owner's to decide. The fit and the résumé decision are the run's own, made from the v2 score when it finished, and an override does not move them (D-32). An override to `unresolved` names no uncertainty code, so step 7 needs one before it can ask a question from a row a person set. And the record's status block and follow-up lines (FR-HF-22, 24) arrive with the clarification loop.

**The hand-over** happens at step 4's exit: the golden run under v13 shows no gate inversions, the result RPC switches to reading `v_match_record` and `v_judgments_effective`, the jsonb writer stops, and `jd_submissions.assessment` stays as history. One PR, one tuning-log entry, one line in `SERVICES.md` saying which run was the last to write the column.

## 5. Go shape

Package `services/api/internal/hiring`, beside `jd`, so the generalised entities do not land inside the package that is being generalised:

- `schema.go`: `const RequirementSchema, JudgmentSchema, EvidenceSchema, ClarificationSchema, MatchRecordSchema = "1.0"`; `Upcast(entity, from string, raw json.RawMessage) (json.RawMessage, error)` dispatching to registered pure functions named `upcastRequirement_1_0_to_1_1` and so on. No upcasts exist at 1.0; the registry is the convention, present from the first row.
- `vocab.go`: `Registry` loaded from `vocab_codes`; `Resolve(vocab, code, atVersion)` returns the known code or `other` with the raw value retained; the judge prompt builder renders its allowed-code lists from here, which is what makes a registry change visible as a prompt fingerprint change.
- `formula.go`: `v3-three-numbers` over effective determinations; `derive(determination) verdict` for FR-HF-07; both pure and table-tested, as `scorer.go`'s `verdictValue` is today.
- `rules.go`: the existing `duration.go` and `relationship.go` logic moved behind one interface that returns a `RuleAdjustment` instead of a weakened verdict string, plus the scope, recency and credential rules; `rule_version` is a constant per rule and changes when its logic does.
- `keyword.go`: the baseline, `kw1`: lower-cased, stemmed-by-suffix term overlap between the requirement text and the offered evidence, found when at least two content terms match; the rule and its version are written on the record so the baseline can be tightened later and old records still say which rule produced them.
- `repo.go`: writers that take `ctx` and call `tenant.FromContext`, one per table, in the style of `users/jd_runs.go`.
- Added at step 5: `requirement.go` (the row, `Thresholds`, `DeriveThresholds`, `Inclusion`, the edit as a patch), `requirement_repo.go` (`Draft`, the sync, list, `UpdateRequirement`, `ReleaseRequirements`), `templates.go` (the evidence-standard templates, set 1), `catalog.go` (the catalog as the extraction stage reads it, `Notes`), `settings.go` (the preferred fraction, in `app_settings`).

## 6. Points for the owner's review before step 3

1. **`roles` now.** A table with one meaningful column in Phase 0, added so the C-06 authored role is a value later. The alternative is `requirements.submission_id` and a migration when authored roles arrive.
2. **Evidence items created lazily** per cited chunk rather than one per chunk at ingest. Keeps the table small and honest (an item exists because something cited it); the cost is that `evidence_items` does not list evidence that was offered and not cited, which `jd.Assessment.EvidenceIDs` already records per requirement.
3. **Catalog counts derived**, not stored, which departs from `05` §1.2's field list and follows `docs/metrics.md`.
4. **Codes as strings in proto**, enums only for lifecycle states (§3.1).
5. **A new `HiringService`** rather than extending `JdService` (§3.3).
6. **The owner candidate's fixed uuid** `…0001`, so `corpus_documents` can default to it. A second candidate gets a generated id like any other row.

Silence on these by the time step 3 opens is taken as agreement, per the owner's delegation; any of them is one line to reverse before a migration exists.
