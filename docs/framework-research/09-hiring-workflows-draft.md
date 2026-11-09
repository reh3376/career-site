# Hiring Workflows — Draft Node Lists (revision 2, owner's answers applied)

**Purpose.** Two high-level process maps, company perspective and candidate perspective, as numbered node lists with every decision point's outcomes and owner. Then each framework component and entity is attached to the nodes it serves (§3), which is the point of the exercise. Diagrams: `10-hiring-workflow-diagrams.md`.

**Conventions.** `C-nn` company nodes, `K-nn` candidate nodes. `◆` marks a decision point. Every deferred or paused outcome carries a revisit date **and an escalation rule** (§4, Q7). "Owner" is the role accountable for the node, not the only participant.

**Status.** Revision 2, 2026-11-08. Seven review questions answered by the owner; four decided, three folded into the pilot, and one (Q7) decided in principle with its priority awaiting pilot data (§4). **Both perspectives approved by the owner 2026-11-08.** Open items C-14, K-04 and K-09 stay marked open until the pilot closes them; the approved maps are the process model the framework is classified against (§3).

---

## 1. Company perspective

### Phase A — Need and approval

| # | Node | Owner | Inputs → outputs | Notes |
|---|---|---|---|---|
| C-01 | **Need identified** | Hiring manager (HM) | Trigger: attrition, growth, new scope, backfill → informal case | Often undocumented; C-02 is the first record |
| C-02 | **Manpower request submitted** | HM | Business case: why, what level, cost, when, backfill vs new headcount → request record | |
| C-03 | **Request review** | **Committee**: finance, HR/people partner, HM's manager, department head | Request → recommendation | *Decided (Q1a).* Checks budget, headcount plan, alternatives (redistribute, contractor, automate) |
| C-04 ◆ | **Approval decision** | Budget owner, on the committee's recommendation | **Approved** → C-05 · **Rejected** → end, reason to HM · **Deferred** → parked **with resubmission date**; on that date returns to C-02/C-03; past date unreviewed → **escalate** (§4 Q7) | *Decided (Q1b):* deferred is a formal outcome with a date |

### Phase B — Role definition

| # | Node | Owner | Inputs → outputs | Notes |
|---|---|---|---|---|
| C-05 | **HR role-definition template** | HR | Standing artifact: required fields, structure, requirement types, weight scale, evidence-standard slots, comp-band and level fields | *Decided (Q2).* HR owns the **structure**; refreshed rarely |
| C-06 | **HM drafts role definition and structured requirements** | **HM, solely responsible for content** | Approved request + C-05 template → role document conforming to HR structure: purpose, outcomes, typed/weighted requirements, alternatives, thresholds, evidence standards, level, comp band, location, working conditions | *Decided (Q2).* The system assists the HM here (extraction from a draft JD, catalog-seeded equivalence notes, template conformance checks) but the HM signs the content |
| C-07 ◆ | **Conformance and comp check** | HR (structure, comp band, compliance); legal where needed | **Conforms** → C-08 · **Non-conforming** → C-06 with the field list · **Hold** → parked **with revisit date**, escalation past date | HR does not re-judge the requirements' substance; that stays with the HM |
| C-08 | **Job description rendered and posted** | Recruiter renders from C-06; HM approves text | Role document → public posting; channels chosen | Posting is a rendering of C-06, never the source of truth |

### Phase C — Sourcing and screening

| # | Node | Owner | Inputs → outputs | Notes |
|---|---|---|---|---|
| C-09 | **Applications received / sourced** | Recruiter | Applications, referrals, outreach → applicant pool | |
| C-10 | **Knockout / compliance filter** | ATS rules set by recruiter | Pool → eligible pool | Hard constraints only (authorisation, location, licence). Keyword knockouts live here today: the first false-negative source |
| C-11 | **Initial screen via match record** | Recruiter / HR screener using the system | Eligible pool → ranked records | |
| C-11a | · Evidence gathered per candidate | System; candidate supplies | Résumé, profile, documents → evidence items | |
| C-11b | · Requirements judged against evidence | System | → judgments, three-number coverage | |
| C-11c | · **System-driven clarification round** | System → candidate → system | Unresolved judgments → structured questions → answers → re-judge | *Decided (Q3): both, by stage.* High-volume, pre-shortlist round, answered in the system with controlled inputs. Trigger policy still open |
| C-12 ◆ | **Screen decision** | Recruiter, using the match record; HM may audit/override | **Advance** → C-13 · **Reject** → C-20 · **Hold** → parked **with revisit date**, escalation past date · **Needs HM input** → HM reviews flagged judgments → C-12 | |
| C-13 | **Recruiter screen call, with remaining clarifications** | Recruiter | Shortlist → interest, logistics, comp, constraints; **recruiter asks the still-unresolved items live from system-generated questions and records the structured answers** | *Decided (Q3).* Second clarification stage; answers carry `candidate_response` provenance with `asked_by=recruiter` |
| C-14 ◆ | **Slate to HM** | Recruiter → HM | **Accept slate** → C-15 · **Reject individuals** → C-20, each recorded as an override against the match record · **Reject slate** → C-09, or C-06 if mis-defined | **Open — pilot (Q4):** whether HMs cut individuals without interview often enough for this to be a real decision point. Kept provisionally |

### Phase D — Assessment and decision

| # | Node | Owner | Inputs → outputs | Notes |
|---|---|---|---|---|
| C-15 | **Interviews** | HM + panel; recruiter coordinates | Slate → interview evidence, working-style indicators | Interview targets `unresolved` and `met_equivalent` items first |
| C-16 | **Assessment / work sample** (optional) | HM / panel | → additional evidence items | |
| C-17 ◆ | **Hire decision** | HM; panel input; HR consult | **Select** → C-18 · **Reject all, re-source** → C-09 · **Reject all, redefine** → C-06 · **Defer** → parked **with revisit date**, escalation past date | Recorded against the match record so outcome links to screening |
| C-18 | **Offer prepared** | HR/comp + HM | → offer; off-band offers loop to budget owner | |
| C-19 ◆ | **Offer outcome** | Candidate decides; recruiter manages | **Accepted** → C-21 · **Negotiation** → C-18 · **Declined** → C-17 or C-09 · **No response by DATE** → treated as declined, escalation to recruiter lead | |
| C-20 | **Notify non-selected** | Recruiter | Rejection with substantive reason where policy allows | The match record makes a substantive reason cheap |

### Phase E — Onboarding and outcome

| # | Node | Owner | Inputs → outputs | Notes |
|---|---|---|---|---|
| C-21 | **Pre-boarding** | HR | Checks, paperwork, equipment → start-ready; **check fails** → C-17 or withdrawal | Verification of self-attested evidence (`verified_external`) belongs here if not earlier |
| C-22 | **Start and onboarding** | HM + HR | → employee in role | |
| C-23 | **30-day check-in** | HM + employee | → fit rating, both directions | Outcome link |
| C-24 | **90-day review** | HM + employee + HR | → fit/performance; **confirm / extend probation / exit** | Outcome link |
| C-25 | **Retention / performance at 6–12 months** | HM, HRIS | → outcome data | Feeds calibration |
| C-26 | **Requisition closed; lessons captured** | Recruiter + HM | Metrics, overrides, hold history → process record | |

---

## 2. Candidate perspective

| # | Node | Candidate's decision / action | Sees today | Sees under the framework |
|---|---|---|---|---|
| K-01 | **Trigger** | Decide to look | — | — |
| K-02 | **Evidence preparation** | Assemble evidence; decide depth and audience | Nothing about how it will be read | Which requirement types their evidence tends to leave `unresolved`, across roles |
| K-03 | **Role discovery** | Search, alerts, referrals, outreach | The posting | The posting and, where offered, the structured requirement list |
| K-04 ◆ | **Apply or not** | **Apply** → K-05 · **Skip** | Own guess at fit | **Open (Q5):** owner inclined to show the gaps (evidenced / unresolved / unmet, no number) and let the candidate supply equivalence evidence before applying; significant build; value uncertain. Noted for a candidate-side study; one employer-side angle added to the pilot (§5) |
| K-05 | **Application submitted** | Tailor, answer screening questions, consent | Confirmation, maybe | Confirmation plus what happens next and by when |
| K-06 | **Knockout filter** (C-10) | None | Silent rejection | Which hard constraint failed, where policy allows |
| K-07 | **Initial screen** (C-11) | None | Nothing | Their match record |
| K-08 ◆ | **System clarification round** (C-11c) | **Answer** (controlled inputs) · **Not applicable** · **Decline** · **Withdraw** | Does not exist | The question, the posting's own wording, controlled options; never the score or threshold |
| K-09 ◆ | **Screen outcome** (C-12) | On reject: **accept** / **request feedback** · On hold: **wait** or **withdraw** | Usually nothing | Outcome, substantive reason, hold revisit date. **Open — pilot (Q6):** whether a post-decision *dispute* exists in v1 |
| K-10 | **Recruiter call** (C-13) | Confirm interest; answer remaining clarification questions live; **withdraw** if misaligned | Comp range sometimes | Comp band and working conditions beforehand |
| K-11 | **Interviews** (C-15/16) | Prepare; continue or **withdraw** per round | Names, format | Which requirements the interview will probe |
| K-12 ◆ | **Decision outcome** (C-17) | On reject: **accept / request feedback** · On select → K-13 | Often nothing for weeks | Timeline commitment; reason on rejection |
| K-13 ◆ | **Offer** (C-18/19) | **Accept** · **Negotiate** · **Decline** · **Ask for time until DATE** | Offer letter | Same, plus working-style comparison (Phase 2) |
| K-14 | **Pre-boarding and start** (C-21/22) | Provide verification documents; resign; start | Checklists | Which self-attested items are being verified |
| K-15 | **30/90-day check-in** (C-23/24) | Rate fit both directions | Manager's review of them | Own rating recorded against the pre-hire record |
| K-16 | **Parallel applications** | Manage many pipelines; **withdraw** others on accepting one | A spreadsheet | One evidence record reused; one view of where each stands |

---

## 3. Where the framework attaches

| Framework element | Company nodes | Candidate nodes | Gap / note |
|---|---|---|---|
| HR role-definition template | C-05 | — | **New artifact**: HR's structural ownership made explicit (Q2) |
| Requirement entity, taxonomy, thresholds, evidence standards | C-06 (HM authors), C-07 (HR checks conformance only) | K-03 read-only | The system's HM-assist at C-06 is the first employer-side surface to build |
| Equivalence catalog | C-06 (seeding notes), C-11b, C-14 (HM overrides feed it) | K-04 (if gap view is built) | Curator role unassigned |
| Evidence item, candidate control | C-11a, C-21 (verification) | K-02, K-05, K-08, K-14, K-16 | `verified_external` owner is C-21 |
| Equivalency judgment | C-11b | K-07 | — |
| Clarification: system stage | C-11c | K-08 | Trigger policy open |
| Clarification: recruiter stage | C-13 | K-10 | **New (Q3)**: same question templates, `asked_by=recruiter`, structured answers recorded by the recruiter |
| Match record, three numbers | C-12, C-14, C-17 | K-07, K-09, K-11 | Employer ranking view across candidates does not exist in the prototype |
| Override | C-12 (escalate to HM), C-14 (HM cuts) | K-09 (dispute — open, Q6) | C-14's weight depends on pilot Q4 |
| Outcome link | C-17, C-19, C-23, C-24, C-25 | K-15 | Employer-side outcomes absent in prototype |
| Calibration and analysis | C-26, C-23–C-25 | — | — |
| **Hold states with date + escalation** | C-04, C-07, C-12, C-17, C-19 | K-09 wait, K-13 ask-for-time | **Framework requirement (Q7)**; priority set after pilot data |
| Process / communication layer | C-20, every dated hold | K-05, K-06, K-09, K-12 | — |
| Working-style (Phase 2) | C-06 (stated conditions), C-15, C-23 | K-13, K-15 | Not scored in v1 |

**Nodes untouched by v1:** C-01–C-04, C-09, C-18, C-22. Declared out of scope; survey items 15 and 18 cover several of them, and if those rank top-10 the scope statement must say why.

---

## 4. Review record (2026-11-08)

| Q | Node(s) | Answer | Effect |
|---|---|---|---|
| 1a | C-03 | **Committee** reviews the manpower request | C-03 owner set to committee; C-04 decided by budget owner on its recommendation |
| 1b | C-04 | **Deferred is formal, with a resubmission date** | Kept as a first-class outcome; combined with Q7's escalation rule |
| 2 | C-05–C-07 | **HR provides the general structure and required fields; the HM is solely responsible for drafting a document that conforms** | Old C-05/C-06 restructured: C-05 is HR's standing template, C-06 is the HM's authored role document (content + structured requirements), C-07 is an HR conformance/comp check, not a substance review |
| 3 | C-11c, C-13 | **Both, by stage**: system-driven clarification for high-volume early screening; recruiter asks the remaining questions live once a shortlist exists | C-13 extended; clarification items gain `asked_by` (system / recruiter) |
| 4 | C-14 | **Uncertain → pilot** | Kept provisionally. Pilot: make D4 required; add HM interview probe (§5) |
| 5 | K-04 | **Uncertain**; inclined to show gaps (no number) and let candidates supply equivalence evidence pre-application; significant build; not sure worth it yet | Recorded as an open design decision with the owner's lean. Candidate-side question, so mainly for a later candidate study; one employer-side probe added to the pilot (§5) |
| 6 | K-09 | **Uncertain → pilot** | Dispute path not in v1 draft; pilot asks employers whether they would accept candidate disputes and on what terms (§5) |
| 7 | all holds | **In the owner's experience holds rarely resurface at all. That is a far-from-ideal state and the framework should definitely address it. The owner wants the ten pilot participants' accounts before setting how high it ranks; priority will be determined in part by that data.** | Two things recorded separately. **(a) Requirement, decided:** the framework addresses hold states — every hold carries a revisit date, the candidate can see it, and an unreviewed hold past its date is treated as an escalation (owner's manager for requisition holds, recruiting lead for candidate holds) and counted as a process metric. **(b) Priority, open → pilot:** where this ranks against the other framework components is set after the pilot's hold-state probe (§5) and, for the main wave, the ranking of survey items 9, 10 and 18. The escalation mechanics are the framework's current proposal, not a settled design |

## 5. Items folded into the pilot

Applied to `06-employer-side-discovery-research-plan.md`, marked **[WF]** there:

- **D4 becomes required** (HM decline-without-interview share), and the HM call gets: *"Think of the last slate a recruiter sent you. How many did you decline before interviewing, and why?"* (Q4)
- **Recruiter and HM call:** *"If a rejected candidate could contest one specific judgment with new evidence, would you want to see it? Under what conditions would you re-open the decision?"* (Q6)
- **Recruiter call:** *"If applicants could see, before applying, which of your requirements their background does and doesn't cover, would that change the quality or quantity of applications you receive?"* (Q5, employer-side angle)
- **Both:** *"Tell me about a requisition or a candidate that was put on hold. What happened to it, and how did you find out? What would have had to exist for it to come back on its own?"* (Q7: tests the premise that holds rarely resurface, and gathers what participants think would fix it, which sets the priority of the hold-state requirement)
- New survey item **D8** on what happens to holds.

## 6. Next

- Fold §3 into `01-problem-definition-and-framework.md` as a process-model section (approved 2026-11-08; scheduled under P1 in the to-do).
- After the pilot: close C-14, K-04, K-09; set the priority of the hold-state requirement.
