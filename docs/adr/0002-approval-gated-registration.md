# ADR 0002 — Approval-gated registration

**Status:** Accepted, 2026-09-19 (owner decision). Resolves FSD D-02.

## Context

The default option in FSD v0.3.2 was "open registration with email verification, approval mode as a flag for later." The owner has decided that approval-gated registration is the correct default from launch:

- The audience is small (hundreds of members over the site's life; FSD A-02).
- The material — including résumés, project narratives, and admin activity — is professional-context content the owner wants a light human review over before granting a session.
- The owner is willing to act as the single approver.
- Because the repository is public (FSD A-05), the gate provides identity and personalization rather than confidentiality, so approval does not need to be strict; it needs to be *easy for the owner to act on*.

The design goal is therefore: the applicant experience stays low-friction, and the owner's decision takes one click from an email inbox on any device.

## Decision

- After email verification (password registration) or successful OIDC callback (LinkedIn), a new user enters state `pending_approval`; no session is issued and no gated route is served.
- The API sends a transactional email to `OWNER_CONTACT_EMAIL` containing the applicant's context (name, email, organization, stated role, submitted at, IP hash, user-agent, reference ID) and two buttons: **Accept** and **Decline**. Each button is a single-use, HMAC-signed URL binding `user_id`, decision, issued-at, and 7-day expiry, using a server-side signing key. Clicking a button requires no admin sign-in.
- A successful Accept click marks the user `active`, records the decision in `approval_decisions`, and triggers the user "approved" email (sign-in URL, owner's contact address for questions).
- A successful Decline click marks the user `declined`, records the decision, and triggers the user "declined" email — plain-language notice that the admin did not recognize the credentials, an invitation to contact the owner if the applicant believes the decision is in error, and a pasteable context block (reference ID, applicant fields, submitted at, signed re-review URL to the admin *Pending approvals* surface) the applicant can copy into a reply.
- If no decision is made within 7 days, the scheduler auto-declines the user (`decision = auto_decline`, `decided_by = null`, `decided_via = scheduler`) and sends the auto-decline template inviting reapplication.
- The admin retains an MFA-protected review surface (FR-ADM-10) that lists every pending user, allows Approve / Decline from within an authenticated session, shows decision history, and can re-open or override a prior decision (recorded via `superseded_by`). The signed re-review URL in the decline email lands on this surface.
- Approval applies equally to password and OIDC registrations; OIDC skips email verification (the provider supplied a verified email) but still enters `pending_approval`.

## Consequences

- The registration flow is one step longer than the FSD's original default, and the median time from *submit* to *first sign-in* now depends on the owner's response latency. G2 (§2.3, "time to relevance") is measured *from the first verified login*, so it is not affected; G1 (registration → verified-member completion rate) becomes registration → **approved-member** completion rate and is expected to be lower under this policy. The target is retained pending observation.
- The four approval-workflow email templates (FR-NOTF-05) are on the Phase 2 critical path, which means Resend (FR-NOTF-03) must be wired and SPF/DKIM/DMARC records in place before Phase 2 exits. The admin *Pending approvals* surface (FR-ADM-10) is promoted from Phase 5 to Phase 2 so that the decline email has a working review destination.
- Compromise of the owner's inbox exposes one-click approval, which is why every decision is auditable (`approval_decisions`), tokens are single-use with a 7-day TTL, and the MFA-protected review surface can override any decision made via a one-click link.
- Invite-code mode (the third option in the original D-02) moves to §7 backlog; if the owner later wants a delegated approval path, an invite grants pre-approval and skips the pending state.
- The site retains the option to relax to open-with-verification post-launch by defaulting the approval decision to auto-approve; the schema, emails, and admin surface remain useful either way.

## Status update (2026-09-22)

- Extended by ADR-0020: an approval now carries an access window. The one-click Accept in the owner's email grants the default 7-day TTL (`AcceptDefaultTTL` in `services/api/internal/handlers/decision.go`); other windows, including permanent, are chosen on the admin surface.
- The "Pending approvals" surface is `/admin/registrations`; the detail page shows the member's expiry and offers day-preset extensions and "permanent".
- Everything else in this record still holds: the pending state, the signed single-use decision links with a 7-day TTL, the scheduler auto-decline after 7 days (`services/api/internal/handlers/expiry.go`), and the audit trail in `approval_decisions`.
