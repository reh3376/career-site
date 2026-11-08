# Framework research: generalising the JD reviewer into a multi-party hiring tool

Working documents for the "Hiring_prod" design effort, copied here from the
claude.ai project on 2026-11-08 for review. The project is the source of
record while this work is in design; these copies are snapshots and may
lag it.

The effort takes the mechanism this repository already runs (JD upload,
structured requirement extraction, per-requirement evidence judging, score
in code, decision log, evaluation harness) and asks what has to change for
it to serve employers and many candidates rather than one owner. Nothing
here is a commitment for this repository; where a document proposes a
change to the reviewer, it still goes through the FSD and an ADR.

**Picking this up as a new agent or contributor? Start with [`agent-handoff.md`](agent-handoff.md).**

| Document | What it is |
|---|---|
| `00-todo.md` | The prioritised to-do list for the design effort; the working order of everything below. |
| `01-problem-definition-and-framework.md` | The founding document: the problem, symptoms from both sides, root causes, what a proper solution looks like, build framework and phases, and (§5.6) the process model mapping components to the approved workflows. Written by the owner. |
| `02-career-site-gap-analysis.md` | What this repository already implements against that framework's data model (§5.1) and component list (§5.2), with status and gaps. |
| `03-verdict-taxonomy-and-clarification-loop.md` | Why met / partial / unmet is not enough; the determination + basis + uncertainty structure; the candidate clarification loop the schema must leave room for. |
| `04-schemas-v1-requirement-and-judgment.md` | Draft v1 schemas (field tables and JSON Schema) for Requirement, Equivalency Judgment, Evidence Item, Clarification Item and Match Record, with the versioning policy. |
| `05-equivalence-catalog-and-evidence-standards.md` | The equivalence catalog entity and evidence-standard templates per requirement type, with starter content for the manufacturing/controls/software domain. |
| `06-employer-side-discovery-research-plan.md` | Survey instrument, interview guide and analysis plan for recruiters/HR and hiring managers. |
| `07-pilot-plan-and-recruitment-post.md` | The 10-person wording pilot and the LinkedIn recruitment posts. |
| `08-pilot-participant-experience.md` | The pilot from the participant's seat: each touchpoint, what they see, what they can change, and how it is captured. |
| `09-hiring-workflows-draft.md` | The two hiring workflows as numbered node lists (company and candidate), decision points with owners, the owner's review record, and the table attaching each framework element to the nodes it serves. Approved 2026-11-08. |
| `10-hiring-workflow-diagrams.md` | Mermaid source for both workflow diagrams; GitHub renders it. `diagrams/` holds the `.mmd` files and a self-contained pan-and-zoom viewer (`workflow-viewer.html`). |

The mocked match record used as the interview stimulus is a published
artifact rather than a file; its link is recorded in `07-…` and in the
project.

Where these documents name prototype behaviour they are describing
`main` as of 2026-11-08 (`e333935`). Prompt versions, the score formula
and the golden set move; `docs/llm-tuning-log.md` is authoritative for
those.
