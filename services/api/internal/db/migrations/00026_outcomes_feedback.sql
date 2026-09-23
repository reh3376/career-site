-- +goose Up
-- +goose StatementBegin

-- Ground truth and judgment about a review (data layer D3).
--
-- Everything measured so far is internal: the model's verdicts, the
-- owner's labels on those verdicts, the timings. All of it answers
-- "was the reasoning sound", and none of it answers "was the answer
-- right". A posting can be scored 0.92 with every requirement properly
-- evidenced and still be a job that goes nowhere, and until that is
-- recorded there is no way to know whether the score predicts anything
-- at all.
--
-- Two tables, because they are different kinds of claim:
--   jd_outcomes  what actually happened in the world, one per posting,
--                updated as it unfolds. Slow, factual, sparse.
--   jd_feedback  what a person thought of the output, many per run.
--                Fast, subjective, attached to a specific run so a
--                later re-score does not inherit an opinion of the
--                thing it replaced.

CREATE TABLE jd_outcomes (
  id            bigserial PRIMARY KEY,
  tenant_id     bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  -- One outcome per posting, revised in place as it progresses.
  submission_id bigint      NOT NULL UNIQUE REFERENCES jd_submissions(id) ON DELETE CASCADE,
  -- not_pursued | applied | screening | interview | offer | rejected |
  -- no_response | withdrew. Validated in the handler, stored as text so
  -- adding a stage is not a migration.
  status        text        NOT NULL,
  -- When that status became true, as the person knows it. Distinct from
  -- when it was typed in, which is updated_at.
  decided_on    date,
  note          text        NOT NULL DEFAULT '',
  recorded_by   bigint      REFERENCES users(id) ON DELETE SET NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE jd_outcomes IS
  'What happened in the world after a review: the only signal that says whether a score predicted anything.';

CREATE INDEX idx_jd_outcomes_status ON jd_outcomes (tenant_id, status, updated_at DESC);

CREATE TABLE jd_feedback (
  id            bigserial PRIMARY KEY,
  tenant_id     bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  -- The run being judged. Null only for feedback on a submission that
  -- predates the run table.
  run_id        uuid        REFERENCES jd_runs(run_id) ON DELETE SET NULL,
  submission_id bigint      NOT NULL REFERENCES jd_submissions(id) ON DELETE CASCADE,
  -- owner | submitter: whose opinion this is. They are not the same
  -- kind of evidence and must never be averaged together.
  source        text        NOT NULL,
  user_id       bigint      REFERENCES users(id) ON DELETE SET NULL,
  -- What is being judged: score | resume.
  target        text        NOT NULL,
  -- Fixed per target, validated in the handler. For `score`:
  -- accurate | too_generous | too_harsh | unusable. For `resume`:
  -- would_send | needs_edits | wrong. Deliberately not a thumb: "too
  -- generous" and "too harsh" point at opposite fixes, and a thumb
  -- down cannot tell them apart.
  rating        text        NOT NULL,
  note          text        NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE jd_feedback IS
  'A person''s judgment of one run''s output, kept apart from the model''s own record.';

CREATE INDEX idx_jd_feedback_run ON jd_feedback (run_id) WHERE run_id IS NOT NULL;
CREATE INDEX idx_jd_feedback_submission ON jd_feedback (tenant_id, submission_id, id DESC);
-- One current opinion per person per target per run: a second rating
-- replaces the first rather than stacking, so counting ratings counts
-- people rather than clicks.
CREATE UNIQUE INDEX idx_jd_feedback_one_per_target
  ON jd_feedback (submission_id, coalesce(run_id, '00000000-0000-0000-0000-000000000000'::uuid), source, target, coalesce(user_id, 0));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS jd_feedback;
DROP TABLE IF EXISTS jd_outcomes;

-- +goose StatementEnd
