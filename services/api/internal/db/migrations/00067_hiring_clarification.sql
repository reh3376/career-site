-- +goose Up
-- +goose StatementBegin

-- The clarification loop's data model (FSD §5.15 FR-HF-13, FR-HF-14;
-- docs/framework-research/14-schema-rendering.md §2.2). Tables only at
-- Phase 0 step 3; the generator and the answer path are step 7. They
-- exist now so the evidence_items foreign key, the judgment columns
-- that name them and the views are settled before any row is written.

-- A questionnaire generated from one run's unresolved judgments. One
-- round in Phase 0; `round` exists so a second does not need a
-- migration (D-35). asked_by distinguishes a system-generated request
-- from one a recruiter triggered (09 Q3).
CREATE TABLE clarification_requests (
  id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  run_id       uuid        NOT NULL REFERENCES jd_runs(run_id) ON DELETE CASCADE,
  role_id      uuid        NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  candidate_id uuid        NOT NULL REFERENCES candidates(id) ON DELETE CASCADE,
  round        integer     NOT NULL DEFAULT 1,
  status       text        NOT NULL DEFAULT 'draft',   -- draft | sent | answered | expired | declined
  asked_by     text        NOT NULL DEFAULT 'system',  -- system | recruiter
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
COMMENT ON TABLE clarification_requests IS
  'One questionnaire to a candidate from one run''s unresolved judgments (FSD FR-HF-13).';
CREATE INDEX idx_clarification_requests_candidate ON clarification_requests (tenant_id, candidate_id, status);
CREATE TRIGGER clarification_requests_set_updated_at
BEFORE UPDATE ON clarification_requests
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One question, generated deterministically from one unresolved
-- judgment. The options offered are stored with their provenance so
-- the answer can be read later even after the corpus changes; the free
-- text supplement becomes evidence and is never parsed (04 §4.1).
CREATE TABLE clarification_items (
  id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id         bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  request_id        uuid        NOT NULL REFERENCES clarification_requests(id) ON DELETE CASCADE,
  judgment_id       uuid        NOT NULL REFERENCES judgments(id) ON DELETE CASCADE,
  requirement_id    uuid        NOT NULL REFERENCES requirements(id) ON DELETE RESTRICT,
  uncertainty_code  text        NOT NULL,   -- vocab uncertainty
  answer_type       text        NOT NULL,   -- vocab answer_type
  question_text     text        NOT NULL,
  template_version  text        NOT NULL DEFAULT '',
  options           jsonb       NOT NULL DEFAULT '[]'::jsonb,   -- [{"value":..., "label":..., "source":"own_corpus|generic_list", "ref":...}]
  answer            jsonb,                                       -- shape per answer_type; null until answered
  answer_free_text  text        NOT NULL DEFAULT '',             -- <= 600 chars; becomes evidence, never parsed
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
COMMENT ON TABLE clarification_items IS
  'One controlled question per unresolved judgment, with the options offered and the answer as given (FSD FR-HF-13, FR-HF-14).';
CREATE INDEX idx_clarification_items_request ON clarification_items (tenant_id, request_id, answer_status);
CREATE TRIGGER clarification_items_set_updated_at
BEFORE UPDATE ON clarification_items
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Closes the loop 00066 left open: an answer-backed evidence item
-- names the item it came from.
ALTER TABLE evidence_items
  ADD CONSTRAINT fk_evidence_items_clarification_item
  FOREIGN KEY (clarification_item_id) REFERENCES clarification_items(id) ON DELETE SET NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE evidence_items DROP CONSTRAINT IF EXISTS fk_evidence_items_clarification_item;
DROP TABLE IF EXISTS clarification_items;
DROP TABLE IF EXISTS clarification_requests;

-- +goose StatementEnd
