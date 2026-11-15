# Roadmap: the hiring platform

**What this is.** The one document that says where this project is
going, where it stands, and why it is worth doing. Opened 2026-11-10,
the day the owner stated the end goal in full. It is a working
document: §4 (where we are) is dated and rewritten at every stage gate,
§6 (enterprise readiness) moves a row when the work lands, and the
revision log in §8 says what changed. Open work lives in
[`backlog.md`](backlog.md) §1c; requirements live in [`FSD.md`](FSD.md);
this file carries direction. When this file and the running system
disagree about what exists, the system wins and this file is corrected,
the same rule the FSD applies to itself (§2b).

**The owner's statement, 2026-11-10.** "Ultimately my intention is to
turn this career-site into a highly valuable means of connecting
employers to job applicants and vice versa in a manner that will
resolve all of the major issues both companies and individual job
applicants have with trying to find a job and finding viable
candidates for an open position. It is likely that I will sell this
project to ADP, Workday, or a similar organization so it must be well
thought out and built in a secure and dynamic manner to allow for its
integration into a larger enterprise environment."

---

## 1. Goals

In one sentence: build the evidence-based screening layer that hiring
should have had, prove it on real hiring people, and sell it to a
company that can put it in front of millions of hires.

The goals, in the order they have to be met. Each is done when the
condition in the right-hand column is true, not before.

| # | Goal | Done when |
|---|---|---|
| G1 | **Prove the mechanism on real hiring people.** | The pilot's ten recruiters, HR people and hiring managers have read the match record and the hiring workflow on the site and been interviewed; the survey is at v1.0; Phase 0 exits with a recruiter identifying strong coverage from three real records alone (`framework-research/13` decision 11). |
| G2 | **Prove it changes outcomes for one employer.** | A design-partner employer runs real postings and real candidates through it with consent; the keyword baseline beside every record shows the false negatives a term filter would have produced; the employer's recruiters use the record without the owner in the room; stage and post-hire outcomes are being collected. |
| G3 | **Make it adoptable by an enterprise.** | A second tenant is onboarded without a code change; identity, data lifecycle, audit and integration meet the bar in §6; an external security review has passed. |
| G4 | **Make it buyable.** | A buyer's technical, security and compliance diligence can be answered from the repository, the data room and the evidence pack without the owner in the room; the IP position is clean. |

**Standing constraints.** These hold at every stage and are not
re-decided in this document.

- No server spend beyond the CPX41; the workload fits the box
  (`backlog.md` standing rules).
- The owner is the one real user until the thing runs very well for
  him; no beta cohort before that (`backlog.md` §1b).
- Funding comes before the platform: G3 and G4 are not started on the
  owner's own money.
- Build so none of it has to be undone: where the long-term shape costs
  almost nothing today, adopt it (ADR 0029 is the model).
- Everything in the public repository is public whatever the gate says
  (FSD A-05). Participant material, candidate documents and anything
  naming an employer stay in the private repository.
- No user-facing element is complete until the owner has reviewed it
  live.

**What it is not.** Not a job board, not an applicant tracking system,
not a résumé optimiser, and not a culture-fit scorer: behavioural
requirements are recorded and shown as "human judgement required",
never scored (`framework-research/05` §2.3).

---

## 2. The problem, and why it is worth solving

Corporate hiring fails at its first step. Screening, by software and by
the first human reader, works on keyword presence: does the posting's
word appear in the résumé. Candidates who have done the work but
describe it in different words, under different titles or with
different tools are removed before anyone qualified to judge them has
looked. Those false negatives are the dominant loss of signal, and
everything downstream, the noisy shortlists, the long time to fill, the
silence, the distrust on both sides, operates on the distorted pool
that step leaves behind.

The second cause compounds the first. The people doing the first read
are generalists. They cannot tell whether differently worded evidence
satisfies a technical requirement, so they fall back on the exact
term, the familiar brand or the standard title. The domain expert, the
hiring manager, sees candidates only after this filter has run.

Nothing links the outcome back to the screen. Quality of hire and early
attrition are rarely joined to the signals that chose the shortlist,
so the system does not learn.

**Evidence of scale** (`framework-research/01` §1): in the Harvard
Business School and Accenture *Hidden Workers* research, 88 percent of
employers agreed that qualified high-skills candidates are vetted out
because they do not match the exact criteria in the job description,
94 percent for middle-skills candidates, with more than 27 million
hidden workers in the United States at the time of the study.
Application volume has roughly doubled since 2022, and employers rank
"too many unqualified applicants" as a top problem while discarding
people who could do the job. The full symptom list from both sides is
in `01` §2.

**Why it is worth doing.**

- **Both sides pay, so both sides are the market.** Employers pay in
  unfilled roles and bad shortlists; candidates pay in applications
  into silence. A fix that both can see working has two customers.
- **The incumbents cannot fix it from inside.** The applicant tracking
  systems are built on the term filter; an evidence layer that reads
  the same requirements and the same résumés and shows where the filter
  was wrong sits on top of them. That is a thing an incumbent buys
  rather than builds, which is the owner's exit.
- **Every judgment is labelled data.** The record of what was claimed,
  what evidence was shown, what the system decided and what the expert
  said instead is the asset that compounds (ADR 0029 context). The
  mechanism that screens is the mechanism that measures its own
  accuracy.
- **Regulation is moving toward per-decision explanations.** New York
  City requires annual bias audits of automated employment decision
  tools; the EU AI Act classifies employment uses as high-risk; U.S.
  states are legislating disclosure. An opaque single score cannot meet
  those rules. A per-requirement record with evidence and rationale is
  the shape they ask for (§6 row 8).

---

## 3. The thesis: how it is solved

Seven mechanisms, each already designed and most already running in
some form. The design documents are in `framework-research/`; the
prototype they generalise is the JD reviewer (FSD §5.11).

1. **Decompose the posting into requirements with evidence
   standards.** Each requirement has a type, a weight, thresholds and a
   statement of what counts as met, and the hiring manager can edit the
   list before anything is judged (`01` §5.1, `04`, `05` §2).
2. **Judge each requirement against evidence, and report, do not
   decide.** The model returns a determination (met, met by
   equivalence, unresolved, not met), the basis for it, its uncertainty
   and the facts it relied on. Durations, scope, credentials and dates
   are compared in code against the requirement's thresholds; the
   model's confidence is logged and never used in arithmetic (`03`,
   `04` §2.3, handoff decision 2).
3. **A match record a non-expert can act on.** Three numbers computed
   in code (evidenced coverage, unresolved weight, unmet weight), a row
   per requirement with its evidence and rationale, lexical gaps told
   apart from substantive ones, and an override that changes the
   effective value while the model's verdict stays logged (`04` §5,
   `15` §14).
4. **An equivalence catalog that grows from use.** Reusable knowledge
   of which differently named things are the same capability, seeded
   for the manufacturing, controls and software domain and fed by
   expert overrides (`05` §1).
5. **A clarification loop that asks for evidence, not confirmation.**
   When a judgment is unresolved the candidate is asked a question
   whose answer shape is controlled; the answer becomes evidence and
   the affected requirements are re-judged. System-driven early,
   recruiter-asked once a shortlist exists (`03` §4, `09` Q3).
6. **A process layer that ends silence.** Every hold carries a revisit
   date the candidate can see; an unreviewed hold past its date is an
   escalation; a rejection carries a substantive reason (`09` Q7, `01`
   §5.6).
7. **A loop that improves the loop.** Stage and post-hire outcomes are
   linked back to the record; calibration views compare system, expert
   and outcome; schemas, catalog, weights and rules are versioned and
   every record snapshots the versions that produced it. A plain
   keyword baseline runs beside every record so the false-negative
   claim is measured, not asserted (`01` §5.3, `13` decision 5).

| For | Read |
|---|---|
| The problem, the solution vision, the component map against the hiring workflows | `framework-research/01` |
| What the prototype already does and where it falls short | `framework-research/02` |
| Verdict structure and the clarification loop | `framework-research/03` |
| The v1 schemas | `framework-research/04`, rendered as migrations and proto in `14` |
| The equivalence catalog and evidence standards | `framework-research/05` |
| The two hiring workflows the components attach to | `framework-research/09`, diagrams in `10` |
| The decision to generalise the reviewer, and the build order | ADR 0032, `framework-research/13` |
| The requirements, row by row | `FSD.md` §5.15 (framework) and §5.16 (pilot) |

---

## 4. Where we are, 2026-11-13

Production is rogerhenley.dev on `38be35c1c15f`, which carries steps 0
to 6 and the owner's decisions on step 6 bar the résumé after review,
which is built and awaits its merge. Step 6 awaits the owner's live
review. Counts are from the
database and `build-stats.json` on this date.

**Built and live.**

- **The site and the JD reviewer.** The reviewer has run since
  2026-09-22: a posting in, structured requirements out, each judged
  against the owner's corpus by a local model, the score computed in
  code, every decision logged with its inputs, and a golden set the
  owner graded for before-and-after evaluation (`evaluation.md`,
  `llm-tuning-log.md`). 118 submissions and 1,761 decision-log rows so
  far, all for one candidate.
- **The framework's Phase 0, steps 0 to 6 of 9** (ADR 0032, `13` §2):
  requirement, judgment, evidence, match record, clarification and
  catalog as versioned rows with `candidate_id` and `tenant_id` from
  birth (migrations 00066 to 00069, package `hiring`); the pipeline
  writes the old representation and the new and a view proves they
  agree; the `kw1` keyword baseline beside every record. Step 4
  (2026-11-10): judge v13 emits the framework's vocabulary, the code
  rules compare what it reports, judgment rows are written per run and
  the match record carries three numbers beside the v2 score the gate
  still reads. Step 5 (2026-11-10): the requirement is an editable
  row. The extractor types it and splits out the posting's
  alternatives; thresholds, an evidence standard and the catalog's
  equivalents are added in code; a person on the hiring side corrects
  it with a reason, each save a new version, and releases it; the
  judge is shown the row and every judgment names the version it
  judged. Requirements are classed required, preferred or ideal, and a
  years or credential item counts only once what is accepted in its
  place is written down (FR-HF-23). 14 of the 24 FR-HF rows are
  shipped or partially shipped. Step 6 (2026-11-12): a finished review
  is read as a match record in the form the pilot's stimulus mocks,
  by the member who submitted the posting and by the owner, from one
  component; a person on the hiring side can override a verdict with a
  reason and a role, the three numbers follow the override and the
  model's verdict stays beside it for the agreement measures. 16 of
  the 24 FR-HF rows are now shipped or partially shipped. The
  golden-set run that settles the gate (D-32) and the hand-over are
  step 4's exit, still open; step 5's editor and step 6's record and
  override await the owner's live review.
- **The employer-side pilot on the site** (FSD §5.16, 18 rows shipped;
  `15`): whitelisted accounts with a pilot flag, a pilot-only
  experience (consent, the survey as versioned data, the match-record
  stimulus and the five-stage company workflow with comments, finish),
  and the admin surface with exports. The participant path and the
  admin pages were approved live by the owner on 2026-11-09 and
  2026-11-10. Five live steps remain (`15` §11 rows 18 to 22). The
  recruitment post went up on LinkedIn on 2026-11-10 with its image
  (`07` §2.1, §2.4) and the first two volunteers were whitelisted the
  same day with the pilot flag; no one has registered yet. Each
  receives the message in `07` §2.5.
- **Platform seams.** A `tenants` table and `tenant_id` on the
  data-layer tables (ADR 0029, attribution only); an event stream of
  85 named events, an audit log for recorded admin operations with the
  rule that a declared control is an enforced one (ADR 0031), and the
  decision log.
- **Engineering posture.** 232 requirements in the FSD, 167 shipped,
  counted by script and checked in CI; 178 RPCs under a `buf` breaking
  check with generated docs; 77 migrations replayed in CI; 11 ADRs;
  467 Go, 91 web and 30 Python tests; 15 CI checks including
  govulncheck, pip-audit, pnpm audit, trivy, CodeQL for all three
  languages and dependabot; nightly backups with a weekly automated
  restore drill; a rollout script with a 33-point live check and a
  one-command rollback.

**Not built, stated plainly.** Single sign-on and directory
provisioning; member self-service (change email or password, close
account, export held data: FR-AUTH-11, FR-HIST-05); the 24-month
retention job (FR-HIST-06); activity consent at registration
(FR-AUTH-10); row-level security; any second candidate; an employer
ranking view across candidates; employer-side outcome links; the
hold-state entity; the clarification loop; catalog write-back; a
hosted-model option. Each is placed in §5 or §6.

---

## 5. The path

Sequence, not schedule. Each stage ends at its gate, and the gate is
written here so it can be checked rather than felt. The pilot runs in
calendar weeks because volunteers are scarce and one-shot
(`roadmap-decision-test.md` says why); everything else is ordered.

| Stage | What | Gate (the stage is over when) | Depends on |
|---|---|---|---|
| **A. The pilot** (now) | Post; whitelist the ten as they volunteer; the owner's remaining live steps (`15` §11 rows 18 to 22); week two on the site; exports to the private folder; the calls; analysis; survey v1.0; workflow content revision 4 | The four items the pilot exists to close are decided and written into `01` §5.6 and `09` §4: C-14 slate cuts, K-04 pre-application gap view, K-09 dispute, the hold-state priority. The pilot summary is in the private repository. | Volunteers from the post |
| **B. Phase 0 complete** | `13` §2 steps 4 to 9: judge v13; requirements editable before judging; match record viewer v2; one clarification round with the owner as candidate; catalog write-back; the exit reading | A recruiter or HR participant identifies strong coverage from three real records alone (`13` decision 11); the keyword comparison is reported for the golden set and three new postings; the exit note is written | Step 4's overnight evaluation (the box must be quiet); A for the exit reader |
| **C. Phase 1, the first employer** | The P5 decisions first (confidential material in the corpus, the baseline design, the throughput path, K-04). Then a design-partner employer: real postings, real candidates with consent, the record as the primary screening artifact, an employer ranking view, the recruiter clarification stage, hold states, stage and 30/90-day outcome collection. Person-level data rights and the threat model (§6 rows 3, 5, 8, 12) are built here because real third parties arrive here | Measured false-negative reduction against the keyword baseline on one employer's pool; the employer's recruiters use it unaided; outcome rows are arriving; the funding case is written from this evidence | B; the P5 decisions; a design partner the owner finds |
| **D. The platform** | Funded. Tenancy machinery (row-level security, host resolution, per-tenant settings); identity (OIDC or SAML single sign-on, SCIM, workflow roles); per-tenant data lifecycle; integration with applicant tracking and HR systems; a hosted-model option with cross-candidate batching; the compliance programme; the private platform repository (`backlog.md` §1b) | A second tenant onboarded without a code change; an external security review passed; the compliance artefacts for one jurisdiction produced | Funding; C |
| **E. Exit readiness** | The evidence pack (pilot, Phase 1 outcomes, calibration history); the data room (FSD, ADRs, SERVICES, runbooks, reviews, licence position); customer references; the IP position | A buyer's diligence is answerable without the owner in the room (G4) | D |

**Deliberately not before the gate.** Billing, support, a second
candidate's documents, pool ranking at scale, a bigger server, a
marketing site and a brand decision all wait for the stage that needs
them. The September direction (`backlog.md` §1b) stands: depth on one
corpus and evidence that it works, before breadth.

---

## 6. Enterprise readiness

What a buyer or an enterprise customer of the ADP and Workday class
asks, where this project stands today, what closes the gap, and at
which stage. Rows move to "in place" when the work lands; the open
items are mirrored in `backlog.md` §1c.

| # | Area | What they expect | Today | What closes it | Stage |
|---|---|---|---|---|---|
| 1 | Tenant isolation | Isolation enforced by the database, not by a condition a handler might forget | Attribution only: `tenants`, `tenant_id` on the data-layer tables, default tenant 1 (ADR 0029). The request-serving tables (`users`, `jd_submissions`, `corpus_documents`) are not tenant-keyed; no row-level security | Row-level security keyed on `tenant_id`; retrofit the core tables; tenant resolution from the host | D |
| 2 | Identity and access | Single sign-on (OIDC, SAML), SCIM provisioning, role-based access, a second factor for privileged users, session controls | Email and password with verification, breached-password check, opaque HttpOnly sessions with a 30-day cap, whitelist with TTL, two roles (member, admin), an admin allow-list. The second factor was cancelled on 2026-10-09 for the single-admin site (FR-AUTH-12). A one-time agent login exists (FR-AUTH-20) | OIDC and SAML; SCIM; roles that follow the workflow (HR, hiring manager, recruiter, candidate); the second factor revisited for a multi-admin tenant | D |
| 3 | Data lifecycle | Consent, export, deletion and retention, per person and per tenant | Consent on the pilot; admin deletion of a member (FR-ADM-25); the pilot's admin export; events anonymised daily. Not built: consent to activity recording at registration (FR-AUTH-10), member self-service (FR-AUTH-11), member export (FR-HIST-05), the 24-month retention job (FR-HIST-06) | Build the four FSD rows; candidate control of evidence depth and audience (`01` §4); per-tenant retention settings | C (person), D (tenant) |
| 4 | Audit trail | Who did what and when, immutable, exportable | `audit_log` for recorded admin operations, with declared controls enforced (ADR 0031); 85 named product events; `decision_log` with the model's full inputs; immutable `jd_runs` | Audit export to a customer's log system; a retention rule for audit rows | D |
| 5 | Security engineering | Scanning, secrets hygiene, a written threat model, incident response, external testing, a recognised attestation | In CI: govulncheck, pip-audit, pnpm audit, trivy (filesystem and config), CodeQL for Go, JavaScript and Python, dependabot with tested dependency PRs. A content security policy; rate limits on sign-in and submissions; a submitted posting cannot reach private material (`backlog.md` 5a); no secrets in the repository; nightly backups with a weekly restore drill; a host firewall script | A written threat model and an incident-response runbook; a software bill of materials; an external penetration test; a SOC 2 readiness assessment | C (threat model, runbook), D (test, attestation) |
| 6 | API and contracts | Versioned, documented, backward-compatible APIs; events out | 170 RPCs in `career.v1` with a `buf` breaking check, generated documentation and a comment on every field | A public API versioning policy; per-tenant service accounts and keys; webhooks for match-record and stage events | D |
| 7 | Integration surface | Connectors to applicant tracking and HR systems; standard schemas | None. Postings arrive by paste; evidence is one person's corpus | Posting ingestion from the systems the buyer runs; candidate evidence import with consent; published JSON schemas for requirement and match record (`04` already carries them) | D |
| 8 | AI governance and employment law | Explainable decisions, human oversight, bias testing, model documentation, candidate notice. New York City Local Law 144 requires an annual bias audit and notices for automated employment decision tools (in force since 2023). The EU AI Act lists employment uses as high-risk; the 2026 Digital Omnibus deferred the high-risk obligations to December 2027 according to secondary sources. Colorado replaced its 2024 AI Act with SB 26-189, effective 2027-01-01, also per secondary sources. U.S. disparate-impact law applies to any selection procedure. Counsel confirms the list before Stage C | Every judgment logged with its inputs; the score computed in code; the model reports and code decides; the human override kept beside the model's verdict; the golden set and agreement metrics; schema, vocabulary, formula, prompt and rule versions on every record; behavioural requirements not scored | A disparate-impact analysis over match records (which needs a lawful design for collecting protected-class data); a model card per judge version; candidate-facing notice and explanation; a bias-audit procedure; legal review of the jurisdiction list | C (design), D (programme) |
| 9 | Reliability and operations | Service-level objectives, alerting, high availability, disaster recovery | One CPX41; a rollout script with a 33-point live check and rollback; the quiet-window rule; backups verified by restore; metrics defined once as SQL views; seven scheduled jobs | Monitoring and alerting; written objectives; a second node or managed Postgres when funded; a disaster-recovery runbook | D |
| 10 | Scale and cost | Many candidates per posting at a predictable cost | One judge call per requirement and candidate, 15 to 30 minutes per posting on the local 4b model; the shape batches across candidates later (`13` decision 6) | A hosted-model option behind the same contract; cross-candidate batching; a cheap first-pass judge with escalation (`00-todo` P5) | C decides, D builds |
| 11 | Accessibility | WCAG 2.1 AA | The NFR-A11Y rows; a no-JavaScript check in CI and against production; design tokens checked | A formal audit before the first enterprise customer | D |
| 12 | Intellectual property and licence | Exclusive ownership of what is sold | The code is MIT-licensed and public (FSD A-05); the content is licensed separately; the data, the catalog content, the labels, the adapters and all participant and candidate material are not in the public repository. Commits are co-authored with an AI assistant | Decide before Stage C where new framework code lives (the private platform repository in `backlog.md` §1b), since MIT code is not exclusive; confirm with counsel the ownership position on AI-assisted code; settle the brand (the "dossier" question in §1b) | C |
| 13 | Documentation | A spec, a decision record, runbooks | The FSD with counts checked in CI, 11 ADRs, `SERVICES.md`, the deploy and backup runbooks, the events and evaluation documents, the tuning log, this roadmap | Keep it current; add the threat model and the incident runbook (row 5) | ongoing |

---

## 7. Risks

- **The model under a richer schema.** Twelve basis codes, eight
  uncertainty codes and a facts object are more for a 4b model to get
  right than three words were. Mitigated by field order before free
  text, consistency rules that reject and retry, the golden set before
  and after, and the derived three-way verdict that keeps the owner's
  graded history comparable (`13` §4).
- **Volunteers are scarce and one-shot.** A participant who finds the
  pilot broken is not coming back. Mitigated by the live-review rule
  and by pre-running every flow locally before it reaches them.
- **Third-party confidential material.** The first second candidate
  brings someone else's documents into the corpus. Decided before Stage
  C, not discovered during it (`00-todo` P5).
- **One operator.** The owner is developer, operator and reviewer. The
  documentation discipline (AGENTS.md, FSD, ADRs, runbooks) exists so
  that a second person can take the work over; it is also what a buyer
  reads.
- **Regulatory drift.** The employment-AI rules in §6 row 8 changed
  twice in 2026 alone. The design, a per-decision record with evidence
  and a human override, is the stable answer; the jurisdiction list is
  checked with counsel at each stage gate.
- **The incumbents' gate.** Integration with an applicant tracking
  system is on the vendor's terms. Mitigated by designing the record
  to stand alone (paste in, record out) so the first employer does not
  need the connector.

---

## 8. Keeping this document

- §4 is rewritten, and dated, at every stage gate in §5 and whenever a
  figure in it stops being true.
- §6 moves a row to "in place" in the same pull request as the work;
  `backlog.md` §1c drops the matching item.
- A new goal, constraint or stage is a change to §1 or §5 and is
  recorded below with the owner's words where they exist.
- Nothing in this file names a participant, a candidate, an employer
  or a credential.

| Date | Change |
|---|---|
| 2026-11-10 | Opened at the owner's request, from his statement of the end goal the same day. Goals, problem, thesis, current state on `0ed804ba9d9a`, the five stages, the enterprise-readiness table and the risks written. Backlog §1c opened to mirror the open items. |
| 2026-11-10, later | §4: stage A is under way; the post is up and the first two participants are on the whitelist. Production on `15007b6fb58f`. |
| 2026-11-13, step 6 decisions | The owner answered seven questions on what step 6 left open: a recruiter is shown the model's verdicts only, the admin record shows the fit after review beside the run's own, overrides carry to the next run while the requirement is unchanged, a résumé after review is the owner's alone and by request, and the pilot keeps its approved sample for this wave. All four behaviours are built, the résumé after review last; 178 RPCs, 77 migrations. Production on `38be35c1c15f`. |
| 2026-11-12, step 6 | §4: Phase 0 steps 0 to 6 of 9. The match record is read by a person in its own form and a verdict can be overridden; 16 of the 24 FR-HF rows shipped or partially shipped; 168 requirements shipped, 177 RPCs, 75 migrations. The production line now names `1627cbd5a609`, which it had not since `0ed804ba9d9a`. |
| 2026-11-10, step 5 | §4: Phase 0 steps 0 to 5 of 9. The requirement is an editable, versioned row with classes; 14 of the 24 FR-HF rows shipped or partially shipped; the engineering counts moved with it (167 requirements shipped, 175 RPCs, 74 migrations). |
