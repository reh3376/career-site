# Hiring_prod — To-do (prioritised)

Open work items for the design effort, in the order they will be worked. Each item names what "done" looks like and which doc it feeds. Items are removed when done; decisions made along the way go into the doc they affect, not here.

Last updated 2026-11-08 (P1 complete; handoff point). Repository copy ahead of project (2026-11-08): the PR #382 item under P6 removed after the PR merged.

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

## P2 — Pilot materials (from `08-pilot-participant-experience.md` §4) — **next; handoff starts here**

- Consent / landing screen text.
- One-page interviewer note template (prompt blocks, quote column), including the [WF] probes.
- Instrument change log started at `survey v0.1` (first entry: the [WF] additions, 2026-11-08).
- Survey platform decision (Google Forms acceptable for the pilot with fixed C1 order; randomising tool for the main wave).
- Stimulus checked on screen-share at phone and laptop widths.

## P3 — Pilot recruiting and fielding (owner)

- Ten names: five recruiters/HR, five hiring managers, ≥3 technical/manufacturing hirers, none from current employers.
- Post the pilot version of the recruitment post; schedule calls.
- Run week 2; revise to `survey v1.0`; field wide week 3.

## P4 — Schema follow-through (parallel with P3)

- Proto and SQL rendering of the v1 entities; column vs jsonb decisions.
- Add `asked_by` (system / recruiter) to the clarification item (workflow Q3).
- Concept table to replace free `kind:slug` refs; decide on O*NET / ESCO as external seed.
- Curation model for the shared base catalog (tenant 0) and tenant inheritance.
- Clarification trigger and round policy (revisit after pilot and main-wave interviews).
- Hold-state entity: revisit date, owner, escalation target, escalation event (workflow Q7; priority after pilot).

## P5 — Decisions needed before Phase 1 (from the gap analysis)

- Third-party confidential material entering the corpus when a second candidate exists (career-site backlog §1b).
- Baseline plan for false-negative claims: keyword filter run in parallel on the same pool.
- Throughput path for pool ranking (hosted model option, cross-candidate batching, or cheap first-pass judge with escalation).
- K-04 pre-application gap view: build or not (owner leans yes, cost high; await candidate-side study).

## P6 — Housekeeping

- Keep `career-site/docs/framework-research/` in step with the project docs; re-copy after material changes.
