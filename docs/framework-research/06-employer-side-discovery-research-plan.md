# Employer-Side Discovery: Research Plan, Survey and Interview Guide

**Purpose.** The problem definition is well grounded on the applicant side and inferred on the employer side. Before Phase 0 locks scope, collect direct input from recruiters/HR and from hiring managers on which problems they most need fixed, so the system is positioned against their top ten rather than our assumptions about them.

**Status.** Draft instrument v0.1, 2026-11-08. Revised the same day with additions from the workflow review (`09-hiring-workflows-draft.md` §5), marked **[WF]** below. Not yet fielded.

---

## 1. Research questions

1. Which hiring-process problems do recruiters/HR and hiring managers rank highest, by frequency and by cost?
2. Do the two groups rank them differently? (Expected: yes — HR feels volume and time-to-fill; HMs feel wasted interviews and missed people.)
3. Does the employer side perceive keyword-driven false negatives and the screener-expertise gap as problems at all — and if not, is that because they don't occur or because they are invisible from that seat?
4. What would make a non-expert screener trust and act on a per-requirement match record, and what would make a hiring manager override one?
5. What outcome data do employers actually have (quality of hire, early attrition), since the recursive-improvement loop depends on it?
6. **[WF]** How do holds and deferrals actually behave — do they resurface, and what would make them? (Sets the priority of the framework's hold-state requirement.)
7. **[WF]** Do hiring managers cut individuals from a slate before interviewing, and would employers accept a candidate contesting a specific judgment?

**A design caution that shapes the whole instrument.** False negatives are invisible to the people who cause them: a recruiter never meets the candidate the filter dropped. If the ranked list alone is used, our central thesis will likely score low on the HR side *regardless of whether it is true*. So the survey asks about it indirectly (confidence, discovery-after-the-fact, HM disagreement rates) and the interviews probe it directly. A low ranking should change **positioning** (sell on the pain they feel; deliver false-negative reduction as the mechanism), not the thesis — unless the indirect items also come back negative.

---

## 2. Method and sample

| Method | Who | Target n | Purpose |
|---|---|---|---|
| Survey (≈10 min) | Recruiters / HR / talent acquisition | 50+ | Ranked problems, process facts, tool attitudes |
| Survey (same instrument, HM branch) | Hiring managers who have hired in the last 24 months | 50+ | Same, from the other seat |
| Interviews (30–40 min) | 6–8 recruiters, 6–8 hiring managers | 12–16 | Why the rankings are what they are; reaction to the match-record concept |

Recruit through the owner's LinkedIn network and article readership, local SHRM chapters, manufacturing and engineering professional groups (the first target domain), and personal contacts. Over-represent technical/specialized-role hiring, since that is where the expertise gap is sharpest. Keep the owner's current employers out of the sample to avoid conflict.

Ethics: anonymous by default; no employer named unless volunteered; no candidate data requested; stated up front that responses inform a software design and may be summarized publicly in aggregate.

---

## 3. Survey instrument

### Section A — Screening (routes the respondent)

A1. In the last 24 months, which best describes your role in hiring?
 ☐ Recruiter / talent acquisition ☐ HR generalist who screens applicants ☐ Hiring manager (I own the role and decide) ☐ Both recruiting and hiring-manager duties ☐ None of these → *end*

A2. Roughly how many roles have you been involved in hiring for in the last 24 months? (1–2 / 3–5 / 6–15 / 16–50 / 50+)

A3. What kinds of roles, mostly? (select all) Engineering/technical · Manufacturing/operations · IT/software · Sales/marketing · Finance/admin · Skilled trades · Leadership · Other

A4. Organisation size (employees): <50 / 50–250 / 250–1,000 / 1,000–10,000 / 10,000+

### Section B — Open-ended first (before any list is shown)

B1. Think of the last role that was hard to fill. In a sentence or two, what made it hard?

B2. If you could fix one thing about how your organisation screens applicants, what would it be?

*(These come before the list so the list does not anchor them. They are also the source of new problems to add.)*

### Section C — Problem ranking

C1. Below are problems people report with hiring. **Select up to 5** that cause you the most difficulty in your own work. *(Order randomised per respondent.)*

| # | Problem statement |
|---|---|
| 1 | Too many applications to review properly |
| 2 | Most applicants appear unqualified on the surface |
| 3 | Hard to tell real capability from keyword-optimised or AI-written résumés |
| 4 | The people doing the first screen don't have the domain knowledge to judge technical or specialised experience |
| 5 | Hiring managers reject most of the candidates screening passes through (interviews that go nowhere) |
| 6 | Strong candidates get missed because their titles, tools or wording don't match the posting |
| 7 | Time-to-fill is too long |
| 8 | Roles close unfilled or have to be reposted |
| 9 | Candidates stop responding or drop out part-way |
| 10 | Poor candidate experience damages our reputation |
| 11 | Job descriptions are inaccurate, out of date, or wish lists |
| 12 | The hiring manager and the recruiter disagree about what the role actually needs |
| 13 | We don't find out whether our hires worked out, so screening never improves |
| 14 | Compliance or bias risk from screening tools and criteria |
| 15 | Salary or level mismatch discovered late in the process |
| 16 | Interviews are inconsistent from one interviewer to the next |
| 17 | Working-style or team fit can't be assessed until very late |
| 18 | Internal delays: approvals, scheduling, decision-making |
| 19 | Our ATS or screening tools are rigid or hard to configure |
| 20 | I can't explain or justify a screening decision when asked |

C2. Of the ones you selected, which **single** problem would you fix first? (dropdown of their C1 picks)

C3. For each selected problem: how often does it occur? (Rarely / Some roles / Most roles / Every role) — and when it does, how costly is it? (Minor / Moderate / Major / Severe)

C4. Anything missing from the list? (free text)

### Section D — Process facts (how screening actually happens)

D1. Who does the first review of applications for a typical role? ☐ Recruiter/HR ☐ Hiring manager ☐ ATS auto-filter ☐ Outside agency ☐ Other

D2. Does your process use required keywords, knockout questions or auto-reject rules before a human looks? Yes / No / Don't know

D3. Typical applications per role: <25 / 25–100 / 100–300 / 300+

D4. **[WF — required, both branches]** Of candidates the first screen passes, roughly what share does the hiring manager decline *without an interview*? (<10% / 10–30% / 30–60% / >60% / Don't know)

D5. Do you track whether hires are performing or still employed at 90 days or 1 year, and link it back to how they were screened? Yes, systematically / Informally / No

D6. *(HM branch)* How often have you later learned that a strong person was screened out before you saw them? Never / Once or twice / Several times / Regularly / No way to know

D7. *(Recruiter branch)* How confident are you that your first screen does not eliminate people who could do the job? (1–5) What gives you that confidence? (free text)

D8. **[WF]** When a requisition or a candidate is put "on hold", what usually happens to it? ☐ Comes back on a planned date ☐ Comes back only if someone pushes ☐ Quietly dies ☐ Don't know

### Section E — Reaction to the concept

Shown as one paragraph: *"A screening tool that breaks the job description into individual requirements and, for each candidate, shows whether each requirement is met, which document or statement is the evidence, a one-sentence reason, and a coverage score computed from those verdicts. Reviewers can see every judgment and overrule any of them."*

E1. How useful would this be in your work? (1–5)

E2. What would make you **trust** a verdict enough to act on it? (select all) Seeing the evidence it cites · Seeing the reasoning · Knowing the hiring manager can audit it · Knowing it was tested on past hires · Nothing, I'd re-check everything · Other

E3. What would make you **overrule** a verdict? (free text)

E4. How much time per application would you accept spending with such a record? (<1 min / 1–3 / 3–10 / 10+)

E5. Biggest concern about a tool like this? (free text)

### Section F — Close

F1. Would you be willing to do a 30-minute follow-up conversation? (contact field, optional)

F2. Would you like the summarised results? (email, optional)

---

## 4. Interview guide (30–40 min)

**Warm-up (3 min).** Role, how many hires, what kinds. Confirm anonymity.

**Current process (8 min).**
- Walk me through the last role you filled, from posting to offer. Where did the time go?
- Who looked at applications first, and what were they looking for?
- *(Recruiter)* For a technical role, how do you decide whether experience described differently from the posting counts?
- *(HM)* When a shortlist reaches you, what share is usable? What's wrong with the rest?
- **[WF] *(HM)*** Think of the last slate a recruiter sent you. How many did you decline before interviewing, and why?

**Problems deep dive (12 min).**
- What is the most expensive mistake your hiring process makes? Expensive in time, money, or lost people.
- *(Probe false negatives)* Have you ever hired or interviewed someone who had been rejected earlier, or found out a good person never got through? How did you find out?
- *(Probe expertise gap, recruiter)* When a posting asks for something you don't personally understand, what do you do?
- *(Probe expertise gap, HM)* How well does your recruiter understand what your role needs? What gets lost?
- Do you know how your hires from a year ago are doing? Does anything about that feed back into screening?
- **[WF] *(Both)*** Tell me about a requisition or a candidate that was put on hold. What happened to it, and how did you find out? What would have had to exist for it to come back on its own?

**Concept reaction (10 min).** Show a mocked match record (one candidate, 8–10 requirements, mixed verdicts including an `unresolved` with a pending question and a `met_equivalent` with the equivalency named).
- Read it as you would at work. Talk aloud.
- Which verdict would you check first? Why?
- Where would you disagree with it?
- *(Recruiter)* Could you rank ten of these without knowing the domain?
- *(HM)* Would you spend time overriding verdicts, or would you ignore it and read the résumé?
- Reaction to asking the candidate a structured follow-up question when evidence is missing.
- **[WF] *(Both)*** If a rejected candidate could contest one specific judgment with new evidence, would you want to see it? Under what conditions would you re-open the decision?
- **[WF] *(Recruiter)*** If applicants could see, before applying, which of your requirements their background does and doesn't cover, would that change the quality or quantity of applications you receive?

**Wrap (3 min).** What did we not ask that we should have? Who else should we talk to?

---

## 5. Analysis plan

1. **Ranking.** Score each problem by selection count (C1), first-fix count (C2, weighted ×3), and a frequency × cost product (C3). Report three lists: recruiters, hiring managers, combined. The deliverable is each group's top 10 and where they diverge.
2. **New problems** from B1, B2, C4: affinity-map; anything mentioned by ≥10% of a group is added to the canonical list for the next wave.
3. **Hypothesis checks.**
   - *Keyword false negatives (item 6):* its direct ranking, **plus** D6 (HM discovery rate) and D7 (recruiter confidence and its basis). Thesis is supported if D6 ≥ "several times" for a majority of HMs even when item 6 ranks low with recruiters.
   - *Expertise gap (item 4):* ranking by group, plus D4 (HM decline-without-interview rate — a proxy for screen quality) and the interview probes.
   - *Outcome data exists (item 13, D5):* determines how much of the recursive loop can run on employer data in Phase 1 versus being built from scratch.
   - **[WF]** *Holds rarely resurface (D8, hold probe):* if "quietly dies" or "only if someone pushes" dominates, the hold-state requirement moves up the framework priority list; the probe's "what would have had to exist" answers shape its mechanics.
   - **[WF]** *HM slate cuts (D4, slate probe):* if the median D4 is ≥30%, C-14 stays a decision point in the workflow and the HM audit view is Phase 0 scope; if <10%, C-14 folds into interviews.
   - **[WF]** *Candidate disputes (dispute probe):* qualitative; decides whether K-09 dispute is v1, Phase 2, or feedback-only.
4. **Acceptance.** E2/E3/E4 define the viewer's requirements: what must be visible per verdict, how fast the record must read, and what override affordances are needed.
5. **Decision rules for scope.**
   - Phase 0/1 scope must address the combined top 10. Problems in the top 10 that the match record does not touch (e.g. internal delays, scheduling) are explicitly declared out of scope in the framework doc rather than silently ignored.
   - If item 6 and the indirect items both come back weak, the problem definition is revisited before building.
   - If recruiters and HMs diverge sharply, the product has two primary users with different first screens; the viewer design splits accordingly.

---

## 6. Outputs

- A synthesis doc: ranked lists by group, divergence, new problems, hypothesis results, viewer requirements derived from Section E, and the three workflow decisions (C-14, K-04 employer angle, K-09).
- An update to `01-problem-definition-and-framework.md` §2 and §3 citing the data, replacing inferred employer complaints with measured ones.
- A revised success-metric list (§4) weighted toward the measured top problems.

---

## 7. Open items

- Survey platform and anonymity mechanics (see `08-pilot-participant-experience.md` §4).
- Pilot first (`07-pilot-plan-and-recruitment-post.md`), then wide.
- The mocked match record for interviews — built; see `07-pilot-plan-and-recruitment-post.md` §3.
- Incentive for participation (summary report is the default; consider nothing else so the sample isn't skewed toward people who want the incentive).
- Length: the **[WF]** additions add roughly one minute to the survey and three to the interview; the pilot's time check decides whether anything is cut.
