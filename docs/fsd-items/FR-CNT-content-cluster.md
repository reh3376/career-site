# The content cluster: FR-CNT-01, 03, 05, 08, 09, 10, 13

**Opened 2026-10-13**, after the partially-shipped roadmap closed at 8
of 9. These are the largest group of unbuilt `Must` rows in the FSD, and
the gap a hiring manager notices first: the site argues well for itself
and does not yet show the record behind the argument.

## The seven rows, and what each asks for

| row | priority | asks for |
|---|---|---|
| FR-CNT-01 | Must | **Timeline.** Roles chronologically: organization, title, dates, location, summary, quantified accomplishments, technologies, links to related projects, articles, awards, photos. |
| FR-CNT-03 | Must | **Projects.** A catalog: name, one-line description, the owner's role, status, technologies, links. |
| FR-CNT-05 | Must | **Project detail.** Problem, approach, architecture, outcomes, links, related writing. |
| FR-CNT-08 | Must | **Presentations.** Decks and papers with an in-page PDF viewer, download, event, date, abstract. |
| FR-CNT-09 | Must | **Skills.** A matrix (skill, category, depth, years, evidence links) where every entry links to at least one piece of evidence. |
| FR-CNT-10 | Must | **Awards and credentials.** Awards, education, military service, eligibility, as the owner chooses to publish. |
| FR-CNT-13 | Must | **Downloads.** Résumé/CV PDF variants, versioned, downloads recorded to member history. |

## It is six rows, not seven

**FR-CNT-13 was declined by the owner on 2026-10-01** and its own note
says so: *"a generic public résumé invites the keyword screening the
site exists to argue against. The JD reviewer's tailored résumé is the
intended path."* The status column still reads `Must`, so a decided
row sits in the unbuilt-Must count inflating it, which is the same
failure mode the partially-shipped roadmap existed to fix: a status
nobody is looking at that no longer describes reality.

It should be marked dropped, with the reason, rather than left to be
re-proposed every time someone counts the gap.

## What exists today

**The content model is already there**, shipped as roadmap item 3
(FR-CNT-16 / FR-CNT-17). `scripts/check_content.py` holds a per-kind
frontmatter schema, enforced in CI, with three kinds today: `article`,
`photo`, `other`. Adding a kind is a schema entry plus a directory, and
the check fails on an unknown key, which is the main thing it catches.
Nothing in this cluster needs a new content *mechanism*; it needs
content kinds and the surfaces that render them.

**The source data for the timeline exists, in prose, and is private.**
`docs/personal/career-facts.md` carries seven roles with organisation,
title, location and dates from 1996 to 2026, plus leadership span,
education, credentials, compliance ownership, measured outcomes and
recognition. It is ingested as corpus kind `profile` and added to every
JD-reviewer judge call.

That is the authority for dates and sequence, and it is the only one.
Nothing in this cluster should restate a date from memory.

**A new content directory cannot break the reindex.** The public corpus
walk iterates `publicCorpusSubdirs`, an explicit map in
`handlers/admin.go`, rather than listing the directory. A new
`content/roles/` is therefore invisible to the corpus until somebody
adds an entry on purpose. (The private walk does use `WalkKinds` and
*does* reject unknown subdirectories, which is why this distinction is
worth stating rather than discovering.)

**Some of the material is already on the landing page.** Three awards
appear there, and the curated GitHub cards cover part of FR-CNT-03's
intent (the row says so, pointing at FR-CNT-25). Neither is a surface,
and neither is linkable.

## The decision this cluster turns on

**Should the public role and project content also enter the corpus?**

The case for: Ask Roger could answer "what did he do at Joy Global"
from text written for a reader rather than from a facts sheet written
for a judge.

The case against, which is the stronger one: `career-facts.md` already
covers these facts for retrieval, and two descriptions of one career
drift apart. The codebase already names this failure in
`components/landing/core-actions.tsx`: *"Two descriptions of one feature
drift apart; if one changes, change both."* A corpus that holds both a
public timeline and a private facts sheet can answer the same question
two ways, and the reviewer would have no way to prefer one.

**Recommendation: public content stays out of the corpus to begin
with.** It costs nothing to add later, and the allow-list means the
decision is explicit either way.

## Order, and why

1. **FR-CNT-01, the timeline.** It is the spine. Projects, skills and
   awards all want to link to a role, and building them first means
   either inventing a link target or going back. It is also the row
   with its source data already written.
2. **FR-CNT-03 + FR-CNT-05, projects and their detail pages.** One
   piece of work, as the catalog is unreadable without somewhere to
   click through to.
3. **FR-CNT-09, skills.** Depends on 1 and 2 existing, because the row
   requires *every* entry to link to evidence, and the evidence is a
   role, a project or an article. Built earlier it would link to
   nothing, which is the shape of a requirement met on paper.
4. **FR-CNT-10, awards and credentials.** Small, and partly on the
   landing page already.
5. **FR-CNT-08, presentations.** Last, because an in-page PDF viewer is
   the only genuinely new mechanism in the cluster and the only part
   that needs a dependency decision.

## What could not be verified

- Whether any project or presentation material exists in a form that
  could be published was not surveyed; `docs/personal/corpus` holds
  white papers and project summaries, but whether the owner wants them
  public is his decision, not a reading of the repo.
- The private repo was read for the career facts only. No claim here
  about what else it holds.

---

## Progress

### FR-CNT-13 — marked dropped, 2026-10-13

No work, a correction. The row was decided on 2026-10-01 and the status
column never caught up, so a settled decision kept appearing in the
unbuilt-Must count. Now `Must (**Dropped** 2026-10-01)` with the reason
in the note. The cluster is six rows.

### FR-CNT-01 — timeline shipped, links deferred, 2026-10-13

`/timeline` renders all seven roles newest first: organisation, title,
dates, location, a written summary, the quantified accomplishments and
the technologies. Public, indexable, in the sitemap, linked first in the
footer.

**Everything on the page comes from `content/roles/*.md`, whose
authority is `docs/personal/career-facts.md`** — the same sheet the JD
reviewer is handed on every judge call. That is deliberate: the public
timeline and the reviewer's facts now cannot disagree about a date,
because there is one place a date is written. Nothing in the content
files restates one from memory.

**`role` is a content kind, not a special case.** It has a schema entry
in `scripts/check_content.py`, validated in CI like articles and
photos, and `end` is required rather than optional: a missing end date
renders as a current role, and a timeline that silently promotes an old
role to present is worse than one that is blank. Mutation-tested by
renaming `end` to `ended` in one file, which reports both the missing
key and the unknown one.

**The tests read the committed files, not a fixture**, because the
likely failure here is content rather than parsing: a date edited in
the wrong direction, two roles with a colliding `order`, a `visibility`
typo that drops a role off the page. A fixture would pass while the
real page lost an entry. Five tests, including that the sequence
descends without gaps and that every role has every field the page
renders.

**The links are not built, and the row is marked partially shipped
rather than shipped.** FR-CNT-01 asks for links to related projects,
articles, awards and photos. Projects and awards have no surface yet
(FR-CNT-03, 05, 10), and adding link fields that point at routes
returning 404 would be a control declared and not applied, which is the
whole subject of ADR 0031. Articles were considered and left out for a
different reason: the published series is about decision quality, not
about any particular role, so a link would assert a relationship that
does not exist. The links become a small change once their targets do.

**A new content directory was confirmed safe before it was created.**
The public corpus walk iterates an explicit allow-list in
`handlers/admin.go` rather than listing the directory, so `content/roles/`
is invisible to the corpus until somebody adds an entry deliberately.
The private walk does reject unknown subdirectories, which is why this
was checked rather than assumed.

**Not done, and worth knowing:** the page is a server component with no
client JavaScript, so it would pass FR-PUB-05's check, but `/timeline`
is not in that check's page list. Worth adding when the cluster's other
surfaces land, as one change rather than four.
