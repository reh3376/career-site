# Verdict Taxonomy and the Candidate Clarification Loop

**Status.** Design direction, recorded 2026-11-08 so it shapes the Requirement and Equivalency Judgment schemas before they are locked. The verdict taxonomy below is a proposal to be refined; the clarification loop is a *capability the schema must leave room for*, with trigger rules, handling and UX explicitly deferred to later investigation.

**Supersedes** item 2 of §4 in `02-career-site-gap-analysis.md` (which proposed only adding `insufficient_evidence` and `gap_kind` to met/partial/unmet).

---

## 1. Why met / partial / unmet is not enough

The prototype's three-way verdict was chosen so the score could be arithmetic (1 / 0.5 / 0). That property is worth keeping. The vocabulary is not, for three reasons:

1. **"Partial" absorbs every hard case.** Different tool but same capability, experience offered in lieu of a degree, the right work at smaller scale, the right work ten years ago, and "the corpus simply doesn't mention it" all collapse into one bucket worth 0.5. They have different fixes, different trust levels, and different meanings to an HR reader and to the candidate.
2. **It hides exactly where the classifier is uncertain.** The design goal of this system is to make equivalency judgments inspectable. A verdict that says *partial* without saying *partial because of what* is not inspectable; it is a smaller black box.
3. **It gives nobody a next action.** The three parties each need a different thing from a non-met verdict: HR needs to know whether to discount it, the hiring manager needs to know whether it is worth a question in the interview, the candidate needs to know whether to supply more evidence. One label cannot carry all three.

The prototype already shows the pressure: `adjusted`, `stated_span_years`, `parties_evidenced` and `rationale_issues` were all bolted on because the verdict alone could not say what happened.

## 2. Proposed structure: determination + basis + uncertainty

Split the judgment into three fields that are set independently and read together.

### 2.1 Determination (drives the arithmetic)

| Value | Meaning | Score contribution |
|---|---|---|
| `met` | The evidence satisfies the requirement as written or via one of its own stated alternatives. | Full weight |
| `met_equivalent` | The evidence satisfies the requirement through an equivalency the system or a reviewer asserted (see basis). | Full weight, but flagged so an expert can audit the equivalency |
| `unresolved` | The evidence is suggestive but a specific, nameable fact is missing (see uncertainty codes). The system does not know. | Reported separately as *unresolved weight*; never silently scored as 0.5 |
| `not_met` | The evidence addresses the requirement and falls short, or the requirement is not addressed and there is no reason to believe it could be. | Zero |

Four values, two of them "met". The score reports **three numbers**: evidenced coverage (met + met_equivalent over total weight), unresolved weight, and unmet weight. An HR reader ranks on evidenced coverage and sees unresolved weight as "questions outstanding", not as a penalty.

### 2.2 Basis (how the determination was reached — the equivalency categories)

This is where the classifier's reasoning becomes a label rather than prose. Initial enumeration; expected to grow from the decision log:

| Code | Description | Typical requirement types |
|---|---|---|
| `direct` | Same thing, same words or trivially the same. | any |
| `vocabulary` | Same capability, different vocabulary, title or framing. The lexical-gap case the whole project exists for. | skill, domain |
| `tool_equivalent` | Different named tool/platform/vendor, same function (e.g. Rockwell vs Siemens PLC, Jira vs Linear). | tool |
| `experience_for_education` | Requirement names a credential; evidence shows the work the credential certifies. | constraint (education) |
| `education_for_experience` | Requirement names years/exposure; evidence shows formal training or credential covering it. | constraint (experience) |
| `adjacent_domain` | Same skill demonstrated in a neighbouring industry or context; transferability is the question. | domain, outcome |
| `scope_shortfall` | Right work, smaller scale, team size, budget, or system size than stated. | outcome, constraint |
| `duration_shortfall` | Right work, fewer years than stated (prototype's `duration.go` rule). | constraint |
| `recency` | Right work, but old relative to what the requirement implies. | skill, tool |
| `relationship_unproven` | Requirement names organisations/parties; evidence uses their products but shows no working relationship (prototype's `relationship.go`). | constraint |
| `not_addressed` | Nothing in the evidence speaks to it. | any |
| `contradicted` | Evidence affirmatively indicates the requirement is not satisfied. | any |

Rules: a `met` determination normally carries `direct`; `met_equivalent` carries one of the equivalency bases; `unresolved` carries whichever basis is the *suspected* route to met plus at least one uncertainty code; `not_met` carries `not_addressed`, `contradicted`, or a shortfall basis when the gap is too large to be a question.

### 2.3 Uncertainty codes (what fact would resolve it)

Attached to `unresolved` judgments, zero or more:

- `scope_unknown` — scale not stated in evidence
- `duration_unknown` — years not stated
- `recency_unknown` — dates not stated
- `depth_unknown` — used vs. owned vs. designed is ambiguous
- `equivalence_unconfirmed` — a vocabulary or tool equivalence is plausible but not asserted by any mapping or reviewer
- `credential_unknown` — the degree/cert is neither confirmed nor denied by evidence
- `party_unknown` — organisation named in the requirement not confirmed as a working relationship
- `evidence_ambiguous` — the text can be read more than one way (the capital-projects "front-end development" case)

Each code is designed to be answerable by one concrete question, which is what makes §3 possible.

### 2.4 What this does to the existing pieces

- **Judge prompt / schema.** `verdict` enum becomes `determination` + `basis` + `uncertainty[]`. Still schema-bounded (the prototype's FR-JD-15 lesson stands). The model *reports* basis and uncertainty; code still decides anything that is arithmetic (duration, parties, scope thresholds once requirements carry them).
- **Code rules.** `duration.go` and `relationship.go` stop downgrading to `partial` and instead set `unresolved` + the matching uncertainty code, or `not_met` when the shortfall is unambiguous and large.
- **Decision log / agreement metrics.** The owner-vs-judge matrix becomes two matrices (determination agreement, basis agreement). Basis disagreement is the more useful one: it says *which kind* of equivalency the model misjudges.
- **Equivalence mappings (framework §5.4).** `tool_equivalent` and `vocabulary` bases that experts repeatedly confirm become candidate entries in an equivalence catalog; `equivalence_unconfirmed` codes become its backlog. This is the first concrete feedback path from judgments into reusable knowledge.
- **Candidate view.** Gap classification the framework promises (substantive vs lexical) falls out of basis: `vocabulary` / `tool_equivalent` / `experience_for_education` are lexical-or-equivalency gaps; `scope_shortfall` / `duration_shortfall` / `contradicted` are substantive.

## 3. Candidate clarification loop (capability to preserve)

### 3.1 Principle

An `unresolved` judgment is a question the system could not answer from the evidence it had. The candidate is usually the cheapest and most accurate source of that answer. The system should therefore be able to generate a **per-candidate, per-role questionnaire** from the unresolved judgments, collect answers, treat them as evidence with their own provenance, and re-judge only the affected requirements.

### 3.2 What must exist in the schema now

Even before the mechanism is designed, the data model must not make it impossible:

- **Judgment → question linkage.** Each `unresolved` judgment carries enough structure (requirement id, basis, uncertainty codes, the evidence ids it almost matched) that a question can be generated from it deterministically, and each answer can be traced back to the judgment that asked.
- **Clarification request** entity: id, match_record_id (run), candidate_id, role_id, generated_at, status (draft / sent / answered / expired / declined), audience/visibility set by the candidate, list of question items.
- **Clarification item**: id, request_id, judgment_id, requirement_id, uncertainty_code, question_text, answer_type (confirm/deny, number, date range, short text, pick-from-list), answer, answered_at.
- **Evidence provenance tier.** Evidence items need a `source_kind` that distinguishes `corpus_document` (candidate-supplied documents), `candidate_response` (questionnaire answer, self-attested, timestamped, tied to a request), and later `verified_external`. Judgments cite evidence ids, so a reviewer can always see when a verdict rests on a self-attested answer.
- **Re-judge scoped by requirement.** A clarification answer triggers re-judgment of only the requirements whose judgments it was linked to, producing a new run that supersedes the earlier one for those requirements and carries the rest forward. The prototype's one-call-per-requirement design makes this cheap; the run model (`jd_runs`, immutable, attempt N) already supports supersession.
- **Rounds.** A request has a round number; a second round may be generated from what remains unresolved. Cap on rounds is a policy decision, deferred.

### 3.3 Design constraints to carry forward

- **Ask for evidence, not confirmation.** "How many years did you spend on X, and where?" not "Did you do X for five years?" Questions must not hand the candidate the exact phrasing the requirement is scored on. The schema's `answer_type` enumeration is where this gets enforced.
- **Self-attestation is visible.** A `met` that rests on a `candidate_response` is flagged as such in the match record. Whether it counts fully, partially, or only after verification is a weight/policy decision for later; the record must be able to express it either way.
- **Candidate control.** The candidate chooses whether to answer, which answers are released to which employers, and can withdraw an answer (soft-delete; the judgment that used it is re-run or marked stale).
- **Every question is logged like a decision.** Which judgment generated it, what the model asked, what was answered, and what changed — same provenance discipline as `decision_log`.

### 3.4 Explicitly open (to investigate later)

- **Trigger.** On analysis completion when unresolved weight exceeds a threshold? Before an employer sees the record? Only at the candidate's request? Only for roles above a coverage floor so candidates are not asked to work on hopeless matches?
- **Who sees unanswered questions.** Whether an employer sees "3 questions outstanding" or nothing until answered.
- **Question generation.** Templated from uncertainty code (deterministic, cheap, predictable) vs. model-generated from the judgment (more natural, needs the same schema-bounding and logging as the judge).
- **Answer trust and weighting.** How `candidate_response` evidence is weighted vs. documents; whether certain uncertainty codes (e.g. `credential_unknown`) require a document rather than an answer.
- **Gaming and fatigue.** Limits on rounds and item counts; detection of answers that contradict the corpus.
- **Employer-side analogue.** Whether hiring managers get the mirror: "the system could not tell whether this requirement means X or Y" — the `evidence_ambiguous` case is often a defect in the posting, not the candidate.

## 4. Effect on the schema task

The Requirement and Equivalency Judgment schemas should now include:

- Requirement: `type` taxonomy, `evidence_standard`, `equivalence_notes`, and a structured place for *thresholds* the code rules need (`min_years`, `named_parties`, `scope_hint`) so shortfall bases can be decided in code.
- Judgment: `determination`, `basis`, `uncertainty[]`, `evidence_ids[]` with provenance visible, `confidence`, `adjusted_by_rule`, and the override fields (`override_determination`, `override_basis`, `override_by`, `override_reason`) with the score computed from effective values.
- New entities: `clarification_request`, `clarification_item`; `evidence_item.source_kind` extended with `candidate_response`.
- Match record: three reported numbers (evidenced coverage, unresolved weight, unmet weight) plus the formula version.
