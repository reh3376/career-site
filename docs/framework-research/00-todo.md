# Hiring_prod — To-do (prioritised)

Open work items for the design effort, in the order they will be worked. Each item names what "done" looks like and which doc it feeds. Items are removed when done; decisions made along the way go into the doc they affect, not here.

Last updated 2026-11-08, late (P1 and P2 complete; the build order delegated to the agent and recorded in `13-phase-0-build-plan.md`). Repository copy ahead of project (2026-11-08): P2 marked complete, P4 and P5 re-pointed at `13`.

---

## 0. Why this order

1. **Workflows first.** Cheap, they classify every framework component, and they change survey Section D and the interview guide. Done before the pilot so the instrument is revised once.
2. **Pilot materials second.** Short; unblocks recruiting, which has lead time.
3. **Field the pilot** while the schema follow-through proceeds in parallel.
4. **Pre-Phase-1 decisions** before Phase 1, not before the pilot.
5. Housekeeping as it comes up.

---

## P1 — Hiring workflows, both perspectives — **complete 2026-11-08**

Done: node lists (`09-hiring-workflows-draft.md`), review record, pilot additions applied to `06`, diagrams (`10`, `diagrams/`), and §5.6 process model folded into `01-problem-definition-and-framework.md`. Approved by the owner.

Carried forward, not P1 work: after the pilot, close C-14 (slate cuts), K-04 (pre-application gap view), K-09 (dispute), and set the priority of the hold-state requirement; then update `01` §5.6 and `09` §4.

## P2 — Pilot materials — **complete 2026-11-08**

Done: `11-pilot-materials.md` (consent and landing text, the one-page interviewer note template with the [WF] probes, the platform decision, the stimulus check), `12-instrument-change-log.md` started at `survey v0.1`, and the private folder `docs/personal/framework-research/` for anything a participant says.

## P2b — Pilot on-site build (`15-pilot-on-site.md` §8) — **next; before P3 and before Phase 0 step 3**

- PL-1: pilot flag on whitelist entries and members, `Me.pilot_approved`, the admin toggle, forgot-password by 5-digit emailed code replacing the link; migration 00064. **Built 2026-11-08.**
- PL-2: pilot tables (00065), `PilotService`, instrument v0.1 and the stimulus as embedded JSON, events. **Built 2026-11-08.**
- PL-3: the `/pilot` pages (consent, survey, record, workflow, done), the company-workflow SVG, `/admin/pilot`, whitelist checkbox and member toggle in the web; live pass. **Built 2026-11-08; deployed 2026-11-09 (`ba6fdc6e04c3`), outside checks pass; the owner's live review (`15` §11, 22 steps) is the completion criterion and is under way (2026-11-09: steps 1 to 8 reviewed; the flow made sequential, the owner's notes and judgement added in the database, `15` §12; participants given a pilot-only experience, `15` §13; the admin account delete and the optional +pilot switch added; third run complete, survey approved, notes box added; the record rebuilt as stimulus v1.1 from the owner's approved proposals and his three framework answers written as FR-HF-22 to 24; the workflow section to be discussed, `15` §14).** An admin delete of a session (FR-PLT-15) added 2026-11-09 so that run can be removed before the ten start.
- PL-4: field it (owner whitelists the ten; exports to the private folder after week 2).

## P3 — Pilot recruiting and fielding (owner)

- Ten names: five recruiters/HR, five hiring managers, ≥3 technical/manufacturing hirers, none from current employers. Confirm each one's email address and add it to the whitelist with the pilot flag (`15` §0 decision 1).
- Post the pilot version of the recruitment post (rewritten for the on-site pilot 2026-11-09, `07` §2.1, first comment names the site); schedule calls.
- Run week 2; revise to `survey v1.0`; field wide week 3.

## P4 — Schema follow-through (parallel with P3) — **steps 2 and 3 of `13-phase-0-build-plan.md` §2, built 2026-11-08 and 2026-11-09 (migrations 00066 to 00069, package `hiring`, the dual write); items 1, 2 and 4 landed as columns and tables, 3, 5 and 6 stay open**

- Proto and SQL rendering of the v1 entities; column vs jsonb decisions.
- Add `asked_by` (system / recruiter) to the clarification item (workflow Q3).
- Concept table to replace free `kind:slug` refs; decide on O*NET / ESCO as external seed.
- Curation model for the shared base catalog (tenant 0) and tenant inheritance.
- Clarification trigger and round policy (revisit after pilot and main-wave interviews).
- Hold-state entity: revisit date, owner, escalation target, escalation event (workflow Q7; priority after pilot).

## P5 — Decisions needed before Phase 1 (from the gap analysis) — **closed for Phase 0 by `13` §0 decisions 4 to 6; reopen before Phase 1**

- Third-party confidential material entering the corpus when a second candidate exists (career-site backlog §1b).
- Baseline plan for false-negative claims: keyword filter run in parallel on the same pool.
- Throughput path for pool ranking (hosted model option, cross-candidate batching, or cheap first-pass judge with escalation).
- K-04 pre-application gap view: build or not (owner leans yes, cost high; await candidate-side study).

## P6 — Housekeeping

- Keep `career-site/docs/framework-research/` in step with the project docs; re-copy after material changes.
