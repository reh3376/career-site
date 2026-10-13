# FR-CNT-19: external links and link health

**State as of 2026-10-12.** The `rel="noopener"` half of this
requirement is genuinely done, but by convention rather than by any
mechanism that would catch a lapse: every external anchor in the app is
hand-written with `rel="noopener noreferrer"` and `target="_blank"`,
repeated identically across roughly fifteen files, with no shared link
component and no lint rule enforcing the pairing. One rendering path,
article markdown bodies, applies the attributes programmatically rather
than by hand, which is the one place a future external link could not
be added without them; a second markdown path, the assistant's chat
answers, takes a different approach entirely and strips external links
down to plain text rather than rendering them as links at all. The
second half of the requirement, CI checking link health on content
changes, does not exist in any form: no workflow, script, or installed
tool references link checking, and the one workflow that already
classifies a diff as content-only versus code (`ci.yml`'s `changes`
job) exists for a different purpose and triggers nothing content-aware.

## What the FSD requires

`docs/FSD.md:353`, FR-CNT-19:

> External links shall open in a new tab with `rel="noopener"`, and CI
> shall check link health on content changes.

Status column: "Should (partially **Shipped**)". Notes column, in
full:

> Partially shipped. External links carry `rel="noopener noreferrer"`
> and `target="_blank"`. No CI link checker.

## What exists today

**`rel`/`target` applied by hand, consistently, with no shared
component.** Every external anchor found carries the identical pair
`rel="noopener noreferrer"` / `target="_blank"`, written at each call
site rather than through a shared `<ExternalLink>` or similar:
`apps/web/src/components/site-header.tsx:40-41,54-55`,
`site-footer.tsx:97-98,109-110,120-121,130-131`,
`hamburger-menu.tsx:246-247`, `github-repos.tsx:55-56,74-75,123-124,140-141,163-164,187-188`,
`components/landing/it-landing.tsx:731-732,752-753,787-788,797-798`,
`components/landing/ot-landing.tsx:533-534,549-550`,
`components/landing/podcasts.tsx:107-108,150-151`,
`app/(site)/home/page.tsx:434-435,449-450,474-475,484-485`,
`app/(site)/contact/page.tsx:53-54,114-115`,
`app/(site)/terms/page.tsx:33-34`,
`app/(site)/meetings/booking.tsx:276-277`,
`app/(site)/meetings/my-meetings.tsx:132-133`,
`app/(site)/admin/(console)/contacts/page.tsx:282-283`,
`app/(site)/admin/(console)/jd/page.tsx:287-288,308-309`, and
`app/(site)/admin/(console)/jd/[id]/page.tsx:198-199` (the two admin JD
ones add `nofollow` as well). This is consistent today, but it is
consistent because every occurrence was typed the same way, not
because anything would fail if a new one left the attributes off.

**No lint rule enforces the pairing.** `apps/web/eslint.config.mjs:1-13`
builds the flat config from `eslint-config-next/core-web-vitals` and
`eslint-config-next/typescript` only. A search of the installed
`eslint-config-next` package for `jsx-no-target-blank` (the React rule
that flags `target="_blank"` without `rel="noopener"`) finds no
reference anywhere in it; that rule is not part of this repo's lint
configuration. Nothing in `pnpm lint` or `pnpm build` would catch a new
external link added without the attributes.

**Article markdown is the one path where this is applied
programmatically.** `apps/web/src/app/(site)/articles/[slug]/page.tsx`'s
`ReactMarkdown` `a` component override sets `rel="noopener noreferrer"`
on every rendered link unconditionally, and `target="_blank"` only when
the href starts with `http`, so relative, same-site links in article
bodies do not get a pointless new tab. Articles themselves are plain
markdown files under an explicit allow-list
(`apps/web/src/lib/articles.ts:1-14`, `PUBLIC_ARTICLES`), currently six
files in `apps/web/content/articles/`.

**The assistant's chat markdown takes a different, stricter approach:
it drops external links rather than rendering them.**
`apps/web/src/components/ask/message.tsx:89-102`:

```tsx
const MARKDOWN = {
  a({ href, children }) {
    const internal =
      typeof href === "string" &&
      href.startsWith("/") &&
      !href.startsWith("//") &&
      !href.startsWith("/\\");
    if (!internal) return <>{children}</>;
    return (
      <Link href={href} className="text-accent underline underline-offset-2">
        {children}
      </Link>
    );
  },
  ...
};
```

An assistant answer containing an external URL renders that URL's link
text as plain, unlinked text; only same-origin, absolute-path links
become clickable. This is not a violation of FR-CNT-19 (there is no
`target="_blank"` anywhere without `rel="noopener"` to flag), but it
means the requirement's "open in a new tab" half simply does not apply
to this one surface: there is nothing to open, by design, not by
oversight.

**No CI link checker exists in any form.** A search across
`.github/workflows/*.yml` for `link` matches nothing. No `linkinator`,
`markdown-link-check`, `lychee`, or similar tool appears in
`apps/web/package.json` or anywhere else in the repository.

**The one workflow that classifies a diff by content versus code does
not check links, and is not scoped to the repo's content
directories specifically.** `ci.yml`'s `changes` job
(`.github/workflows/ci.yml:399-441`) computes `code=true`/`false` from
a regex allow-list of doc-only paths (`:432`: `docs/`, top-level
`*.md`, a couple of named READMEs, `LICENSE*`), purely to decide
whether to skip the Docker image build on a doc-only PR. It does not
single out `apps/web/content/articles/`, `/other/`, or `/photos/` (the
actual rendered content directories, confirmed at
`apps/web/content/{articles,other,photos}`), and running or not running
it has no connection to link health; it would classify a typo fix in an
article body as "code" (the content lives under `apps/web/`, outside
the doc-only regex) and run the full `web` build, which does not check
outbound links either, only `pnpm typecheck` / `lint` / `test` / `build`
(`ci.yml:376-397`).

## What is missing

1. No shared component or lint rule holds the `rel`/`target` pairing in
   place; it is correct today purely because every occurrence was
   copied correctly.
2. No tool, script, or workflow checks that a link (in article
   markdown, in the `other`/`photos` content files, or in hand-written
   JSX) actually resolves.
3. No workflow trigger or job distinguishes "a content file changed"
   from "any other file changed" in a way useful for a link-health
   check; the existing `changes` job answers a different question
   (build vs. skip the image) and does not treat `apps/web/content/`
   as its own category.
4. The assistant's chat surface is undocumented as an exception: FR-CNT-19
   as worded implies all external links get the new-tab treatment, and
   this is the one place that is structurally untrue because the links
   are not rendered at all.

## Dependencies and constraints

- A link checker has to decide what it is checking: outbound URLs in
  the six article markdown files and the `other`/`photos` content
  files only, or every rendered page (which would need the site built
  and served first, a heavier CI step than a markdown-only pass).
- Checking rendered pages would need to run after `pnpm build`
  (`ci.yml:395-397`), which already exists as a step in the `web` job;
  checking markdown source only could run standalone, earlier, and
  cheaper.
- A job scoped to "content changes" needs its own path filter;
  `ci.yml` has none today (`grep -n "paths:"` across all workflows
  returns nothing), so this would be new, not an extension of an
  existing filter.
- Whatever is added should not duplicate the `changes` job's purpose
  (gating the image build), since that job already runs on every push
  per the branch-ignore filter at `ci.yml:43-76` and is unrelated to
  link health.

## Decisions the owner must make

1. Whether to add a shared link component or a lint rule (e.g.
   enabling `react/jsx-no-target-blank` via `eslint-plugin-react`,
   which is not currently part of `eslint-config-next`'s rule set) so
   the current consistency survives the next new external link,
   rather than remaining a matter of everyone remembering.
2. Whether the chat markdown's link-stripping behavior
   (`message.tsx:90-96`) is the intended permanent design or an
   incidental gap; if intended, it is worth a line in the FSD or in
   `docs/events/README.md`-adjacent documentation so a future reader
   does not mistake it for a bug.
3. What "link health" means for this site in practice: broken outbound
   URLs only, or also internal links (e.g. a renamed article slug still
   referenced from another article), since the tooling choice differs
   (a markdown link checker typically does both for free).
4. Whether the check runs on every push (matching this repo's existing
   philosophy, stated in `ci.yml:43-62`, of denylisting rather than
   allowlisting what gets CI) or only when `apps/web/content/` changes,
   which needs a new path filter either way.

## Task checklist

1. Decide the tool and scope (Decisions 3 and 4).
2. Add the chosen tool as a dev dependency (if JS-based, under
   `apps/web/package.json`; this repo already runs `pnpm` for the `web`
   job) or as a standalone CI step (if a standalone binary like
   `lychee`).
3. Add a new CI job, or a new step inside an existing job, that runs it
   against `apps/web/content/**/*.md` at minimum.
4. If scoping to "content changes" rather than every push, add a path
   filter (there is no precedent for one in this repo's workflows
   today; `ci.yml`'s existing `changes` job computes its own ad hoc
   diff rather than using a declarative filter, and would need a
   second, independent filter for this purpose rather than extending
   the existing one, since the two answer different questions).
5. Decide whether a broken link fails the build or only reports
   (`ci.yml`'s general philosophy, per its own comments at
   `:9-15` and `:55-62`, favors loud failure over silent skip).
6. Separately, decide and act on Decision 1 (shared component or lint
   rule) so the existing, working `rel`/`target` convention does not
   depend on memory going forward.
7. Document the chat markdown's link-stripping behavior explicitly
   (Decision 2), rather than leaving it implicit in the component code.
8. Update `docs/FSD.md:353`'s status to Shipped once the CI check is
   live and passing on a real PR, not merely merged.

## What could not be verified

- Whether `apps/web/content/other/career-site-guide.md` or the photo
  caption files under `apps/web/content/photos/` contain any outbound
  links at all was not checked line by line; the finding here is that
  no tooling checks them, not a count of what would need fixing today.
- Whether any external link currently in the app is actually broken
  (the thing a link checker would catch) was not tested; this document
  is about the absence of the check, not the health of today's links.
- Whether `eslint-plugin-react`'s `jsx-no-target-blank` rule, if
  enabled, would produce any findings against the current codebase
  (it should not, since every occurrence already carries `rel`) was
  not tested by actually enabling it.

---

## Outcome, 2026-10-13

**Shipped: `scripts/check_links.py`, run by CI's `web` job before the
build, alongside `check_design_tokens.py` and `check_content.py`.** It
holds both halves of the requirement that can be held deterministically
and names the half that cannot.

**The scope decision (Decisions 3 and 4, checklist 1 and 4): both kinds
of link, every push, no path filter.** "Link health" here means
internal links resolve and the `rel`/`target` pairing holds, checked
across `apps/web/src/**/*.tsx` and `apps/web/content/**/*.md` on every
push. A path filter was considered and rejected: the requirement says
"on content changes", but a renamed *route* breaks links from content
without the content file changing, so a filter keyed on
`apps/web/content/` would miss the common case. The check takes well
under a second, which removes the only argument for scoping it.

**Reachability of outbound URLs is deliberately not in CI.** It needs
the network, and a job that fails because GitHub rate-limited a runner
is a job people learn to re-run until it passes, which is worse than
not having the check. It lives in `deploy/live-check.sh`, which already
talks to the network, runs against the real site, and reports to a
person doing a deploy rather than blocking a merge. That is also the
honest reading of the private FSD's "broken links warn, not fail":
the thing that can break through nobody's fault warns, and the thing
that is always our own bug fails.

**Fails the build, for the deterministic half (Decision 5).** An
internal link with no route behind it is a 404 the author did not
intend, and `ci.yml`'s stated philosophy is loud failure over silent
skip.

**Both halves were mutation-tested before being trusted.** Renaming a
route produces `links to /x, which no page.tsx serves`; stripping a
`rel` from one anchor produces `target="_blank" without rel="noopener"`.
A checker that has never failed on purpose is a checker nobody knows
works. It also refuses to run if the route walk finds fewer than 20
routes, because a broken walk and a clean pass are otherwise the same
output.

**The first finding was a false positive, and that is recorded here on
purpose.** The first version looked only for `page.tsx` and reported
`/admin/decisions/export` as broken; it is a `route.ts` returning a
CSV. Fixed by walking both. A checker's first finding being wrong is
how a checker gets switched off.

**What the run found: nothing broken.** 50 routes, 163 internal links,
12 external, 0 links in content, 0 problems; all four outbound URLs
answered under 400 on the live pass. The `rel="noopener"` half of the
requirement really was correct by convention. What changed is that it
is now correct by mechanism.

**Content contains no links at all.** Not one markdown link, internal
or external, across all 16 files. Stated explicitly because a checker
that silently matches nothing looks identical to one that passes, and
"link health on content changes" sounds like it is doing more work than
it is today.

**Decision 1 (shared component or lint rule): resolved as the
checker, not a lint rule.** `eslint-plugin-react` is not installed and
`eslint-config-next` does not carry `jsx-no-target-blank`; adding a
plugin for one rule, when the pairing check is fifteen lines inside a
script that was being written anyway, is more dependency than the
problem is worth. The tradeoff accepted is that the feedback arrives at
`check_links.py` rather than in the editor.

**Decision 2 (the chat surface): intended, now documented.**
`components/ask/message.tsx:80-88` already carries the reasoning in
code: a retrieved passage is untrusted data, so an external link in
generated text would let corpus content put a destination in front of a
reader under Roger's name. The FSD row now says so too, so a future
reader does not read it as an oversight in this requirement.

**Not done:** a shared `<ExternalLink>` component. The checker makes
the pairing enforced rather than remembered, which was the actual
exposure; consolidating 15 call sites is a refactor with no behaviour
change behind it.
