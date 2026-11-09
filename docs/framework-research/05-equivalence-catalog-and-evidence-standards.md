# Equivalence Catalog and Evidence-Standard Templates

**Purpose.** Fill the two Requirement fields that would otherwise sit empty — `equivalence_notes` and `evidence_standard` — with a data model and a starter set, so the extractor can seed them and the judge can use them from the first run. Companion to `04-schemas-v1-requirement-and-judgment.md`.

**Status.** Draft 2026-11-08. The starter content is for the first target domain (industrial/manufacturing engineering, controls, software, applied AI) and is expected to be wrong in places; the whole point of the catalog is that overrides correct it.

---

## 1. Equivalence Catalog

### 1.1 What it is

A versioned, queryable store of assertions of the form *"X satisfies a requirement for Y, under conditions C, with confidence from N confirmations."* It is the reusable knowledge the framework doc calls "equivalence mappings" (§5.4) and the first concrete place the recursive loop writes back to: every `met_equivalent` judgment an expert confirms, and every `unresolved / equivalence_unconfirmed` an expert resolves, becomes evidence for or against a catalog entry.

It is **not** a synonym list. Equivalence is directional (a Siemens PLC programmer can likely work on Rockwell; a PLC programmer is not a software engineer), conditional (experience substitutes for a degree *in the same field*), and scoped (an equivalence that holds in manufacturing may not hold in pharma validation). The entity carries all three.

### 1.2 Fields

| Field | Type | Notes |
|---|---|---|
| `id` | uuid | |
| `schema_version`, `vocab_version` | string | Per the versioning policy |
| `kind` | enum | `tool` · `vocabulary` · `title` · `credential_experience` · `domain_transfer` · `method` |
| `from` | concept ref | What the evidence shows (e.g. `tool:siemens_tia_portal`) |
| `to` | concept ref | What the requirement asks for (e.g. `tool:rockwell_studio5000`) |
| `direction` | enum | `one_way` (from→to only) · `symmetric` |
| `strength` | enum | `full` (counts as met_equivalent) · `partial` (counts as unresolved with `equivalence_unconfirmed`, prompts a clarification) · `none` (explicitly not equivalent — a negative entry, so the judge stops proposing it) |
| `conditions` | array\<string\> | Human-readable conditions under which it holds ("same industry", "≥3 years on the from-side", "requires familiarity with ladder logic") |
| `domain_scope` | array\<string\> | Industries/domains where it applies; empty = general |
| `rationale` | string ≤500 | Why, for a non-expert reader |
| `source` | enum | `seed` (curated by a domain expert at setup) · `override_confirmed` (promoted from reviewed judgments) · `candidate_asserted` (proposed through a clarification answer; never `full` until confirmed) · `external` (published framework, e.g. O*NET crosswalk) |
| `confirmations` | int | Count of expert confirmations in judgments |
| `rejections` | int | Count of expert rejections |
| `status` | enum | `proposed` · `active` · `deprecated` |
| `introduced_in`, `retired_in` | version | |
| `tenant_id` | id | Catalog entries are tenant-scoped by default; a shared base catalog is tenant 0 |

**Concept refs.** `kind:slug` strings in v1 (`tool:ignition_scada`, `title:controls_engineer`, `credential:bs_engineering`, `domain:food_beverage_manufacturing`). A controlled concept table can replace free slugs later without changing the entry schema. The extractor and judge both normalise to these slugs where they can; where they can't, `vocabulary` entries carry raw phrases.

### 1.3 How it is used

1. **At extraction.** For each requirement, catalog entries whose `to` matches a concept in the requirement are rendered into `equivalence_notes` (active entries only, `full` and `partial`, with conditions). The hiring manager sees them and can strike any — a struck entry is recorded as a rejection.
2. **At judging.** The judge prompt receives `equivalence_notes`. A `met_equivalent` must name the entry it relied on (new field in `reported_facts`: `catalog_entry_id`), so the equivalency is auditable and countable. A `none` entry is rendered as "do not treat X as Y", closing a known failure (the prototype's "calling an API is not a working relationship" case would be a `none` entry of kind `method`).
3. **At override.** When a reviewer changes `met_equivalent → not_met`, the cited entry gets a rejection; `unresolved → met_equivalent` with a named entry gets a confirmation; an override with reason text that names an equivalence not in the catalog creates a `proposed` entry.
4. **Promotion rule (v1, deliberately simple).** `proposed → active` at ≥3 confirmations and confirmations ≥ 3× rejections from at least two distinct reviewers. `active → deprecated` when rejections exceed confirmations over the trailing 20 uses. Thresholds are configuration, versioned with `rule_version`.
5. **Candidate-side.** A clarification answer of type `evidence_pointer` that asserts an equivalence creates a `candidate_asserted` entry at `partial` strength for that candidate's judgment only; it does not enter the shared catalog until an expert confirms it.

### 1.4 Starter entries (first domain, illustrative)

| kind | from → to | direction | strength | conditions |
|---|---|---|---|---|
| tool | Siemens TIA Portal → Rockwell Studio 5000 | symmetric | partial | Ladder/structured text concepts transfer; vendor-specific instruction sets and networking (Profinet vs EtherNet/IP) do not. Clarify depth |
| tool | Ignition SCADA → Wonderware/AVEVA System Platform | symmetric | partial | HMI/SCADA concepts transfer; scripting and tag model differ |
| tool | Ignition SCADA → "SCADA platform experience" | one_way | full | Requirement is generic |
| tool | Grafana → Power BI (for operational dashboards) | one_way | partial | Visualisation transfers; data-model and DAX do not |
| tool | PostgreSQL → "relational database experience" | one_way | full | Requirement is generic |
| tool | Go → "backend development in a compiled language" | one_way | full | |
| tool | Python ML stack → "machine learning frameworks" | one_way | full | |
| vocabulary | "front-end loading / FEL" ↔ "front-end development (capital projects)" | symmetric | full | Capital-projects domain only; **none** in software domain |
| vocabulary | "controls engineer" ↔ "automation engineer" | symmetric | full | Manufacturing |
| vocabulary | "co-manufacturing" ↔ "contract manufacturing" | symmetric | full | |
| vocabulary | "MES" ↔ "manufacturing operations management (MOM)" | symmetric | full | |
| vocabulary | "Unified Namespace / MQTT broker architecture" → "IIoT data architecture" | one_way | full | |
| title | "AI Principal" → "Director/Head of AI" | one_way | partial | Depends on reports and budget; clarify scope |
| title | "Lead engineer" → "Engineering manager" | one_way | partial | Requires evidence of people management |
| credential_experience | ≥8 yrs engineering practice in field → B.S. in that engineering field | one_way | partial | Only when the requirement offers "or equivalent experience" or the role is not licensure-gated; **none** where PE/licensure is required |
| credential_experience | B.S. + ≥5 yrs in field → M.S. in field | one_way | partial | Same conditions |
| credential_experience | Vendor certification (e.g. Rockwell, Siemens) → "formal training in PLC programming" | one_way | full | |
| domain_transfer | Distilled spirits manufacturing → food & beverage manufacturing | one_way | full | Regulated batch processing in common |
| domain_transfer | Aerospace batch manufacturing → "regulated manufacturing" | one_way | full | |
| domain_transfer | Food & beverage → pharmaceutical manufacturing (GMP) | one_way | partial | cGMP validation discipline (CQV, 21 CFR Part 11) must be separately evidenced |
| method | Integrating a vendor's API → "partnership/collaboration with [vendor]" | one_way | **none** | From prototype relationship rule |
| method | Supervising a system → "designing a system" | one_way | **none** | Depth ladder: `used/configured` does not satisfy `designed` |
| method | Capital project ownership ≥$50M → "large capital project experience" | one_way | full | |

Negative (`none`) entries are as valuable as positive ones: each is a documented judge failure that will not recur.

---

## 2. Evidence-Standard Templates

### 2.1 What they are

Short, non-expert-readable statements of what counts as `met`, what counts as `unresolved` (and which uncertainty code), and what counts as `not_met`, keyed by requirement `type`. The extractor instantiates the template for each requirement; a hiring manager edits the instance. The judge prompt renders the instance. The candidate never sees it.

Templates are versioned as data (`template_version`), one per `type`, with optional sub-templates keyed by pattern (e.g. a `credential` requirement that offers "or equivalent experience").

### 2.2 Template fields

| Field | Notes |
|---|---|
| `type` | The requirement type it applies to |
| `met_when` | 1–2 sentences, with `{placeholders}` filled from the requirement text and thresholds |
| `unresolved_when` | What is suggestive but insufficient; names the uncertainty code(s) the judge should emit |
| `not_met_when` | |
| `accepted_evidence_kinds` | Which `source_kind`s are acceptable and whether `candidate_response` alone can satisfy it |
| `default_answer_type` | Which clarification answer type resolves the common uncertainty |
| `template_version` | |

### 2.3 Starter templates

**skill**
- *met_when:* Evidence describes the candidate performing `{skill}` in a work or project context, at `{involvement_level or "used"}` depth or above.
- *unresolved_when:* The skill is named but not demonstrated (listed in a skills section with no context) → `depth_unknown`; or a closely related skill is demonstrated and a catalog `partial` entry exists → `equivalence_unconfirmed`.
- *not_met_when:* No evidence names or demonstrates the skill or a catalog-equivalent.
- *accepted:* any; `candidate_response` alone may satisfy at `used`, not at `designed/owned`.
- *default_answer_type:* `involvement`

**outcome** (e.g. "reduced downtime", "delivered projects on budget")
- *met_when:* Evidence states the candidate's role in producing a comparable outcome, with the outcome itself stated (a result, not an activity).
- *unresolved_when:* The activity is evidenced but no result is stated → `scope_unknown`; or the result is stated at a materially smaller scale than `{min_scope}` → `scope_shortfall` basis, `unresolved` if within 50% of threshold, `not_met` otherwise (rule).
- *not_met_when:* No related activity evidenced.
- *accepted:* documents preferred; `candidate_response` supplies the number, flagged self-attested.
- *default_answer_type:* `magnitude`

**tool**
- *met_when:* Evidence names `{tool}` or a catalog `full` equivalent in a work context.
- *unresolved_when:* A catalog `partial` equivalent is evidenced → `equivalence_unconfirmed`; or the tool is named only in a skills list → `depth_unknown`.
- *not_met_when:* Neither the tool nor any catalog equivalent appears.
- *accepted:* any; `candidate_response` may satisfy at `used`.
- *default_answer_type:* `involvement`

**domain**
- *met_when:* Evidence shows sustained work (≥1 engagement) in `{domain}` or a catalog `full` domain-transfer source.
- *unresolved_when:* Work in a catalog `partial` transfer domain → `equivalence_unconfirmed`; or domain is mentioned without an engagement → `depth_unknown`.
- *not_met_when:* No engagement in the domain or a transfer source.
- *accepted:* documents; `candidate_response` via `evidence_pointer` to an engagement.
- *default_answer_type:* `evidence_pointer`

**credential**
- *met_when:* Evidence states the credential at `{credential_level}` in `{credential_field}` (or a stated alternative), status completed; **or** the requirement offers an experience alternative and evidence satisfies it (see `credential_experience` catalog entries).
- *unresolved_when:* A credential is evidenced but level or field is unclear → `credential_unknown`; or experience is evidenced, the requirement offers no explicit alternative, but a catalog `partial` `credential_experience` entry applies → `equivalence_unconfirmed`.
- *not_met_when:* No credential evidenced and no alternative satisfied; or licensure is required and absent.
- *accepted:* `candidate_response` may name the credential but the judgment is flagged self-attested; verification is a later `verified_external` kind.
- *default_answer_type:* `credential`

**experience** (duration requirements)
- *met_when:* `reported_facts.stated_span_years ≥ {min_years}` for work matching the requirement, decided in code.
- *unresolved_when:* Matching work is evidenced but years are not stated → `duration_unknown`; or span is within 25% below threshold → `duration_shortfall` basis, `unresolved`.
- *not_met_when:* Span more than 25% below threshold (code), or no matching work.
- *accepted:* any; `candidate_response` of type `span` supplies dates.
- *default_answer_type:* `span`

**relationship** (named parties)
- *met_when:* `reported_facts.parties_evidenced` ∩ `named_parties` ≠ ∅ with a working relationship of `{relationship_kind}` or stronger, decided in code.
- *unresolved_when:* A named party appears in evidence but the relationship kind is not stated → `party_unknown`.
- *not_met_when:* No named party appears, or only product use is evidenced (catalog `none`).
- *accepted:* `candidate_response` of type `relationship` supplies the kind; flagged.
- *default_answer_type:* `relationship`

**constraint** (location, clearance, travel, shift, authorisation)
- *met_when:* Evidence or candidate response states the constraint is satisfied.
- *unresolved_when:* Not addressed → the uncertainty code is `credential_unknown` for clearances/authorisations, otherwise `evidence_ambiguous`.
- *not_met_when:* Evidence states it is not satisfied.
- *accepted:* `candidate_response` is the normal source; constraints are rarely in a corpus. Weight default 1 unless must.
- *default_answer_type:* `credential` (for clearance/authorisation) or `disambiguation`

**behavioral** (working style, "thrives in ambiguity", "collaborative")
- *met_when:* Not judged by the model in v1. Recorded as `unresolved` with `evidence_ambiguous` and excluded from coverage arithmetic (weight 0) until the Phase 2 working-style framework exists.
- *rationale:* Scoring personality adjectives from documents is exactly the opaque culture-fit judgment the framework doc identifies as a root cause. Better to show "not assessed" than a verdict nobody should trust.

### 2.4 Rendering rules

- The judge sees `met_when / unresolved_when / not_met_when` for the requirement, after the requirement text and `source_quote`, before the evidence. Order matters for schema-constrained output, as the prototype found.
- Instances are stored on the requirement version (`evidence_standard` becomes the rendered text; `evidence_standard_template` records `type@template_version` so the text can be regenerated or diffed).
- An HM edit to the instance is a new requirement version with `edit_reason`; edits that recur across many requirements of the same type are the signal to revise the template.

---

## 3. What this closes and what it opens

**Closes.** Two empty fields; a concrete write-back path from overrides into reusable knowledge; a place for negative knowledge (judge failures that must not recur); a principled exclusion of behavioral requirements from v1 arithmetic.

**Opens.**
- A concept table to replace free slugs, and whether to adopt an external crosswalk (O*NET tools/technology, ESCO skills) as the `external` seed source.
- Who curates the shared base catalog (tenant 0) and how tenant catalogs inherit from it.
- The promotion thresholds in §1.3 are guesses; the first few hundred reviewed judgments should set them.
- Whether `partial` catalog strength should ever count in coverage arithmetic (v1: no — it only triggers a clarification).
