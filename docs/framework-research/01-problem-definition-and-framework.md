# Hiring System Redesign: Problem Definition, Solution Vision, and Build Framework

**Document purpose**  
This document defines the core problem in corporate hiring, the most common symptoms and complaints from both sides, the root causes (with special emphasis on keyword-based screening and the lack of domain expertise among initial reviewers), what a proper solution looks like, and a practical framework for building software that improves decisions and manages expectations for candidates, hiring managers, and HR. The design treats continuous, data-driven improvement as a first-class property rather than an afterthought.

**Status**  
Starting document for design and implementation. First iteration is not expected to solve the full problem; the system is designed so that measurement and feedback produce measurable improvement over successive cycles. §5.6 (process model) added 2026-11-08 from the approved hiring workflows.

---

## 1. The Problem

Corporate hiring systematically fails to surface and correctly evaluate candidates who can perform the work. The dominant early-stage failure is that initial screening relies on keyword presence (exact or near-exact string matching in ATS systems and human scans) rather than on skill and experience equivalency. This produces systematic false negatives: candidates who have performed the required work are eliminated because their language, titles, tools, or framing differ from the job description's vocabulary.

Skill equivalency—mapping different descriptions of the same capability, outcome, or domain knowledge—is underdeveloped both in tooling and in practice. The problem is compounded by the fact that the HR and recruiting personnel who review résumés and ATS output generally lack the domain expertise required to assess equivalency, especially for technical or specialized roles. Expertise arrives too late, after the largest signal loss has already occurred.

Secondary failures (ghosting, high volume, opaque culture-fit judgments, long timelines, mutual distrust) are real and compounding, but they are largely downstream of this signal-loss step. Once qualified candidates are filtered out or never surfaced with their actual equivalencies visible, later stages operate on a distorted pool.

### Evidence of scale

- In the Harvard Business School / Accenture *Hidden Workers* research, 88% of surveyed employers agreed that qualified high-skills candidates are vetted out of the process because they do not match the exact criteria established by the job description (94% for middle-skills candidates). The research estimated more than 27 million "hidden workers" in the U.S. alone at the time of the study.
- Application volume has roughly doubled since 2022 (Greenhouse and other platform data). Employers consistently rank "too many unqualified applicants" as a top problem while simultaneously discarding people who could do the job.
- Keyword and proxy-based filters reward résumé optimization more than demonstrated capability.

---

## 2. Most Common Symptoms and Complaints

### From candidates / job seekers

- Applying and never hearing back (consistently the top frustration; ~57% in recent iHire data).
- Being rejected without any human or meaningful AI contact (a large fraction of applicants report this happens often).
- Feeling the process is a "black box": no visibility into whether a human reviewed the application or why it was rejected.
- Exhaustion from long, repetitive applications and the need to apply to dozens or hundreds of roles.
- Suspicion of ghost jobs, scams, or postings with no real intent to hire.
- Pressure to keyword-optimize or use AI tools simply to survive automated filters, while also being suspected of using AI.
- Poor or absent feedback even after interviews.
- Mismatch between stated requirements and the actual work once hired (or the inability to evaluate culture/working style before investing heavily).

### From employers (HR, recruiters, hiring managers)

- Too many applications from candidates who appear unqualified on the surface (top online recruiting challenge in multiple surveys).
- Candidate ghosting or unresponsiveness (cited by ~50%+ of employers).
- Difficulty determining real capability from keyword-optimized or AI-assisted applications.
- Time wasted reviewing noisy shortlists while stronger matches remain invisible.
- Longer time-to-fill and a material share of roles that close unfilled.
- Inability of generalist reviewers to confidently judge technical or specialized equivalency.
- Mutual distrust and brand damage from poor candidate experience.
- Low measurement of quality-of-hire; limited ability to calibrate screening decisions against later outcomes.

Both sides describe a process that feels opaque, high-effort, and low-signal. Trust is eroded in both directions.

---

## 3. Root Causes (Prioritized)

1. **Keyword-based initial screening with weak or absent equivalency modeling.**  
   Requirements are treated as terms to be found rather than capabilities or outcomes to be evidenced. Vocabulary mismatch, non-standard titles, equivalent tools or methods, and transferable experience are systematically under-recognized. ATS filters and early human scans amplify this.

2. **Initial reviewers lack domain expertise for equivalency judgments.**  
   HR and recruiting staff are typically generalists. They cannot reliably determine whether differently worded evidence satisfies a technical requirement. They default to exact keywords, familiar brands, or standard titles. Domain experts (hiring managers) usually see candidates only after this filter.

3. **Opaque, high-volume process with poor expectation management.**  
   Little structured feedback, inconsistent communication ownership, and limited visibility into fit on either side.

4. **Subjective or late assessment of working-style and cultural alignment.**  
   "Culture fit" is often judged by affinity or self-presentation rather than explicit, observable behaviors and conditions. This compounds professional-fit errors.

5. **Limited feedback from outcomes to screening decisions.**  
   Quality-of-hire and early retention data are rarely systematically linked back to the signals used in early screening, so the system does not improve.

---

## 4. What a Proper Solution Looks Like

A system whose core operation is transparent, requirement-level equivalency judgment against evidence. The result is presented so that a non-expert reviewer can understand and act on it without needing deep domain knowledge. The system is designed as a recursive improvement loop: every stage of the process and every post-process outcome generates structured data that is collected, analyzed, and used to refine the schemas, judgments, weights, and workflows.

The first iteration is not expected to solve the problem. The design goal is that the system measurably improves its accuracy, coverage, and usefulness with each cycle of data.

### Desired properties

- **Higher-fidelity signal.** Job descriptions are decomposed into discrete requirements (skills, outcomes, constraints, domain knowledge, working conditions), each with an explicit evidence standard. Candidate evidence is examined for whether it satisfies the requirement, including cases where the language differs.
- **Transparent match record.** Matching produces a per-requirement verdict (met / partial / unmet) with pointers to supporting evidence and a short rationale. An overall coverage score is computed arithmetically from the verdicts and weights. No opaque single "AI score" is the sole decision artifact.
- **Usable by non-experts.** HR or recruiting staff can rank and filter using the match record without possessing domain expertise. Hiring managers can audit evidence and override individual judgments. Candidates see the same record and understand which gaps are substantive versus lexical.
- **Bidirectional expectation setting.** Both sides can see, before heavy investment, where alignment is strong, partial, or weak on professional requirements and (later) on explicit working-style / cultural indicators.
- **Recursive improvement framework.** At every phase the system records:
  - Structured requirements and evidence standards used.
  - Equivalency judgments (automated or human), including rationales.
  - Human overrides and the reasons given.
  - Stage outcomes (advanced, rejected, withdrawn, offer extended, accepted).
  - Post-process outcomes (30/90-day manager and employee ratings of professional and working-style fit, early retention, available performance indicators).
  
  These records are linked so prediction accuracy can be measured (correlation between pre-hire match coverage and later fit/performance; systematic disagreement patterns). Analysis is used to refine requirement taxonomies, evidence standards, equivalence mappings, judgment logic, weights, and process steps. Improvements compound because the same match-record structure that enables the initial screen also enables measurement of its own accuracy.
- **Privacy and control.** Candidates control the depth and audience of their evidence. Employers control role definitions and scoring weights. Local or self-hosted options remain possible where data sensitivity requires it.

### Success metrics (examples)

- Reduction in keyword-driven false negatives for roles that use the structured matching.
- Higher interview-to-offer and offer-to-accept rates on roles using the match record.
- Decrease in early attrition attributed to fit mismatch.
- Higher rates of timely, substantive status communication.
- Documented improvement over time in prediction accuracy (system judgments vs. later outcomes) and reduction in expert override rates on well-calibrated requirement types.

---

## 5. Framework for Building the System

### 5.1 Core data model

- **Requirement**: ID, text, type (skill / outcome / tool / domain / constraint / behavioral), weight, evidence standard (what counts as met or partial), optional equivalence notes, version.
- **Evidence item**: Attributable claim or artifact linked to a person, with context (dates, scope, metrics, links).
- **Equivalency judgment**: Requirement ID + evidence item pointer(s) + verdict (met / partial / unmet) + rationale + optional confidence + reviewer note. Always inspectable. Includes fields for override (who, when, reason, new verdict).
- **Match record**: Set of judgments for one role–candidate pair, arithmetic coverage score, explicit gaps, version of schemas/rules used. Designed to be readable by a non-expert.
- **Outcome link**: Later-stage or post-hire data attached to the match record (stage reached, offer status, 30/90-day fit and performance ratings) for calibration.

### 5.2 Core software components

- Role / JD parser that extracts a structured, editable requirement list.
- Evidence store and query interface for candidates (or for employers holding candidate data).
- Equivalency engine that, for each requirement, retrieves relevant evidence and produces a judgment with rationale (local or remote inference acceptable).
- Match-record viewer optimized for non-expert use: clear coverage summary, expandable evidence, easy override.
- Calibration and analysis layer: reports on system vs. expert vs. outcome, versioned updates to schemas and logic.
- Lightweight process and communication layer once the match record exists (status, ownership, response expectations).

### 5.3 Recursive improvement layer (cross-cutting)

- Instrumentation of every match record, judgment, override, stage transition, and post-hire outcome with identifiers that allow later joining. Privacy and retention controls defined up front.
- Calibration views that show, for a given requirement type or domain, system verdict vs. expert override vs. later outcome.
- Controlled, versioned update paths for schemas, evidence standards, equivalence notes, scoring weights, and judgment logic.
- Success criteria for the loop: measurable lift in prediction accuracy, reduction in expert override rate on calibrated requirement types, and documented decrease in keyword-driven false negatives.

### 5.4 Data collection and research priorities

- Catalog common equivalence classes in target domains.
- Measure false-negative rates of current keyword and ATS filters against human or evidence-based judgments.
- Collect paired judgments (system vs. expert vs. generalist) and later outcomes.
- Track which candidates advance under keyword screening versus under match-record screening, and later outcomes.

### 5.5 Phased build sequence

- **Phase 0 – Ontology and pilot instruments.** Define the requirement and judgment schemas (including override and outcome-link fields). Build a minimal extractor + equivalency judge + readable match record. Validate that a non-expert can use the record to identify strong coverage on technical roles. Run against a small set of real JDs and evidence corpora.
- **Phase 1 – Match record as primary screening artifact.** Deploy for a hiring team or for candidates applying to specific roles. Measure reduction in expertise-dependent and keyword-driven false negatives and improvement in requirement coverage visibility. Begin collecting outcome data.
- **Phase 2 – Bidirectional matching and process layer.** Mutual visibility of fit, controlled volume based on match quality, working-style indicators expressed in the same framework, and outcome feedback.
- **Phase 3 – Calibration and expansion.** Use accumulated data to refine weights, surface systematic biases, expand the behavioral taxonomy, and offer self-hosted or privacy-preserving deployment options.

### 5.6 Process model: where each component sits

*Added 2026-11-08 from the approved hiring workflows (company perspective C-01–C-26, candidate perspective K-01–K-16; node lists and review record in `09-hiring-workflows-draft.md`, project copy `hiring-workflows-draft.md`; diagrams in `10-hiring-workflow-diagrams.md`).*

The two workflows are the physical process the system operates within. Every entity in §5.1 and every component in §5.2 attaches to specific nodes, which is how the framework's sections are classified and how gaps in either direction are made visible: a node with no system support, or a component with no node.

**Decisions from the workflow review that shape the components.** Manpower requests are reviewed by a committee and *deferred* is a formal outcome with a resubmission date (C-03/C-04). HR owns the structure of role definition through a standing template (C-05); the hiring manager is solely responsible for drafting the content and the structured requirements to it (C-06); HR's sign-off is a conformance and compensation check, not a substance review (C-07). Clarification runs in two stages: system-driven for high-volume early screening (C-11c), then recruiter-asked live from system-generated questions once a shortlist exists (C-13). Hold states are a framework requirement: every hold carries a revisit date the candidate can see, and an unreviewed hold past its date is an escalation, not a reminder; its priority among the components is set after the employer-side pilot.

| Framework element | Company nodes | Candidate nodes | Status / gap |
|---|---|---|---|
| HR role-definition template | C-05 | — | New artifact; makes HR's structural ownership explicit |
| Requirement entity, type taxonomy, thresholds, evidence standards (§5.1) | C-06 (HM authors), C-07 (HR checks conformance) | K-03 read-only | The system's HM-assist at C-06 is the first employer-side surface to build |
| Equivalence catalog | C-06 (seeding equivalence notes), C-11b, C-14 (HM overrides feed it) | K-04 (if a pre-application gap view is built) | Curator role unassigned |
| Evidence item; candidate control of depth and audience (§5.1) | C-11a, C-21 (verification) | K-02, K-05, K-08, K-14, K-16 | `verified_external` evidence is owned at C-21 |
| Equivalency judgment (§5.1) | C-11b | K-07 | — |
| Clarification, system stage | C-11c | K-08 | Trigger policy open |
| Clarification, recruiter stage | C-13 | K-10 | Same question templates; answers carry `asked_by=recruiter` |
| Match record with three coverage numbers (§5.1) | C-12, C-14, C-17 | K-07, K-09, K-11 | Employer ranking view across candidates does not exist in the prototype |
| Override | C-12 (recruiter escalates to HM), C-14 (HM cuts from slate) | K-09 (candidate dispute: open) | C-14's weight and K-09's existence are decided by the pilot |
| Outcome link (§5.1) | C-17, C-19, C-23, C-24, C-25 | K-15 | Employer-side outcomes absent in the prototype |
| Calibration and analysis layer (§5.2, §5.3) | C-26, and across C-23–C-25 | — | — |
| Hold states with revisit date and escalation | C-04, C-07, C-12, C-17, C-19 | K-09 wait, K-13 ask-for-time | Requirement decided; priority after pilot |
| Process and communication layer (§5.2) | C-20 (substantive reason on rejection), every dated hold | K-05, K-06, K-09, K-12 (what happens next and by when) | Dated, escalating holds are the concrete form of "no ghosting" |
| Working-style and behavioral indicators (Phase 2) | C-06 (stated conditions), C-15, C-23 | K-13, K-15 | Not scored in v1 |

**Nodes the first version does not touch:** C-01–C-04 (need and approval), C-09 (sourcing), C-18 (offer construction), C-22 (onboarding). These are declared out of scope rather than ignored. The employer-side survey's problem list includes items that live in them (internal delays, salary mismatch discovered late); if those rank in the measured top ten, this section is where the scope statement must say why the match record does not address them.

**Open items the pilot closes:** whether hiring managers cut individuals from a slate before interviewing often enough for C-14 to be a decision point; whether a rejected candidate can dispute a specific judgment in v1 (K-09); whether a pre-application gap view for candidates is worth building (K-04); and the priority of the hold-state requirement.

---

## 6. Next Concrete Step

Formalize the precise fields for a **Requirement** and an **Equivalency Judgment** (including the fields needed for overrides, versioning, and outcome linking). Once those schemas are locked, a minimal matching prototype and data-collection plan can be specified.

This document is the starting point. It prioritizes the equivalency layer and the recursive improvement loop so that the system can improve over time rather than remaining a static first attempt.

---

*Document prepared as the foundational definition for software design and implementation.*
