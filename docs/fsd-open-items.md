# FSD open items, for ranking

Working document, opened 2026-10-12. Every item the FSD does not record
as fully shipped, so we can decide together what stays, add the context
that makes each one judgeable, and then rank them.

**Two kinds of item here, and the second is the one that surprises.**

*Not built* is what it says. *Partially shipped* is a row the FSD marks
shipped with a hole in it, and those were invisible until the
documentation audit on 2026-10-12 went looking: `FR-CNT-22` promises
Turnstile on anonymous contact and enforces none, `FR-AUTH-09` promises
it on registration and has no check at all, `FR-ADM-09` promises an
audit log whose table has never been written to. A gap behind a
"Shipped" marker is likelier to bite than something nobody started,
because nobody is looking for it.

**What this list is not.** It is not a plan, and the order below is the
FSD's own, not a priority. Nothing here has been costed.

**How to use it.** For each item: decide *keep / drop / defer*, add the
context that makes it judgeable (who it is for, what breaks without it,
what it depends on), then rank. Blank fields are deliberate — they are
for the pass we do together.

| | |
|---|---|
| Not built | 48 |
| Partially shipped | 13 |
| **Total** | **61** |

`FR-AUTH-12` (TOTP MFA for admin) is deliberately absent: cancelled by
the owner's 2026-10-09 decision, not pending.

---

# Partially shipped — a hole behind a Shipped marker

## Content (CNT) — 4

### FR-CNT-16 · Must

> Content model. All content shall be stored under `content/` as Markdown/MDX or YAML with frontmatter validated against a shared schema at build time.

**FSD status today:** Marked Shipped until 2026-10-02, which overstated it: the storage half is done, the validation half is not. All content is Markdown with front matter under `apps/web/content/`, but nothing validates that front matter at build time. `packages/schema/` was deleted on 2026-10-02 (§7.2) because it was…

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-17 · Must

> Each content item shall carry: `id`, `slug`, `type`, `title`, `summary`, dates, `tracks` (weights), `tags`, `visibility` (`landing` or `member`), `chatbot_include`, `assets`, and `related` links.

**FSD status today:** Partially shipped. Articles carry title, subtitle, date, series, part and tags. No `id` or `type` field.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-19 · Should

> External links shall open in a new tab with `rel="noopener"`, and CI shall check link health on content changes.

**FSD status today:** Partially shipped. External links carry `rel="noopener noreferrer"` and `target="_blank"`. No CI link checker.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-CNT-22 · Must

> Contact / support form (public + member views differ). Visitors and members shall be able to reach the owner through a typed form with category buckets — `general_question`, `bug_report`, `feature_request`, `contributor_access`,…

**FSD status today:** `/contact` live; messages reach the admin console. Turnstile is not enforced: `Contact.turnstileEnforced()` returns false unconditionally, so the token check is skipped and an anonymous submission needs nothing. Found 2026-10-12.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Admin console (ADM) — 2

### FR-ADM-09 · Must

> All admin actions shall be recorded in the audit log.

**FSD status today:** Partially shipped. Admin actions emit registry events (`admin.decision_reviewed`, `admin.qa_entry_*`, `admin.rescore`, `admin.jd_limit_changed`). There is no separate immutable audit log.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-ADM-13 · Must

> The admin shall have a *Scheduler* surface (`/admin/scheduler`) with (a) a Google-Calendar connect flow that stores an OAuth refresh token encrypted-at-rest; (b) availability windows (days-of-week + hour ranges + slot length + per-day…

**FSD status today:** Shipped: `/admin/scheduler` editing zone, windows, lengths, clearance, step, per-day cap, lead time and horizon, stored in `app_settings` and read by the booking flow within 15 s. Saved whole or refused whole, with the validator's own message, because a half-applied calendar is worse than an…

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Evaluation (EVAL) — 2

### FR-EVAL-06 · Must

> An evaluation shall not be silently destroyed by a deploy.

**FSD status today:** The job runner keeps jobs in memory, so recreating the api container kills a run. `/admin/ops` (FR-ADM-19) answers whether one is in flight, which is the control today. All three services share one `IMAGE_TAG`, so there is no partial deploy that avoids this; per-service tags are in the backlog.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-EVAL-09 · Must

> An evaluation interrupted part way through shall keep the results it produced.

**FSD status today:** The counters lived only in memory until a run finished, and there is no cancel RPC, so stopping a run means restarting the api (FR-EVAL-06), which is exactly the path that skips the close. Run 14 was stopped at 5 of 9 and its row read "scored 0, errors 0" beside five finished `eval_items`.…

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Decision test (DT) — 2

### FR-DT-09 · Must

> A recall shall be graded by failure type, and its severity scored against what was retained rather than against what was correct.

**FSD status today:** Outcomes are exact, partial, untransformed, wrong_digits and expired. `digits_held` counts positional matches against whichever of the presented or expected number the response is closer to, so giving back the raw digits scores as perfect retention with a failed transformation rather than as total…

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

### FR-DT-20 · Should

> The tap-along check shall separate real tracking from mashing, on thresholds set from pilot data rather than guessed.

**FSD status today:** The check is built and its result and baseline reaction time are recorded on every session. The cut is provisional: at least seven of ten presses inside 300 ms with an offset standard deviation at or under 80 ms. The design document's starting figure of 150 ms is unreachable, since the maximum…

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Public site (PUB) — 1

### FR-PUB-05 · Could

> The site shall render acceptably with JavaScript disabled for the landing and legal pages.

**FSD status today:** Partially shipped. Pages are server-rendered so the landing and legal pages read without JavaScript; the mode toggle, the assistant panel and the action modals need it.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Auth and accounts (AUTH) — 1

### FR-AUTH-09 · Must

> Authentication endpoints shall be rate-limited per IP and per account, with progressive delays after failed attempts, and registration shall be protected by Cloudflare Turnstile (or equivalent).

**FSD status today:** Partially shipped. Login is rate-limited on (IP, email) jointly, 5 attempts then one token every three minutes (`ratelimit.New(5, 5.0/(15*60))`, `handlers/auth_session.go`), which counts successful logins as well as failures. In-process and single-instance only, and no progressive lockout.…

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## History and data rights (HIST) — 1

### FR-HIST-02 · Must

> Returning members shall see a welcome-back panel: last visit date, up to three items viewed, topics asked about, and items new since the last visit.

**FSD status today:** Partially shipped. `/home` shows a RECENT ACTIVITY list and what is new since last time. No last-visit date and no three-item resume list.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

# Not built

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

### FR-AUTH-14 · Must

> The site shall require admin approval for every new registration EXCEPT those whose verified email matches an active entry in `access_grants` (FR-AUTH-17). A verified user in `pending_approval` state has no session and no access to any…

**FSD status today:** Invite-code mode moves to §7 backlog. Shipped behaviour: approval via the one-click email or `AdminService.ApproveRegistration` / `DeclineRegistration`; the default access window on approval is 7 days, and the registration detail page in the console shows the expiry. OIDC registration is not built.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

## Admin console (ADM) — 6

### FR-ADM-03 · Must

> The admin shall have a conversation-review queue: negative feedback, "I don't know" answers, escalations, and out-of-scope refusals.

**FSD status today:** Partially shipped. `/admin/decisions` reviews and grades `decision_log`, including chat answers. Not driven by negative feedback, because FR-CHAT-14 (member rating) is not built.

- **Keep / drop / defer:**
- **Who it is for:**
- **What breaks without it:**
- **Depends on:**
- **Rank:**

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

### FR-ADM-10 · Must

> The admin shall have a *Pending approvals* review surface listing every `pending_approval` user (name, email, organization, stated role, submitted at, IP hash, user-agent) with per-row Approve and Decline actions and decision history. The…

**FSD status today:** Ships in Phase 2 as the destination for review links in FR-NOTF-05 emails; expanded coverage of admin activity lives in the full admin console (Phase 5). Shipped: list and per-registration detail with the access expiry; approve with a TTL (default 7 days), decline, extend, and resend a notification…

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

### FR-ADM-14 · Must

> The admin shall have a *Contact messages* surface (`/admin/contacts`) listing every `support_messages` row (category, subject, submitter, body preview, submitted at, status) with per-row Resolve, Reopen, and Delete actions plus category…

**FSD status today:** Reply flow is external — the admin replies from their normal mailbox — this surface only tracks status so nothing falls through. Verified 2026-09-29: the proto has `ListContactMessages` and `ResolveContactMessage` only. Reopen and Delete are not built; the page filters by category client-side. The…

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
