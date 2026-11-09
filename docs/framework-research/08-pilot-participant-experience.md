# Pilot Participant Experience: What the Ten See and What They Can Change

Companion to `06-employer-side-discovery-research-plan.md` and `07-pilot-plan-and-recruitment-post.md`. This document walks the pilot from the participant's seat: each touchpoint, what is on their screen, what they are asked to do, what they can influence, and how that influence is captured.

**Status.** Written 2026-11-08. The materials in §4 are in `11-pilot-materials.md`. Repository copy ahead of project (2026-11-08): this status line added; the call gained a third part the same day at the owner's instruction, the company workflow review (`11` §7), and §1 and §2.3 say so.

**A word on words.** The ten are recruiters, HR staff and hiring managers who test the research instrument. In all materials they are **participants**. "Candidate" is reserved for job seekers whose evidence the system judges; using it for the pilot group would confuse every conversation about the mocked match record, which has a candidate in it.

---

## 1. The four pieces, in the order a participant meets them

| # | Piece | Participant's time | Channel | What it tests |
|---|---|---|---|---|
| 1 | Invitation and consent | 2 min | LinkedIn post or direct message → link | Whether the ask is clear and the anonymity promise is believed |
| 2 | Survey v0.1 | ≤10 min | The pilot area on rogerhenley.dev, phone or desktop (`15`) | Wording, length, option fit, whether the concept paragraph communicates without a picture |
| 3 | Cognitive-interview call | About 30 min: 15 on the survey, 5 on the stimulus, 8 to 10 on the company workflow | Video or phone, screen-share for the stimulus and the workflow chart | Where the survey failed them; then, separately, first reaction to the mocked match record; then where the company hiring workflow matches their own and where it does not |
| 4 | Close-out | 1 min, then async | Email | Whether they want the summary; what changed because of them |

The order is deliberate: the survey's Section E asks them to picture the tool from one paragraph **before** they see the mocked record, and the call then shows the record. The gap between what they pictured and what they see is the measurement.

---

## 2. Touchpoint by touchpoint

### 2.1 Invitation and consent

**They see.** The pilot post (07 §2.1) or a direct message with the same content, then a one-screen landing page before the survey:

- Who is running it (owner's name; no company)
- What it is for: fixing the wording of a survey about hiring-process problems before it goes wide
- What is asked: a 10-minute survey and a 15-minute call about the survey
- What is kept: answers without name or employer; the call is not recorded unless they agree; notes are kept
- What they get: the summarised results, and a note on what changed because of their feedback
- A single consent checkbox and a "start" button

**They can affect.** Whether to proceed; whether the call is recorded.

**Captured.** Consent timestamp; recording preference. Nothing else.

### 2.2 Survey v0.1

**They see.** The instrument in 06 §3, sections A to F, one section per screen, progress shown. Three pilot-only additions (the third added 2026-11-09 at the owner's instruction after his own run: a notes box on every question, for whatever the participant wants to say while answering, stored on the answer's row and read by the owner):

- A visible **"flag this question"** control on every item (a small checkbox: "This was unclear or didn't fit me"). Flags are the cheapest cognitive-interview data and they are collected while the confusion is fresh, not reconstructed on the call.
- A closing pilot-only item: *"Roughly how long did that take?"* and *"Which question did you most want to skip?"*

Section E's concept paragraph is shown as text only. No image, no link to the mocked record.

**They can affect.**

| Survey element | How they affect it | Through |
|---|---|---|
| The 20-item problem list (C1) | Merge, reword, drop or add items | Flags on items; C4 free text; call questions on duplicates and meaning |
| Answer options in A, C3, D | Change scales or add options | Flags; call question "which options didn't fit" |
| Process-fact items D4, D6, D7 | Keep, reword, or add "why don't you know" follow-ups | Whether they answer "don't know"; call question "could you actually answer this" |
| Concept paragraph (E) | Rewrite | Call question "what did you picture", compared with their reaction to the stimulus |
| Length and order | Cut C3 to top-3; reorder sections | Self-reported time; "most wanted to skip" |
| The route by role (A1) | Fix branching | Whether an HM saw a recruiter question or vice versa (they will say so) |

**They cannot affect.** The research questions themselves, the two-group analysis design, or the decision to ask open-ended questions before the list. Those are the study, not the instrument; a participant who argues with them is giving content feedback, which is noted and set aside.

**Captured.** Responses tagged `pilot=true, survey_version=0.1`; per-item flags; time and skip answers. Pilot rows never merge into the main wave.

### 2.3 Cognitive-interview call

Fifteen minutes on the survey, then five on the stimulus, then eight to ten on the company hiring workflow (Part C, `11` §7), in that order. The interviewer has the participant's own survey responses open and refers to them ("you flagged item 11").

**Part A — the survey (15 min). They see** nothing new; they talk. Prompts from 07 §1, in this order:

1. Which questions did you read twice? (start with their flags)
2. Which options didn't fit? Where did you want to say "it depends"?
3. On the list: any two items that were the same thing? Any you didn't understand? Anything missing that you've lived with?
4. D4 / D6 / D7: did you know the answer, or guess? If you didn't know, why not — is that number anywhere in your organisation?
5. The concept paragraph: in your own words, what did you picture? What would it look like on your screen?

**Part B — the stimulus (5 min). They see** the mocked match record (07 §3), screen-shared, read-only. The interviewer does not explain it first; they are asked to read it as if it arrived at work and talk aloud. Then three questions only:

1. Is this what you pictured from the paragraph? What is different?
2. Which row would you check first, and why?
3. *(Recruiter)* Could you rank ten of these for a role outside your field? *(HM)* Would you touch the override button, or read the résumé instead?

On the site (`15`), where the record is read before the call, its general box asks three questions instead of "which row first": which rows they would verify themselves before relying on the record and why those; what they could not understand without someone explaining it; whether they would use Audit or Override on any row and what they would do with it. The call's question 2 is read against those answers.

The stimulus is shown last so that Section E has already captured an unprimed reading of the paragraph. Their reaction to the record itself is **not** a pilot finding about the survey; it is early input to the viewer design and is filed under that heading.

**They can affect.**

- The concept paragraph (if the picture and the record diverge, the paragraph is wrong)
- The viewer's requirements backlog: which row they checked first, what they wanted to click, what they distrusted. These become candidate requirements for the match-record viewer, tagged `source=pilot-stimulus`, not accepted requirements.
- The interview guide for the main wave: if a prompt produced nothing from most of the ten, it is cut or reworded.

**They cannot affect.** The match record's verdicts, structure or vocabulary during the call. If a participant says "that should be partial, not unresolved", that is recorded verbatim as a viewer reaction, and the interviewer does not debate the taxonomy. The pilot is not the forum for redesigning the record; the main-wave interviews and the eventual Phase 0 trials are.

**Captured.** Interviewer notes on a fixed template: one block per prompt, verbatim phrases marked as quotes, the participant's own flags cross-referenced. Recording only if consented at 2.1. Filed as `pilot-interview-NN.md` with role (recruiter / HM), roles-hired type, and organisation size band from A2–A4 — no name.

### 2.4 Close-out

**They see.** A thank-you email within a day: a line on what happens next (changes made, survey goes wide in week 3), whether they asked for the summary, and an invitation to one more 5-minute read of the revised questions if they are willing (optional; three volunteers is plenty).

Four to six weeks later, those who asked for the summary receive it, plus a short **"what changed because of the pilot"** list: each instrument change with the signal that caused it (07 §1 table). Participants can see their effect without being identified.

**They can affect.** Whether they re-read the revised survey; whether they are contacted for the main-wave interviews (opt-in at F1).

---

## 3. Influence map (summary)

| Participant input | What it changes | Who decides | Where the decision is recorded |
|---|---|---|---|
| Item flags, call answers on wording | Survey item text, options, order, length | Owner, after all ten | Instrument change log (`survey v0.1 → v1.0`) |
| C4 additions, "what's missing" on the call | New problem-list items | Owner; added if ≥2 of 10 raise the same thing | Instrument change log |
| "Don't know" rates on D4–D7 | Adds "why don't you know" follow-ups; **does not** remove the items | Fixed rule in 07 §1 | Instrument change log |
| What they pictured vs the record | Concept paragraph rewrite | Owner | Instrument change log |
| First-row-checked, trust and override reactions | Viewer requirements backlog (proposed, not accepted) | Owner, later, with main-wave data | `viewer-requirements-backlog.md` (to be created when the first entries exist) |
| Objections to the verdict vocabulary | Nothing in the pilot; logged for the taxonomy's open-items list | Deferred | `03-…` §3.4 open items, if a pattern appears |

---

## 4. What the owner needs before week 1

1. **Survey platform** with: section-per-screen, per-item optional checkbox (the flag), randomised option order for C1, role-based branching from A1, and export to CSV. Google Forms meets all but randomised order within a checkbox grid; Tally and Typeform meet all. Decision open; Google Forms is acceptable for the pilot if the C1 order is fixed and the main wave moves to a tool that randomises.
2. **Landing/consent page** text (2.1) — can be the form's first screen.
3. **Interview note template** (2.3) — one page, prompts pre-printed, a quote column.
4. **Instrument change log** — a markdown file started at `survey v0.1` with the date fielded.
5. **The stimulus link** tested on a screen-share at phone and laptop widths; the record is a fluid page but the expanded question block on r6 should be checked at narrow width.
6. **Ten names**, five and five, with at least three technical/manufacturing hirers, none from current employers.

---

## 5. What the pilot does not do, stated so nobody expects it

- It does not produce rankings. Ten people cannot rank twenty problems meaningfully; their C1 answers are read only for item confusion.
- It does not validate the thesis. D6/D7 answers from ten people are a check that the questions are answerable, not evidence either way.
- It does not test the match record's design. Five minutes of first reaction is enough to catch a paragraph that mis-describes it and to seed a backlog; usability testing of the viewer is Phase 0 work with a working prototype.
