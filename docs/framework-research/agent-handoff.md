# Agent Handoff: Hiring_prod framework research

**Read this first.** It is written for an AI coding agent (or a person) picking up the design effort in this directory with no prior context. It says what the effort is, where the source of record lives, what has been decided, what is open, what to do next, and the conventions to keep. Last updated 2026-11-08, at the handoff point: P1 complete, P2 next.

---

## 1. What this effort is

The owner (Roger Henley, repository owner) is designing a software system that replaces keyword-based initial screening in hiring with **transparent, requirement-level equivalency matching against evidence**, serving both employers (recruiters/HR and hiring managers) and job seekers. It generalises a mechanism that already runs in this repository: the career-site JD reviewer, which extracts structured requirements from a posting, judges each against the owner's career corpus with a local LLM, computes a score in code, logs every decision, and evaluates itself against a golden set.

The design effort lives in a claude.ai Project called **Hiring_prod**. This directory, `docs/framework-research/`, holds **snapshot copies** of that project's documents so the work can be reviewed and continued from the repository. The numbering (`00`–`10`) is the recommended reading order after this file.

**Nothing here changes the reviewer yet.** Where a document proposes a change to this repository's code, it still goes through `docs/FSD.md` and an ADR first (see `AGENTS.md`). These are design inputs.

## 2. Where things live, and which copy wins

| What | Where | Notes |
|---|---|---|
| Source of record for design docs | The claude.ai Project "Hiring_prod" (docs named `claude/<name>.md`) | If you are working in that project, edit there and re-copy here |
| Repository snapshots | This directory | Same content as the project at the time of copy; the `## Status` lines carry dates |
| Code being generalised | `services/api/internal/jd/` (pipeline), `services/api/internal/prompts/` (extractor and judge prompts and JSON schemas), `services/api/internal/db/migrations/` (`decision_log` 00019, `jd_runs` 00025, `jd_outcomes`/`jd_feedback` 00026, golden set and evals 00027) | Read `docs/llm-tuning-log.md` before touching any of it |
| Mocked match record (interview stimulus) | Published design artifact: <https://claude.ai/artifact/RumwAKPYaD2jerW5yc7Umv> | Owner-only until shared; reconstructed from documented runs, anonymised |
| Workflow diagrams | `10-hiring-workflow-diagrams.md` (Mermaid, renders on GitHub), `diagrams/*.mmd`, `diagrams/workflow-viewer.html` (pan/zoom), and <https://claude.ai/artifact/AJWsmPtYodKEfbcbBRbieL> | |
| Branch | `claude_dev01` (the repository's designated Claude Code branch per `AGENTS.md`), draft PR #382 | The PR also carries two unrelated `content:` commits; retitle or split before merging |

If you cannot reach the claude.ai project, work from this directory and say so in the status line of any document you change.

## 3. The documents, in reading order

| File | What it is | Read it when |
|---|---|---|
| [`00-todo.md`](00-todo.md) | The prioritised to-do list (P1–P6) with the reasoning for the order. The working agenda. | Always, first |
| [`01-problem-definition-and-framework.md`](01-problem-definition-and-framework.md) | The owner's founding document: the problem (keyword false negatives, screener expertise gap), symptoms from both sides, root causes, what a proper solution looks like, the data model (§5.1), components (§5.2), recursive-improvement layer (§5.3), phases (§5.5), and the process model attaching each component to the approved workflow nodes (§5.6). | Before anything else of substance |
| [`02-career-site-gap-analysis.md`](02-career-site-gap-analysis.md) | What this repository already implements against §5.1/§5.2, with status (exists / partial / missing) and the design principles worth carrying forward. | Before touching code or proposing a schema |
| [`03-verdict-taxonomy-and-clarification-loop.md`](03-verdict-taxonomy-and-clarification-loop.md) | Why met/partial/unmet is replaced by **determination + basis + uncertainty**, and the candidate clarification loop the schema must leave room for. | Before reading the schemas |
| [`04-schemas-v1-requirement-and-judgment.md`](04-schemas-v1-requirement-and-judgment.md) | Draft v1 schemas (field tables + JSON Schema) for Requirement, Equivalency Judgment, Evidence Item, Clarification Item, Match Record; the **versioning policy** (§0); controlled answer types (§4). | Before any data-model or prompt work |
| [`05-equivalence-catalog-and-evidence-standards.md`](05-equivalence-catalog-and-evidence-standards.md) | The equivalence catalog entity (directional, conditional, scoped; `full`/`partial`/`none`) and evidence-standard templates per requirement type, with starter content. | When filling `equivalence_notes` or `evidence_standard` |
| [`06-employer-side-discovery-research-plan.md`](06-employer-side-discovery-research-plan.md) | Survey instrument (A–F), interview guide, analysis plan and scope decision rules for recruiters/HR and hiring managers. Items marked **[WF]** came from the workflow review. | Before any research work |
| [`07-pilot-plan-and-recruitment-post.md`](07-pilot-plan-and-recruitment-post.md) | The 10-person wording pilot, its decision table, and two LinkedIn post drafts. | Before fielding anything |
| [`08-pilot-participant-experience.md`](08-pilot-participant-experience.md) | The pilot from the participant's seat: four touchpoints, what they see, what they can and cannot change, what is captured. §4 lists what the owner needs before week 1. | When building pilot materials (P2) |
| [`09-hiring-workflows-draft.md`](09-hiring-workflows-draft.md) | **Approved 2026-11-08.** Company (C-01–C-26) and candidate (K-01–K-16) workflows as node lists with decision points and owners; the owner's review record (§4); the table attaching every framework element to the nodes it serves (§3). | Whenever you need to know where in the process a component sits |
| [`10-hiring-workflow-diagrams.md`](10-hiring-workflow-diagrams.md) | Mermaid source for both approved diagrams. | With `09` |
| [`README.md`](README.md) | One-paragraph index of the directory. | Optional |

## 4. Decisions already made (do not re-open without the owner)

From the owner directly, recorded in `09` §4 and the docs named:

1. **Verdict structure.** `determination` (`met` · `met_equivalent` · `unresolved` · `not_met`) + `basis` (12 equivalency categories) + `uncertainty[]` codes. Three reported numbers: evidenced coverage, unresolved weight, unmet weight. `unresolved` is never silently scored as 0.5. (`03`, `04`)
2. **Model reports, code decides.** Duration, scope, credential, recency and named-party comparisons are recomputed in code from `reported_facts` vs requirement `thresholds`; the model's `confidence` is logged, never used in arithmetic. Carried from the prototype, where it was proven. (`02`, `04`)
3. **Clarification questions ask for evidence, not confirmation**, and the answer *shape* is controlled, never the content: options come only from the candidate's own corpus or from generic, requirement-independent ladders. Free text is a capped supplement that goes through the judge as evidence, never parsed. (`03`, `04` §4)
4. **Clarification runs in two stages**: system-driven for high-volume early screening (C-11c), recruiter-asked live from system-generated questions once a shortlist exists (C-13). Items carry `asked_by`. (`09` Q3)
5. **Role definition**: HR owns the structure (a standing template, C-05); the hiring manager is solely responsible for drafting the content and structured requirements to that template (C-06); HR's sign-off is a conformance and comp check, not a substance review (C-07). (`09` Q2)
6. **Manpower requests** are reviewed by a committee; **deferred is a formal outcome with a resubmission date**. (`09` Q1)
7. **Hold states are a framework requirement**: every hold carries a revisit date the candidate can see, and an unreviewed hold past its date is an **escalation**, not a reminder. Its *priority* is set after pilot data. (`09` Q7)
8. **Behavioral requirements are not scored in v1** (weight 0, "not assessed"). (`05` §2.3)
9. **Versioning**: separate `schema_version`, `vocab_version`, `formula_version`, `prompt_fingerprint`, `rule_version`; additive within a major; records stored at the version written and upcast on read; every match record snapshots the full version set. (`04` §0)
10. **Research before scope**: Phase 0/1 scope must address the employer side's measured top-10 problems; items the match record cannot touch are declared out of scope explicitly. (`06` §5)
11. **Participants, not candidates**: the ten pilot participants are recruiters/HR and hiring managers; "candidate" is reserved for job seekers. (`08`)

## 5. Open items the pilot is expected to close

- **C-14** — do hiring managers cut individuals from a slate before interviewing often enough for this to be a decision point? (survey D4 required; HM probe)
- **K-09** — should a rejected candidate be able to dispute a specific judgment in v1? (dispute probe)
- **K-04** — pre-application gap view for candidates; owner leans yes, cost high, value uncertain; employer-side probe in the pilot, candidate-side study later.
- **Hold-state priority** — where the hold requirement ranks (survey D8; hold probe).

Other open design items are listed in `00-todo.md` P4 and P5 and in each document's "Open items" section.

## 6. What to do next

Work **from `00-todo.md`, in order**. As of this handoff:

- **P1** is complete: the workflows are approved and their framework-attachment table is now `01` §5.6.
- **P2** (pilot materials) is where you start: consent/landing text, one-page interviewer note template including the `[WF]` probes, instrument change log starting at `survey v0.1`, survey platform decision, stimulus check at phone and laptop widths. Specs are in `08` §4.
- **P3** is the owner's (recruiting ten names, posting).
- **P4** schema follow-through runs in parallel with P3.

Do not start building the system itself from these documents. Phase 0 (`01` §5.5) begins after the pilot and the schema follow-through, and any change to this repository's reviewer goes through the FSD and an ADR.

## 7. Conventions to keep

- **Edit the source of record** (the claude.ai project) when you can; re-copy here afterwards. If you can only edit here, update the document's `## Status` line with the date and the words "repository copy ahead of project".
- **Record decisions in the document they affect**, not in the to-do. The to-do holds work items only; remove items when done rather than striking them through.
- **Keep the numbered prefixes** and add new documents at the next number. Update `README.md`'s table and this file's §3 when you add one.
- **Cross-reference by file name.** Project copies use `claude/<name>.md`; repository copies use `NN-<name>.md`. When copying, fix links to the repository names.
- **Commit messages** follow the repository's Conventional Commits rule (`docs:` for this directory) and explain *why* in the body.
- **Branch**: `claude_dev01`. A push updates draft PR #382 automatically; do not open a second PR by hand (see `CONTRIBUTING.md`).
- **No em dashes in anything user-facing** on the site (`AGENTS.md`). These design documents are internal and do use them; do not copy their prose into `apps/web` as-is.
- **Diagrams**: Mermaid, validated before publishing; quote edge labels that contain punctuation; keep `.mmd` sources in `diagrams/` and the rendered doc in `10`.
- **Tone of the documents**: plain, specific, no filler; state what was decided and why, name what is open, and give every open item an owner or a trigger.

## 8. Questions to ask the owner before deviating

- Anything in §4 above.
- Whether a proposed schema field is a vocab change (minor) or a structure change (major) under `04` §0.
- Whether a new research question goes into the pilot (wording-only) or waits for the main wave.
- Anything that would put third-party or candidate data into this repository or its corpus (`02` §5; backlog §1b): not without the owner.
