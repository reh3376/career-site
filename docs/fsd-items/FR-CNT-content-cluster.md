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

**A new content directory must be registered, and a test enforces it.**
The public corpus walk iterates `publicCorpusSubdirs`, an explicit map
in `handlers/admin.go`, rather than listing the directory, so a new
directory is invisible to the corpus at runtime. But
`TestEveryPublicContentDirectoryIsReindexable` fails on any directory
under `content/` that has no entry in that map, with one named
exemption (`photos`, which holds images the walker does not read).

That test is the repo taking a position: content the corpus never reads
is a failure, not a default. A new kind therefore needs an entry in
`publicCorpusSubdirs` **and** in `ingest.KnownKinds`, because the
second is what lets a single kind be reindexed on its own rather than
only in bulk.

**Some of the material is already on the landing page.** Three awards
appear there, and the curated GitHub cards cover part of FR-CNT-03's
intent (the row says so, pointing at FR-CNT-25). Neither is a surface,
and neither is linkable.

## The decision this cluster turns on, and the answer

**Should the public role and project content also enter the corpus?**

**Yes. This document first argued no, and that was wrong.**

The argument against was drift: `career-facts.md` already covers these
facts, and two descriptions of one career can disagree, which
`components/landing/core-actions.tsx` names as a failure in a different
context. The argument for was that the assistant could answer "what did
he do at Joy Global" in sentences rather than from a bullet list.

What settled it was `TestEveryPublicContentDirectoryIsReindexable`,
which the first version of the timeline failed. The repo has already
decided this: a content directory the corpus never reads is a defect,
and the only exemption is `photos`, which holds images. Weighed against
that, the drift concern is weaker than it looked, because the role
files are *derived from* the facts sheet and say so, and a judge seeing
both a terse sheet and a prose description of the same job is reading
corroboration rather than contradiction.

So `role` is in `KnownKinds` and `publicCorpusSubdirs`, and the same
will be true of `project`. Drift is managed by the content naming its
source, not by withholding the content.

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

**The corpus registration was got wrong first, and CI caught it.**
Before creating the directory I checked that the public walk iterates
an explicit allow-list rather than listing the directory, concluded a
new directory could not break the reindex, and shipped it unregistered.
That was true of the runtime and false of the repo's intent:
`TestEveryPublicContentDirectoryIsReindexable` fails on exactly that,
because content the corpus never reads is the failure this codebase
keeps finding. `role` is now in `publicCorpusSubdirs` and in
`ingest.KnownKinds`, the second because that is what allows a single
kind to be reindexed rather than only the whole corpus.

Checking the mechanism was not the same as checking the decision, and
the test held the decision.

**Not done, and worth knowing:** the page is a server component with no
client JavaScript, so it would pass FR-PUB-05's check, but `/timeline`
is not in that check's page list. Worth adding when the cluster's other
surfaces land, as one change rather than four.

### FR-CNT-03 + FR-CNT-05 — projects and detail pages, 2026-10-13

`/projects` and `/projects/[slug]`, six published projects, all backed
by public repositories so every entry is checkable: `career-site`,
`mdemg`, `forge`, `acd-l5x-tool-lib`, `plc-workflows-mpc` and
`ann-research`.

**The source material is Roger's own repository descriptions**, read
from the GitHub API, plus the spans in `career-facts.md` for the dates
and the fifteen-year MPC/APC history. Nothing in the bodies describes
work from inference.

**Status is a closed set with reader-facing labels.** `active`,
`maintained`, `paused`, `complete` in the schema; "Maintained, not
actively extended" on the page, because `maintained` and `paused` are
words a visitor can read as the same thing. An unrecognised value falls
back to `paused` rather than rendering a raw token, though CI rejects
one first.

**The catalog deliberately does not call GitHub.** Live repository
metadata is FR-CNT-25's job, served by the landing cards. A catalog
that fails to render when `api.github.com` is slow is a catalog that is
sometimes missing, and this one is the page a reader lands on from a
CV. `repo` is optional for the same kind of reason: the day a project
cannot be published without a repository is the day the catalog is
describing GitHub rather than the work.

**The detail body is markdown with its own headings, not four fixed
fields.** FR-CNT-05 names problem, approach, architecture and outcomes.
Fixed fields would render an empty "Architecture" for every project
without a diagram, and an empty heading claims more than an absent one.
A test asserts each body carries headings and runs to more than twice
its summary, because a detail page that restates the catalog entry is a
catalog entry in a bigger font.

**Registered as a corpus kind in the same change as the content**,
rather than discovered by CI as `role` was. `project` is in
`ingest.KnownKinds` and in `publicCorpusSubdirs`, so "has he built
anything with vector search" becomes answerable from something other
than a job history.

**`ann-research` is thinner than the others and says so in its own
body.** Its repository description is one line, "Phase 2 of ANN
topological feature build out", and writing a confident problem and
outcomes section from that would be invention. It is published with
what is known and an honest note that the work is in progress. Roger
may want to say more, or to hold it back until there is more to say.

**Not done:** diagrams, which is the row's "where available" clause
rather than a hidden gap, and links to related writing, left out for
the same reason as on the timeline.

### FR-CNT-09 — skills, 2026-10-13

`/skills`, 15 skills in 5 categories, each with depth, years and its
evidence as links.

**The row's condition is the only interesting thing about it**, and it
is now enforced rather than asserted. Evidence is `kind:id` refs,
resolved at render time against the published timeline, projects and
articles. A ref naming nothing is dropped rather than rendered as a
dead link; a skill with no surviving evidence is not rendered at all.

**Three tests, because the obvious one is not enough.** "Every
published skill has evidence" can never fail on its own, since the
loader drops an unsupported skill before the page sees it: a broken ref
makes a skill *disappear*, not appear unsupported. So there is also a
test that every individual ref resolves, and one that the published
count matches the file count. Mutation-tested both ways: renaming a
project slug fails with `skill-go -> project:mdemg-renamed`, and
orphaning every ref on one skill fails with `expected 14 to be 15`.

That matters because this promise can be broken by content alone, with
nobody touching code. Rename a project slug six months from now and the
test says which skill just lost its support.

**Depth is three levels** (`working`, `deep`, `owned`) with labels that
say what each claims. Five would invite a self-assessment nobody can
check, and the difference between a self-scored 7 and 8 is not
evidenceable.

**Role evidence links to `/timeline#role-id`**, so the timeline's list
items gained ids and `scroll-mt` to clear the sticky header.

**The ordering argument held.** This was third in the cluster because
the row requires evidence and, until the timeline and the projects
shipped, there was nothing to link to. Built first it would have been a
table of adjectives.

**One thing I was wrong about.** I expected skills like NFPA 70E
ownership to have no linkable evidence and need leaving off. They do
have evidence: the Whiskey House role body already describes owning
electrical safety and directing the arc-flash studies. Writing the
roles properly in the first piece of work is what made the third piece
honest, which was the argument for the order but not one I expected to
be able to check.

### FR-CNT-10 — awards and credentials, 2026-10-13

`/credentials`, 12 entries in five sections: Recognition, Education,
Certifications, Service and Availability. All four things the row names
are present.

**The lapsed certifications are published as lapsed.** This is the only
decision on the page worth arguing about, and it was Roger's to make
rather than mine, so it is recorded here with the reasoning rather than
buried. Listing only current credentials is the CV convention, and the
convention is exactly what lets a reader assume the omissions are
flattering. BICSI, the FCC licence, A+, Networking Essentials and LCTE
all appear with when they were held and that they were not renewed,
using the facts sheet's own framing: *the certification lapsed; the
work did not stop.*

A test asserts every certification carries a status, because one that
silently loses it reads as current. That turns an omission into a
claim, which is the opposite of what this page is for.

**One kind, not five directories.** Awards, degrees, certifications,
service and eligibility are read together on one page and differ only
in which fields apply: an award has a year and no status, a
certification has a status and often no year, an eligibility statement
has neither. Five directories would be five schemas for one table.

**"As the owner chooses to publish them" is taken literally.** Nothing
is derived from the résumé or the facts sheet at build time. Every
entry is a file written on purpose, and removing one is deleting a file
rather than hunting for the flag that hides it.

**Availability is the row's "eligibility statement".** It carries what
the landing page already says, that he is open to relocation for the
right role, so the two cannot drift.

### FR-CNT-08 — deferred, 2026-10-13

Asked directly which papers should be public, Roger's answer was that
what he wants public is already available from the UI: the six articles
at `/articles`.

So the row is deferred on content, not on code. The unpublished
material exists under the private `docs/personal/corpus` (data-centre
cooling, dual-loop liquid cooling, the SST 3MW summary, the US spirits
structural-decline study), and publishing any of it is his decision.

**Building the surface anyway would be this FSD's recurring mistake in
reverse.** An in-page PDF viewer, a route, a content kind and a nav
link, all pointing at nothing, is a declared capability with nothing
behind it. A reader who clicks "Presentations" and finds an empty page
has been told something false about the site, which is worse than not
being offered the link.

It becomes a day's work the moment there is a first paper, and the four
content kinds shipped in this cluster are the pattern to follow.

---

## The cluster, closed 2026-10-13

| row | outcome |
|---|---|
| FR-CNT-01 Timeline | Shipped, links deferred until their targets exist |
| FR-CNT-03 Projects | Shipped |
| FR-CNT-05 Project detail | Shipped |
| FR-CNT-09 Skills | Shipped, evidence condition enforced |
| FR-CNT-10 Awards and credentials | Shipped |
| FR-CNT-08 Presentations | Deferred, no material to publish |
| FR-CNT-13 Downloads | Dropped, already decided 2026-10-01 |

Five rows built in one day, one dropped as already decided, one
deferred on content.

**What changed, in one line:** the site went from arguing for itself to
showing the record it argues from.

**What carried across all five.** Content is files somebody wrote, not
data derived at build time. Every kind is validated in CI by the same
schema mechanism. Every kind was mutation-tested, because a check that
has never failed on purpose is a check nobody knows works. And every
kind is registered in the corpus, so the assistant can answer from the
same record a reader sees: 38 documents this morning, 66 by the
evening, 78 once credentials deploys.

**The one thing I got wrong**, worth keeping because it cost a CI
failure: I checked that the corpus walker *tolerated* an unregistered
content directory and concluded that meant it was safe, when a test was
holding the opposite position. Checking a mechanism is not the same as
checking a decision.
