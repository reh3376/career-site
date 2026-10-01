# rogerhenley.dev: Improvement Roadmap

**Prepared:** October 1, 2026
**Scope:** The live site at https://rogerhenley.dev (signed out), the public repository `reh3376/career-site` at commit `8ee974c` (merged 2026-10-01 14:13 ET), and the résumés in the job-search project.
**Lens:** The site exists to move a hiring manager from "who is this" to "let's talk." Every item is ranked by how much it helps that outcome, not by engineering interest.

## How this was verified

Every finding below cites what was checked. Nothing comes from memory or assumption.

| Method | What it covered |
|---|---|
| Source read | `apps/web/src/components/landing/it-landing.tsx`, `ot-landing.tsx`, `site-header.tsx`, `site-footer.tsx`, `hamburger-menu.tsx`, `lib/public-routes.ts`, `lib/github-repos.ts`, `content/articles/*`, `README.md`, `docs/FSD.md`, `docs/backlog.md` |
| HTTP checks | Status, redirects and page weight for 13 routes; `robots.txt`; `sitemap.xml`; meta, Open Graph and canonical tags on `/` and an article |
| Rendered pages | Headless Chromium screenshots of `/` at 390 px (phone) and 1440 px (desktop); horizontal overflow measured |
| GitHub | Public repo pages for `mdemg`, `acd-l5x-tool-lib`, `plc-gbt`, `nexttrend`, `NextTrend`, `bosc_ims` |
| Project files | Master résumé and tailored résumés searched for repo links, `rogerhenley.dev`, and MDEMG database claims |

**Limits.** I did not sign in, so the JD reviewer, Ask Roger, the meeting scheduler and the member gallery were not exercised. I cannot see production analytics. Those limits are listed again in the last section.

---

## Phase 0: This week (before October 6)

### 0.1 Reconnect the Google Calendar before booking breaks

- **Evidence.** `docs/backlog.md` §4b: refresh tokens expire after seven days while the OAuth consent screen is in Testing. Connected 2026-09-30, so "booking stops around 2026-10-06."
- **Why it matters now.** "Book a meeting" is one of the three hero buttons. A recruiter who gets approved and finds booking broken has hit the worst possible failure, at the moment of highest intent.
- **Action.** Reconnect at `/admin/scheduler` on or before October 6, then either publish the consent screen through Google verification or set a weekly reminder. Until then, confirm the failure message reads as "temporarily unavailable, use the contact form" and not as an empty calendar.
- **Effort.** Minutes now; verification is a separate, slower track.

### 0.2 Fix the dead repository links on your résumés

- **Evidence.** Signed-out requests to `github.com/reh3376/nexttrend`, `github.com/reh3376/NextTrend` and `github.com/reh3376/bosc_ims` return **404**. The master résumé and the Anduril résumé print both links. The Cognite résumé prints the `nexttrend` link, and the Hadrian résumé prints the `bosc_ims` link. `github.com/reh3376/plc-gbt` does resolve.
- **Why it matters.** A reviewer who checks your proof of work finds nothing there. On a résumé built around "operator-builder," that reads worse than having no link.
- **Action.** Pick one per repo: make it public, or change the résumé line to "private repository; walkthrough on request." Then apply that to every résumé still in circulation.
- **Effort.** Small.

### 0.3 Correct the hero sentence that overstates the record

- **Evidence.** `it-landing.tsx` line 85 and `ot-landing.tsx` line 131: "Thirty years running regulated, 24-hour manufacturing." The stat block on the same page says the 30 years were "across power, mining, telecom, industrial automation, and applied AI." Your master résumé puts manufacturing roles from 2007 (Joy Global) onward. The site's own meta description already uses defensible wording: "Thirty years running regulated 24/7 industrial systems."
- **Action.** Use the meta-description wording in both landing modes.
- **Effort.** One-line change in two files.

### 0.4 Name both distilleries in the "8 yr" stat

- **Evidence.** `it-landing.tsx` line 205 reads "Whiskey House of Kentucky and its predecessors." `ot-landing.tsx` line 179 says "Whiskey House of Kentucky + predecessors." Your résumé lists Bardstown Bourbon Company (1/2018 to 10/2022) and Whiskey House (10/2022 to 7/2026) as separate employers. Bardstown Bourbon Company does not appear anywhere in the site's visible copy (searched `src/` and `content/`).
- **Action.** Write "Bardstown Bourbon Company, then Whiskey House of Kentucky." It is accurate, and it puts your award-winning employer on the page.
- **Effort.** One line in two files.

---

## Phase 1: Make the first screen work for a hiring manager

The FSD's own persona P6 (anonymous visitor, 30 to 90 seconds) and requirement FR-PUB-01 call for a landing page with "name, headline, one photo, three to five headline accomplishments." The live page doesn't meet that requirement yet.

### 1.1 Put role, record and proof above the fold

- **Evidence.** Screenshots at 390 px and 1440 px. The first screen shows a beta notice, the headline "Industrial automation, plant operations, applied AI," the paragraph above, and three members-only buttons. In the IT landing source, these strings each appear **zero** times: "Director," "Vice President," "Bardstown Bourbon," "Sazerac," "135," "Discovery," "ENERGY STAR," "ISO 27001."
- **Why it matters.** The visitor can't see what level you work at or what you have delivered without registering and waiting for approval.
- **Action.** Add one line naming your last two titles and employers. Then replace or follow the stat row with three or four outcomes from the master résumé that a hiring manager can verify:
  - Greenfield distillery delivered (scope and timeline as you approve them for public use)
  - 200% capacity scale-up at Bardstown Bourbon Company, delivered during a live-plant controls migration (Ignition Discovery Award 2019)
  - 40%+ natural-gas reduction (EPA / ENERGY STAR Project of the Year 2021)
  - ISO 27001 certification leadership
- **Note.** The backlog's 6c scrub decision keeps the $135M figure, since it already appears on 21 of your résumés. Whether it goes on a public, indexed page is your call.
- **Effort.** Small to medium.

### 1.2 Reframe the copy from consultancy to candidate

- **Evidence.**
  - Section heading (line 286): "Five areas, usually mixed together on one engagement."
  - Line 289: "Naming them separately makes it easier for you to decide whether we should talk."
  - Footer (`site-footer.tsx` line 18): "A practice, not a portfolio."
  - Hero line: "Not looking to hire? Reach out directly." This implies the person who *is* hiring must register first.
- **Why it matters.** Your positioning rejects purely advisory and consulting roles. This copy reads as a consulting practice soliciting engagements, so a hiring manager may not realize you want a full-time role.
- **Action.** Add one explicit availability line, as FR-CNT-15 already specifies ("open to roles, location/remote preferences"). Then change "engagement" to "role," and "Not looking to hire?" to "Hiring? Request access, or contact me directly."
- **Effort.** Small.

### 1.3 Remove or contain the "beta" signal

- **Evidence.** Two beta labels sit on the first screen: the header badge ("SYSTEM · BETA," `site-header.tsx` lines 77 to 85) and the hero eyebrow ("in beta · the core functions work, some are rough," `it-landing.tsx` line 68). The source comment explains the honesty rationale.
- **Recommendation.** Keep the honesty but move it. One label in the header is enough, and the hero eyebrow is the most valuable line on the page. A visitor's first impression of a candidate shouldn't be "some are rough."
- **Effort.** Trivial.

### 1.4 Link the public evidence from the landing page body

- **Evidence.** The live `/` HTML contains no links to `/articles` or `/gallery`. Its internal links are `/contact`, `/how-ask-roger-works`, `/login`, `/register`, `/privacy` and `/terms`. In the IT landing, those pages are reachable only through the hamburger menu. The hero sentence "The writing and the work photos are open; read them first" (line 152) has no link. A `RecentWriting` component (`components/landing/recent-writing.tsx`) is built but imported nowhere. The OT landing does link both pages (`ot-landing.tsx` lines 232 and 233).
- **Action.** Link "the writing" and "the work photos" in that sentence, and drop `RecentWriting` into the IT landing after the practice section.
- **Effort.** Small, since the component already exists.

### 1.5 Offer a public résumé

- **Evidence.** There's no `/downloads` route in `apps/web/src/app` (it redirects to `/login`). No public page links a PDF (checked `/`, `/contact`, `/how-ask-roger-works`). FR-CNT-13 specifies résumé downloads, and it's unbuilt.
- **Why it matters.** A recruiter's first action is to forward a résumé. Right now that requires an approved account.
- **Action.** Publish one general résumé PDF behind no gate, with the phone number removed if you prefer. This doesn't weaken the reviewer, because the reviewer's value is the *tailored* résumé.
- **Effort.** Small.

---

## Phase 2: Make the link travel well

The URL will mostly reach people as a link in a résumé header, a LinkedIn post or an email.

### 2.1 Add Open Graph, Twitter Card and canonical tags

- **Evidence.** Live HTML for `/` and `/articles/better-business-decisions-part-1` has no `og:*` properties, no `twitter:*` names, no `rel="canonical"` and no JSON-LD. `openGraph` appears nowhere in `apps/web/src`. Titles and descriptions are present and good.
- **Why it matters.** Pasted into LinkedIn, Slack or email, the link renders as bare text with no image or card. Your articles are the most shareable thing on the site.
- **Action.** Add `metadataBase`, `openGraph` (title, description, image) and `twitter: { card: "summary_large_image" }` in the root layout, with per-article overrides from front matter. Add a `Person` JSON-LD block on `/` (name, jobTitle, sameAs LinkedIn and GitHub).
- **Effort.** Small to medium.

### 2.2 Put rogerhenley.dev on your résumés, pointed at the strongest page

- **Evidence.** Searched the project's résumés for `rogerhenley.dev` and found no hits. The headers list LinkedIn and GitHub only.
- **Action.** Add it to the contact line, at least on the AI, architecture and DataOps résumés. On those, consider pointing directly to `/how-ask-roger-works`, which is public (`public-routes.ts`) and is the strongest single proof of hands-on AI engineering on the site.
- **Dependency.** Do Phase 1 first, so the people it sends see the improved landing.
- **Effort.** Small per résumé.

### 2.3 Consider a direct contact path

- **Evidence.** No public page has a `mailto:` or `tel:` link. Contact is form-only (`/contact`), and the form emails you.
- **Action.** Your choice. Many hiring managers prefer an address they can paste into their own email. If spam is the concern, an obfuscated address or a dedicated alias works.
- **Effort.** Trivial.

---

## Phase 3: Content completeness and consistency

### 3.1 Resolve the missing Part 3

- **Evidence.** `content/articles/` contains Parts 1, 2, 4 and 5 of "Better Business Decisions Require Better Information." Parts 1 and 2 both describe "a five-part series" and name Part 3 ("When Being Right Is Not Enough"). Part 5 cites Part 3 by page and paragraph ("Part 3, p. 7… para. 2"). Part 3 is not on the site, and the sitemap lists only 1, 2, 4 and 5. The project holds `better-business-decisions-part3.md`.
- **Action.** Publish Part 3, or, if it's held for a reason, add a one-line note on Parts 4 and 5 saying it's forthcoming. As it stands, a careful reader hits a citation to a page that doesn't exist.
- **Effort.** Small (content already exists).

### 3.2 Remove the stale "later phase" gallery line

- **Evidence.** `it-landing.tsx` line 395: "More ship with the gallery in a later phase." The gallery is live and public at `/gallery`, and six images render for a signed-out visitor.
- **Action.** Replace it with a link to `/gallery`.
- **Effort.** Trivial.

### 3.3 Curate the repository cards instead of showing "most recently updated"

- **Evidence.** `lib/github-repos.ts` fetches `sort=updated` and shows the first six (`github-repos.tsx` line 16). The live cards include `homebrew-mdemg` (a packaging tap) and `duMod` ("Digital You Concept map," no language listed). `plc-gbt`, which your résumés cite, is not shown. The cards will reshuffle whenever any repo is touched.
- **Action.** Replace the hidden-list with a pinned allow-list in display order, for example `mdemg`, `forge`, `acd-l5x-tool-lib`, `plc-gbt` and `ann-research`. Keep live stars and the updated date.
- **Effort.** Small.

### 3.4 Bring `acd-l5x-tool-lib` onto your controls résumés

- **Evidence.** It's the most-starred repository in the site's card list, with 25 stars and 5 forks on its GitHub page today. It converts Rockwell `.acd` to `.l5x` and back to enable Git-based PLC version control. I searched the project's résumés for it and found nothing.
- **Action.** Add one line to the controls, automation and OT résumés (Reynolds, Fortna, AAK, Fuji Seal, Vantage, NTT and similar). It's direct evidence for the IT/OT CI/CD claim those résumés already make.
- **Effort.** Small.

### 3.5 Reconcile the MDEMG database claim

- **Evidence.** The public `mdemg` README describes Neo4j 5.x throughout (badge, description, prerequisites, `mdemg db start`) and never mentions TypeDB. The Anduril and Cognite résumés say "MDEMG: … Go + TypeDB (previously used Neo4j)." The site's landing links the repo.
- **Action.** Update the README if the migration has happened, or change the résumé line to match the README. A technical interviewer will open the repo.
- **Effort.** Small.

### 3.6 Decide deliberately whether the spirits-decline work belongs here

- **Evidence.** "The Same Path, a Faster Clock" (Sept 2026 LinkedIn essay) and the "This Time Is Different" thesis are not on the site. Backlog §6c already excludes the Rev2 document for authorship reasons.
- **Consideration.** It's strong independent analysis, and you're presenting it at the Beam Institute conference. It's also a public forecast of structural decline in the industry where several of your active applications sit (Heaven Hill, Diageo, and earlier Sazerac and Brown-Forman). This is a positioning call only you can make. It's listed so the decision is made on purpose rather than by default.

---

## Phase 4: Funnel friction for approved visitors

### 4.1 Revisit the 7-day default access window

- **Evidence.** `services/api/internal/users/access_grants.go` line 32 defaults to `7 * 24 * time.Hour`. FR-AUTH-14 confirms that the one-click Accept grants 7 days. FR-AUTH-16 auto-declines unapproved requests after 7 days.
- **Why it matters.** Hiring loops often run three to six weeks. A hiring manager who registers in week one is locked out by the panel interview.
- **Action.** Default to 30 days, and auto-extend while the member is active. The reminder email (FR-NOTF-06c) already exists.
- **Effort.** Small (one constant plus copy).

### 4.2 Return a real 404 for routes that don't exist

- **Evidence.** `/timeline`, `/projects` and `/downloads` have no page in `apps/web/src/app`, but each returns `307 → /login?next=…`. An approved member who follows a stale link signs in and then lands on a 404.
- **Action.** In the proxy, only redirect paths that match a known member route. Let everything else fall through to `not-found`.
- **Effort.** Small.

### 4.3 Read the funnel before redesigning further

- **Evidence.** The data layer exists: the event stream (FR-DATA-01), `/admin/analytics`, and the funnel query documented in `docs/events/README.md`. I couldn't see production numbers.
- **Action.** Before Phase 1 ships, record landing → register → verify → approve → first action counts as a baseline. Re-check two weeks after. That turns the landing rework into a measured change, the same way you treat reviewer changes.
- **Effort.** Small.

---

## Phase 5: Repository presentation (for the technical reviewer)

A technical interviewer will open `reh3376/career-site`, and it's already one of your strongest work samples. These items keep its public face as accurate as its internals.

### 5.1 Update the README status block

- **Evidence.** `README.md` line 7 is dated 2026-09-22. It says everything except the landing, contact, register, login, privacy and terms is members-only, and lists "Ask Roger chat" as "still ahead." The live `public-routes.ts` makes `/articles`, `/gallery` and `/how-ask-roger-works` public. `/ask` exists, `services/api/internal/chat/` exists with tests, and the live how-it-works page reports Ask Roger "status: live · members only."
- **Action.** Refresh the status paragraph. It's the first thing a reviewer reads.
- **Effort.** Small.

### 5.2 Sync the FSD and backlog on the same three points

- **Evidence.**
  - FSD §3.2, FR-PUB-03 and the §7.1 sitemap all say `/how-ask-roger-works` is members-only. `public-routes.ts` records it as public since 2026-09-24.
  - Backlog §6 says Ask Roger "does not run on the current web server." The live how-it-works page says it runs on the same server as the reviewer.
- **Action.** Reconcile them. NFR-DOC-02 makes the FSD authoritative, so drift here contradicts the repo's own rule.
- **Effort.** Small.

### 5.3 Close the web-app test gap you've already identified

- **Evidence.** `apps/web` has zero `*.test.*` or `*.spec.*` files and no test script in `package.json`. Backlog §4c already calls this out.
- **Action.** Follow the backlog's own priority order: booking slot arithmetic, JD upload form states, then scheduler save-or-refuse. A reviewer who checks will see Go and Python tested and the front end not.
- **Effort.** Medium.

---

## Verified as working well (no action)

- **No horizontal scroll** at 390 px or 1440 px, and the hero CTAs are reachable on a phone's first screen.
- **The public/gated split is principled and consistent.** One list (`public-routes.ts`) drives the proxy, `robots.txt` and the sitemap, and the live `robots.txt` and `sitemap.xml` match it.
- **`/how-ask-roger-works` is public and excellent.** It publishes live agreement figures (83.9% over 31 graded verdicts), including the direction of every disagreement, plus a before-and-after determinism table. Few candidates can show anything like it.
- **Pages are fast.** TTFB was 0.2 to 0.55 s on every route checked, and titles and meta descriptions are present and specific.
- **The landing source documents its own design decisions inline.** This is a good signal to a technical reviewer reading the code.

## Corrections to my earlier reply in this conversation

- I said "everything useful sits behind manual approval." **That was wrong.** Articles, the public gallery subset and the how-it-works page are all public. The real problem is narrower: the default landing page doesn't link to them (item 1.4), and there's no public résumé (item 1.5).
- I gave `acd-l5x-tool-lib` as 27 stars. GitHub shows **25 stars and 5 forks** today. The site's card is cached for up to an hour (`REVALIDATE_SECONDS`).
- I said the site's repo list "doesn't show" NextTrend, BOSC IMS and WHK-WMS. The more important fact is that two of those links **404 on your résumés** (item 0.2).

## Not verified (needs a signed-in session or your input)

- JD reviewer, Ask Roger and the scheduler, end to end as an approved member. Backlog §3 already lists "verify the submitter workflow on prod" as open.
- Production funnel and reach numbers (item 4.3).
- Whether `nexttrend` and `bosc_ims` are private or were renamed (item 0.2).
- The OT landing mode on a real phone. Backlog §6 already lists the WCAG AA contrast fix for the dark tokens.

---

## Summary sequence

| When | Items | Outcome |
|---|---|---|
| By Oct 6 | 0.1 to 0.4 | Nothing broken or overstated reaches a reviewer |
| Next 1 to 2 weeks | 1.1 to 1.5, 2.1 | A cold visitor sees who you are, what you delivered, and how to get a résumé |
| After Phase 1 | 2.2, 2.3, 3.x | The URL goes on résumés; content and cross-asset claims are consistent |
| Ongoing | 4.x, 5.x | Lower-friction access for hiring teams; the repo reads as accurately as it is built |
