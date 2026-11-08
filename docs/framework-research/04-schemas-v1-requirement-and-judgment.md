# Schemas v1: Requirement, Equivalency Judgment, and Supporting Entities

**Status.** First draft of the two core schemas the framework doc names as the next concrete step, plus the supporting entities they depend on (evidence item, clarification item, match record). Written 2026-11-08 against the verdict taxonomy in `03-verdict-taxonomy-and-clarification-loop.md` and the gap analysis in `02-career-site-gap-analysis.md`. Open items are in §7.

**Form.** Field tables plus JSON Schema. JSON Schema is used because the prototype already bounds model output with it (career-site FR-JD-15), so the judgment schema here is also the shape the judge is constrained to emit. Proto and SQL mappings are mechanical and deferred to implementation.

---

## 0. Versioning policy

Every stored record carries the versions needed to parse and interpret it. Nothing is rewritten in place; readers upcast.

### 0.1 What is versioned, separately

| Version field | Covers | Changes when |
|---|---|---|
| `schema_version` (per entity) | Field structure of Requirement, Judgment, EvidenceItem, ClarificationItem, MatchRecord | A field is added, removed, renamed, or retyped |
| `vocab_version` | The enumerations: requirement `type`, `basis`, `uncertainty` codes, `answer_type`, `source_kind`, `determination` | A code is added, retired, or redefined |
| `formula_version` | The arithmetic from judgments to coverage numbers | The formula changes |
| `prompt_version` / `prompt_fingerprint` | Extractor and judge prompts (carried from prototype: `v{n}:{sha256[:4]}`) | Prompt text or its output schema changes |
| `rule_version` | Code-side decision rules (duration, relationship, scope) | A rule's logic changes |

Vocabularies are versioned apart from structure because they will change far more often: every reviewed batch of judgments is expected to propose new `basis` or `uncertainty` codes, and that must not be a schema migration.

### 0.2 Rules

1. **Integer versions, additive within a major.** `schema_version` is `MAJOR.MINOR`. Adding an optional field or a vocab code is a minor bump. Removing, renaming, retyping a field, or changing the meaning of an existing code is a major bump. (Mirrors the prototype's `buf breaking` additive-only policy.)
2. **Writers write current; storage keeps original.** A record is stored at the version it was produced under and never rewritten. Readers upcast through registered pure functions `upcast_{entity}_{from}_{to}` to the current version in memory.
3. **Readers tolerate unknown codes.** A reader at vocab 1.3 meeting a 1.5 code keeps the raw value and treats it as `other` for logic. Vocab codes are never reused.
4. **Every match record snapshots the full version set** it was produced under (`versions` object, §5). Two match records are comparable only when their version sets are comparable; the calibration layer filters on this, as the prototype's agreement metrics already filter on `prompt_version` and `model`.
5. **Vocab registry is data, not code.** Each code carries: id, label, description, `introduced_in`, `retired_in` (nullable), `replaced_by` (nullable). The judge prompt renders its allowed codes from this registry, so a vocab change is one edit.

---

## 1. Requirement

One requirement extracted from a role posting, edited by a human before judging. A role has many. A requirement is **versioned per edit**: editing creates a new row with `supersedes` pointing at the prior one, so a judgment can always name the exact requirement text it was judged against.

### 1.1 Fields

| Field | Type | Req | Notes |
|---|---|---|---|
| `id` | uuid | ✓ | Global id of this version of the requirement |
| `role_id` | uuid | ✓ | The role / posting it belongs to |
| `local_id` | string ≤8 | ✓ | Stable within the role across edits (`r1`, `r2`…); the judge refers to this |
| `schema_version` | string | ✓ | e.g. `1.0` |
| `vocab_version` | string | ✓ | |
| `supersedes` | uuid | | Prior version of this requirement; null for the first |
| `text` | string ≤500 | ✓ | The requirement as it will be judged. Starts as the extractor's output; editable |
| `source_quote` | string ≤1000 | ✓ | Verbatim span from the posting; verified to occur in the posting text (carried from prototype) |
| `type` | enum | ✓ | `skill` · `outcome` · `tool` · `domain` · `credential` · `experience` · `relationship` · `constraint` · `behavioral` |
| `category` | enum | ✓ | `must` · `nice` (as the posting stated it) |
| `weight` | int 1–3 | ✓ | 3 core purpose, 2 stated requirement, 1 preference. Default from extractor; editable |
| `evidence_standard` | string ≤500 | | What counts as met for this requirement, written for a non-expert. Empty until a human or a curated template fills it |
| `alternatives` | array\<string\> | | Alternatives the posting itself offers ("or equivalent experience", "or related field"), split out so each is independently checkable. Extractor populates; editable |
| `equivalence_notes` | string ≤1000 | | Known equivalents for this requirement (tools, titles, vocabulary). Seeded from the equivalence catalog; editable |
| `thresholds` | object | | Structured values code rules compare against, so shortfalls are decided arithmetically: see §1.2 |
| `named_parties` | array\<string\> ≤8 | | Organisations the requirement names (prototype `named_parties`) |
| `extraction` | object | | Provenance of the extractor's draft: `prompt_id`, `prompt_version`, `model`, `decision_id`. Null for a human-authored requirement |
| `edited_by` | uuid | | User who produced this version; null when it is the untouched extractor draft |
| `edited_at` | timestamp | | |
| `edit_reason` | string ≤500 | | Why the text changed; feeds extractor calibration |
| `status` | enum | ✓ | `draft` · `active` · `retired` |
| `tenant_id` | id | ✓ | Carried from prototype seam |

### 1.2 `thresholds` object

All optional. Present only when the requirement states or implies one.

| Key | Type | Used by rule |
|---|---|---|
| `min_years` | number | duration |
| `recency_years` | number | recency (work must be within N years to count as current) |
| `min_scope` | `{ "unit": enum, "value": number }` | scope. Units: `team_size` · `budget_usd` · `users` · `sites` · `io_points` · `systems` · `revenue_usd` · `other` |
| `credential_level` | enum | credential. `certificate` · `associate` · `bachelor` · `master` · `doctorate` · `license` |
| `credential_field` | string | credential |
| `relationship_kind` | enum | relationship. `employed_by` · `contracted_to` · `customer_of` · `partner_of` · `any_working` |

### 1.3 JSON Schema (abridged)

```json
{
  "$id": "hiring/requirement/1.0",
  "type": "object",
  "required": ["id","role_id","local_id","schema_version","vocab_version",
               "text","source_quote","type","category","weight","status","tenant_id"],
  "properties": {
    "id": {"type":"string","format":"uuid"},
    "role_id": {"type":"string","format":"uuid"},
    "local_id": {"type":"string","maxLength":8},
    "schema_version": {"type":"string","const":"1.0"},
    "vocab_version": {"type":"string"},
    "supersedes": {"type":["string","null"],"format":"uuid"},
    "text": {"type":"string","maxLength":500},
    "source_quote": {"type":"string","maxLength":1000},
    "type": {"type":"string","enum":["skill","outcome","tool","domain","credential",
                                      "experience","relationship","constraint","behavioral"]},
    "category": {"type":"string","enum":["must","nice"]},
    "weight": {"type":"integer","minimum":1,"maximum":3},
    "evidence_standard": {"type":"string","maxLength":500},
    "alternatives": {"type":"array","maxItems":6,"items":{"type":"string","maxLength":300}},
    "equivalence_notes": {"type":"string","maxLength":1000},
    "thresholds": {"type":"object","additionalProperties":false,
      "properties": {
        "min_years": {"type":"number","minimum":0},
        "recency_years": {"type":"number","minimum":0},
        "min_scope": {"type":"object","required":["unit","value"],
          "properties":{"unit":{"type":"string"},"value":{"type":"number"}}},
        "credential_level": {"type":"string"},
        "credential_field": {"type":"string","maxLength":120},
        "relationship_kind": {"type":"string"}
      }},
    "named_parties": {"type":"array","maxItems":8,"items":{"type":"string","maxLength":64}},
    "extraction": {"type":["object","null"]},
    "edited_by": {"type":["string","null"]},
    "edited_at": {"type":["string","null"],"format":"date-time"},
    "edit_reason": {"type":["string","null"],"maxLength":500},
    "status": {"type":"string","enum":["draft","active","retired"]},
    "tenant_id": {"type":"string"}
  }
}
```

---

## 2. Equivalency Judgment

One judgment of one requirement version against one candidate's evidence, within one run. **Immutable.** An override does not modify it; it adds override fields that are read as the *effective* values while the model's values are preserved for agreement metrics. A re-judge produces a new judgment in a new run.

### 2.1 Fields

| Field | Type | Req | Notes |
|---|---|---|---|
| `id` | uuid | ✓ | |
| `run_id` | uuid | ✓ | The match-record run this belongs to |
| `requirement_id` | uuid | ✓ | The exact requirement *version* judged |
| `requirement_local_id` | string | ✓ | For readability and prompt I/O |
| `candidate_id` | uuid | ✓ | |
| `schema_version`, `vocab_version` | string | ✓ | |
| **Model-reported** | | | Emitted by the judge under the output schema in §2.3 |
| `determination` | enum | ✓ | `met` · `met_equivalent` · `unresolved` · `not_met` |
| `basis` | enum | ✓ | See vocab table in taxonomy doc §2.2 |
| `uncertainty` | array\<enum\> ≤4 | ✓ | Empty unless `unresolved` |
| `evidence_ids` | array\<uuid\> ≤12 | ✓ | Must be empty for `not_met/not_addressed`; non-empty otherwise |
| `rationale` | string ≤400 | ✓ | One grounded sentence |
| `reported_facts` | object | ✓ | Facts the model extracted for code to compare (§2.2) |
| `confidence` | number 0–1 | | Model's self-report; **never used in arithmetic**, logged for calibration only |
| **Code-applied** | | | |
| `rule_adjustments` | array\<object\> | | Each: `rule`, `rule_version`, `from_determination`, `to_determination`, `from_basis`, `to_basis`, `added_uncertainty[]`, `reason`. Empty when the model's output stood. Replaces prototype `adjusted` string |
| `rationale_issues` | array\<object\> | | Carried from prototype `rationale.go`; report-only, never changes a determination |
| `evidence_provenance_summary` | object | ✓ | Counts of cited evidence by `source_kind` (`corpus_document`, `candidate_response`, `verified_external`), so a reader sees at a glance whether the verdict rests on self-attestation |
| **Override** | | | Set by a human reviewer; null until then |
| `override_determination` | enum | | |
| `override_basis` | enum | | |
| `override_by` | uuid | | |
| `override_at` | timestamp | | |
| `override_reason` | string ≤1000 | ✓ if override | |
| `override_role` | enum | | `hiring_manager` · `recruiter` · `candidate_dispute` · `system_admin` |
| **Effective** (derived, not stored) | | | `effective_determination = override_determination ?? post-rule determination`; same for basis. The match record is computed from effective values |
| **Provenance** | | | |
| `decision_id` | uuid | ✓ | Row in the decision log holding prompt text, response, model, tokens, latency (prototype `decision_log`) |
| `prompt_id`, `prompt_version`, `prompt_fingerprint`, `model` | string | ✓ | |
| `clarification_item_ids` | array\<uuid\> | | Clarification answers that were evidence for this judgment (a re-judge after a questionnaire) |
| `supersedes` | uuid | | Prior judgment of the same requirement for the same candidate, when this is a re-judge |
| `created_at` | timestamp | ✓ | |
| `tenant_id` | id | ✓ | |

### 2.2 `reported_facts` object

The model reports; code decides. These are the values the code rules compare against `requirement.thresholds`.

| Key | Type | Meaning |
|---|---|---|
| `stated_span_years` | number | Years the evidence explicitly states for the relevant work; 0 when none (prototype) |
| `most_recent_year` | int | Latest year the relevant work is evidenced; 0 when unknown |
| `stated_scope` | `{unit, value}` | Scale the evidence explicitly states; null when none |
| `credential_found` | `{level, field, status}` | Credential evidenced, if any |
| `parties_evidenced` | array\<string\> ≤8 | Organisations the evidence names as working relationships (prototype) |
| `alternative_satisfied` | int | Index into `requirement.alternatives` the evidence satisfies; -1 if none / the primary |
| `involvement_level` | enum | `aware` · `used` · `configured` · `designed` · `owned` · `taught` — the depth ladder, as the evidence shows it |

### 2.3 Judge output schema (what the model is constrained to emit)

Field order is load-bearing (prototype FR-JD-15): determination and evidence are settled before free text.

```json
{
  "$id": "hiring/judge-output/1.0",
  "type": "object",
  "required": ["requirement_local_id","determination","basis","uncertainty",
               "evidence_ids","reported_facts","rationale"],
  "additionalProperties": false,
  "properties": {
    "requirement_local_id": {"type":"string","maxLength":8},
    "determination": {"type":"string","enum":["met","met_equivalent","unresolved","not_met"]},
    "basis": {"type":"string","enum":["direct","vocabulary","tool_equivalent",
       "experience_for_education","education_for_experience","adjacent_domain",
       "scope_shortfall","duration_shortfall","recency","relationship_unproven",
       "not_addressed","contradicted"]},
    "uncertainty": {"type":"array","maxItems":4,"items":{"type":"string",
       "enum":["scope_unknown","duration_unknown","recency_unknown","depth_unknown",
               "equivalence_unconfirmed","credential_unknown","party_unknown",
               "evidence_ambiguous"]}},
    "evidence_ids": {"type":"array","maxItems":12,"items":{"type":"string","maxLength":36}},
    "reported_facts": {"type":"object","additionalProperties":false,
      "required":["stated_span_years","most_recent_year","parties_evidenced",
                  "alternative_satisfied","involvement_level"],
      "properties": {
        "stated_span_years": {"type":"number","minimum":0,"maximum":60},
        "most_recent_year": {"type":"integer","minimum":0,"maximum":2100},
        "stated_scope": {"type":["object","null"],
          "properties":{"unit":{"type":"string","maxLength":24},"value":{"type":"number"}}},
        "credential_found": {"type":["object","null"],
          "properties":{"level":{"type":"string","maxLength":24},
                        "field":{"type":"string","maxLength":120},
                        "status":{"type":"string","enum":["completed","in_progress","unknown"]}}},
        "parties_evidenced": {"type":"array","maxItems":8,"items":{"type":"string","maxLength":64}},
        "alternative_satisfied": {"type":"integer","minimum":-1,"maximum":5},
        "involvement_level": {"type":"string",
          "enum":["aware","used","configured","designed","owned","taught","unknown"]}
      }},
    "rationale": {"type":"string","maxLength":400},
    "confidence": {"type":"number","minimum":0,"maximum":1}
  }
}
```

Consistency rules enforced in code after decode (reject and retry, as the prototype does for truncation):
- `determination=met` ⇒ `basis=direct` or `alternative_satisfied ≥ 0`
- `determination=met_equivalent` ⇒ basis ∈ equivalency set
- `determination=unresolved` ⇒ `uncertainty` non-empty
- `determination=not_met` ⇒ `uncertainty` empty; `evidence_ids` empty iff `basis=not_addressed`
- Duration, recency, scope, credential and relationship comparisons are **never** accepted from the model; code recomputes from `reported_facts` vs `thresholds` and records any change in `rule_adjustments`.

---

## 3. Evidence Item

Minimum fields the judgment depends on. Full evidence-store design is later work.

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | |
| `candidate_id` | uuid | |
| `source_kind` | enum | `corpus_document` · `candidate_response` · `verified_external` · `profile_sheet` |
| `source_ref` | uuid | Document id, or `clarification_item.id`, or verification record |
| `text` | string | The chunk / answer text the judge sees |
| `attested_at` | timestamp | When the candidate supplied it |
| `visibility` | enum | `private` · `employer_scoped` · `public`; plus `audience[]` of employer ids when scoped |
| `withdrawn_at` | timestamp | Soft delete; judgments citing it are marked stale |
| `schema_version`, `tenant_id` | | |

---

## 4. Clarification Item and controlled answer types

### 4.1 The tension, and how the schema resolves it

Asking for evidence rather than confirmation protects the score from being handed to the candidate, but free-text answers are hard to parse and harder to judge consistently. The resolution is to **constrain the shape of the answer, never its content**, and to draw every option list from one of two sources that cannot leak the scoring target:

1. **The candidate's own corpus** — their organisations, roles, projects, dates, already extractable from their evidence. "Which of *your* roles involved this?" offers choices the system already knows about them.
2. **Generic, requirement-independent ladders and lists** — involvement depth, relationship kind, credential level, scope units. These are the same for every requirement, so choosing from them reveals nothing about what *this* requirement is scored on.

Free text is permitted only as a capped supplement, and it is **never parsed by the system**: it becomes a `candidate_response` evidence item and goes through the same judge as any document. The structured part of the answer is what code compares against `thresholds`.

Each uncertainty code maps to exactly one answer type:

| Uncertainty code | `answer_type` | Structured value collected | Options drawn from |
|---|---|---|---|
| `duration_unknown`, `recency_unknown` | `span` | One or more `{org, role, start, end\|ongoing}` | Candidate's orgs/roles (own corpus) + "other" |
| `scope_unknown` | `magnitude` | `{unit, value}`; unit chosen from fixed unit list | Generic unit list (same as `thresholds.min_scope.unit`) |
| `depth_unknown` | `involvement` | One rung from the depth ladder + which engagement | Generic ladder + candidate's own engagements |
| `credential_unknown` | `credential` | `{level, field, institution, year, status}` or `none` | Generic level list; field free text ≤120 |
| `party_unknown` | `relationship` | `{org, kind}` | Org from requirement's `named_parties` (already public in the posting); kind from generic list |
| `equivalence_unconfirmed` | `evidence_pointer` | Pick engagement(s) from own corpus + ≤80-word description of what they did | Own corpus; free text → judge |
| `evidence_ambiguous` | `disambiguation` | Pick which of own engagements the passage refers to + activity class from a generic taxonomy | Own corpus + generic activity classes |

Every item also offers `not_applicable` ("I have not done this") and `decline` ("prefer not to answer"). A `not_applicable` resolves the judgment to `not_met` with basis `not_addressed` and provenance `candidate_response`; a `decline` leaves it `unresolved` and ends the loop for that item.

What the candidate sees for an item: the requirement's `source_quote` (already public in the posting — nothing is leaked by showing it), the question, and the controlled inputs. What the candidate never sees: `thresholds`, `evidence_standard`, `equivalence_notes`, the current determination, or the score.

### 4.2 Fields

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | |
| `request_id` | uuid | Groups items into one questionnaire (round, status, sent/answered timestamps) |
| `judgment_id` | uuid | The `unresolved` judgment that generated it |
| `requirement_id` | uuid | Requirement version |
| `uncertainty_code` | enum | The one code this item resolves |
| `answer_type` | enum | `span` · `magnitude` · `involvement` · `credential` · `relationship` · `evidence_pointer` · `disambiguation` |
| `question_text` | string ≤300 | Rendered from a template keyed by `(answer_type, requirement.type)` in v1; model-generated is a later option and must be logged as a decision |
| `options` | array\<object\> | The controlled choices offered, with provenance (`own_corpus` ref or `generic_list` id), stored so the answer is interpretable later |
| `answer` | object | Shape per `answer_type`; null until answered |
| `answer_free_text` | string ≤600 | Supplement; becomes an evidence item, never parsed |
| `answer_status` | enum | `pending` · `answered` · `not_applicable` · `declined` · `withdrawn` |
| `answered_at`, `withdrawn_at` | timestamp | |
| `evidence_item_id` | uuid | The `candidate_response` evidence item created from this answer |
| `rejudge_run_id` | uuid | The run that consumed it |
| `schema_version`, `vocab_version`, `tenant_id` | | |

---

## 5. Match Record (per run)

| Field | Notes |
|---|---|
| `run_id`, `role_id`, `candidate_id`, `attempt`, `supersedes_run_id` | Immutable per run, as prototype `jd_runs` |
| `requirement_ids[]` | Exact requirement versions judged |
| `judgment_ids[]` | |
| `coverage` | `{ evidenced: number, unresolved: number, unmet: number }` — three numbers over effective determinations, each as a fraction of total weight. `evidenced + unresolved + unmet = 1` for must items; nice items follow the prototype rule (denominator only when evidenced) |
| `coverage_equivalent_share` | Fraction of `evidenced` that came from `met_equivalent`, so an auditor sees how much of the score rests on equivalency reasoning |
| `self_attested_share` | Fraction of `evidenced` weight whose judgments cite any `candidate_response` evidence |
| `override_count` | |
| `formula_version` | |
| `versions` | Snapshot: `{requirement_schema, judgment_schema, vocab, formula, rules, prompts: {extractor, judge}, model, embedder, corpus_fingerprint}` |
| `clarification_request_ids[]` | Questionnaires generated from this run |
| `created_at`, `tenant_id` | |

---

## 6. Mapping from the prototype (for implementation)

| Prototype | v1 |
|---|---|
| `prompts.Requirement` in `assessment` jsonb | `requirement` table, versioned rows |
| `jd.Judgment.verdict` met/partial/unmet | `determination` + `basis` + `uncertainty` |
| `adjusted` string | `rule_adjustments[]` |
| `stated_span_years`, `parties_evidenced` | `reported_facts.*` |
| `duration.go`, `relationship.go` downgrade to partial | set `unresolved` + code, or `not_met` with shortfall basis; record in `rule_adjustments` |
| `decision_log.human_verdict` (review-only) | kept as reviewer label **and** new override fields on the judgment that change the effective value |
| `jd_runs` | match record; add `coverage{}` and `versions{}` |
| `source_kind=profile` | `source_kind=profile_sheet` |
| — | `clarification_request`, `clarification_item`, `candidate_response` evidence |
| Agreement matrix (verdict) | Two matrices: determination agreement, basis agreement |

---

## 7. Open items

- **Weights.** Still 1–3 by hand. The record now captures enough (basis, overrides, outcomes) to learn them later; v1 does not attempt to.
- **`evidence_standard` templates** per requirement `type` — see `05-equivalence-catalog-and-evidence-standards.md`.
- **Equivalence catalog** entity — see `05-equivalence-catalog-and-evidence-standards.md`.
- **Trigger and round policy** for clarification requests — deliberately open per the taxonomy doc.
- **Self-attested weighting.** v1 counts `candidate_response`-backed judgments at full weight and reports `self_attested_share`; whether to discount is a policy decision for the calibration data to inform.
- **Proto and SQL rendering** of the above, including which fields are columns vs jsonb. Suggested: everything a view or index needs is a column (`determination`, `basis`, `requirement_id`, `run_id`, versions); `reported_facts`, `rule_adjustments`, `options`, `answer` are jsonb.
