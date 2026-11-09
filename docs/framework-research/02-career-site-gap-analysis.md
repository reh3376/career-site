# Career-site → Hiring System: Gap Analysis

**Purpose.** Map what `reh3376/career-site` already implements (at the 2026-11-08 `main`) against the data model (§5.1) and component list (§5.2) in `01-problem-definition-and-framework.md`, so the schema work and the Phase 0 build start from what exists rather than from zero.

**Status.** Written 2026-11-08 against `main` at `e333935`. The status columns describe the reviewer at that commit; re-check them when `services/api/internal/jd/` changes. Repository copy ahead of project (2026-11-08): this status line added.

**Short version.** The prototype is further along than the framework doc assumes on the *judgment, provenance, and calibration* side, and further behind on the *requirement-as-entity* and *multi-party* side. The single biggest structural gap is that requirements and judgments live as JSON inside one submission row rather than as first-class, versioned, editable records — which is exactly what the "next concrete step" (lock the Requirement and Equivalency Judgment schemas) has to fix.

---

## 1. Data model (§5.1)

| Framework entity | Career-site today | Status | Notes / gap |
|---|---|---|---|
| **Requirement** — id, text, type, weight, evidence standard, equivalence notes, version | `prompts.Requirement` {id, text, source_quote, category must/nice, weight 1–3, named_parties}; produced by `jd_requirements` prompt v2; validated in code (`validateRequirements` checks `source_quote` appears in the JD). Stored only inside `jd_submissions.assessment` jsonb. | **Partial** | No `type` taxonomy (skill/outcome/tool/domain/constraint/behavioral); must/nice is the only classification. No evidence standard or equivalence notes. Not editable by a human after extraction. Not a row, so cannot be versioned, shared across candidates, or joined to outcomes on its own. `named_parties` and the duration regex (`duration.go`) are de-facto "constraint" sub-types handled ad hoc. |
| **Evidence item** — attributable claim/artifact with context | `corpus_documents` + `corpus_chunks` (768-dim pgvector, HNSW cosine), `visibility` public/corpus_only, `source_kind` incl. `profile` (career facts sheet), `content_hash`, `embedder_model`. | **Exists (single-candidate)** | Evidence is chunks of the owner's corpus, not a per-candidate evidence store. Chunks are retrieval units, not claims; no dates/scope/metrics fields on a chunk. Tenancy seam (`tenant_id`, ADR 0029) exists but nothing resolves a tenant other than row 1. |
| **Equivalency judgment** — req id, evidence pointers, verdict, rationale, confidence, override fields | `jd.Judgment` {requirement_id, verdict met/partial/unmet, evidence_ids, rationale ≤400 chars, stated_span_years, parties_evidenced, adjusted, rationale_issues}. Every verdict also written to `decision_log` with prompt text, response, model, prompt version, tokens, latency, and `human_verdict` / `human_note` / `reviewed_by` / `reviewed_at`. | **Exists, strong** | This is the best-developed part. Override exists (`human_verdict`) but is *review-only*: it records disagreement for training, it does not change the stored verdict or recompute the score. No confidence field. No "insufficient evidence" verdict. Code-side rules (`duration.go`, `relationship.go`) weaken verdicts and record why in `adjusted`. |
| **Match record** — judgments + arithmetic score + gaps + schema/rule versions | `jd.Assessment` {requirements, judgments, evidence_ids, score, weight_total, score_formula `v2-nice-bonus`, model, prompts→version}. `jd_runs` (one immutable row per pipeline run: build, host, model, num_ctx, prompt fingerprints `v{n}:{sha4}`, corpus fingerprint, phase_ms). Re-score opens a new run rather than overwriting. | **Exists** | Score is in code (met 1, partial 0.5, unmet 0; must always in denominator, nice only when evidenced). Fit bands owner-editable in `app_settings`. Versioning is thorough. Gap: the record is readable by *one* non-expert (the owner's submitter view) but has never been tested with an HR-type reader; no per-requirement *gap classification* (substantive vs lexical). |
| **Outcome link** — stage reached, offer, 30/90-day fit ratings | `jd_outcomes` (not_pursued/applied/screening/interview/offer/rejected/no_response/withdrew, decided_on, note) and `jd_feedback` (source owner/submitter, target score/resume, rating accurate/too_generous/too_harsh/unusable). Ten SQL views in `docs/metrics.md`. | **Partial** | Candidate-side process outcomes exist. No employer-side outcomes, no post-hire fit/performance ratings (none possible with one user). Feedback vocabulary is deliberately directional (too generous vs too harsh), which is the right idea for calibration. |

## 2. Software components (§5.2)

| Framework component | Career-site today | Status | Notes / gap |
|---|---|---|---|
| **Role/JD parser → structured, editable requirement list** | `posting_check` (is this a posting?) → `jd_requirements` v2 extraction, JSON-schema constrained, `source_quote` verified against JD text. | **Partial** | Extraction works but is not editable. Known failure documented in backlog: extractor drops disambiguating words ("front-end development" in capital projects read as UI work). The framework doc assumes a clean decomposition; the prototype proves it isn't. Human edit of the requirement list before judging is the missing step. |
| **Evidence store + query interface** | Corpus ingest jobs, reindex/embed sweeps, public/private mounts, admin corpus console, `eval_run_documents` manifest with `content_hash`. | **Exists (single-tenant)** | Solid for one corpus. Multi-candidate needs: per-candidate corpora, candidate-controlled depth/audience (framework "privacy and control"), ingest from résumé/LinkedIn-type documents rather than curated markdown. |
| **Equivalency engine** | Per-requirement retrieval (4 chunks + profile sheet), one judge call per requirement (`requirement_judge` v12, schema-bounded output), code rules for duration and named-party relationships, rationale consistency checks (`rationale.go`), citation entailment checks for résumé lines (`entail.go`). Runs on `qwen3:4b-q8_0` on a CPX41; 100–135 s per judgment. | **Exists, strong design / weak throughput** | The "model reports, code decides" division is proven and should be kept as a design principle. Gaps: no explicit equivalence mappings or catalog (§5.4 research item); equivalency is implicit in the prompt. Throughput (15–30 min per JD) is acceptable for one candidate, not for ranking a pool. |
| **Match-record viewer for non-experts** | Member result page: fit category, score, "N of M evidenced", every requirement with verdict and rationale; admin `/admin/jd/[id]` with run history, outcome, feedback. | **Partial** | Built for the candidate and the owner. No employer/HR-facing ranking or filtering view, no cross-candidate comparison, no override UI that changes a verdict. |
| **Calibration and analysis layer** | `golden_postings` (owner label above/below gate), `eval_runs` / `eval_items` (gate accuracy, inversions, margin), agreement panel on `/admin/decisions` (judge vs owner matrix by prompt version and model), `llm-tuning-log.md` as the experiment record, metric views. | **Exists, strong** | This is well ahead of what the framework doc specifies for Phase 0. Gaps: golden set is 10 postings (7 above, 3 below — acknowledged thin on negatives); labels are gate-level, not requirement-level; no adverse-impact / disparate-verdict reporting; no feedback path from agreement data into equivalence notes or weights (the "controlled, versioned update path" in §5.3). |
| **Process / communication layer** | Email notifications on terminal states, progress modal, meeting scheduler. | **Exists (candidate side)** | Nothing for the employer side of the process. Fine to defer. |

## 3. Cross-cutting: recursive improvement layer (§5.3)

- **Instrumentation with joinable ids** — **Exists.** `run_id` rides the context onto every `llm_usage` and `decision_log` row; `tenant_id` on data tables; prompt fingerprints; corpus manifests with content hashes.
- **Calibration views (system vs expert vs outcome)** — **Partial.** System vs expert exists (agreement panel). Outcome joins exist at the posting level, not per requirement.
- **Controlled, versioned update path for schemas/weights/equivalence notes** — **Missing.** Prompts are versioned in code; weights and equivalence knowledge are not data at all.

## 4. What this changes about the next step

The schema work should be scoped as *promoting existing JSON to first-class entities*, not inventing new ones:

1. **Requirement** becomes a row with: id, role_id, text, source_quote, `type` (new taxonomy), category, weight, `evidence_standard` (new), `equivalence_notes` (new), `edited_by` / `edited_at` (new — HM edits before judging), version. The extractor's output becomes a *draft* of this row.
2. **Equivalency judgment** — *superseded by `03-verdict-taxonomy-and-clarification-loop.md`*, which replaces the single verdict with determination + basis + uncertainty and adds an effective override path with the match record recomputed from effective values while the model verdict is preserved for agreement metrics.
3. **Evidence item** needs a per-candidate owner and a visibility/audience control before any second candidate exists; the tenancy seam makes this a column change, not a redesign.
4. Keep as design principles carried over from the prototype: score computed in code; model reports facts and code decides comparisons; every verdict logged with its inputs; re-score creates a new run rather than overwriting; a run records the configuration that produced it.

## 5. Risks carried over from the prototype

- Requirement extraction quality is the limiting factor, and the prototype shows it fails on vocabulary collisions across domains. A human-editable requirement list is a Phase 0 need, not Phase 2.
- Latency on local inference is 15–30 min per JD. Pool ranking needs either a hosted model option, batching by requirement across candidates, or a cheaper first-pass judge with escalation.
- Negative labels are scarce. Any claim about false-negative reduction needs a baseline (keyword filter run in parallel on the same pool), which does not yet exist.
- Second-candidate data brings third-party confidential material into the corpus (noted open in backlog §1b). Needs a decision before Phase 1.
