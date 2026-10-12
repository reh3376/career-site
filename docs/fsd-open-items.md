# FSD open items, for ranking

Working document, opened 2026-10-12. Every FSD row not recorded as fully
shipped, so we can decide what stays, add the context that makes each one
judgeable, and rank them.

**Two kinds of item here.** *Not built* is what it says. *Partially
shipped* is a row the FSD marks shipped with a hole in it, and those were
invisible until the documentation audit on 2026-10-12 went looking:
`FR-CNT-22` promised Turnstile on anonymous contact and enforced none,
`FR-AUTH-09` promised it on registration and had no check at all (both
closed 2026-10-12), `FR-ADM-09` promised an audit log whose table had
never been written to (closed 2026-10-12).
A gap behind a "Shipped" marker is likelier to bite than something nobody
started, because nobody is looking for it.

**One decision already collapses several.** D-31 (ADR 0031): a declared
control is an enforced control. That fixes the direction on the Turnstile
pair, on `audit_log`, and on the `buf.validate` gap behind several
contract rows. Those still need ranking and costing; they no longer need
a decision.

**Counting note.** This list said 61 items when first generated on
2026-10-12 and the number was wrong. The classifier matched the word
"partial" anywhere in a row, which caught `FR-EVAL-09`, whose status
discusses a partial *run*, and `FR-DT-09`, which has an outcome literally
named `partial`; both are shipped. `FR-ADM-13` was also listed as
partial on the strength of a status line reading "Not yet: the connect
flow, the encrypted refresh token, and the bookings list" — all three
exist. Reclassifying from the priority cell alone gives the figures
below.

| | |
|---|---|
| Not built | 45 |
| Partially shipped | 11 |
| **Total** | **56** |

`FR-AUTH-12` (TOTP MFA) is deliberately absent: cancelled by the owner's
2026-10-09 decision, not pending.

**How to use it.** For each item: decide *keep / drop / defer*, add the
context that makes it judgeable, then rank. The partially-shipped section
below is ranked as of 2026-10-12; the not-built section is not yet.

---

# Partially shipped — a hole behind a Shipped marker

Ranked 2026-10-12. Ranks 1 and 2 are decided by D-31 and need only costing.

### 1. FR-AUTH-09 · Must — CLOSED 2026-10-12

> Authentication endpoints shall be rate-limited per IP and per account, with progressive delays after failed attempts, and registration shall be protected by Cloudflare Turnstile (or equivalent).

**Closed.** Turnstile is verified on `Register` against Cloudflare's
siteverify endpoint, and `Register` gained the per-IP rate limit it never
had. Misconfiguration is a boot failure; an unreachable Cloudflare admits
the request and logs, which is why the rate limit matters. Still open and
deliberately out of scope: progressive delays, and a limiter that
survives more than one process. Those are not this row's Turnstile
promise, so they are tracked in `docs/backlog.md` rather than held here.

### 1. FR-CNT-22 · Must — CLOSED 2026-10-12

> Contact / support form (public + member views differ)…

**Closed.** `turnstileEnforced()`, which returned false unconditionally,
is gone; the anonymous path verifies the token through the same client as
`Register`. Nothing else in the row changed.

### 2. FR-ADM-09 · Must — CLOSED 2026-10-12

> All admin actions shall be recorded in the audit log.

**Closed.** Every ADMIN method now declares whether it is recorded, the
server refuses to start if one does not, and an interceptor writes the
row. 42 of 81 admin procedures are recorded. The read surface at
`/admin/audit` ships with it, because a write-only table would have been
the next empty control. The lost-actor bug on approve/decline is fixed
in the same change. Retention was decided rather than deferred:
`audit_log` is never anonymised, unlike `events`.

### 3. FR-CNT-16 · Must — CLOSED 2026-10-12

**Closed.** The validation half of FR-CNT-16 now exists as
`scripts/check_content.py`, wired into CI before the build, with a
schema authored from the content that exists rather than from §7.2.
`id` and `type` were added everywhere. `tracks`, `related` and
`summary` are deliberately not added and are named as deferred in the
FSD row: nothing reads them, and `tracks` waits on FR-PROF-03.

### 4. FR-CNT-17 · Must — CLOSED 2026-10-12

**Closed.** The validation half of FR-CNT-16 now exists as
`scripts/check_content.py`, wired into CI before the build, with a
schema authored from the content that exists rather than from §7.2.
`id` and `type` were added everywhere. `tracks`, `related` and
`summary` are deliberately not added and are named as deferred in the
FSD row: nothing reads them, and `tracks` waits on FR-PROF-03.

### 5. FR-HIST-02 · Must

> Returning members shall see a welcome-back panel: last visit date, up to three items viewed, topics asked about, and items new since the last visit.

**FSD status today:** Partially shipped. `/home` shows a RECENT ACTIVITY list and what is new since last time. No last-visit date and no three-item resume list.

**Why here:** Half there: /home already shows recent activity and what is new. Missing the last-visit date and the three-item resume.

- **Keep / drop / defer:**
- **Depends on:**

### 6. FR-ADM-03 · Must

> The admin shall have a conversation-review queue: negative feedback, "I don't know" answers, escalations, and out-of-scope refusals.

**FSD status today:** Partially shipped. `/admin/decisions` reviews and grades `decision_log`, including chat answers. Not driven by negative feedback, because FR-CHAT-14 (member rating) is not built.

**Why here:** Needs verifying before costing — not yet checked against the code.

- **Keep / drop / defer:**
- **Depends on:**

### 7. FR-EVAL-06 · Must

> An evaluation shall not be silently destroyed by a deploy.

**FSD status today:** The job runner keeps jobs in memory, so recreating the api container kills a run. `/admin/ops` (FR-ADM-19) answers whether one is in flight, which is the control today. All three services share one `IMAGE_TAG`, so there is no partial deploy that avoids this; per-service tags are in the backlog.

**Why here:** Real architectural work: the job runner is in-memory. /admin/ops is the mitigation and it is a control, not a fix.

- **Keep / drop / defer:**
- **Depends on:**

### 8. FR-CNT-19 · Should

> External links shall open in a new tab with `rel="noopener"`, and CI shall check link health on content changes.

**FSD status today:** Partially shipped. External links carry `rel="noopener noreferrer"` and `target="_blank"`. No CI link checker.

**Why here:** Mechanical. rel="noopener" is done; the CI link checker is not.

- **Keep / drop / defer:**
- **Depends on:**

### 9. FR-DT-20 · Should

> The tap-along check shall separate real tracking from mashing, on thresholds set from pilot data rather than guessed.

**FSD status today:** The check is built and its result and baseline reaction time are recorded on every session. The cut is provisional: at least seven of ten presses inside 300 ms with an offset standard deviation at or under 80 ms. The design document's starting figure of 150 ms is unreachable, since the maximum…

**Why here:** Blocked on participant data, not on code. Thresholds cannot be set from pilot data that does not exist yet.

- **Keep / drop / defer:**
- **Depends on:**

### 10. FR-PUB-05 · Could

> The site shall render acceptably with JavaScript disabled for the landing and legal pages.

**FSD status today:** Partially shipped. Pages are server-rendered so the landing and legal pages read without JavaScript; the mode toggle, the assistant panel and the action modals need it.

**Why here:** Arguably already acceptable: the landing and legal pages read without JS. Only the toggle, assistant and modals need it.

- **Keep / drop / defer:**
- **Depends on:**

---

# Not built

Not yet ranked.

## Content (CNT) — 12

### FR-CNT-01 · Must

> Timeline. The site shall present roles chronologically with organization, title, dates, location, summary, quantified accomplishments, technologies, and links to related projects, articles, awards, and photos.

**FSD status today:** Not built. No `/timeline` route; it 404s (roadmap 4.2).

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-02 · Should

> Accomplishments shall be expandable to a situation / action / result detail where the owner provides it.

**FSD status today:** Not built. Depends on FR-CNT-01.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-03 · Must

> Projects. The site shall present a catalog of projects and repositories with name, one-line description, the owner's role, status, technologies, and links.

**FSD status today:** Not built. No `/projects` route. The landing's curated GitHub cards cover part of the intent; see FR-CNT-25.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-05 · Must

> Each project shall have a detail page: problem, approach, architecture (diagram where available), outcomes, links, related writing.

**FSD status today:** Not built. No project detail pages.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-08 · Must

> Presentations. The site shall present decks and papers with an in-page PDF viewer, download, event, date, and abstract.

**FSD status today:** Not built. No presentations surface or in-page PDF viewer.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-09 · Must

> Skills. The site shall present a skills matrix (skill, category, depth level, years, evidence links) where every entry links to at least one piece of evidence.

**FSD status today:** Not built. No skills matrix.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-10 · Must

> Awards and credentials. The site shall present awards, education, military service, and eligibility statements as the owner chooses to publish them.

**FSD status today:** Not built as a surface. The awards exist in the career facts sheet and three now appear on the landing.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-12 · Could

> The hero image on the tailored home page may vary by primary track (e.g., plant-floor photo for automation tracks, portrait for leadership tracks).

**FSD status today:** Not built. Depends on FR-PROF tracks, which are not built.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-13 · Must

> Downloads. The site shall offer résumé/CV PDF variants, versioned, with downloads recorded to the member's history.

**FSD status today:** Not built, and declined 2026-10-01. See roadmap 1.5: a generic public résumé invites the keyword screening the site exists to argue against. The JD reviewer's tailored résumé is the intended path.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-14 · Should

> Résumé variants should be generated from structured content (`content/resumes/*.yaml` + role data) by a build script so that site and PDFs never diverge.

**FSD status today:** Not built as described. The JD reviewer does generate a résumé per posting from structured records, which is the stronger form of this.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-18 · Should

> A "What's new" feed shall list items added or materially updated in the last 90 days.

**FSD status today:** Not built. No what's-new feed.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-24 · Should

> Quote of the day. The public landing page shall show one rotating quote from a curated set of 15–100 short quotations. Rotation is deterministic per calendar day (same visitor, same day, same quote). Quotes carry optional attribution and a…

**FSD status today:** Not built. No quote-of-the-day surface.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Profile and tailoring (PROF) — 8

### FR-PROF-01 · Must

> On first verified sign-in, members shall be offered a questionnaire of at most five skippable questions: role being hired for, seniority, up to three tracks, what matters most, and how they heard of the owner.

**FSD status today:** Not built. No onboarding questionnaire.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-PROF-02 · Must

> Tracks shall be defined in `content/tracks.yaml` (initial set in §7.3) and be changeable without code changes.

**FSD status today:** Not built. `content/tracks.yaml` does not exist.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-PROF-03 · Must

> Every content item shall carry track weights (0–1 per track) and the home page and section listings shall order and highlight items by relevance to the member's declared tracks.

**FSD status today:** Not built. Content items carry no track weights.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-PROF-04 · Should

> Each track shall define a "Start here" reading path of five to eight items.

**FSD status today:** Not built. No reading paths.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-PROF-05 · Must

> Members shall be able to change their tracks at any time and toggle *Show everything* to disable tailoring.

**FSD status today:** Not built. No tracks to change.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-PROF-06 · Must

> Tailoring shall never remove items from listings; it only changes order and emphasis.

**FSD status today:** Not built. No tailoring exists, so the constraint is vacuous today.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-PROF-07 · Should

> Every tailored block shall show a one-line reason and a link to change interests.

**FSD status today:** Not built. No tailored blocks.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-PROF-08 · Should

> The downloads section shall present the résumé variant that best matches the member's tracks first, with all variants available.

**FSD status today:** Not built, and overtaken: the JD reviewer writes a résumé for a specific posting, which is the stronger form of the same idea. See roadmap 1.5, where a generic public résumé was declined.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Ask Roger (CHAT) — 7

### FR-CHAT-10 · Should

> Members shall be able to escalate any conversation with *Ask Roger directly*; the owner receives the question with context, and the member is notified by email when the owner replies.

**FSD status today:** Not built. No escalation path from a conversation; the only `escalat` references are legacy activity events.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CHAT-11 · Should

> Responses shall stream token-by-token; first token within 2 s at p50 and 5 s at p95.

**FSD status today:** Not met, and not on this hardware. Answers are single-shot, not streamed (`chat/answer.go` states the gateway is single-shot). Measured on production: 39.4 s mean on the model path, 0.1 s from the Q&A bank.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CHAT-12 · Must

> Usage shall be limited per member (default 50 messages/day, 20 per conversation before a fresh-start prompt) and globally by a monthly budget cap that switches the assistant to Q&A-bank-only mode when reached.

**FSD status today:** Not built. No per-member message quota exists.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CHAT-14 · Should

> Members shall be able to rate each answer (up/down) with an optional comment.

**FSD status today:** Not built. No answer rating in the member UI; grading is admin-only via `decision_log`.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CHAT-15 · Should

> The assistant shall be evaluated against a golden question set (≥ 50 questions incl. out-of-scope and adversarial) with automated grading, run when corpus, prompt, or retrieval code changes.

**FSD status today:** Not built. The Q&A bank holds 13 approved entries; there is no golden question set.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CHAT-19 · Should

> With the member's consent (default on, disclosed in FR-AUTH-10), the assistant may use a summary of the member's own history (tracks, viewed items, previous questions) to personalize answers and suggestions.

**FSD status today:** Not built. No rolling conversation summary or memory layer.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CHAT-21 · Could

> The assistant's retrieval and memory layer may be implemented on MDEMG in a later phase as a live demonstration of the owner's framework.

**FSD status today:** Not built, by design. Explicitly a later phase.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Auth and accounts (AUTH) — 6

### FR-AUTH-04 · Should

> The site shall offer *Continue with LinkedIn* (OpenID Connect).

**FSD status today:** Not built. No LinkedIn OIDC.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-AUTH-05 · Could

> The site may offer *Continue with GitHub* and *Continue with Google*.

**FSD status today:** Not built. No GitHub or Google sign-in.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-AUTH-06 · Could

> The site may offer passkeys (WebAuthn) as an additional sign-in method.

**FSD status today:** Not built. No WebAuthn.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-AUTH-10 · Must

> At registration the visitor shall explicitly acknowledge that their activity on the site and their conversations with the assistant are stored and visible to the owner.

**FSD status today:** Not built. No explicit activity-recording acknowledgement at registration. Worth noting against FR-CHAT-19, which assumed this consent exists.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-AUTH-11 · Must

> Members shall be able to change email (re-verification required), change password (current password required), and delete their account.

**FSD status today:** Not built. `/settings` offers sign-out only: no change email, change password, or close account.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-AUTH-12 · Must

> The admin role shall be granted only to allow-listed emails. (This also required TOTP MFA at sign-in until 2026-10-12.)

**FSD status today:** Allow-list shipped; MFA cancelled. The owner decided on 2026-10-09 not to build a second factor, and `mfa_fresh` was removed from all 79 methods that declared it. This is not a pending requirement.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## History and data rights (HIST) — 5

### FR-HIST-03 · Should

> Members shall be able to resume their reading path with unread indicators.

**FSD status today:** Not built. No reading paths to resume, no unread indicators.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-HIST-04 · Should

> Members shall be able to save items to a personal list and add private notes.

**FSD status today:** Not built. No saved list, no private notes.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-HIST-05 · Must

> Members shall be able to view and export all data held about them (profile, activity, conversations) as JSON, and delete it.

**FSD status today:** Not built. No member-facing export of held data.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-HIST-06 · Should

> Activity data shall be retained for 24 months from the member's last activity, then deleted or anonymized.

**FSD status today:** Not built. No 24-month retention job for activity data. `events-anonymize` runs daily but is a different control.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-HIST-07 · Could

> Members may generate an invite link that pre-selects tracks for a colleague and attributes the referral.

**FSD status today:** Not built. No invite links.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Admin console (ADM) — 3

### FR-ADM-04 · Should

> The admin shall be able to reply to escalations from the console; replies are delivered into the member's conversation and by email.

**FSD status today:** Not built. Depends on FR-CHAT-10 escalation, which is not built.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-ADM-08 · Could

> The admin may send an opt-in digest to members summarizing new content.

**FSD status today:** Not built. No member digest; no non-transactional email at all (FR-NOTF-04).

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-ADM-12 · Should

> The admin shall have a *Quotes* surface (`/admin/quotes`) listing every `quotes` row with add, edit, reorder, activate/deactivate actions. An "active" quote is eligible for rotation on the public landing page (FR-CNT-24); a deactivated one…

**FSD status today:** Not built. No `/admin/quotes`; depends on FR-CNT-24.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Notifications (NOTF) — 2

### FR-NOTF-04 · Must

> Non-transactional email shall be opt-in with one-click unsubscribe.

**FSD status today:** Not built, and not yet needed: no non-transactional email is sent.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-NOTF-06 · Should

> The whitelist + expiry workflow (FR-AUTH-17/18) shall use four additional templates: (a) whitelist auto-approved: sent instead of the admin email when a verified user matches an active `access_grants` entry; welcomes the user, states the…

**FSD status today:** The digest cadence and channel are admin-configurable; not enabled by default in v1. Verified 2026-09-29: only `access_ending_soon` and `access_ended` exist in `services/api/internal`. Template (a), the whitelist auto-approval notice, is not a distinct kind, and the digest (b) is not built. Both…

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Search (SRCH) — 2

### FR-SRCH-01 · Should

> Members shall be able to search all content by title, summary, body, and tags, filtered by type and track.

**FSD status today:** Not built. No search surface; `/search` does not exist.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-SRCH-02 · Could

> Search results shall offer *Ask Roger about this* to hand the query to the assistant.

**FSD status today:** Not built. Depends on FR-SRCH-01.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**
