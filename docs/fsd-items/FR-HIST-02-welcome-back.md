# FR-HIST-02: the welcome-back panel

**Built 2026-10-12.** The state this was written from is kept below.

## Outcome

Shipped for two of the four elements.

**The finding that reshaped the item.** This document said the data was
thin "for a structural reason rather than a missing query": the only
client code recording a view was scoped to `/home`. Querying production
made that concrete. **27 view events across exactly one distinct content
id, and that id was `/home` itself.** FR-HIST-02's "up to three items
viewed" had only the page the member was standing on to list. So this
was not a panel to build; it was instrumentation to add, and the panel
was the easy half.

The server action that records activity lived in
`app/(site)/home/actions.ts`, a route-scoped `"use server"` file, which
is why no other page could call it. It and the beacon component are now
shared, and the article reader, the gallery and `/jd-upload` each record
their own view against the content's front-matter `id`, which is what
`ContentSummary.id` has always meant and what item 3 added to every
content file a few hours earlier.

**Decision #1, `HomeService` or `MemberService`: MemberService.**
`HomeService` specified this panel field for field and was never
implemented or mounted. Building a second service for one panel would
add a surface for no gain, so the panel is served from
`MemberService.GetHistory`, which `/home` already calls, reusing the
`WelcomeBack` message the contract already described rather than
redefining it. `HomeService` stays declared and unmounted; the reasoning
is written at the new field.

**Decision #2, the false FSD claim: corrected.** The row said `/home`
already showed "what is new since last time". No such logic has ever
existed.

**Decision #3, where to instrument: the article reader, the gallery and
`/jd-upload`**, which is what this document recommended. Each sends a
real content id rather than a route.

**Decision #4, chat topics: out of scope**, and named in the FSD row
rather than left implied. It needs an instrumentation call on the chat
send path that does not exist, plus a decision about what a useful
"topic" phrase is.

**Decision #5, selection logic: most recent distinct content, newest
first, excluding the home page by name.** Across all history rather than
only the previous visit: a member who read one article last week and
nothing since should still be greeted with it, and scoping to the last
session makes the panel empty for exactly the people it exists to greet.

**What is populated and what is not.** `WelcomeBack` has six fields.
`last_visit_at`, `previous_visits` and `recently_viewed` are filled.
`recent_topics` needs the chat instrumentation above;
`new_since_last_visit` needs per-item update timestamps the content
model does not carry, since articles date to a bare year;
`resume_conversation_id` is a separate feature. Each is left unset
rather than filled with a guess, and the proto says why at the field.

**A visit is a run of activity with no gap over 30 minutes**, derived
from `occurred_at`.

This was wrong when it shipped, and the live pass on 2026-10-12 caught
it within the hour. The first version counted distinct
`activity_events.session_id`, on the reasoning that the column was
"already on every activity row". It is on every row and it is NULL on
every row: nothing in the write path has ever set it. The panel
rendered nothing on production.

The local check passed because it inserted rows with `session_id`
populated. The data was shaped to match the assumption rather than the
assumption tested against the data, which is the same mistake this whole
roadmap keeps finding, made by me, in the middle of fixing it. A column
being present is not evidence that it is written.

`users/welcome_back_test.go` now seeds activity the way the application
does, with `session_id` untouched, and restoring the old query fails two
of its four cases. The unwritten column is in `docs/backlog.md`.

**Verified by rendering it.** Three prior sessions and two article views
were created in the local stack; the API returned `previousVisits: 2`,
the correct `lastVisitAt`, and three content ids newest-first excluding
`/home`; `/home` rendered "Last here 4 days ago, your 3rd visit" and
named the articles as links, in both UI modes. The synthetic rows were
removed afterwards.

**One thing worth knowing about the API's shape.** It returns content
ids and nothing else. The web app owns content metadata and resolves the
title and link from its own files; duplicating that into the API would
give the two a way to disagree, which is the failure this roadmap keeps
finding.

---

**State as of 2026-10-12, before the build.** The welcome-back panel is fully designed,
field for field, in a proto service that has never been implemented or
mounted: `HomeService.GetHome` and its `WelcomeBack` message
(`proto/career/v1/home.proto:14-40`) already specify `last_visit_at`,
`previous_visits`, up to three `recently_viewed` items, `recent_topics`,
and `new_since_last_visit`, but `HomeService` is one of three declared
services `server.go` does not mount
(`services/api/internal/authpolicy/policy.go:83`), and no handler for
it exists anywhere in the codebase. What `/home` actually renders comes
from a different, older RPC, `MemberService.GetHistory`, which returns
nothing but the raw `activity_events` feed: no last-visit date, and,
despite the FSD's own status note, no "new since last visit"
computation either, since no such logic exists in the frontend or the
API. The data that would feed a real resume list is thin for a
structural reason rather than a missing query: the only client code
that ever records a page-view event is scoped to `/home` itself, and no
client code anywhere records a chat-topic event, so "items viewed" and
"topics asked about" have almost no real data to draw from regardless
of which service assembles the panel.

## What the FSD requires

`docs/FSD.md:401`, FR-HIST-02:

> Returning members shall see a welcome-back panel: last visit date, up
> to three items viewed, topics asked about, and items new since the
> last visit.

Status column: "Must (partially **Shipped**)". Notes column, in full:

> Partially shipped. `/home` shows a RECENT ACTIVITY list and what is
> new since last time. No last-visit date and no three-item resume
> list.

## What exists today

**The proto already matches the requirement, field for field.**
`proto/career/v1/home.proto:27-40`:

```proto
message WelcomeBack {
  google.protobuf.Timestamp last_visit_at = 1;
  int32 previous_visits = 2;
  repeated ContentSummary recently_viewed = 3;
  repeated string recent_topics = 4;
  repeated ContentSummary new_since_last_visit = 5;
  string resume_conversation_id = 6;
}
```

`HomeService.GetHome` (`home.proto:14-20`) is specced as one call,
`AUTH_LEVEL_MEMBER`, returning this plus a "Start here" path, highlight
sections, a recommended résumé, a hero image, and suggested questions
(`home.proto:88-111`). Someone already designed the exact shape FR-HIST-02
asks for.

**`HomeService` is not mounted, and has no handler.**
`services/api/internal/authpolicy/policy.go:83` names it directly:

> Every declared method is included, including the methods of services
> that `server.go` does not currently mount (`HomeService`,
> `ContentService`, `DownloadService` as of 2026-10-09).

`services/api/internal/server/authpolicy_coverage_test.go:65-74` and
`:126-136` assert exactly three unmounted services, `career.v1.HomeService`
among them, as a locked-in property of the production server shape. A
repo-wide search for a `Home` handler struct under
`services/api/internal/handlers` finds none; any call to `GetHome`
today would resolve to the generated `UnimplementedHomeServiceHandler`
and return `Unimplemented`.

**`/home` calls a different, narrower RPC.**
`apps/web/src/app/(site)/home/page.tsx:53-64` (`fetchHistory`) calls
`MemberService/GetHistory`, not `HomeService/GetHome`. The handler,
`services/api/internal/handlers/member.go:45-77`, returns a flat,
newest-first slice of `activity_events` rows via
`users.Repo.RecentActivity` (`services/api/internal/users/activity.go:79`),
nothing else. No last-visit timestamp, no distinguishing of "new"
items, no cap at three: the page itself caps the rendered list at 5
rows in OT mode (`page.tsx:162`) or 8 in IT mode (`page.tsx:371`), both
larger than the FSD's "up to three."

**The FSD's "what is new since last time" claim does not match the
code.** A repo-wide search for `new since`, `isNew`, `new_since`, and
similar across `apps/web/src` turns up nothing related to content or to
`/home`; the only `isNew` in the tree belongs to an unrelated admin
form (`apps/web/src/app/(site)/admin/(console)/qa/entry-form.tsx:44`).
`/home` renders exactly one list, labeled RECENT ACTIVITY
(`page.tsx:156-167` OT mode, `:365-376` IT mode), and nothing else. The
status note at `docs/FSD.md:401` describes a feature that is not in the
repository.

**Last-visit date: not computed anywhere, but the data to compute it
already exists.** `sessions.created_at` and `sessions.last_active_at`
(`services/api/internal/db/migrations/00001_init.sql:76-87`) are set
per session and bumped on every lookup
(`services/api/internal/users/sessions.go:48-52`), and
`activity_events.session_id` ties each event to the session it
happened in
(`services/api/internal/db/migrations/00008_activity_events.sql:21`,
`:36`). "The previous session's start time, excluding the current one"
is a query against data that is already written on every sign-in; it
needs no schema change.

**"Up to three items viewed" and "topics asked about": the columns
exist, the data almost doesn't.** `activity_events.content_id` and
`.query` (`00008_activity_events.sql:21-25`) are exactly the fields a
resume list and a topics list would read. But the only client code
anywhere that records a `KIND_VIEW` event is `ActivityBeacon`
(`apps/web/src/app/(site)/home/activity-beacon.tsx`), and it is mounted
exactly once in the whole app, on `/home` itself
(`home/page.tsx:100`, `:248`), always with `contentId="/home"`
(`activity-beacon.tsx:17-27`). No article page, no gallery page, no
`jd-upload` page fires a view event. A repo-wide search for
`KIND_VIEW` / `KIND_DOWNLOAD` / `KIND_SAVE` usage in `apps/web/src`
matches only that one file and its caller, so the real content of
`RecentActivity`'s rows today is overwhelmingly "viewed /home," plus
whatever `KIND_LOGIN` / `KIND_LOGOUT` the server records itself
(`services/api/internal/handlers/auth_session.go:99`). `KIND_CHAT` is
mapped on the server side
(`services/api/internal/handlers/activity.go:113`, `:137`) but is never
emitted by any client code anywhere in the repository (a search for
`KIND_CHAT` outside that mapping function and a display-label lookup
table in `home/page.tsx` finds nothing), so `recent_topics` has zero
source data today, independent of which service eventually assembles
the panel.

## What is missing

1. `HomeService` has no handler implementation and is not mounted;
   `GetHome` has never executed against real data.
2. No query exists for "the previous session's start time"
   (`last_visit_at`), even though `sessions` already carries everything
   needed to answer it.
3. No code anywhere computes a "new since last visit" set, despite the
   FSD status note implying one is already shipped.
4. Real view-event data for non-`/home` content: nothing fires a
   `KIND_VIEW` event for an article, a gallery photo, or the JD-upload
   flow, so "up to three items viewed" has no real items to draw from.
5. Chat-topic data: no client code emits a `KIND_CHAT` activity event,
   so "topics asked about" has no source data under the current
   mechanism, whatever reads it.
6. Selection logic: the live `/home` page shows 5 or 8 raw events, not
   a curated three-item resume list; nothing distinguishes "the three
   worth resuming" from "the rest of the feed."

## Dependencies and constraints

- Mounting `HomeService` means building a handler from nothing (no
  partial implementation exists to extend) and adding it to the
  `mount` calls in `services/api/internal/server/server.go:145-249`;
  the auth-policy map already covers it since the method declares
  `AUTH_LEVEL_MEMBER` in the proto.
- Fixing "items viewed" and "topics asked about" is a product-surface
  problem before it is a storage problem: the schema (`content_id`,
  `query`, `conversation_id` on `activity_events`) is ready; what is
  missing is instrumentation calls on the actual content pages and on
  the chat send path.
- Last-visit date is the one sub-feature answerable today purely with
  a query, no schema change and no new instrumentation.
- Whatever carries the welcome-back fields, `HomeService` revived or
  `MemberService` extended, has to settle whether `GetHistory` stays as
  a separate RPC or is absorbed, since the two would otherwise overlap
  in what they return.

## Decisions the owner must make

1. Build `HomeService.GetHome` for real, or fold the welcome-back
   fields into `MemberService` and drop the unmounted `HomeService`
   from the proto. Either way, the proto currently describes a feature
   that was fully specified and never built; that gap needs an answer.
2. Correct the FSD status note at `docs/FSD.md:401`, which claims
   `/home` already shows "what is new since last time." That claim is
   not true of the current code; it is either a documentation error to
   fix now, independent of everything else, or a commitment to build
   the matching feature.
3. Where to add view-event instrumentation: at minimum, the article
   reader, the gallery, and `jd-upload` each need their own
   `ActivityBeacon`-equivalent call with a real `contentId`, rather
   than the one call that exists today, hardcoded to `/home`.
4. Whether chat-topic recording is in scope for this release, given it
   requires a new instrumentation call on the chat send path
   (`apps/web/src/components/ask`) that does not exist today, plus a
   decision about what a useful "topic" phrase looks like: the raw
   question text, a summarized phrase, or something else.
5. What selection logic picks three items out of a member's view
   history, once that history has real content in it (most recent
   distinct items, longest dwell, or some other rule).

## Task checklist

Ordered; each step assumes the ones before it are done.

1. Decide the `HomeService` question (Decision 1); it determines
   whether the rest of this touches a new handler or an existing one.
2. If keeping `HomeService`: implement the handler, computing
   `WelcomeBack` from `sessions` (`last_visit_at`, `previous_visits`)
   and `activity_events` (`recently_viewed`, `recent_topics`), mount it
   in `server.go`, and update
   `authpolicy_coverage_test.go:126-136`'s expectation of exactly three
   unmounted services to two, in the same change.
3. Add the last-visit query: the most recent `sessions.created_at` for
   the user, excluding the current session's id. This step is
   deliverable on its own, independent of everything else here.
4. Instrument the article reader, gallery, and `jd-upload` pages with
   real `KIND_VIEW` events carrying their own content ids.
5. Decide and, if in scope, implement chat-topic recording (Decision
   4); otherwise mark `recent_topics` explicitly deferred rather than
   letting the status note imply it already works.
6. Build the three-item resume selection logic once real view data
   exists to select from.
7. Correct `docs/FSD.md:401` to match what actually ships, including
   removing the "new since last time" claim if that sub-feature is not
   built in this pass.
8. Verify live: sign in as a real member with real view history,
   confirm the welcome-back panel shows an accurate last-visit date and
   real viewed items (and real topics, if built) on a deployed build,
   before marking the row Shipped.

## What could not be verified

- Whether anything outside the handlers and client paths checked here
  (a cron job, the sidecar process) independently writes `KIND_VIEW` or
  `KIND_CHAT` rows into `activity_events` was not exhaustively ruled
  out beyond the repo-wide searches performed.
- Production row counts and the real distribution of `content_id`
  values in `activity_events` could not be checked directly: this
  environment's local Postgres is at migration 48
  (`goose_db_version`), predates several unrelated tables, and no
  production database access was available from here. The claim that
  real rows are overwhelmingly `/home` views is inferred from which
  code paths can write a `KIND_VIEW` event at all, not from reading
  production rows.
- Whether `docs/FSD.md`'s status note for FR-HIST-02 was accurate at
  some earlier point and has since drifted, versus having always
  overstated what shipped, was not investigated against git history.
