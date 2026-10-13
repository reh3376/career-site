# FR-ADM-03: the conversation-review queue

**Built 2026-10-12.** The state this was written from is kept below.

## Outcome

Shipped for three of the four signals.

**The production figures this document could not verify, verified.** It
asked for an independent check of the 9 ratings and could not tell
whether any were negative. Queried on 2026-10-12: **9 ratings, 6 up and
3 down, and 3 carrying a typed comment**, on 19 assistant messages. All
of it invisible to the admin until now. The answer paths in production
are `model` (9) and `qa_bank` (7); no refusal or no-support row has been
produced yet, so those two filters are correct and currently empty.

**Decision #2, where the rating surfaces: joined into `/admin/decisions`,
not a second list.** The join is exact, since a chat decision records
`ref_kind='chat_message'` and `ref_id = chat_messages.id`, so one
`LEFT JOIN` does it and every other decision kind gets nulls. A separate
surface would have been a second place to check and a second thing to
remember.

**Decision #3, a dedicated filter: yes.** `?queue=1` narrows to
down-rated answers, `out_of_scope`, `no_support`, and failed calls. The
three chat signals were all visible before, to an admin willing to open
every row and read its path, which is not a queue. The rating and the
member's comment now show on the row itself, so it can be scanned.

**Decision #4, escalation: out of scope, named rather than silent.**
`ChatService.Escalate` has no handler, so there is nothing to queue. The
FSD row says so and says what unblocks it.

**Decision #5, clearable ratings: left as it is**, deliberately. The
handler refuses `RATING_UNSPECIFIED` because the store cannot represent
a path back to unrated. That is a conscious limitation rather than a
silent one now that it is written down here; changing it would mean a
nullable column and a migration, for a case nobody has hit.

**A bug found while building it.** The "failed calls only" link on
`/admin/decisions` computed its query parameter and never wrote it to
the URL, so clicking it produced an unfiltered page. That filter had
never worked. Fixed in the same change, and the new one is checked
against the same mistake.

**Live on production 2026-10-12, and the queue shows one of the three
negative ratings, not three.** The two others have no `decision_log`
row, and the queue reads `decision_log`. On 2026-10-01, 15 assistant
messages produced 11 rows; the missing ones all carry the "I don't have
anything in my records" text while their `no_support` column reads
false, which points at a path that answers without going through
`chat.finish`'s logging. That gap predates this item and bounds what any
surface built on `decision_log` can show, the adapter's training labels
included. It is in `docs/backlog.md` to be traced rather than guessed
at here.

The one that does surface is worth reading: a member on 2026-09-30
asked how to upload a JD, got "I don't have anything in my records about
that", and wrote back "you state that you can help with questions about
this site, but my prompt was a question specifically about how to use
the site and you didnt answer it". The site guide is in the corpus, so
that is a retrieval miss, and it sat unread for two weeks.

**Verified by creating each signal and watching the filter find it**, in
the local stack: a down-rated message with a comment, an `out_of_scope`
row and a `no_support` row. The filter returned exactly those three, the
member's comment came through the API, the console rendered
`member: not helpful` on the row with the comment beneath it, and the
repaired link carried `failed=1`. The synthetic rows were removed
afterwards.

**One thing the compiler could not catch.** `ExportDecisions` builds its
SQL by string concatenation from the same column list. Alias-qualifying
the columns for the join would have broken it at runtime only; it was
updated to use the same `FROM` clause and the reason is written at the
call site.

---

**State as of 2026-10-12, before the build.** The FSD's one-line verdict on FR-ADM-03 ("not
driven by negative feedback, because FR-CHAT-14 is not built") is wrong
about the reason, even though the surface-level conclusion (admin review is
not driven by negative feedback today) is correct. FR-CHAT-14 (member
rating) is fully built end to end — proto, handler, repository, schema,
and a live member-facing UI under every assistant answer, shipped
2026-09-30 and rated by real members starting the next day. What is
missing is not the rating feature; it is any admin-side query or surface
that reads the ratings it has been collecting. The same pattern holds for
two of the other three signals FR-ADM-03 names: the data for "I don't
know" answers and out-of-scope refusals already exists on every
`decision_log` row, visible if an admin opens one, but nothing sorts or
filters the queue by it. Escalation is the one signal that is genuinely
unbuilt, because `ChatService.Escalate` has no handler at all.

## What the FSD requires

> FR-ADM-03 (Must, partially shipped): The admin shall have a
> conversation-review queue: negative feedback, "I don't know" answers,
> escalations, and out-of-scope refusals.

Current FSD text (`docs/FSD.md:415`, mirrored in `docs/fsd-open-items.md:117-124`):

> Partially shipped. `/admin/decisions` reviews and grades `decision_log`,
> including chat answers. Not driven by negative feedback, because
> FR-CHAT-14 (member rating) is not built.

FR-CHAT-14 itself (`docs/FSD.md:388`, `docs/fsd-open-items.md:460-465`):

> Not built. No answer rating in the member UI; grading is admin-only via
> `decision_log`.

Both of these FR-CHAT-14 claims are false as written. See below.

## What exists today

**The review surface.** `/admin/decisions` (`apps/web/src/app/(site)/admin/(console)/decisions/page.tsx`)
lists `decision_log` rows via `AdminService.ListDecisionLog`
(`services/api/internal/handlers/admin_decisions.go:24`), which accepts
only `Kind`, `RefID`, `UnreviewedOnly`, and `FailedOnly`
(`services/api/internal/users/decision_log.go:112-122`). A chat answer is
one `kind = 'chat_answer'` row, `ref_kind = 'chat_message'`
(`services/api/internal/chat/answer_test.go:424`), written by
`Service.finish` in `services/api/internal/chat/answer.go:419-484` as a
best-effort last step after the member's answer is already persisted.

Opening a `chat_answer` row renders `ChatAnswerCard`
(`apps/web/src/app/(site)/admin/(console)/decisions/page.tsx:406`), which
shows the path label (`qa_bank` / `model` / `degraded` / `out_of_scope` /
`no_support` / `error`, from `CHAT_PATH_LABEL` at line 168) and the
`output_json` flags `out_of_scope`, `no_support`, `degraded`, `qa_match`
(`ChatOutput` type, lines 152-164). So an admin *can* see, per row, that an
answer was out-of-scope or no-support — once they open that row. There is
no filter, sort, badge, or count that surfaces this at the list level, and
no `kind`-shortcut button for `chat_answer` the way there is for
`jd_gate` (line 321).

**FR-CHAT-14 is built, not "not built."**

- Proto: `RateMessageRequest` / `RateMessageResponse`, `Rating` enum
  (`UP` / `DOWN`), declared on `ChatService.RateMessage`
  (visible in the generated `apps/web/src/gen/career/v1/chat_pb.ts:957-1001`,
  and the human-readable contract in `docs/api/README.md:1797-1811`).
- Handler: `Chat.RateMessage`, `services/api/internal/handlers/chat.go:361-389`.
  Requires a caller (`RequireCaller`), resolves the message id, refuses
  `RATING_UNSPECIFIED` (clearing a rating isn't supported — the column
  distinguishes unrated from neutral), trims the comment, and calls the
  repo.
- Repository: `Repo.RateMessage`, `services/api/internal/users/chat.go:306-324`.
  `UPDATE chat_messages ... SET rating_up = $4, rating_comment = $5,
  rated_at = now()`, scoped by tenant, by the calling user owning the
  conversation, and `role = 'assistant'` (a member cannot rate their own
  question). Zero rows affected returns `ErrConversationNotFound`.
- Schema: `chat_messages.rating_up boolean`, `rating_comment text NOT NULL
  DEFAULT ''`, `rated_at timestamptz`, added in the original table
  definition, `services/api/internal/db/migrations/00045_chat_conversations.sql:78-80`.
  The comment on that migration is explicit: "NULL is unrated, which is
  different from neutral and has to stay distinguishable."
- **Member-facing UI**, live: `Rating` component,
  `apps/web/src/components/ask/message.tsx:212-280`, rendered under every
  non-ephemeral assistant message that isn't the owner replying
  (`message.tsx:73-75`). "Helpful" fires immediately; "Not helpful" opens
  an optional comment box (1000 chars) before sending. Both call
  `chatClient.rateMessage(...)` (`message.tsx:220-224`), a live Connect
  call through `apps/web/src/lib/chat-client.ts` (`createClient(ChatService,
  transport)` against `/api/career.v1.ChatService/RateMessage`). This is a
  hand-written caller, not generated code; the earlier claim that
  `RateMessage` appears in `apps/web/src` only in generated code is
  incorrect — see the dependency section below for the full correction.

**Production data.** The task brief states production holds 9 rated
messages out of 38, all rated 2026-10-01, at least one with a typed
comment. I did not query production in this session to re-verify the row
counts myself, but the dates are consistent with what the code shows: the
`Rating` component was added in commit `6759dc7` ("feat(chat): /ask and
the side panel"), dated 2026-09-30, and the ratings are dated the next
day. That is the expected shape of real member usage starting the day
after a feature ships, not a seeded or scripted backfill — see the
dependency section for how I ruled out the alternatives.

## What is missing

1. **No admin surface reads `chat_messages.rating_up` / `rating_comment` /
   `rated_at` at all.** `grep -rln "rating_up|RateMessage|ratingUp"
   apps/web/src/app/(site)/admin/` returns nothing. The 9 production
   ratings, including the typed comment, are sitting in the database with
   no admin page, no `AdminService` RPC, and no join from `decision_log`
   that ever reads them.
2. **No query joins `decision_log` to `chat_messages` rating.** `
   DecisionFilter` has no rating field, and `ChatDecisionOutput` /
   `ChatOutput` (the JSON shapes written to and read from
   `decision_log.output`) have no rating field either — rating lives only
   on `chat_messages`, which `ListDecisionLog` never touches.
3. **No list-level signal for out-of-scope / no-support**, even though
   the data exists on every row. An admin has to open each `chat_answer`
   row to learn its path; there's no count, filter, or sort by
   `out_of_scope` / `no_support` the way `FailedOnly` exists for JD rows.
4. **No escalation signal exists anywhere**, because escalation itself is
   not built (see below) — this one is a real missing feature, not a
   missing query.
5. **No connection from a negative rating back to the decision_log row it
   corresponds to**, for an admin who does get as far as building a
   rating-based view. The join key would be `chat_messages.id =
   decision_log.ref_id AND decision_log.ref_kind = 'chat_message'`, which
   is available but unused.

## The FR-CHAT-14 dependency, which is not what the FSD says it is

The brief for this document, and the FSD text it was built from, both
assert that FR-CHAT-14 "is not built" and that `RateMessage` "appears in
`apps/web/src` only in generated client code." Both are wrong. I want to
be precise about what I found and why the earlier claim was incorrect,
since getting this right is the point of the document.

**What's actually true:** `apps/web/src/components/ask/message.tsx` has a
hand-written `Rating` function (lines 212-280) that renders "Helpful" /
"Not helpful" controls under every assistant message and calls
`chatClient.rateMessage(...)`. A literal-string search for `RateMessage`
in `apps/web/src` does turn up only generated code, because the
hand-written call site uses the camelCase method name the Connect client
exposes (`chatClient.rateMessage`), not the PascalCase RPC name. Searching
for `rateMessage` (lowercase first letter) or for the `Rating` component
itself finds it immediately. This is a real, live, member-facing feature,
not dead code: it's wired into `Thread` (`apps/web/src/components/ask/thread.tsx:7,205`),
which is what the `/ask` panel renders, and the file was last touched
2026-10-01 with no flag or condition hiding it.

**How the 9 production ratings likely happened:** through this UI,
by members, starting the day after it shipped. I ruled out the two
obvious alternatives:

- **Not a script.** `deploy/exercise-admin.sh` is the only
  repo-maintained script that exercises live RPCs against production,
  and its own comment at line 258 explicitly excludes `RateMessage` as a
  mutation it will not call. No other `.sh`, `.go`, or `.sql` file in the
  repository references `RateMessage`, `rating_up`, or `rated_at` outside
  the implementation and its test (`services/api/internal/users/chat_test.go:107,175`,
  which runs against a test database, not production).
- **Not a dead or removed control re-added later.** `git log --all -S
  "function Rating"` finds exactly one commit introducing it
  (`6759dc7`, 2026-09-30), and it has not been deleted or disabled since.

I could not independently query production to confirm the 9/38 figure or
the comment text myself in this session (see "What could not be
verified"), but nothing in the code or its history is inconsistent with
real member use.

**Net correction:** FR-CHAT-14 should read "Shipped" (member-facing rating
UI, live since 2026-09-30, collecting real data), not "Not built." FR-ADM-03
should read that the review queue is not driven by negative feedback
because **no admin surface or query reads the ratings that already
exist**, not because the ratings don't exist.

## Dependencies and constraints

- **Escalation (FR-CHAT-10) is genuinely not built.** `ChatService.Escalate`
  and `ChatService.GetQuota` (FR-CHAT-12) both fall through to
  `careerv1connect.UnimplementedChatServiceHandler`, embedded in `Chat`
  (`services/api/internal/handlers/chat.go:36`). The struct's own comment
  (lines 22-30) says so plainly: "Escalate (FR-CHAT-10, nothing behind it
  yet) and GetQuota (FR-CHAT-12, no quotas yet)." `grep -rn "func.*Escalate"
  services/api/internal/handlers/` returns nothing — there is no
  escalation handler to search further into. A queue cannot be "driven
  by escalations" when escalation itself doesn't exist; this part of
  FR-ADM-03 is blocked on FR-CHAT-10, exactly as `docs/FSD.md:416`
  (FR-ADM-04) already documents for a related requirement.
- **Out-of-scope and no-support data exists but only inside
  `decision_log.output` / `chat_messages` columns**, not in any
  admin-queryable shape. Building a queue view for these needs a new
  `AdminService` filter (e.g., extend `DecisionFilter` with an
  `OutOfScopeOnly` / `NoSupportOnly` bool, mirroring `FailedOnly`) plus a
  UI affordance — no schema change, no new handler plumbing beyond the
  filter.
- **Negative-rating data exists only in `chat_messages`**, not
  `decision_log`. Surfacing it means either (a) a join in
  `ListDecisionLog`'s SQL from `decision_log.ref_id` to
  `chat_messages.id` where `ref_kind = 'chat_message'`, or (b) a
  dedicated `AdminService.ListRatedMessages`-style RPC. (a) keeps one
  review surface; (b) is a second surface an admin has to also check.
- **The decision_log write is best-effort**, by design
  (`services/api/internal/chat/answer.go:156-158,`479-484`): a chat
  answer can be shown to the member with no corresponding `decision_log`
  row if the insert fails (logged as an error, "training data lost").
  Any queue built on `decision_log` inherits this gap — a small number of
  real answers, including possibly rated ones, may have no row to review
  at all. This is a pre-existing, documented risk, not new to this
  requirement.

## Decisions the owner must make

1. **Fix the FSD text itself.** FR-CHAT-14's "Not built" and FR-ADM-03's
   "because FR-CHAT-14 is not built" are both factually wrong and should
   be corrected regardless of what else happens with this requirement —
   this is a documentation bug independent of any build decision.
2. **Where does the rating signal surface?** Join it into
   `/admin/decisions` (one review surface, more complex query) or build a
   separate small "rated answers" list (simpler query, a second place to
   check). The 9 existing production rows, including a typed comment, are
   real, actionable feedback sitting unread either way.
3. **Is the out-of-scope / no-support queue worth a dedicated filter**,
   given the data is already visible to an admin willing to open every
   row? This is a cheap addition (one bool on `DecisionFilter`, one query
   predicate) if the owner wants it prioritized rather than merely
   visible.
4. **Escalation stays out of scope for this requirement** until FR-CHAT-10
   is built — there's nothing to query yet. Worth deciding whether
   FR-ADM-03's acceptance criteria should be split so the three available
   signals (rating, out-of-scope, no-support) can ship independently of
   the fourth (escalation).
5. **Should rating ever be clearable/revisable?** Today `RATING_UNSPECIFIED`
   is explicitly refused by the handler ("a rating cannot be cleared once
   given") because the store can't represent a path back to unrated. Not
   blocking for the queue, but worth a conscious decision rather than a
   silent limitation discovered later.

## Task checklist

- [ ] Correct `docs/FSD.md` and `docs/fsd-open-items.md`: FR-CHAT-14 to
      "Shipped" (member-facing rating UI, `message.tsx`, live since
      2026-09-30); FR-ADM-03's note to say the queue isn't driven by
      negative feedback because no admin query/surface reads the ratings
      that exist, not because rating isn't built.
- [ ] Decide join-vs-separate-surface for exposing `chat_messages.rating_up`
      /`rating_comment` to an admin.
- [ ] Add a `DecisionFilter.OutOfScopeOnly` / `NoSupportOnly` (or combined
      "needs attention") predicate, mirroring the existing `FailedOnly`
      pattern, and a corresponding list-level filter control.
- [ ] Add a visible badge/column on the `/admin/decisions` list itself
      (not just inside the opened row) for out-of-scope, no-support, and
      negative-rating rows, so the queue is actually scannable rather than
      requiring every row to be opened.
- [ ] Leave escalation out of this requirement's build until FR-CHAT-10
      ships a handler; track it as an explicit follow-on, not a silent gap.
- [ ] Independently verify the production row counts (9/38) and the typed
      comment via `/admin/db` or a direct query, since this document
      relied on the figure as given rather than querying it itself.

## What could not be verified

- **The 9/38 production rating count, the 2026-10-01 date, and the typed
  comment's content** were given as context for this task and are
  consistent with the code and git history (feature shipped 2026-09-30,
  ratings dated the day after), but I did not run a query against
  production in this session to confirm the figures myself.
- **Whether any of the 9 ratings are negative (`rating_up = false`) versus
  all positive** — I have no visibility into the actual data, only the
  schema and code path that would produce it.
- **Whether members other than the account used for manual testing
  produced these ratings** — cannot be determined from the repository
  alone; would require production session/user data.
- **Whether `decision_log` rows exist for all 9 rated messages** — given
  the best-effort, can-silently-fail nature of the `decision_log` insert
  in `chat/answer.go`, it's possible one or more rated messages have no
  corresponding decision row. Not checked against production.
