# Instrument Change Log: Employer-Side Discovery Survey and Interview Guide

**Status.** Started 2026-11-08 at `survey v0.1`, not yet fielded. Repository copy ahead of project (2026-11-08): new document.

One row per change to the survey instrument or the interview guide in `06-employer-side-discovery-research-plan.md`, with the signal that caused it. This is the same discipline the prototype keeps for prompt versions in `docs/llm-tuning-log.md`: a version is a thing you can name, and a change you cannot explain afterwards did not need making. Pilot responses are tagged with the version they were collected under and never merge into the main wave (`07` §1).

---

## 1. Versions

| Version | Date | Fielded to | State |
|---|---|---|---|
| v0.1 | 2026-11-08 | Nobody yet. Pilot, n≈10, planned for week 2 (`07` §1) | Current |
| v1.0 | — | Main wave, after the pilot's revisions are applied as one batch | Not yet cut |

## 2. Changes

| # | Version | Date | Section / item | Change | Signal (why) | Decided by |
|---|---|---|---|---|---|---|
| 1 | v0.1 | 2026-11-08 | §1 research questions | Added RQ6 (how holds and deferrals behave) and RQ7 (slate cuts before interview; candidate disputes) | Workflow review Q4, Q6, Q7 (`09` §4) | Owner |
| 2 | v0.1 | 2026-11-08 | D4 | Made required in both branches | Q4: whether C-14 is a real decision point depends on the HM decline-without-interview share | Owner |
| 3 | v0.1 | 2026-11-08 | D8 | New item: what usually happens to a requisition or candidate on hold | Q7: the owner's experience that holds rarely resurface; sets the hold-state requirement's priority | Owner |
| 4 | v0.1 | 2026-11-08 | Interview guide, current process (HM) | Added the slate-cut probe ("the last slate a recruiter sent you...") | Q4 | Owner |
| 5 | v0.1 | 2026-11-08 | Interview guide, problems deep dive (both) | Added the hold probe ("a requisition or a candidate that was put on hold...") | Q7 | Owner |
| 6 | v0.1 | 2026-11-08 | Interview guide, concept reaction (both) | Added the dispute probe ("if a rejected candidate could contest one specific judgment...") | Q6 (K-09) | Owner |
| 7 | v0.1 | 2026-11-08 | Interview guide, concept reaction (recruiter) | Added the pre-application gap-view probe | Q5 (K-04), employer-side angle only | Owner |
| 8 | v0.1 | 2026-11-08 | Pilot-only additions | Per-item "flag this question" control; closing items for elapsed time and the question most wanted to skip | `08` §2.2: flags are the cheapest cognitive-interview data and are collected while the confusion is fresh | Owner |
| 10 | v0.1 | 2026-11-08 | Interview guide, new Part C (both branches) | The company hiring workflow (C-01 to C-26) shown for review and comment after the stimulus, six prompts, eight to ten minutes; call length about thirty minutes; consent text and recruitment post updated to match | Owner's instruction 2026-11-08: the workflows are the process model the framework is classified against and the ten participants' corrections are wanted before the main wave (`11` §7) | Owner |
| 11 | v0.1 | 2026-11-08 | Whole instrument (channel) | The survey runs on rogerhenley.dev behind a per-participant flag, rendered from a versioned JSON definition; the per-item flag is a true per-item control and C3 is a grid over the items selected in C1, so row 9's two renderings are withdrawn | Owner's decision 2026-11-08 (late): participants create whitelisted accounts and the whole pilot runs on-site (`15` §0) | Owner |
| 12 | v0.1 | 2026-11-09 | Every item (pilot-only control) | A free notes box beside every question, stored with the answer (`pilot_answers.note`); item text unchanged | Owner's instruction after completing the pilot himself: the flag is the cheap signal, the note is the rich one, and both belong on the answer's row | Owner |
| 13 | v0.1 | 2026-11-09 | Stimulus v1 to v1.1 (the match record read on the site before the call) | Every part of a row labelled; a definitions panel; a specific basis on every row; the two-run story on r1 and a header line that this is the second run; follow-up and inclusion lines; r10 as human judgement required; the record's status inconclusive while a question is open; three general questions in place of one | The owner's own run as a participant and the conversation that followed (`15` §14); his ten record comments are in `pilot_comments` | Owner |
| 14 | v0.1 | 2026-11-09 | Company workflow v2 to v3 (how it is read on the site before the call) | One stage per screen with a small diagram each and an overview of the five stages; every step in plain language; a three-way answer per step; an open outcomes question at each decision; stage questions and missing steps per stage; two closing questions | The owner's own run as a participant (nineteen comments: too complex, an unreadable chart, undefined terms, unclear questions, the wrong question at decisions) and the conversation that followed (`15` §14) | Owner |
| 15 | v0.1 | 2026-11-10 | Company workflow v3, amended | A notes box on every step whatever the answer; each stage diagram starts and ends with what the stage receives and hands on; the stage and missing-step questions say to leave the box empty if there is nothing | The owner's review of revision 3 on the site, all five stages (`15` §11 step 11): no box on steps answered "like this" or "does not happen"; approval named as missing from stage B when it is stage A's last step; "No" and "NA" typed into boxes that should have been left empty | Owner |
| 9 | v0.1 | 2026-11-08 | C3 (rendering), per-item flag (rendering) — **withdrawn by row 11** | C3 asked for the C2 first-fix pick plus up to two more the respondent names, each with a frequency and a cost dropdown, instead of a grid over every selected item; the per-item flag rendered as one end-of-section checkbox question listing the section's item ids | Google Forms cannot filter a grid by earlier answers and has no per-question flag (`11` §3). Both revert to the spec in v1.0 on Tally if the pilot's time check allows | Agent, under the owner's delegation of 2026-11-08 |

## 3. Rules

- A change gets a row **before** it is fielded, never reconstructed afterwards.
- The pilot's changes are applied as one batch to produce v1.0. Each row names the pilot signal from the decision table in `07` §1 (duplicates called by ≥3, an item nobody selects and ≥3 call unclear, a problem raised by ≥2 in B1/B2/C4, median time over 12 minutes, the concept paragraph pictured as "an ATS score").
- A change to the problem list (C1) says whether it merges, rewords, drops or adds an item, and gives the item numbers before and after, so a C1 answer collected under v0.1 can still be read.
- Nothing is edited in place after fielding. v1.0 is a new version in the export, not a rewrite of v0.1's rows.
- "Don't know" rates on D4 to D7 add a "why don't you know" follow-up; they never remove the item (`07` §1, fixed rule).
