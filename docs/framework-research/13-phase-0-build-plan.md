# Phase 0 Build Plan: From the Prototype to the Framework's First Running Version

**Status.** Written 2026-11-08 on the owner's instruction of the same day ("you have all the necessary context to make these decisions yourself; you can decide the order to build the framework out"). For the scope named in §1 it supersedes the "do not start building" line in `agent-handoff.md` §6. Repository copy ahead of project (2026-11-08): new document.

---

## 0. Decisions taken here

The owner delegated the order of work and the open operational questions. Each decision is recorded with its reasoning so it can be overturned knowingly rather than drifted past.

1. **Where participant material lives.** Anything a pilot participant said or is (interviewer notes `pilot-interview-NN.md`, the viewer-requirements backlog seeded from stimulus reactions, survey exports) goes in the private repository at `docs/personal/framework-research/`, never in this public one. Instrument text, templates and the change log stay here. Reason: FSD A-05, everything committed to the public repository is public whatever the gate says, and `agent-handoff.md` §8 covered candidate data but not participant data.
2. **P4 outputs are repository documents first.** Design documents are written here at the next numbered slots, marked "repository copy ahead of project", and re-copied to the Hiring_prod project by the owner. Reason: the schema rendering has to be checked against the code it generalises, and the code is here.
3. **The build starts before the pilot, on the parts the pilot cannot change.** The handoff deferred Phase 0 until after the pilot so that scope would follow measured employer problems. The pilot decides four things (C-14 slate cuts, K-04 pre-application gap view, K-09 dispute, hold-state priority), all of them workflow surfaces. It cannot change what the owner has already approved: requirement as a versioned entity, determination + basis + uncertainty, three coverage numbers, evidence provenance, the clarification data model, the versioning policy. That core is built now. The four pilot-dependent surfaces are out of Phase 0 by name (§1).
4. **One candidate in Phase 0.** The owner's corpus is the only evidence store. Every new table carries `candidate_id` and `tenant_id` from birth so a second candidate is a row rather than a redesign, but no second person's documents enter the corpus until the owner decides the confidential-material question (`02` §5, `docs/backlog.md` §1b). This closes P5 item 1 for Phase 0 without closing it for Phase 1.
5. **A keyword baseline ships with Phase 0.** Every match record also records, per requirement, whether a plain term-overlap filter over the same evidence would have found it. In code, no model, cheap. Reason: without it the false-negative claim has nothing to compare against (P5 item 2), and the comparison is the product's argument.
6. **Throughput is not Phase 0's problem.** One candidate, a handful of postings, and the CPX41 is the ceiling: the owner has said no further server spend. The pipeline stays one judge call per (requirement, candidate), which is the shape that batches across candidates later. Pool ranking waits for a second candidate and a funding decision (P5 item 3).
7. **Reviewer changes go through the FSD and an ADR**, as the handoff requires. One ADR records the generalisation decision; a new FSD section carries the framework's requirements, each row **Not built** until it lands; the open questions become `D-` rows.
8. **Survey platform.** Google Forms for the pilot (section per screen, role branching from A1, per-item flag as a checkbox, fixed C1 order), Tally for the main wave (randomised option order, branching, CSV export, free tier). The pilot reads C1 only for item confusion (`08` §5), so randomisation is not needed there.

Answered by the owner on 2026-11-08, in reply to the questions in §5:

9. **The agreement history is a closed series.** The 31 owner-graded met/partial/unmet verdicts are kept as they are. Step 4 derives the three-way verdict from the new `determination` in code (`met` and `met_equivalent` → met; `unresolved` → partial; `not_met` → unmet) so the historical agreement rate stays comparable across the change, and basis agreement starts fresh from step 4. No re-grading.
10. **`candidate_response` evidence counts at full weight in Phase 0.** A judgment resting on a clarification answer counts the same as one resting on a document; the match record reports `self_attested_share` so a reader sees how much of the coverage rests on self-attestation. Whether to discount is decided later from calibration data, as `04` §7 proposed.
11. **The Phase 0 exit reader.** The owner reads the golden-set records first as a dry run, to catch anything unreadable; then one recruiter or HR participant from the pilot's ten reads three records for fifteen minutes and says which candidate-role pairs look strong and why. The exit claim rests on the participant's reading; the owner's is the rehearsal.

## 1. What Phase 0 delivers, and what it does not

**Delivers**, per `01` §5.5: the Requirement and Equivalency Judgment schemas as data rather than JSON in a submission row; a minimal extractor, judge and readable match record under the new vocabulary; run against the owner's corpus and the golden set with the calibration layer still working; a keyword baseline beside every record.

**Does not deliver**, because the pilot decides it or it belongs to Phase 1 and later: the HM slate-cut override view (C-14); candidate dispute (K-09); the pre-application gap view (K-04); the hold-state entity's priority and escalation mechanics; an employer-side ranking view across candidates; outcome links beyond what `jd_outcomes` and `jd_feedback` already record; working-style indicators.

## 2. Sequence

Each step is one pull request or a short series. Each carries its tests, its documentation (FSD row status, `SERVICES.md` migration rows, `docs/llm-tuning-log.md` for anything that touches the judge, `build-stats.json` regenerated), and a golden-set evaluation wherever the judge changes. The order is chosen so that the reviewer keeps working after every step.

| Step | Scope | Why here | Exit |
|---|---|---|---|
| 0 | P2 pilot materials (`11`, `12`), the private folder, the stimulus checked at phone and laptop widths | Short, and it unblocks the owner's recruiting (P3), which has lead time | The owner can post and schedule |
| 1 | ADR 0032, generalising the reviewer into the framework; FSD §5.15 `FR-HF-01..` with every row Not built; D-32 onward opened for the questions in §5 | Spec-first (`AGENTS.md`); nothing below is permitted before it | `check_docs.py` green, §2b counts moved, build-stats regenerated |
| 2 | `14-schema-rendering.md`: proto messages and SQL DDL for Requirement, Judgment, EvidenceItem, ClarificationRequest and Item, MatchRecord, EquivalenceCatalog, Concept; columns vs jsonb decided per `04` §7; `asked_by`; upcast functions named; dual-write plan. **Written 2026-11-08.** | P4 items 1 to 4 as design before any migration | Owner review, asynchronous; `14` §6 lists the six points, silence by the time step 3 opens is agreement |
| 3 | Migrations and the Go repository layer per `14` §2 and §5: `roles`, `candidates`, `vocab_codes`, `requirements` (versioned rows, `supersedes`), `judgments`, `evidence_items` (new kinds beside `corpus_chunks`), `match_records` (extends `jd_runs`), `clarification_requests` and `clarification_items`, `concepts`, `equivalence_catalog`, `catalog_events`, and the views; `candidate_id` and `tenant_id` on every one; the pipeline writes both the old jsonb and the new rows (judgment rows begin at step 4, when the judge emits the fields they need; `14` §4) | Data before logic; the old representation stays authoritative until step 4 proves the new one | Migrations replay in CI; `TestDualWriteAgrees` proves the two representations agree on every golden run |
| 4 | Judge v13: output schema per `04` §2.3 (`determination`, `basis`, `uncertainty`, `evidence_ids`, `reported_facts`, `rationale`, `confidence`); consistency rules reject and retry; `duration.go` and `relationship.go` emit `rule_adjustments` and set `unresolved` plus a code or `not_met` with a shortfall basis; new scope, recency and credential rules; scorer formula v3 with three numbers; the old met/partial/unmet **derived** from the new determination so the agreement matrices keep their history | The core of the framework; everything after reads its output | Golden set: no inversions, margin reported; tuning-log entry with before and after |
| 5 | Requirement as an editable entity: the extractor writes draft rows with `type`, `alternatives`, `thresholds`, `evidence_standard` rendered from the templates in `05` §2, `equivalence_notes` from the seeded catalog in `05` §1.4; an admin edit surface before judging; an edit creates a new version; re-judge on demand | The handoff's "first employer-side surface", and the fix for the prototype's known extraction failure (the capital-projects "front-end development" case) | An edited requirement is judged as edited and the judgment names the version |
| 6 | Match record viewer v2: three numbers, basis label per row, `coverage_equivalent_share`, `self_attested_share`, the keyword-baseline column, gap classification (lexical vs substantive from basis); member view and admin view; an override that changes the effective value while the model's stays logged | Makes the new vocabulary legible; it is what the pilot stimulus mocks | The owner reads a real record in the new form and the pilot stimulus is regenerated from it |
| 7 | Clarification loop, system stage, with the owner as the candidate: question templates keyed by `(answer_type, requirement.type)`; options drawn from own corpus and generic ladders only; `candidate_response` evidence; re-judge scoped to the affected requirements; one round in Phase 0 | Proves the data model end to end on one person before anyone else is asked anything | One `unresolved` judgment resolved by an answer, visible in the record with its provenance |
| 8 | Catalog write-back: an override confirms or rejects the cited entry; an override reason naming an unknown equivalence creates a `proposed` entry; the promotion rule as configuration under `rule_version` | The first feedback path from judgments into reusable knowledge (`05` §1.3) | A `met_equivalent` names its entry and the counts move |
| 9 | Phase 0 exit: the golden set plus three new technical postings; the owner dry-runs the reading, then one pilot participant (recruiter or HR) identifies strong coverage from three records alone (decision 11); the exit note written here | `01` §5.5's validation criterion | Exit note at the next numbered slot |

Steps 0 to 2 are documents and need no quiet window. Steps 4 and 5 each need a golden-set evaluation, 5 to 6 hours on the box, which refuses to run while the site is busy; those are scheduled overnight.

## 3. What stays out of the public repository

Participant material (decision 1); the owner's private corpus (already excluded); any second candidate's documents (decision 4); survey exports; anything that would name an employer a participant works for.

## 4. Risks carried into the build

- **A 4b model under a richer output schema.** Twelve basis codes, eight uncertainty codes and a `reported_facts` object are more for `qwen3:4b-q8_0` to get right than three words were. Mitigations: field order settled before free text (FR-JD-15), consistency rules that reject and retry, the golden set before and after, and the derived three-way verdict so the owner's 31 graded judgments stay comparable.
- **Two representations of one judgment during steps 3 and 4.** Bounded: step 4's exit removes the jsonb writer's authority, and a test fails if the two disagree in the meantime.
- **Evaluation cost.** Each judge change costs an overnight run. Steps are grouped so the judge changes once (step 4) and the extractor once (step 5).
- **Every step touches `docs/FSD.md`.** The status table and `build-stats.json` both count FR rows, so each PR regenerates the JSON and moves the §2b counts, or CI fails.

## 5. Questions for the owner, not blocking steps 0 to 3

Answered questions move to §0 with their answer; what remains is open.

Nothing open as of 2026-11-08 (late); all three questions were answered the same day.
