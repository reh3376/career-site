# Pilot Materials: Consent Text, Interviewer Template, Platform, Stimulus Check

**Status.** Written 2026-11-08; closes P2 in `00-todo.md`. Extended the same day at the owner's instruction: the company hiring workflow is shown to every participant for review and comment (§7), which lengthens the call to about thirty minutes. Everything the participant sees or the interviewer holds, ready to field. The instrument itself stays in `06-employer-side-discovery-research-plan.md`; its version history is `12-instrument-change-log.md`. Repository copy ahead of project (2026-11-08): new document.

Specs these satisfy: `08-pilot-participant-experience.md` §4, items 1 to 5. Item 6, the ten names, is the owner's (P3).

---

## 1. Consent and landing screen

The first screen of the survey form, before Section A. Plain text, no logo, no company. It covers the six things `08` §2.1 requires and nothing else.

> **A short survey about hiring, and fifteen minutes on the survey itself**
>
> Run by Roger Henley, independently. No company is behind this.
>
> **What this is for.** I am designing software that screens job applications by evidence against each requirement rather than by keywords. Before I ask fifty recruiters and hiring managers which hiring problems matter most, I need ten people to try the survey and tell me where its wording fails. That is what you are being asked to do.
>
> **What I am asking of you.** About ten minutes on the survey, then a call of about thirty minutes. Fifteen of those are about the survey: which questions you read twice, which answers did not fit, how long it took. Then I will show you a one-page mock-up for five minutes and ask what you make of it, and a one-page chart of how a company hires, for about ten minutes, and ask where it matches how hiring runs where you work and where it does not. Not what you think of the idea; what the questions did to you, and what the chart gets wrong.
>
> **What is kept.** Your answers, without your name or your employer. Notes from the call, with quotes marked as quotes. The call is not recorded unless you tick the box below. Nothing here asks about any candidate or any named person.
>
> **What you get.** The summarised results when they exist, and a short list of what changed in the survey because of you.
>
> ☐ I understand what is being asked and what is kept, and I want to take part.
> ☐ You may record the call. *(Optional.)*
>
> **[Start]**

Captured from this screen: the consent timestamp and the recording preference. Nothing else, per `08` §2.1.

## 2. Interviewer note template (one page)

One file per participant, `pilot-interview-NN.md`, in the private repository (§5). The interviewer has the participant's own survey responses open and refers to their flags by item. Prompts are pre-printed; the right-hand column is for verbatim phrases only, marked as quotes.

```markdown
# Pilot interview P-NN

| Field | Value |
|---|---|
| Role branch (A1) | recruiter/HR · hiring manager · both |
| Roles hired, mostly (A3) | |
| Organisation size band (A4) | |
| Survey version | v0.1 |
| Date, duration | |
| Recorded | yes · no |
| Items the participant flagged | |

## Part A: the survey (15 min)

| Prompt | Notes | Verbatim |
|---|---|---|
| A1. Which questions did you read twice? (start from their flags) | | |
| A2. Which options didn't fit? Where did you want to say "it depends"? | | |
| A3. The problem list: any two items that were the same thing to you? Any you didn't understand? Anything missing that you've lived with? | | |
| A4. D4 / D6 / D7: did you know the answer or guess? If you didn't know, why not; is that number anywhere in your organisation? | | |
| A5. The concept paragraph: in your own words, what did you picture? What would it look like on your screen? | | |

## [WF] probes (ask in Part A where they fit; mark which were reached)

| Prompt | Branch | Notes | Verbatim |
|---|---|---|---|
| Think of the last slate a recruiter sent you. How many did you decline before interviewing, and why? | HM | | |
| Tell me about a requisition or a candidate that was put on hold. What happened to it, and how did you find out? What would have had to exist for it to come back on its own? | both | | |
| If a rejected candidate could contest one specific judgment with new evidence, would you want to see it? Under what conditions would you re-open the decision? | both | | |
| If applicants could see, before applying, which of your requirements their background does and doesn't cover, would that change the quality or quantity of applications you receive? | recruiter | | |

## Part B: the stimulus (5 min, shown after Part A, read-only, no explanation first)

| Prompt | Notes | Verbatim |
|---|---|---|
| B1. Is this what you pictured from the paragraph? What is different? | | |
| B2. Which row would you check first, and why? | | |
| B3. (Recruiter) Could you rank ten of these for a role outside your field? (HM) Would you touch the override button, or read the résumé instead? | | |

## Part C: the company hiring workflow (8 to 10 min, shown after Part B)

Screen-share the company perspective only (C-01 to C-26). Say nothing
about the framework's components; the chart is shown as "how we think a
hire runs" and the participant corrects it.

| Prompt | Notes | Verbatim |
|---|---|---|
| C1. Walk me along this from C-01. Where does it match how a hire actually runs where you work, and where does it not? | | |
| C2. Which step is missing? Which two are really one step? | | |
| C3. Who owns C-06 (writing the role and its requirements) and C-07 (the sign-off) in your organisation? | | |
| C4. Where do holds actually happen in this picture, and what brings one back? | | |
| C5. At C-12 to C-14, who decides? Does the hiring manager cut people from the slate before interviewing? | | |
| C6. Is there anything on this chart you would not want a candidate to see? | | |

Per node, mark **M** (matches), **D** (differs; say how) or **X** (does
not exist here) in the tally sheet `workflow-review-tally.md` in the
same private folder, one column per participant.

## Close

| Prompt | Notes |
|---|---|
| What did we not ask that we should have? | |
| Who else should we talk to? | |
| Willing to re-read the revised questions (5 min)? | yes · no |

## Filing

Part A and the probes feed the instrument change log. Part B feeds
`viewer-requirements-backlog.md` as proposed items tagged
`source=pilot-stimulus`; an objection to the verdict vocabulary is
recorded here verbatim and not debated (08 §2.3). Part C feeds
`workflow-review-tally.md` and, after all ten, revision 3 of the
company perspective in 09.
```

Rules for filling it (from `08` §2.3): the stimulus is never explained before the participant reads it; a participant arguing with the study design rather than the instrument is noted and set aside; nothing about the match record's verdicts, structure or vocabulary is changed on the call.

## 3. Survey platform

**Pilot: Google Forms. Main wave: Tally.** Decision taken 2026-11-08 (`13` §0 decision 8).

Why two tools: the pilot reads C1 only for item confusion, so option order does not matter there, and Google Forms is already in the owner's hands. The main wave needs randomised option order on C1, role branching and a clean CSV export, which Tally's free tier provides.

How the pilot form is built, section by section, so the instrument in `06` §3 survives the tool:

| Need (`08` §4.1) | Google Forms | Note |
|---|---|---|
| Landing and consent | Section 1: the text in §1 above; consent as a required checkbox; recording as an optional checkbox | |
| Section per screen | One form section per instrument section A to F, plus the pilot close | Progress bar on |
| Role branching from A1 | A1 as multiple choice with "Go to section based on answer": recruiter/HR and HM reach the same B and C, then D with the branch-specific items in separate sub-sections; "Both" routes through both D sub-sections; "None of these" submits | D6 is HM-only, D7 is recruiter-only |
| Per-item flag (`08` §2.2) | Forms has no checkbox beside a question, so each section ends with one checkbox question: "Which of the questions above were unclear or didn't fit you? Tick any", listing the item ids | A deviation from the spec, logged as such; the main wave gets a true per-item flag |
| Fixed C1 order | "Shuffle option order" off on C1 | |
| C3, frequency and cost per selected problem | Forms cannot filter a grid by earlier answers. Rendered as three blocks: the C2 first-fix pick plus up to two more the respondent names, each with a frequency dropdown and a cost dropdown | Logged as change 9 in `12`; the full per-selection grid returns in v1.0 if the pilot's time check allows |
| Pilot close | "Roughly how long did that take?" and "Which question did you most want to skip?" | Pilot only |
| Anonymity | Do not collect email addresses; F1 and F2 are optional text fields inside the form; responses limited to one per device only if that does not require sign-in | |
| Export | Responses to Sheets, then CSV to the private repository as `survey-export-v0.1.csv` | Never into this repository |

Tally for the main wave: the same sections, randomised option order on C1, conditional logic on A1, a per-item flag is not needed (pilot only), and CSV export.

## 4. Stimulus check

The interview stimulus is the published design artifact in `07` §3. Checked 2026-11-08.

**Laptop width (1280 px): passes.** Viewed in Chrome. The header with the three coverage numbers, the equivalency and self-attestation lines, the legend, rows r1 to r6 with verdict, basis, evidence and the Audit and Override buttons, and the r6 question block with its two controlled selects all render as `08` §2.3 expects.

**Narrow width: checked statically, one defect found and fixed.** The browser extension could not shrink the Chrome window below its minimum, so the artboard's source was read instead. The page is fluid with a 1280 px maximum and one breakpoint at 900 px that stacks the requirement rows and the header into one column, so a phone shows each requirement as a readable stack. The three coverage numbers stay in three columns, which fits. The r6 question block kept its two selects side by side at every width, which the breakpoint did not cover; the breakpoint was extended to stack them. **The owner should still open the link on a phone once before the first call**; that is the only check that could not be made here.

## 5. Where things live

| Private (`docs/personal/framework-research/`, never in this repository) | Public (this directory) |
|---|---|
| `pilot/pilot-interview-NN.md`, one per participant, no name, no employer | The template in §2 |
| `pilot/survey-export-v0.1.csv` and later versions | The instrument (`06`) and its change log (`12`) |
| `viewer-requirements-backlog.md`, reactions to the stimulus, proposed not accepted | The build plan (`13`) and the schemas they feed (`04`, `05`) |
| Anything naming a participant's employer | Aggregate findings only |

The private folder exists with a README stating these rules (`13` §0 decision 1).

## 6. Left for the owner (P3)

- Ten names: five recruiters or HR, five hiring managers, at least three who hire for technical or manufacturing roles, none from current employers.
- Build the Google Form from §3 (about an hour), or ask the agent to draft every question's text as a paste-ready list.
- Post the pilot version of the recruitment post (`07` §2.1), link in the first comment.
- Open the stimulus on a phone once (§4).
- Schedule the ten calls for week 2, at thirty minutes each.
- Decide how the workflow chart reaches participants (§7): the GitHub-rendered page needs no sign-in; the artifact does until it is shared.

## 7. Company workflow review on the call

Added 2026-11-08 at the owner's instruction: the company perspective of the hiring workflow (`09` §1, `10` §1, C-01 to C-26) is shown to each of the ten participants for review and comment. The workflows are the process model every framework component is classified against (`01` §5.6), so ten practitioners' corrections to them are worth more at this stage than anything else the pilot collects.

**When.** Part C of the call, after the survey questions and after the stimulus, so neither the Section E measurement nor the first reaction to the match record is primed by a picture of the whole process. Eight to ten minutes; the six prompts are in the template (§2). The candidate perspective (K-01 to K-16) is not shown to employer-side participants; it belongs to the candidate-side study.

**What is shown.** The company flowchart, revision 2, screen-shared by the interviewer. Two sources carry the same diagram: the design artifact <https://claude.ai/artifact/AJWsmPtYodKEfbcbBRbieL> (tabs for each perspective, pan and zoom; private until the owner shares it) and the GitHub-rendered Mermaid in [`10-hiring-workflow-diagrams.md`](10-hiring-workflow-diagrams.md), which needs no sign-in and is the link to put in the close-out email for written comments. The participant is told it is a draft of how a hire runs and asked to correct it, not told what the system will do at any node.

**What is captured.** Notes and verbatim phrases in the participant's `pilot-interview-NN.md` under Part C, and a mark per node (matches, differs, does not exist here) in `workflow-review-tally.md`, one column per participant, both in the private folder (§5). Nothing a participant says about their own organisation's process is published; the aggregate (how many of ten said a node differs, and the recurring correction) feeds revision 3 of `09` after the pilot, with the review record in `09` §4 extended to say what moved and why.

**What it changes elsewhere.** The consent text (§1) and the recruitment post (`07` §2.1) now say about thirty minutes; `08` §1 and §2.3 carry the third part; `12` records it as change 10.
