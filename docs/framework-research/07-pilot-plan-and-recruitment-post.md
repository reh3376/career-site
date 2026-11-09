# Survey Pilot (n≈10) and Recruitment Post

Companion to `06-employer-side-discovery-research-plan.md`. The pilot exists to fix wording, length and branching before the survey goes wide; it is not a data-collection wave.

**Status.** Draft 2026-11-08, not yet fielded. The interview stimulus (§3) is built; the recruitment post is not yet published; the ten names are P3 in `00-todo.md`. Repository copy ahead of project (2026-11-08): this status line added; the call was extended to about thirty minutes the same day for the company workflow review (`11` §7), and §1 and §2.1 say so.

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

> I'm building a tool to fix a specific failure in hiring: qualified people getting screened out because their résumé doesn't use the posting's exact words, and screeners who can't tell the difference because the role isn't their field.
>
> I have a strong view of this from the applicant side. I need the other side.
>
> If you've recruited or been a hiring manager in the last two years, I'm looking for ten people to take a 10-minute survey and then spend about thirty minutes on a call: which questions were badly worded, and where a one-page chart of how companies hire gets it wrong. Not what you think of the idea — what you think of the survey and the chart. That's the pilot; the real one goes out after you've fixed it.
>
> Recruiters and hiring managers both, technical and manufacturing roles especially welcome. Anonymous, no employer named, and you get the summary when it's done.
>
> Comment or message me and I'll send the link.

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
- The survey link belongs in the first comment on LinkedIn, not the post body, for reach.
- If the owner's articles have an existing readership in manufacturing/controls, the wide post can be followed a week later with a one-line nudge citing the pilot's most surprising finding, which is the honest way to boost response.

---

## 3. Interview stimulus

The mocked match record used in Section E of the interviews and in the interview guide's concept-reaction block is a published design artifact (claude.ai, "Mocked Match Record — Interview Stimulus", <https://claude.ai/artifact/RumwAKPYaD2jerW5yc7Umv>). It is reconstructed from runs documented in `docs/llm-tuning-log.md` — the capital-projects posting with the "front-end development" misreading, the "Master's or equivalent experience" alternative, and the "10+ years" duration case — anonymised and recast in the v1 verdict vocabulary so interviewees react to the design we intend to build, not the prototype's three-way verdict.
