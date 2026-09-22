# ADR 0020 — Registration whitelist and time-limited access

**Status:** Accepted, 2026-09-19 (owner decision). Resolves FSD D-20. Amends ADR-0002 (approval-gated registration).

## Context

ADR-0002 made approval-gated registration the default: every verified user waits in `pending_approval` for a one-click admin decision. That works for cold recruiters, but adds friction for people the owner already knows and expects to grant access to (past colleagues, hiring managers he's already spoken with, service providers, references). Two related needs came in together:

1. A way to pre-authorize known addresses so their access is granted at verify-time without a personal email round-trip.
2. Access that isn't necessarily forever — an interviewer who needs 7 days for a hiring cycle shouldn't retain member access indefinitely.

The site is a live portfolio, not confidential material (FSD A-05), so this is about *audience discipline and freshness*, not about secrecy.

## Decision

- **Whitelist.** A new `access_grants` table holds one row per pre-authorized email (case-insensitive), with a `default_ttl` chosen from `{1d, 3d, 7d, 30d, permanent}` and an optional `entry_expires_at` so a whitelist entry itself can be time-boxed. On email verification the API queries this table; a hit that has not itself expired promotes the user directly to `active`, sets `users.expires_at` from the grant's TTL (or `NULL` for permanent), and sends the "you're in" template. Roger receives no per-approval email in this case (an optional periodic digest — FR-NOTF-06.b — surfaces auto-approval volume for spot checks).
- **Time-limited access.** `users` gains a nullable `expires_at`. Every approval — whitelist auto, one-click Accept, or console — records both the decision and the effective TTL in `approval_decisions.granted_ttl`. A new `MEMBER_STATUS_EXPIRED` value in the member-status enum marks accounts whose `expires_at` has passed. Expiry runs hourly in the API's in-process scheduler: three days before `expires_at`, a warning email fires; at or past `expires_at`, the user is flipped to `expired`, every active session is revoked, and a closing email is sent.
- **TTL selection UX.** Because the admin approval email must remain a *one-click* action to keep the promise made in ADR-0002, the emailed Accept button grants the default TTL (7 days). Other TTLs require Roger to open the MFA-protected review page (FR-ADM-10), where the Accept action is a small dropdown of the five options. A manual approval always wins over the whitelist default, since the human is actively deciding.
- **Whitelist management.** A new admin surface (`/admin/whitelist`, FR-ADM-11) exposes CRUD for the table. Adding or removing entries is prospective only — existing users are unaffected; a removed entry stops future auto-approvals but does not revoke access already granted. Every mutation writes to `audit_log`.
- **Re-activation.** An `expired` whitelisted user can be extended with a one-click "extend" action in the admin console; an `expired` non-whitelisted user re-applies through the standard flow.

## Consequences

- The default flow stays "everyone waits for Roger," so nothing about the guarantees in ADR-0002 changes for people he doesn't know.
- The one-click Accept email keeps its promise (no login required, one action → done). Custom TTLs require MFA — a modest cost that rules out inbox-compromise attacks on non-default grants.
- The expiry job is a scheduled write that must be scoped correctly: only users with `status='active' AND expires_at IS NOT NULL AND expires_at < now()`; `permanent` grants (null `expires_at`) are untouched. A UBTS spec pins this once the governance harness is live (Phase 6).
- Whitelist entries are stored in plaintext (they are the *keys* used at lookup time). This is standard practice — the same as an email allow-list on any admin tool — but it is worth stating that anyone with DB access can read the list. Access to the DB is already gated by the same secrets as the API host.
- FSD FR-AUTH-11 (account deletion) remains authoritative for privacy: an expired row still exists and can still be deleted by the user; an expired non-deleted row can be re-activated by Roger without a fresh registration cycle.
- No wildcards, no domain matches, no CSV import in v1 — deliberately kept simple. Bulk import moves to §7 backlog if the list ever grows past ~50 entries.
- Phase 2 effort revised 6–8 days → 8–10 days to cover the additional surfaces, templates, and scheduler.

## Status update (2026-09-22)

- The whitelist surface lives at `/admin/access` ("Access & whitelist"), not `/admin/whitelist`. Its add / edit form defaults `default_ttl` to 7 days (`GRANT_TTL_7D` in `apps/web/src/app/admin/(console)/access/grant-form.tsx`); the five TTL values are unchanged.
- The default access window everywhere is 7 days: the emailed one-click Accept grants 7 days (`AcceptDefaultTTL`), a whitelist hit grants the entry's TTL (7 days unless changed), and the registration detail page shows the expiry with day-preset and "permanent" extension actions (`ExtendAccess` on `AdminService`).
- The hourly expiry job, the 3-day warning and the closing email are in `services/api/internal/handlers/expiry.go`, as decided. An `expired` user is refused at sign-in with "your access has expired, reach out to Roger to renew" (`handlers/auth_session.go`); renewal is the owner extending access from `/admin/registrations/[id]`, not a new application (verified 2026-09-22).
