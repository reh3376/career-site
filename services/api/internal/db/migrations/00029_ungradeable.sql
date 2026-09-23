-- +goose Up
-- +goose StatementBegin

-- "I could not judge this" is a third answer, and it is not a
-- disagreement.
--
-- The first real review found the gap: on a posting where the evidence
-- put in front of the reviewer was not enough to make a credible call,
-- the only options were to agree or disagree with the model. Forcing a
-- verdict there produces a wrong label, and a wrong label is worse than
-- no label, because the agreement rate then counts it as signal.
--
-- `insufficient_evidence` is excluded from the agreement rate rather
-- than counted against it, and surfaced on its own. It is the most
-- actionable grade in the set: a pile of them says retrieval is not
-- putting the right documents in front of the judge, which no amount of
-- prompt work will fix.

DROP VIEW IF EXISTS v_judge_agreement_summary;
DROP VIEW IF EXISTS v_judge_agreement;

CREATE VIEW v_judge_agreement AS
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
       -- A row the reviewer could actually judge. Everything downstream
       -- filters on this rather than assuming every reviewed row is a
       -- comparison.
       d.human_verdict <> 'insufficient_evidence' AS gradeable,
       (d.output->>'verdict') = d.human_verdict   AS agreed,
       d.human_note,
       d.reviewed_at
  FROM decision_log d
 WHERE d.kind = 'jd_requirement_verdict'
   AND d.reviewed_at IS NOT NULL
   AND d.human_verdict IS NOT NULL;

COMMENT ON VIEW v_judge_agreement IS 'Reviewed requirement verdicts: the model''s call beside the owner''s, with whether it was gradeable at all.';

CREATE VIEW v_judge_agreement_summary AS
SELECT tenant_id,
       count(*)                                        AS reviewed,
       count(*) FILTER (WHERE gradeable)               AS gradeable,
       count(*) FILTER (WHERE NOT gradeable)           AS ungradeable,
       count(*) FILTER (WHERE gradeable AND agreed)    AS agreed,
       -- Out of what could be judged, not out of everything reviewed.
       round(100.0 * count(*) FILTER (WHERE gradeable AND agreed)
             / NULLIF(count(*) FILTER (WHERE gradeable), 0), 1) AS agreement_pct,
       -- A disagreement touching "partial" is the judge being unsure; one
       -- between met and unmet is the judge being wrong. Different fixes.
       count(*) FILTER (WHERE gradeable AND NOT agreed
                        AND 'partial' IN (model_verdict, human_verdict)) AS soft_disagreements,
       count(*) FILTER (WHERE gradeable AND NOT agreed
                        AND 'partial' NOT IN (model_verdict, human_verdict)) AS hard_disagreements,
       -- Which way the judge errs. Three harsh and no generous is a bias
       -- to fix; a mix of both is noise, and the two call for opposite
       -- changes, so counting them together would hide the answer.
       count(*) FILTER (WHERE gradeable AND NOT agreed
                        AND model_verdict = 'unmet' AND human_verdict IN ('met', 'partial')) AS too_harsh,
       count(*) FILTER (WHERE gradeable AND NOT agreed
                        AND model_verdict IN ('met', 'partial') AND human_verdict = 'unmet') AS too_generous
  FROM v_judge_agreement
 GROUP BY tenant_id;

COMMENT ON VIEW v_judge_agreement_summary IS 'Agreement over the rows that could be judged, with which way the judge errs.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP VIEW IF EXISTS v_judge_agreement_summary;
DROP VIEW IF EXISTS v_judge_agreement;

CREATE VIEW v_judge_agreement AS
SELECT d.tenant_id, d.id, d.run_id, d.ref_id AS submission_id, d.key AS requirement_id,
       d.model, d.prompt_id, d.prompt_version,
       d.output->>'verdict' AS model_verdict, d.human_verdict,
       (d.output->>'verdict') = d.human_verdict AS agreed, d.reviewed_at
  FROM decision_log d
 WHERE d.kind = 'jd_requirement_verdict'
   AND d.reviewed_at IS NOT NULL AND d.human_verdict IS NOT NULL;

CREATE VIEW v_judge_agreement_summary AS
SELECT tenant_id, count(*) AS reviewed, count(*) FILTER (WHERE agreed) AS agreed,
       round(100.0 * count(*) FILTER (WHERE agreed) / NULLIF(count(*), 0), 1) AS agreement_pct,
       count(*) FILTER (WHERE NOT agreed AND 'partial' IN (model_verdict, human_verdict)) AS soft_disagreements,
       count(*) FILTER (WHERE NOT agreed AND 'partial' NOT IN (model_verdict, human_verdict)) AS hard_disagreements
  FROM v_judge_agreement GROUP BY tenant_id;

-- +goose StatementEnd
