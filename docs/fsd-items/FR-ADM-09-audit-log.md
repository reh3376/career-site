# FR-ADM-09: the audit log

**State as of 2026-10-12.** The `audit_log` table has existed since the
first migration, has the right shape to receive rows, and has never
received one: no code path in the repository inserts into it, and
production holds 0 rows (verified 2026-10-12). FR-ADM-09 and FR-ADM-11
both promise admin actions land in it; neither promise is kept. What
exists instead is a product event stream that covers some admin
actions well (with actor identity) and one admin action poorly (without
it), plus four mutating RPCs that accept a `reason` string the proto
comment says is "recorded in the audit log" and that is discarded the
moment it reaches the handler. ADR 0031 (2026-10-12) decided the
direction — write to it, or remove the table and the three requirements
that cite it — and named this the cheapest of its three open items and
the easiest to do badly, because a log nobody reads is another empty
control.

## What the FSD requires

`docs/FSD.md:421`, FR-ADM-09:

> All admin actions shall be recorded in the audit log.

Status column: "Must (partially **Shipped**)". The notes column, in
full:

> Partially shipped, and the gap is wider than it reads. Admin actions
> emit registry events (`admin.decision_reviewed`, `admin.qa_entry_*`,
> `admin.rescore`, `admin.jd_limit_changed`), which is real coverage of
> *some* actions. The `audit_log` table has existed since migration
> 00001 and **nothing in the service has ever written to it**: no code
> path inserts, and production holds 0 rows (checked 2026-10-12). So
> any requirement or UI copy that promises "recorded in the audit log"
> is promising a table that is empty, and `ExtendAccess` accepts a
> `reason` field that is not persisted anywhere. Either drop the table
> and the promise, or write to it.

`docs/FSD.md:423`, FR-ADM-11 (the access & whitelist surface):

> The admin shall have an *Access & whitelist* surface (`/admin/access`)
> listing every `access_grants` entry (email, default TTL, notes, entry
> expiry, created by, created at) with add / edit / remove actions.
> Adding an entry does not retroactively affect existing users; it only
> affects future registrations. Removing an entry does not revoke
> access already granted; it only stops future auto-approvals. Access
> requires an admin session; every mutation is written to `audit_log`.

Status column: "Must (**D-20**; **Shipped**)" — i.e. the surface itself
shipped; the audit clause inside the row did not. Notes column, in
part:

> Nothing writes `audit_log` today (no code path inserts into it); the
> MFA-fresh gate was declared and never enforced, and `mfa_fresh` was
> removed from every method on 2026-10-09 when the owner decided
> against a second factor.

## What exists today

**The table.** `services/api/internal/db/migrations/00001_init.sql:136-149`:

```sql
CREATE TABLE audit_log (
  id             bigserial PRIMARY KEY,
  actor_user_id  bigint REFERENCES users(id) ON DELETE SET NULL,
  action         text NOT NULL,
  target_type    text NOT NULL DEFAULT '',
  target_id      text NOT NULL DEFAULT '',
  payload_json   jsonb NOT NULL DEFAULT '{}'::jsonb,
  ip_hash        bytea,
  occurred_at    timestamptz NOT NULL DEFAULT now()
);
```

Indexed on `actor_user_id`, `action`, and `occurred_at DESC`
(`services/api/internal/db/migrations/00001_init.sql:147-149`). The
shape is sound for the job: actor, action name, a typed target, a JSON
payload for whatever varies by action, an IP hash, a timestamp. Nothing
about the schema is blocking the write.

**Confirmed empty of writers.** A repo-wide search for `audit_log`
across `.go`, `.sql`, `.ts`, and `.tsx` returns only the migration file
itself — the `CREATE TABLE` and its three `CREATE INDEX` statements and
the `DROP TABLE` in the migration's down-step. No handler, job, or
script references it.

**The event registry covers some admin actions, with actor identity.**
`docs/events/README.md` describes one append-only `events` table that
already records, among much else, five admin-originated names, each
emitted with the acting admin's own ID as `UserID`
(`services/api/internal/handlers/events_wire.go:71-81`, `requestEvent`
takes the actor id as a direct parameter):

| Event | Emitted from | Actor captured as |
|---|---|---|
| `admin.decision_reviewed` | `services/api/internal/handlers/admin_decisions.go:117` | `me.ID` |
| `admin.rescore` | `services/api/internal/handlers/admin.go:1339` | `admin.ID` |
| `admin.jd_limit_changed` | `services/api/internal/handlers/admin_fit.go:86` | `admin.ID` |
| `admin.fit_bands_changed` | `services/api/internal/handlers/admin_fit.go:49` | `admin.ID` |
| `admin.qa_entry_created` | `services/api/internal/handlers/admin_qa.go:98` | `me.ID` |
| `admin.qa_entry_updated` | `services/api/internal/handlers/admin_qa.go:133` | `me.ID` |
| `admin.qa_entry_enabled` | `services/api/internal/handlers/admin_qa.go:160` | `me.ID` |
| `admin.qa_entry_deleted` | `services/api/internal/handlers/admin_qa.go:185` | `me.ID` |

For these eight, "who did what, when" is already answered by a query
against `events`, and has been since each shipped.

**One admin action emits an event without the actor.** Registration
approve/decline (`AdminService.ApproveRegistration` /
`DeclineRegistration`) route through
`services/api/internal/handlers/decision.go:147` (`ApproveUser`) and
`:179` (`DeclineUser`), which both emit `approval.decided`
(`decision.go:167`, `:193`) with `UserID: u.ID` — the member being
approved or declined, not the admin doing it. The doc comment on
`ApproveUser` (`decision.go:144-146`) says `via` ("email_link" vs.
"console") "distinguishes them in the audit log" — language written
before anyone checked whether an audit log entry exists; it does not.
The same functions call `h.users.RecordApprovalDecision` into the
`approval_decisions` table (`services/api/internal/db/migrations/00001_init.sql:118-129`),
which has a `decided_by` column for exactly this purpose
(`services/api/internal/users/access_grants.go:210`), but the call
site never sets it (`decision.go:154-160`, `:184-189` pass `UserID`,
`Decision`, `DecidedVia`, `GrantedTTL`/none, `TokenHash` — no
`DecidedBy`). So for a console approve/decline, there is today no
stored record of which admin acted, anywhere, by any mechanism.

**Two admin actions emit no event at all.** `SetMemberStatus`
(`services/api/internal/handlers/admin.go:217-262`) and `ExtendAccess`
(`admin.go:406-463`) contain no `a.events.Emit` call. Disabling a
member, reactivating one, or changing their expiry leaves no row in
`events` and no row in `audit_log` — only the mutation to the `users`
row itself, with no history of the change.

**A working precedent for exactly this pattern already exists**, just
not for admin actions on members. Migration `00056_decision_test_review.sql`
built `dt_review_events` (`:121-139`) specifically so that a curation
judgement is never silently overwritten: every status change is
appended with `reason`, `note`, `reviewed_by`, and `created_at`, and the
migration's own comment states the reasoning plainly (`:121-128`):

> Every judgement, appended. [...] "why is this excluded, and since
> when" is the question somebody will ask about a published finding,
> and an overwrite cannot answer it.

That table's `reason` column is actually read:
`services/api/internal/handlers/admin_decisiontest.go:185` and `:363`
both reference `req.Msg.GetReason()` / `e.Reason` on live code paths.
It is the one place in the codebase where a `reason` field on an admin
mutation is not discarded, and the one place an append-only per-change
table like `audit_log` already exists and works. It is the model to
follow, not `audit_log`'s schema itself, which is close but untested
by any writer.

**Four mutating RPCs already carry a `reason` field that is discarded.**
`proto/career/v1/admin.proto`:

| RPC / message | `reason` field | Proto comment |
|---|---|---|
| `ApproveRegistrationRequest` | `:1230-1231` | "Optional free-text reason recorded in the audit log." |
| `DeclineRegistrationRequest` | `:1247-1248` | "Optional free-text reason recorded in the audit log." |
| `ExtendAccessRequest` | `:1275-1276` | "Optional free-text reason recorded in the audit log." |
| `SetMemberStatusRequest` | `:681-682` | "Reason, recorded in the audit log." |

A repo-wide search for `req.Msg.GetReason()` / `req.Msg.Reason` across
`services/api/internal/handlers/*.go` turns up exactly one hit, and
it belongs to a different feature: `admin_decisiontest.go:363`
(`Reason: req.Msg.GetReason()`, the decision-test review RPC, writing
to `dt_review_events` above). None of the four handlers above —
`ApproveRegistration` (`admin.go:475-496`), `DeclineRegistration`
(`:500-521`), `ExtendAccess` (`:406-463`), `SetMemberStatus`
(`:217-262`) — read `req.Msg.Reason` at all. The value a caller sends
is validated by `buf.validate` (max 1000 or 32 chars), accepted by the
handler, and then never referenced again in that function body.

**No console UI collects the reason today either.** A search for
`reason` across `apps/web/src/app/(site)/admin/` matches only the
decision-test review surface and three unrelated files (`corpus`
ingest-form, `decisions` page, `jd/[id]` page checked and found
unrelated to this field). The registrations action button, the access
grant form, and any member-status control do not have a reason input
in the UI — so even where the proto field exists and is wired through
to a handler, nothing today lets an admin type into it.

**No console surface displays an audit log.** The admin console tree
under `apps/web/src/app/(site)/admin/(console)/` has an `activity/`
page, but it is per-*member* engagement (`activity/page.tsx:89-178`:
sessions, active time, Ask Roger count, last event — sourced from
`ListMemberActivity`), not a log of admin actions. There is no
`audit`, `audit-log`, or equivalent route anywhere in that tree. An
admin wanting "who approved this member, and why" has no page to open;
the closest thing is the read-only SQL console at `/admin/db`
(`db/page.tsx`, `db/query-console.tsx`), which can query `events` or
`approval_decisions` directly but presents neither as a log.

## What is missing

1. No insert path into `audit_log` exists anywhere in the service.
2. No admin console route reads or renders `audit_log`, or any
   equivalent audit view assembled from existing tables.
3. The `reason` field on four mutating RPCs (`ApproveRegistration`,
   `DeclineRegistration`, `ExtendAccess`, `SetMemberStatus`) is accepted
   by the proto, validated, and discarded by every handler.
4. The console UI does not collect `reason` for any of those four
   actions, so even a fixed backend would have nothing to receive from
   today's forms without a UI change.
5. `ApproveRegistration` / `DeclineRegistration` lose the acting admin's
   identity entirely: `approval_decisions.decided_by` is never set, and
   the `approval.decided` event carries the target member's id, not the
   admin's.
6. `SetMemberStatus` and `ExtendAccess` emit no event and no audit row —
   the only trace of either action is the resulting state on the
   `users` row, with no record of when it changed or who changed it.

## Dependencies and constraints

- **Schema is ready.** `audit_log` needs no migration to start
  receiving rows; the columns (`actor_user_id`, `action`, `target_type`,
  `target_id`, `payload_json`, `ip_hash`, `occurred_at`) already cover
  actor, action, target, and a JSON payload for whatever varies by
  action.
- **The `reason` plumbing is the cheap part**, per ADR 0031's framing
  (`docs/adr/0031-declared-controls-are-enforced-controls.md:97-98`):
  "`audit_log` is the cheapest of the three and the easiest to do
  badly: a log nobody reads is another empty control." The fields
  already exist on the request protos; the work is the insert and the
  read surface, not new plumbing (`docs/backlog.md:1191-1194`).
- **The read surface is not optional.** Both the ADR and the backlog
  entry (`docs/backlog.md:1200-1201`) treat a write-only table as a
  repeat of the same failure: "Whatever lands should be reachable from
  the console, or it is another empty control."
- **`dt_review_events` is the pattern to copy**, not `audit_log`'s own
  (unused, untested) design: append-only, one row per change, actor +
  reason + note + timestamp, never overwritten. Building `audit_log`
  writes in that same shape — one insert per state-changing call,
  never an update — avoids re-deriving the design from scratch.
- **Scope boundary with the event registry**: `events` already has
  working actor-correct writes for eight admin actions
  (`admin.decision_reviewed`, `admin.rescore`, `admin.jd_limit_changed`,
  `admin.fit_bands_changed`, four `admin.qa_entry_*` names). Whatever
  design is chosen should not duplicate that coverage; the open gap is
  specifically the member-lifecycle actions (approve, decline, set
  status, extend access) and the lost-actor case on approve/decline.
- **ADR 0031's stated alternative is real**: drop the table and the
  two requirement rows that promise it, rather than build the write
  path. The ADR did not pick between "write to it" and "remove it" —
  it named both as live options and deferred the choice to the owner
  (`docs/adr/0031-...md:58-60`: "`audit_log` is written on the mutating
  admin paths [...] or the table and the three requirements that
  promise it are removed").

## Decisions the owner must make

1. **Write to `audit_log`, or delete it and correct FR-ADM-09 /
   FR-ADM-11 to stop promising it.** ADR 0031 named this choice and
   left it open.
2. **If writing to it: one insert function, called from how many call
   sites?** The candidates are the four RPCs with a discarded `reason`
   (`ApproveRegistration`, `DeclineRegistration`, `ExtendAccess`,
   `SetMemberStatus`) plus the two that already have actor-correct
   `events` coverage but might still warrant an `audit_log` row (the
   eight listed in "What exists today") if the intent is "every admin
   action" rather than "every member-lifecycle action."
3. **Does `audit_log` replace or sit alongside `events` and
   `approval_decisions`?** Three mechanisms already overlap in partial
   and inconsistent ways (`events` for some actions with actor,
   `approval_decisions` for approve/decline without actor,
   `audit_log` for nothing). A decision to keep three is a decision to
   keep reconciling them.
4. **Fix the lost-actor gap on approve/decline regardless of the above.**
   `approval_decisions.decided_by` already exists and is simply never
   populated at the one call site that matters
   (`decision.go:154-160`, `:184-189`). This is independent of whether
   `audit_log` gets written to.
5. **Does the console UI need a reason input before the backend reason
   plumbing is worth fixing?** A reason field nobody can type into has
   no value filled in to discard or to keep.
6. **What does the read surface look like, and where does it live?**
   Options include a new `/admin/audit` route, folding it into the
   existing `/admin/db` SQL console (already capable of querying
   `audit_log` once rows exist), or a per-member audit tab on the
   existing registrations detail page (`registrations/[id]/page.tsx`).
7. **Retention and the events-anonymize precedent.** `events` has a
   daily anonymize job that blanks identity columns after
   `EVENT_IDENTITY_RETENTION_DAYS` (`docs/events/README.md`, "Privacy
   rules"). Decide whether `audit_log`, as a record of admin actions on
   members, should ever anonymize its `actor_user_id` — an audit trail
   that forgets who acted undermines the point of having one.

## Task checklist

Ordered; each step assumes the ones before it are done.

1. Decide: write to `audit_log`, or remove it and correct FR-ADM-09 /
   FR-ADM-11. (Decision 1 above — blocks everything else.)
2. If removing: delete the table in a new migration, update FR-ADM-09
   and FR-ADM-11 in `docs/FSD.md` to drop the audit-log clause, and
   close out this backlog item and the corresponding line in ADR 0031.
   Stop here on this path.
3. If writing: settle Decision 3 (relationship to `events` and
   `approval_decisions`) before touching code, since it determines
   whether this is one new write path or a consolidation of three.
4. Write one `audit_log` insert helper (actor, action, target_type,
   target_id, payload_json, ip_hash) modeled on `dt_review_events`'s
   append-only shape, not on any existing `audit_log`-specific code,
   since none exists.
5. Wire the helper into `SetMemberStatus` (`admin.go:217-262`) and
   `ExtendAccess` (`admin.go:406-463`) first — these are the two
   actions with no record of any kind today.
6. Wire the helper into `ApproveRegistration` / `DeclineRegistration`
   (`admin.go:475-521`, via `decision.go:147`/`179`), and at the same
   call sites, set `approval_decisions.decided_by` to the acting
   admin's id (`decision.go:154-160`, `:184-189`) so the existing table
   stops silently losing the actor.
7. Add a `reason` input to the console UI for all four actions
   (registrations approve/decline action, access/extend-access
   control, member-status control), since none collects it today.
8. Confirm each of the four handlers now reads `req.Msg.Reason` (or
   `GetReason()`) and persists it in the `audit_log` payload — today
   none of them do.
9. Build the read surface: a route (new `/admin/audit`, or an
   extension of an existing page per Decision 6) that lists `audit_log`
   rows with actor, action, target, reason, and timestamp, filterable
   at minimum by target member.
10. Verify live in production: perform one of each of the four actions,
    confirm a row lands in `audit_log` with the correct actor and
    reason, and confirm the new read surface shows it. This is the
    check ADR 0031 and the backlog entry both call out as the one that
    actually distinguishes "working" from "declared" — compare to how
    the auth-interceptor and Turnstile items in the same ADR were
    verified by observing real behavior change, not by reading the code
    that was supposed to cause it.
11. Update FR-ADM-09 and FR-ADM-11 in `docs/FSD.md` from "partially
    shipped" / the discrepancy note to "Shipped," with the date, and
    update `docs/backlog.md` to close this item and `docs/adr/0031-...md`
    if it tracks per-item closure.

## What could not be verified

- Whether any other admin RPC beyond the six named here (the four with
  a discarded `reason`, plus `ApproveRegistration`/`DeclineRegistration`
  losing actor identity) is missing audit coverage was not exhaustively
  checked against every handler in
  `services/api/internal/handlers/admin*.go` — the registry README's
  own admin event list and a targeted grep were used as the coverage
  map, not a line-by-line read of every admin handler file.
- Whether `payload_json`'s shape (an untyed JSONB blob) is adequate for
  before/after values on `ExtendAccess` and `SetMemberStatus` specifically
  (e.g. old vs. new `expires_at`, old vs. new status) was not evaluated
  against what a reader of the audit log would actually need to see;
  this is a design question for whoever writes the insert helper, not
  a verified fact about the current schema.
- Whether `ip_hash` on `audit_log` is meant to use the same salt and
  truncation scheme as `events.ip_hash` (`EVENT_IP_SALT`, first 8 bytes
  of SHA-256) was not checked; there is no existing writer to compare
  against.
- Retention policy for `audit_log` (Decision 7) was not investigated
  beyond noting the `events` precedent; no owner decision on it exists
  yet to record.
