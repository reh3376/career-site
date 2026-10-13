# Roadmap: the partially-shipped FSD rows

Opened 2026-10-12. Eleven rows the FSD marks shipped with a hole in
them, in the order they will be worked, each with a checklist and a link
to its own working document.

**Why these before the 45 not-built items.** A row marked *Shipped* with
a gap underneath is more dangerous than a row marked *Not built*,
because nobody is looking for it. Every one of these was invisible until
the documentation audit on 2026-10-12 went looking, and three of them
turned out to be controls that exist on paper and nowhere else.

**The ordering rule**, so it can be argued with rather than guessed at:

1. **Decided before undecided.** Items 1 and 2 have their direction
   settled by [ADR 0031](adr/0031-declared-controls-are-enforced-controls.md);
   they need costing, not debate.
2. **Enforcement gaps before features.** Items 1 to 3 are all the same
   shape — something declared and never applied — which is the failure
   this repository has produced eleven times.
3. **Then descending by how much already exists**, so the cheapest
   closures come first and the architectural work is reached with the
   small things out of the way.
4. **Except where a dependency outranks all of it.** Item 9 cannot move
   regardless of priority; it is gated on participant data, not code.

**Status key:** `[ ]` not started · `[~]` in progress · `[x]` done ·
`[-]` dropped, with the reason recorded in the working document.

---

## Order

| # | item(s) | working document | state |
|---|---|---|---|
| 1 | FR-AUTH-09 + FR-CNT-22 — Turnstile, and registration's missing rate limit | [FR-AUTH-09-FR-CNT-22-turnstile.md](fsd-items/FR-AUTH-09-FR-CNT-22-turnstile.md) | `[x]` |
| 2 | FR-ADM-09 — write the audit log, and fix the lost approval actor | [FR-ADM-09-audit-log.md](fsd-items/FR-ADM-09-audit-log.md) | `[x]` |
| 3 | FR-CNT-16 + FR-CNT-17 — content model and item fields | [FR-CNT-16-FR-CNT-17-content-model.md](fsd-items/FR-CNT-16-FR-CNT-17-content-model.md) | `[x]` |
| 4 | FR-ADM-03 — surface the ratings that already exist | [FR-ADM-03-review-queue.md](fsd-items/FR-ADM-03-review-queue.md) | `[x]` |
| 5 | FR-HIST-02 — the welcome-back panel | [FR-HIST-02-welcome-back.md](fsd-items/FR-HIST-02-welcome-back.md) | `[x]` |
| 6 | FR-EVAL-06 — an evaluation must survive a deploy | [FR-EVAL-06-eval-durability.md](fsd-items/FR-EVAL-06-eval-durability.md) | `[x]` |
| 7 | FR-CNT-19 — CI link health | [FR-CNT-19-link-health.md](fsd-items/FR-CNT-19-link-health.md) | `[x]` |
| 8 | FR-PUB-05 — rendering without JavaScript | [FR-PUB-05-no-javascript.md](fsd-items/FR-PUB-05-no-javascript.md) | `[x]` |
| 9 | FR-DT-20 — tap-check thresholds from pilot data | [FR-DT-20-tap-check.md](fsd-items/FR-DT-20-tap-check.md) | `[ ]` |

Nine rows for eleven requirements: `FR-AUTH-09`/`FR-CNT-22` and
`FR-CNT-16`/`FR-CNT-17` are each one piece of work.

**Moved off this list on 2026-10-12.** `FR-ADM-13` (scheduler) was
listed as partially shipped on the strength of a status line reading
"Not yet: the connect flow, the encrypted refresh token, and the
bookings list". All three exist. The row is now marked shipped, with
what closed it named.

---

## Findings that changed the shape of an item

**The landing page is two pages, and the survey loaded one** (item 8).
FR-PUB-05's working document counted three controls that do nothing
without JavaScript, read off the component tree. Checking the live site
found twelve, across four page variants. `/` renders from two separate
component trees depending on the `ui_mode` cookie, and the HMI one
carries six dead controls the editorial one does not. Reading the code
found the shape of the problem; only loading both modes found its size.
The check that now holds this requests `/` twice, with and without the
cookie, for that reason.

**The deploy path had never looked at evaluations** (item 6). The quiet
check counted decision tests and recent events and stopped there, so a
five-hour run was no obstacle to a rollout at all. 6 of the first 15
production runs ended at `failed` with partial scores. The FSD row also
understated the controls that did exist: it said per-service image tags
were "in the backlog" when `WEB_IMAGE_TAG` and `--only web` had already
shipped.

**One page in the whole app recorded a view** (item 5). The server
action that records activity lived inside the `/home` route, so nothing
else could call it. Production held 27 view events across exactly one
distinct content id, and that id was `/home`. FR-HIST-02's "up to three
items viewed" had only the page you were standing on to list. The item
was not a panel to build, it was instrumentation to add; the panel was
the easy half.

**The "failed calls only" filter had never worked** (item 4). The link on
`/admin/decisions` computed its own query parameter and then never wrote
it to the URL, so clicking it produced an unfiltered page. Found while
adding the queue filter beside it. The same class as everything else in
this roadmap: a control that is present, looks like it works, and does
nothing.

**Three of FR-ADM-03's four signals already had data** (item 4).
Production held 9 ratings, 3 negative and 3 with a typed comment, and
the refusal and no-support paths were recorded on every row. What was
missing was a query, not a feature. The row had blamed FR-CHAT-14 for
not being built; it shipped 2026-09-30.

**The front-matter parsers had already drifted** (item 3). The Go
walker's comment said it handled "the same shape the TypeScript articles
library expects (kept in sync by convention)". Comparing them found two
real differences: `articles.ts` did not normalise CRLF, and it stripped
a lone `"` to an empty string where the other two left it alone. Neither
mattered for the content committed today, which is exactly why neither
was noticed. Two TypeScript parsers became one, and all three are now
asserted against a shared fixture.

**The audit log already had a read surface in the contract** (item 2).
`AdminService.GetAudit` was declared from the first proto, commented
"Lists audit-log entries", and had no handler, so it answered
UNIMPLEMENTED against a table holding no rows. Two declared-and-unbuilt
halves of the same requirement, neither visible from the other. The item
got smaller as a result: the read surface needed implementing, not
designing.

**"All admin actions" is 42 procedures, not 4** (item 2). The working
document listed four call sites. Counting the contract found 81 methods
declaring ADMIN, of which 42 change something or move member data. Four
hand-written inserts would have left 38 actions unrecorded behind a row
marked Shipped, which is the exact shape of the problem being fixed, so
this went in as a declaration plus an interceptor instead.

These came out of establishing current state and were not visible from
the FSD rows.

**The page can believe you are signed in when the API does not**
(item 1). `/contact` decides whether to show the anonymous name, email
and Turnstile fields from `getSessionCookie()`, which only asks whether
a session cookie exists. The API resolves that cookie to a user. An
expired or revoked cookie therefore renders the member view and submits
down the anonymous path. Before Turnstile this produced a confusing but
recoverable error; with Turnstile it would have been an unrecoverable
one, so the widget now renders in both states. The underlying mismatch
is older than this item and is in `docs/backlog.md`.

**The constant that names the hidden token field cannot live beside the
widget** (item 1). The server actions read `formData.get(TURNSTILE_FIELD)`.
Importing that constant from the `"use client"` component gave them a
client reference rather than the string, so the lookup missed, the token
read as empty, and every submission failed the bot check with nothing in
any log to explain it. Caught only because the end-to-end run was done
before the PR, not after. It now lives in `lib/turnstile.ts`.

**Approval decisions have no actor** (item 2). `approval_decisions.decided_by`
exists, the repo struct carries it, the INSERT names it, and nothing
has ever set it: production holds 5 decisions and 0 actors. The
`approval.decided` event emits the *target member's* id as the actor, so
the event stream cannot recover it. This is a correctness bug sitting
inside the audit item rather than a missing feature, and it makes item 2
bigger than "insert rows into a table".

**The quiet check gives evaluations no protection at all** (item 6).
`rollout.sh`'s `quiet_check` reads `dt_sessions` and `events`; it does
not look at `eval_runs`. An evaluation runs for hours and emits roughly
one event when it starts, so a run at minute 11 is already invisible to
the check. I had assumed this was partial cover. It is none, which makes
the cheapest mitigation — query `eval_runs` in the same place — more
valuable than it looked.

**No `stop_grace_period` is set on any service**, so a deploy gives a
running posting about ten seconds before SIGKILL. Raising it is a
one-line change that converts some killed runs into clean ones.

**FR-ADM-03 was never blocked on what the FSD said** (item 5). Its row
claimed the review queue is not driven by negative feedback "because
FR-CHAT-14 (member rating) is not built". FR-CHAT-14 is built and has
been since 2026-09-30, with a live control under every assistant
message; production holds 9 ratings. The real gap is that nothing
queries them. That makes item 5 a query and a surface rather than a
feature, and it moves up.

I reported the opposite earlier on 2026-10-12, from a case-sensitive
grep for `RateMessage` that missed the camelCase client call
`rateMessage`, and read no-match as not-built. Recorded because it is
the same error shape as the rest of this list.

**Photos are not in the corpus** (item 3). `publicCorpusSubdirs`
(`handlers/admin.go`) names `articles` and `other`; `apps/web/content/photos`
exists with frontmatter and is never walked, so nothing in it is
retrievable by Ask Roger or the JD reviewer. The map's own comment
records this trap catching someone before — "the job reported 'kinds
article' and ingested nothing". Whether photos *should* be retrievable
is a decision, not a defect, but the current state was not written down
anywhere.

**Three independent frontmatter parsers exist** (item 3):
`apps/web/src/lib/articles.ts`, `apps/web/src/lib/photos.ts` and
`services/api/internal/ingest/walker.go`. None validates, each reads a
different subset of keys, and the Go one's comment says they are "kept
in sync by convention". The articles themselves are already inconsistent:
five of six carry author/series/part/tags, the teaser carries only
title/subtitle/date.

**FR-HIST-02 has no data for two of its four parts** (item 5). The FSD
row claims `/home` shows "what is new since last time"; no such logic
exists. `ActivityBeacon` is mounted only on `/home` and emits only
`KIND_VIEW` for `/home` itself, and nothing emits `KIND_CHAT` anywhere.
So "items viewed" and "topics asked about" need instrumentation before
the panel can show anything true. The last-visit date is a cheap query.
`HomeService.GetHome` and its `WelcomeBack` message are fully designed
in the proto, on a service that is never mounted.

**FR-DT-20 is blocked structurally, not by sample size** (item 9). I
ranked it last as "waiting for participant data". That reasoning was
wrong: failed tap attempts are scored client-side and never transmitted,
so every stored session passed under the current cut and no amount of
waiting yields a labelled mashing example. Calibrating requires changing
what is recorded first. The rank stands; the reason does not.

I then got the mechanism wrong twice in one day. The briefing renders
Continue only when `(tap?.passed || synthetic)`, and the label
"Continue without passing" appears only on the `synthetic` branch. I saw
that label, concluded a real participant could proceed without passing,
and said the check reports rather than gates. It gates: a real
participant who fails can retry, and after two failures is offered the
visual beat, but there is no path past it with a failing result. Which
is precisely why no failing row can exist to calibrate against.

**Four mutating admin RPCs accept a `reason` and discard it**
(`ApproveRegistration`, `DeclineRegistration`, `ExtendAccess`,
`SetMemberStatus`). The fields exist in the contract, no handler reads
them, and no console form collects one. I used `ExtendAccess`'s reason
on 2026-10-12 believing it was recorded; it was not.

---

## What each set has to clear before it is called done

Every item below, not only the ones that touch auth:

- [ ] The change is verified **against production**, not against a green
      suite. A green suite says nothing about whether a surface works.
- [ ] Where a control is being added, there is a test that fails when
      the control is removed. Verified by removing it, not by assuming.
- [ ] The FSD row, `docs/fsd-open-items.md` and this file are updated in
      the same PR, so the three cannot disagree.
- [ ] If anything turned out differently from this document, the
      document says what it claimed as well as what is true.

The second of those is not ceremony. Of the eleven controls this
repository has declared and not applied, every one had something that
looked like coverage: a config field that was read, a proto option that
was declared, a comment that described enforcement. None of them had a
test that would have failed.
