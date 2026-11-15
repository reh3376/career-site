-- +goose Up
-- +goose StatementBegin

-- A résumé written after review (the owner's decision of 2026-11-13,
-- FSD FR-HF-08). When a person's verdicts lift a posting that scored
-- below the résumé gate to at or above it, the owner may ask for the
-- tailored résumé to be written on the verdicts as they now stand. It
-- is his alone: shown on the admin page, never emailed, never shown to
-- the member who submitted the posting.
--
-- That is why it has a table of its own. The member's review reads its
-- résumé from jd_submissions (resume_markdown, the PDF, the status
-- `ready`); a résumé stored there would be released to the recruiter by
-- the result RPC and would flip a below-threshold submission to ready.
-- Nothing the member's page reads is touched by this table.
--
-- One row per run, keyed by the run whose record the review was made
-- on. Asking again replaces it. `writing` means a write is in flight in
-- the running API process; the API fails any row left at `writing` when
-- it starts, because a new process has nothing in flight.
CREATE TABLE review_resumes (
  run_id          uuid        PRIMARY KEY REFERENCES jd_runs(run_id) ON DELETE CASCADE,
  tenant_id       bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  submission_id   bigint      NOT NULL REFERENCES jd_submissions(id) ON DELETE CASCADE,
  status          text        NOT NULL DEFAULT 'writing',
  error           text        NOT NULL DEFAULT '',
  resume_json     jsonb,
  markdown        text        NOT NULL DEFAULT '',
  pdf             bytea,
  pdf_pages       integer     NOT NULL DEFAULT 0,
  model           text        NOT NULL DEFAULT '',
  prompt_id       text        NOT NULL DEFAULT '',
  prompt_version  integer     NOT NULL DEFAULT 0,
  -- What the review stood at when the résumé was asked for: the gate's
  -- score with the person's verdicts, the gate, and how many verdicts a
  -- person had set. Kept so the row says on what ground it was written
  -- even after the record changes.
  reviewed_score  double precision NOT NULL,
  threshold       double precision NOT NULL,
  override_count  integer     NOT NULL DEFAULT 0,
  requested_by    bigint      REFERENCES users(id) ON DELETE SET NULL,
  requested_at    timestamptz NOT NULL DEFAULT now(),
  finished_at     timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT review_resumes_status_ck CHECK (status IN ('writing', 'ready', 'failed'))
);
COMMENT ON TABLE review_resumes IS
  'A résumé written at the owner''s request on a run''s verdicts after review; the owner''s alone, never read by the member''s review (FSD FR-HF-08).';
CREATE INDEX idx_review_resumes_submission ON review_resumes (tenant_id, submission_id);
CREATE TRIGGER review_resumes_set_updated_at
BEFORE UPDATE ON review_resumes
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- The record view learns whether the run wrote a résumé of its own,
-- which decides whether one after review is offered at all.
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
       m.requirement_ids, r.trigger AS run_trigger, r.corpus_documents, m.updated_at,
       r.resume_generated AS run_resume_generated, r.retrieval_scope AS run_retrieval_scope
FROM match_records m
JOIN jd_runs r ON r.run_id = m.run_id;
COMMENT ON VIEW v_match_record IS
  'One row per run: the three numbers, the two shares, override count, the requirement versions judged and provenance; what the viewer and exports read (FSD FR-HF-10, FR-HF-17).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

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

DROP TABLE IF EXISTS review_resumes;

-- +goose StatementEnd
