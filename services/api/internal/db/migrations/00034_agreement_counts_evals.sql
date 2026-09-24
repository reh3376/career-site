-- +goose Up
-- +goose StatementBegin

-- Evaluation runs count towards agreement after all.
--
-- Migration 00032 excluded them, reasoning by analogy with v_jd_runs,
-- which leaves them out because they are a test of the reviewer rather
-- than use of it. That analogy is wrong, and it took one afternoon to
-- prove: the owner reviewed twenty-nine requirement verdicts and every
-- one of them was on a golden-set run, so the metric silently
-- discarded the lot and went on reporting four.
--
-- The distinction that matters is what a metric is for. v_jd_runs
-- measures use: how long a real submission waits, how many arrive, and
-- there a test run would flatter both. Agreement measures the judge's
-- quality against the owner's labels, and an evaluation run is the
-- same pipeline reading the same corpus about the same postings. Its
-- verdicts are exactly as good a sample of judgment as a live
-- submission's, and the golden set exists precisely to produce them.
--
-- Worse than wrong, it was incoherent: /admin/decisions offers those
-- rows for review, so the console asked for work the metric then threw
-- away. A surface that invites a judgment and a number that ignores it
-- cannot both be right.
--
-- The posting-check exclusion stays. Input the gatekeeper refused is
-- not a posting, whoever submitted it.

CREATE OR REPLACE VIEW v_judge_agreement AS
SELECT d.tenant_id,
       d.id,
       d.run_id,
       d.ref_id       AS submission_id,
       d.key          AS requirement_id,
       d.model,
       d.prompt_id,
       d.prompt_version,
       d.output->>'verdict' AS model_verdict,
       d.human_verdict,
       d.human_verdict <> 'insufficient_evidence' AS gradeable,
       (d.output->>'verdict') = d.human_verdict AS agreed,
       d.human_note,
       d.reviewed_at
  FROM decision_log d
 WHERE d.kind = 'jd_requirement_verdict'
   AND d.reviewed_at IS NOT NULL
   AND d.human_verdict IS NOT NULL
   -- Excluded only when the gatekeeper actually said no. A submission
   -- it never examined is counted, because absence of a check is not a
   -- finding.
   AND NOT EXISTS (
         SELECT 1 FROM decision_log p
          WHERE p.kind = 'jd_posting_check'
            AND p.ref_id = d.ref_id
            AND p.output->>'is_posting' = 'false'
       );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE VIEW v_judge_agreement AS
SELECT d.tenant_id,
       d.id,
       d.run_id,
       d.ref_id       AS submission_id,
       d.key          AS requirement_id,
       d.model,
       d.prompt_id,
       d.prompt_version,
       d.output->>'verdict' AS model_verdict,
       d.human_verdict,
       d.human_verdict <> 'insufficient_evidence' AS gradeable,
       (d.output->>'verdict') = d.human_verdict AS agreed,
       d.human_note,
       d.reviewed_at
  FROM decision_log d
  JOIN jd_submissions s ON s.id = d.ref_id
 WHERE d.kind = 'jd_requirement_verdict'
   AND d.reviewed_at IS NOT NULL
   AND d.human_verdict IS NOT NULL
   AND NOT s.is_eval
   AND NOT EXISTS (
         SELECT 1 FROM decision_log p
          WHERE p.kind = 'jd_posting_check'
            AND p.ref_id = d.ref_id
            AND p.output->>'is_posting' = 'false'
       );
-- +goose StatementEnd
