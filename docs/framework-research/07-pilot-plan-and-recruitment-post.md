# Survey Pilot (n≈10) and Recruitment Post

Companion to `06-employer-side-discovery-research-plan.md`. The pilot exists to fix wording, length and branching before the survey goes wide; it is not a data-collection wave.

**Status.** Draft 2026-11-08, not yet fielded. The interview stimulus (§3) is built; the ten names are P3 in `00-todo.md`. Repository copy ahead of project (2026-11-08): this status line added; the call was extended to about thirty minutes the same day for the company workflow review (`11` §7), and §1 and §2.1 say so. **2026-11-09:** §2.1 rewritten for the on-site pilot (`15`): the participant creates a login on rogerhenley.dev and reads the match record and the workflow chart there, with comment boxes, before the call; the post now states the problem, the ask and the profile of the ten in full, at the owner's request. Ready to post; the owner edits to taste.

---

## 1. Pilot design

**Who.** 10 people: 5 recruiters/HR, 5 hiring managers, drawn from personal contacts who will tolerate a follow-up call. At least 3 should hire for technical or manufacturing roles. Nobody from the owner's current employers.

**Format.** Each pilot participant completes the survey, then does a call of about thirty minutes (or written notes): fifteen on the survey itself — a cognitive interview, not a content interview — then five on the stimulus and eight to ten on the company hiring workflow (`11` §7). The survey part asks:

- Which questions did you have to read twice?
- Which answer options didn't fit your situation?
- Where did you want to say "it depends"?
- For the ranked list (C1): were any two items the same thing to you? Any item you didn't understand?
- For the process-fact questions (D4, D6, D7): could you actually answer these, or did you guess?
- For the concept paragraph (E): what did you picture? (Tests whether the paragraph communicates the match record without a picture.)
- How long did it take? (Target ≤10 min; cut if the median exceeds 12.)

**What the pilot decides.**

| Signal | Action |
|---|---|
| Two C1 items read as duplicates by ≥3 people | Merge them |
| An item nobody selects and ≥3 call unclear | Reword or drop |
| B1/B2 answers surface a problem not in C1 from ≥2 people | Add it before fielding |
| D4/D6/D7 answered "don't know / no way to know" by most | Keep — that *is* a finding — but add a "why don't you know" follow-up |
| Concept paragraph pictured as "an ATS score" by most | Rewrite to lead with "every requirement shown separately with its evidence" |
| Median time >12 min | Drop C3 (frequency × cost) to top-3 picks only |

**Record keeping.** Pilot responses are kept separately and not merged into the main wave. Changes to the instrument are logged with the reason, same discipline as the prototype's prompt versions: `survey v0.1 → v1.0`, one line per change.

**Timeline.** Recruit in week 1, run in week 2, revise and field wide in week 3.

---

## 2. LinkedIn recruitment post (draft)

Two versions: one for the pilot (personal network, asks for the follow-up call) and one for the wide wave. The owner's voice is plain and direct; these are drafted to match and should be edited to taste.

### 2.1 Pilot version (personal network)

Rewritten 2026-11-09 for the on-site pilot. Plain text for LinkedIn: no link in the body (it goes in the first comment, §2.3), no hashtags unless the owner wants them, short paragraphs because LinkedIn folds the post after the first three lines, so the first line has to carry the ask.

> I need ten people who hire, for a short pilot. Recruiters, HR, hiring managers. Details below.
>
> The problem I am working on. Qualified people are screened out of hiring because their résumé does not use the posting's exact words. The first screen is a keyword match, done by software or by a reviewer who is a generalist, and it cannot tell that two differently worded descriptions are the same capability. "Front-end development" in a capital projects posting means front-end engineering design, not web pages; a keyword filter does not know that. The person who would know, the hiring manager, sees the pool after the damage is done. In Harvard Business School and Accenture's Hidden Workers study, 88% of employers surveyed agreed that qualified high-skill candidates are vetted out because they do not match the job description's exact criteria. Employers say they have too many applicants and, in the same breath, that they cannot find anyone.
>
> What I am building. Software that screens the other way round: every requirement in a posting is judged separately against the evidence a candidate actually offers, with that evidence shown next to the judgment, so a recruiter can see the match without being the domain expert, and a hiring manager can audit or override any single line.
>
> I know the applicant side of this well. Before I design any further I need the employer side, from the people who do the work. Ten people first, to tell me where my survey and my picture of your process are wrong, before I ask fifty of you to rank the problems.
>
> Who I am looking for. Ten people who have hired in the last two years: five recruiters or HR, five hiring managers who owned the role. At least three of the ten should hire for technical or manufacturing roles (engineering, controls, maintenance, operations, skilled trades), because that is where the keyword failure bites hardest. Any industry otherwise.
>
> What I am asking. About twenty minutes on my site, on your own time, then one call of about thirty minutes. You create a login, take a ten-minute survey, then read a one-page mock-up of a match record and a one-page chart of how a company hires, with a comment box under every row and every step. On the call we go through what you wrote. This is a pilot: you are testing the survey and the chart, not endorsing the idea. The real survey goes out after you have fixed it.
>
> What is kept. Your answers, without your name or your employer. Nothing that identifies you or your company is published. You get the summary when it exists and a list of what changed because of you.
>
> If that is you, comment or message me and I will set you up. Sharing this to anyone in talent acquisition or anyone who manages a team is the most useful thing you can do with it.

**First comment**, posted by the owner right after the post:

> The site is rogerhenley.dev. Access to the pilot is by invitation after we have spoken, so there is nothing to click yet; message me and I will set it up.

### 2.2 Wide version

> A question for anyone who has hired in the last two years, as a recruiter, in HR, or as the hiring manager who owned the role:
>
> What is actually broken in how applications get screened?
>
> I know the applicant-side answer well. I'm building a tool that replaces keyword screening with requirement-by-requirement evidence matching, and before I lock the design I want the employer side's own ranking of the problems — not my assumptions about them.
>
> 10-minute survey. You'll rank a list of known problems, add anything missing, and react to a short description of the approach. Anonymous; no employer named; aggregate results shared back with everyone who asks.
>
> Link in the first comment. Recruiters and hiring managers are analysed separately, so both are needed. Sharing to anyone in talent acquisition or who manages a team is the most useful thing you can do with this post.

### 2.3 Notes on the posts

- Neither post names the thesis as settled ("qualified people are screened out" is stated as the problem being studied, and the wide version removes even that framing from the question itself, so it doesn't prime the ranking).
- Neither offers an incentive beyond the summary, per the research plan's note on sample skew.
- Any link belongs in the first comment on LinkedIn, not the post body, for reach. For the on-site pilot there is no survey link: the pilot area opens for a whitelisted address only after the owner has spoken to the person (`15` §0 decision 1), so the first comment names the site and says access is by invitation.
- The pilot post names the profile of the ten in full (five recruiters or HR, five hiring managers, at least three technical or manufacturing hirers) because the owner asked for it and because it lets readers self-select and forward. "None from the owner's current employers" is the owner's own screening rule and stays out of the post.
- The pilot post states the problem as the problem being studied and gives one example and one published figure (`01` §1). It does not claim the tool works; it says what it is meant to do.
- If the owner's articles have an existing readership in manufacturing/controls, the wide post can be followed a week later with a one-line nudge citing the pilot's most surprising finding, which is the honest way to boost response.

---

## 3. Interview stimulus

The mocked match record used in Section E of the interviews and in the interview guide's concept-reaction block is a published design artifact (claude.ai, "Mocked Match Record — Interview Stimulus", <https://claude.ai/artifact/RumwAKPYaD2jerW5yc7Umv>). It is reconstructed from runs documented in `docs/llm-tuning-log.md` — the capital-projects posting with the "front-end development" misreading, the "Master's or equivalent experience" alternative, and the "10+ years" duration case — anonymised and recast in the v1 verdict vocabulary so interviewees react to the design we intend to build, not the prototype's three-way verdict.

**v1.1 (2026-11-09).** Rewritten from the owner's comments after his own run of the on-site pilot (`15` §14): every part of a row labelled, a definitions panel, a specific basis on every row, the two-run story on r1, follow-up and inclusion lines, "Human judgement required" on r10, the record's status inconclusive while a question is open, three general questions. The embedded JSON in `services/api/internal/pilot/data/` is the record of the stimulus from here on; the artifact above is the v1 original.
