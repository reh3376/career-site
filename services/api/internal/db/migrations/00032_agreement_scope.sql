-- +goose Up
-- +goose StatementBegin

-- What the agreement metric is allowed to count.
--
-- Agreement was 63.6% over 11 reviewed verdicts with 4 hard
-- disagreements. Seven of those eleven came from one submission that
-- was not a job description: a 365-character job-search worksheet whose
-- extracted "requirements" were the fragments "Data Science", "Machine
-- Learning" and "Principal Software Engineer". There is no assertion in
-- "Data Science" to judge, so neither agreeing nor disagreeing with a
-- verdict on it says anything about the reviewer.
--
-- The posting-check gatekeeper refuses that input now. A metric that
-- holds the judge accountable for work the pipeline would no longer
-- accept is measuring the wrong thing.
--
-- The obvious hazard: dropping the rows that make a number look bad is
-- how metrics get gamed. Two things keep this honest and both matter
-- more than the rule itself. The gatekeeper decides what counts, not
-- the verdict, so a refused posting is excluded whether its verdicts
-- were flattering or not. And the exclusion is written here rather than
-- applied case by case, so it cannot be reached for the next time a
-- number disappoints.
--
-- Evaluation runs are excluded for the reason v_jd_runs already gives:
-- they are a test of the reviewer, not use of it. There are none in the
-- table today, so this states the rule rather than fixing a fault, and
-- states it before it can become one.

-- Replaced in place rather than dropped. The column list is identical,
-- only the rows it admits change, so CREATE OR REPLACE applies without
-- disturbing anything built on top of it.
--
-- The first version of this migration dropped the view instead, which
-- failed in production: goose runs 00031 first, so v_gate_agreement
-- already depended on v_judge_agreement_summary by the time this ran,
-- and Postgres refused. It passed locally only because the two were
-- applied in the opposite order by hand, which is not an order goose
-- can ever produce.
--
-- The column list below is the one migration 00029 left behind, which
-- added gradeable and human_note. Writing this from the 00028
-- definition dropped two columns and Postgres refused that too. A view
-- replacement has to be written against the view as it now stands, not
-- as the migration that first created it left it.
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
   -- Excluded only when the gatekeeper actually said no. A submission
   -- it never examined is counted, because absence of a check is not a
   -- finding.
   AND NOT EXISTS (
         SELECT 1 FROM decision_log p
          WHERE p.kind = 'jd_posting_check'
            AND p.ref_id = d.ref_id
            AND p.output->>'is_posting' = 'false'
       );

-- v_judge_agreement_summary is unchanged and keeps reading from the
-- view above, so it is left alone.

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
 WHERE d.kind = 'jd_requirement_verdict'
   AND d.reviewed_at IS NOT NULL
   AND d.human_verdict IS NOT NULL;
-- +goose StatementEnd
