-- +goose Up
-- +goose StatementBegin

-- What reads the hiring rows (docs/framework-research/14 §2.4). Every
-- count and every agreement figure is a view over the rows, never a
-- stored number (docs/metrics.md). Several of these are empty until
-- step 4 writes the first judgment; they exist now so the questions
-- they answer are settled before the data arrives, and so the dual
-- write of step 3 can be checked on every real run from the first.

-- The flat projection the result RPC and any export read: one row per
-- run with the three numbers, the two shares, the override count, and
-- the provenance jd_runs already holds.
CREATE VIEW v_match_record AS
SELECT m.run_id, m.tenant_id, m.role_id, m.candidate_id, m.attempt, m.supersedes_run_id,
       r.submission_id, r.status AS run_status,
       m.formula_version, m.evidenced, m.unresolved, m.unmet,
       m.coverage_equivalent_share, m.self_attested_share, m.override_count,
       cardinality(m.requirement_ids) AS requirement_count,
       m.keyword_baseline, m.versions, m.schema_version,
       r.model, r.num_ctx, r.embedder_model, r.prompts, r.corpus_fingerprint,
       r.score_formula AS run_score_formula, r.match_score AS run_match_score, r.fit,
       r.started_at, r.finished_at, m.created_at
FROM match_records m
JOIN jd_runs r ON r.run_id = m.run_id;
COMMENT ON VIEW v_match_record IS
  'One row per run: the three numbers, the two shares, override count and provenance; what the viewer and exports read (FSD FR-HF-10).';

-- Every judgment with its effective determination and basis, the
-- requirement text and version it was judged against, and the kinds of
-- evidence it cited. What the viewer renders.
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

-- confirmations and rejections per catalog entry, derived from the
-- events. The promotion rule in 05 §1.3 reads these.
CREATE VIEW v_catalog_counts AS
SELECT c.id AS entry_id, c.tenant_id, c.kind, c.strength, c.status, c.source,
       count(*) FILTER (WHERE ev.kind = 'confirm') AS confirmations,
       count(*) FILTER (WHERE ev.kind = 'reject')  AS rejections,
       count(*) FILTER (WHERE ev.kind = 'propose') AS proposals,
       count(DISTINCT ev.actor) FILTER (WHERE ev.kind = 'confirm') AS confirming_reviewers,
       max(ev.created_at) AS last_event_at
FROM equivalence_catalog c
LEFT JOIN catalog_events ev ON ev.entry_id = c.id
GROUP BY c.id;
COMMENT ON VIEW v_catalog_counts IS
  'Confirmations and rejections per catalog entry, counted from catalog_events rather than stored (FSD FR-HF-16).';

-- Owner override against model determination, by prompt version and
-- model. Successor to v_judge_agreement for the new vocabulary; the
-- old view keeps reading decision_log.output.verdict, which FR-HF-07
-- continues to write.
CREATE VIEW v_determination_agreement AS
SELECT tenant_id, prompt_version, model,
       determination AS model_determination,
       override_determination,
       determination = override_determination AS agreed,
       count(*) AS judgments
FROM judgments
WHERE override_determination IS NOT NULL
GROUP BY tenant_id, prompt_version, model, determination, override_determination;
COMMENT ON VIEW v_determination_agreement IS
  'Owner override versus model determination, by prompt version and model (FSD FR-HF-08; successor to v_judge_agreement).';

-- The more useful matrix (03 §2.4): which kind of equivalency the model
-- misjudges.
CREATE VIEW v_basis_agreement AS
SELECT tenant_id, prompt_version, model,
       basis AS model_basis,
       override_basis,
       basis = override_basis AS agreed,
       count(*) AS judgments
FROM judgments
WHERE override_basis IS NOT NULL
GROUP BY tenant_id, prompt_version, model, basis, override_basis;
COMMENT ON VIEW v_basis_agreement IS
  'Owner override basis versus model basis, by basis code: which kind of equivalency is misjudged (FSD FR-HF-08).';

-- Per run, per requirement: did the judge find evidence, and would a
-- plain term-overlap filter over the same offered evidence have found
-- it (FR-HF-12). Judge columns are null until step 4; until then the
-- view reports the baseline alone, which is still the half of the
-- comparison that needs no model.
CREATE VIEW v_keyword_baseline AS
SELECT m.run_id, m.tenant_id, m.role_id, m.candidate_id,
       m.keyword_baseline->>'rule' AS keyword_rule,
       kb.key AS requirement_local_id,
       (kb.value->>'found')::boolean AS keyword_found,
       kb.value->'terms' AS keyword_terms,
       j.effective_determination,
       CASE WHEN j.id IS NULL THEN NULL
            ELSE j.effective_determination IN ('met', 'met_equivalent') END AS judge_found,
       m.created_at
FROM match_records m
CROSS JOIN LATERAL jsonb_each(COALESCE(m.keyword_baseline->'items', '{}'::jsonb)) AS kb(key, value)
LEFT JOIN judgments j ON j.run_id = m.run_id AND j.requirement_local_id = kb.key;
COMMENT ON VIEW v_keyword_baseline IS
  'Per run and requirement: whether a keyword filter over the offered evidence found it, beside what the judge found (FSD FR-HF-12).';

-- The dual write of step 3, checked on every real run (14 §4). The
-- CI test TestDualWriteAgrees proves the writer on a fixture; this view
-- says whether production agrees, run by run, so the hand-over at step
-- 4 can be decided from data rather than from the test alone.
CREATE VIEW v_dual_write_agreement AS
SELECT r.run_id, r.tenant_id, r.submission_id, r.attempt, r.status, r.finished_at,
       r.match_score, m.evidenced,
       r.requirement_count, cardinality(m.requirement_ids) AS requirement_rows,
       m.run_id IS NOT NULL AS record_written,
       m.run_id IS NOT NULL
         AND r.requirement_count = cardinality(m.requirement_ids)
         AND (r.match_score IS NULL AND m.evidenced IS NULL
              OR abs(COALESCE(r.match_score, 0) - COALESCE(m.evidenced, 0)) < 1e-9) AS agrees
FROM jd_runs r
LEFT JOIN match_records m ON m.run_id = r.run_id
JOIN jd_submissions s ON s.id = r.submission_id
WHERE r.status IN ('generating', 'below_threshold', 'ready')
   OR m.run_id IS NOT NULL;
COMMENT ON VIEW v_dual_write_agreement IS
  'Per run: does the match record agree with jd_runs on the score and the requirement count (14 §4; the production half of TestDualWriteAgrees).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP VIEW IF EXISTS v_dual_write_agreement;
DROP VIEW IF EXISTS v_keyword_baseline;
DROP VIEW IF EXISTS v_basis_agreement;
DROP VIEW IF EXISTS v_determination_agreement;
DROP VIEW IF EXISTS v_catalog_counts;
DROP VIEW IF EXISTS v_judgments_effective;
DROP VIEW IF EXISTS v_match_record;

-- +goose StatementEnd
