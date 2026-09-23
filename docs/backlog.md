# Backlog

Open work, reviewed 2026-09-22 after the event stream (data layer D1)
landed. Shipped items are dropped from this file; their history is in
`docs/llm-tuning-log.md`, `docs/decision-log.md` and the PR list.
Items are grouped, and ordered within a group by how much they unblock.
Nothing here names a member or carries a credential.

## 1. Owner decisions

Answered 2026-09-22 unless marked open. Each decided item is now work
in the sections below.

- **Postgres app role split. DECIDED: yes, as recommended.** Run the
  api as `career_api` holding only the grants it needs, keep
  `career_admin_readonly` for `/admin/db`. One migration and one
  `.env.prod` change.
- **Backups. DECIDED 2026-09-22: yes, build it.** Open detail: the
  server cannot reach the Mac (Starlink CGNAT gives no inbound), so the
  Mac has to pull. Plan unless Roger says otherwise: nightly `pg_dump`
  on the server with seven days kept there, a launchd job on the Mac
  pulling over ssh whenever it is awake, a restore rehearsal written
  into `deploy/README.md`. Dumps include the résumé PDFs stored on
  `jd_submissions`, so they are treated as secrets at rest.
- **Admin second factor. DECIDED 2026-09-22: yes, with the admin
  choosing email or SMS per sign-in.** Open detail: email codes cost
  nothing and reuse the existing provider; SMS needs a paid number and
  a provider account, so the SMS path ships behind a flag and stays off
  until Roger funds it. Same phone column and provider then serve the
  SMS password reset already in section 4.
- **Registration friction. DECIDED: organization and stated role become
  optional.** Only name, email and password are required. The fields
  stay on the form because they are useful when they are filled in.
- **Anonymous access. DECIDED in principle: registration is still
  required for the reviewer, but more evidence is shown before the
  gate.** The shape is in section 6; the remaining call is how far to
  go, which Roger picks from the tiers written there.
- **OT mode contrast tokens.** Not a decision; treated as a fix and
  scheduled in section 6.
- **Résumé length. DECIDED: two pages is the maximum.** Shorter is
  fine. Anything that does not fit becomes a link to a page on
  rogerhenley.dev rather than a third page.
- **Facts sheet. DECIDED: the "robotics and vision" line stays.**
- **Gallery. DECIDED: the `reh-shot0x` portraits are real professional
  headshots, and any image under `docs/personal` may be published.**
  That covers the client photos as the owner's call; captions still
  need writing per photo.
- **Articles. DECIDED: public.** Most were posted publicly on LinkedIn
  already, so gating them buys nothing and costs discovery.

## 1b. Direction, settled 2026-09-22

A go-to-market plan (`docs/personal/review/GTM.md`, gitignored) proposes
turning the reviewer into a multi-tenant product. The owner's reading of
it, which sets the order of everything below:

- **The platform is proof for a senior role first**, a product second.
  So the work is depth on one corpus and evidence that it works, not
  tenancy, billing or support. Timelines in that document are not
  commitments.
- **The one real user is the owner.** No beta cohort until the thing
  runs very well for him.
- **Funding comes before the platform.** The repository split, the
  private platform repo and anything with a monthly bill wait for it,
  so no answer yet on the spend ceiling.
- **Build so none of it has to be undone.** Where a long-term shape can
  be adopted for almost nothing today, adopt it. The first instance is
  the tenancy seam, ADR 0029, shipped the same day.

Open from that conversation: whether "dossier" is the right word to
build a brand on, and the fact that seeker documents would bring other
people's confidential material into the corpus. Neither blocks anything
while the only user is the owner.

## 2. Data layer (Roger's stated priority)

D1 shipped 2026-09-22 (`docs/events/README.md`). Remaining, in order:

- **D1 follow-ups.**
  - `EVENT_IP_SALT` was generated and set on the server during the
    2026-09-22 deploy. Done, listed here so the next environment knows
    it exists.
  - Retire `ActivityService.RecordEvents` and the `/home` activity
    beacon; they still write `activity_events` while the new beacon
    writes `events`, so `/home` views are recorded twice. The admin
    member page's activity counts should read from `events`.
  - Emit the reserved browser names once their surfaces exist:
    `article.view`, `gallery.view`, `gallery.photo_open`, `repo.click`,
    `jd.poll_abandoned`.
  - The daily anonymize job runs 24 hours from boot; move it to a fixed
    hour when the scheduler grows a cron form.
  - After the deploy, run the funnel query in the events README on
    prod and confirm rows arrive from a real phone (device class,
    referrer host, utm).
- **D2 `jd_runs`. SHIPPED 2026-09-22.** One immutable row per pipeline
  run: build, host, model, context size, prompt fingerprints (version
  plus a hash of the text, so an unversioned edit is still visible),
  corpus fingerprint with counts, queue and work time apart, and the
  outcome with its verdict counts. `run_id` rides the context onto every
  `llm_usage` and `decision_log` row. A re-score opens attempt two
  instead of overwriting attempt one, and records who asked. The admin
  JD page shows the history, so a superseded verdict can be compared
  with the one that replaced it.
  **Left over:** the old submissions have no runs, which the page says
  plainly rather than hiding. Chat will need its own run kind when it
  lands; the column is nullable for that reason.
- **D3 feedback and outcomes. SHIPPED 2026-09-22.** `jd_outcomes`, one
  revisable row per posting for what happened in the world (not
  pursued, applied, screening, interview, offer, rejected, no response,
  withdrew), and `jd_feedback`, one judgment per person per run per
  target. Ratings are a fixed vocabulary rather than thumbs, because
  "too generous" and "too harsh" point at opposite fixes and one bit
  cannot tell them apart. Feedback is tied to the run, so a re-score
  does not inherit an opinion of what it replaced, and re-rating
  replaces rather than stacks. Both are on `/admin/jd/[id]`, where the
  posting is in front of you.
  **Left over:** the submitter-facing rating on `/jd-upload` is not
  built; with one user it would only ever be the owner rating his own
  work through a second door. Build it when there is a second user.
- **D4 golden set and `eval_runs`.** The calibration JDs plus reviewed
  verdicts as a fixed set; a job that rescores the set on demand and
  stores per-requirement agreement, so a prompt or model change is
  measured before it ships.
- **D5 views and `/admin/analytics`.** SQL views over `events`,
  `jd_runs` and `eval_runs`, and one admin page: funnel, JD outcomes by
  fit and commit, judge agreement over time.
- **Metrics for adapter training** (Roger opted in 2026-09-21). D2 to
  D4 cover the shape; still open are the nightly export of interactions
  and labels to object storage, and a corpus lineage table
  (`content_hash` per document version) so an answer can be replayed
  against the corpus revision that produced it.

## 3. JD reviewer

- **Verify the submitter workflow on prod** end to end with a real
  submission after the next deploy: access request, approval, sign in,
  submit with an apply link, progress modal, email by category with the
  PDF, reopen from the list. Record gaps in
  `docs/jd-submitter-workflow.md`.
- **Member data export and removal.** The privacy page promises both
  by email. Add an admin action that exports a member's rows as JSON
  and one that removes the account with the cascade rules written down
  (sessions end; reviews and events detach; reviews removed on request).
- **JD text extraction for PDF and docx uploads** rides the sidecar;
  today paste and plain text are the reliable paths.
- **Phone-width check of the progress modal and result page** on a
  real device.

## 4. Admin console

- **Access and whitelist surface** (`/admin/access`): whitelist CRUD,
  active grants with TTL bumps, still partly hand-driven.
- **Activity by user** (`/admin/activity`): sortable table (user, event,
  sessions, active time, JD activity) reading from `events`; a
  sessions tab on the member detail page.
- **Personalized member `/home`**: content preview, saved items, Ask
  Roger placeholder.
- **Content management UI** (`/admin/content`): upload articles,
  presentations and documents to MinIO plus a `content` row; the
  public articles reader then reads from the database instead of the
  filesystem allow-list. Text extraction for the corpus rides the same
  sidecar job as JD uploads.
- **Forgot password by SMS code.** The email reset exists; the SMS
  variant needs a verified phone column and a provider (Twilio or SNS,
  Roger to pick).

## 4b. Server operations

- **Disk. Cleared 2026-09-22**, from 93 percent full to 68 percent
  (2.7 GB free to 12 GB), by dropping the build cache and 159 stale
  deploy image tags; the current and previous tags were kept so a
  rollback is still a local `up`, and all seven containers stayed up.
  The prune is now the last step of the rollout in `deploy/README.md`.
  Roger is also enlarging the Hetzner volume. Left to do: make the
  prune automatic rather than a step someone remembers, either in a
  deploy script or a weekly timer on the box.
- **Backups. Built 2026-09-22**, see `deploy/backup/README.md`. Nightly
  dump plus cluster roles, verified by reading the archive, pruned to 14
  daily, 8 weekly and 6 monthly; a weekly restore into a scratch
  database that compares row counts against live; a launchd pull to the
  Mac that also takes `.env.prod` and fails loudly if the newest dump
  goes stale. Proven end to end on the server and the Mac the same day:
  a 1.4 MB dump of 271 objects restored with every table matching.
  **Live since the 2026-09-22 evening deploy:** both timers are armed
  (dump 03:15 UTC daily, restore test Sunday 04:30 UTC), one of each has
  run green through systemd, and the Mac holds a copy. Nothing left to
  do here; re-run `deploy/backup/install-server.sh` after any deploy
  that changes the scripts or the schedule.

## 5. Hardening (public repo)

Shipped so far: ruleset on `main` with required checks, gitleaks in
CI, rate limits on login and events, client IP behind Caddy, body
limits, security headers and CSP, noindex on member pages, HIBP fix.
Still open:

- Pin third-party GitHub Actions by commit SHA.
- CodeQL `security-extended` on a daily schedule; Trivy on the images.
- `GITHUB_TOKEN` least privilege per workflow.
- Containers as non-root with `read_only` where nothing is written.
- Rotate the session cookie on role change; shorter admin session TTL.
- Move `.env.prod` secrets to a manager (Hetzner, Doppler, 1Password
  Connect) or at least document rotation for every secret.
- One `git log -p` scan for accidentally committed secrets (none known).
- `SECURITY.md` response time and contact.
- Ask Roger inputs: sanitize before persisting, prompt-injection rules
  (lands with Phase 4).

## 6. Site and content

- **Anonymous access. SHIPPED 2026-09-22.** The policy is one list,
  `apps/web/src/lib/public-routes.ts`, shared by the proxy, robots.txt
  and the new sitemap so the three cannot drift. Public: the landing,
  the articles list and every article, six of the nine gallery photos,
  contact and the legal pages. Gated: the reviewer and its results, the
  résumé PDFs, the three personal gallery photos, `/home` and admin.
  The gallery serves both audiences from one URL and filters on
  `visibility` in each photo's front matter, so the split is content
  rather than code. Articles now prerender instead of running
  per-request, a sitemap lists them, and each public page ends by
  naming what an account buys.
  **Left over:** re-check the split once more photos land, and decide
  whether the footer should still offer "Request access" to people who
  are already signed in.
- **OT mode.** First pass shipped (PR 86); check on a real phone, and
  raise the body and muted text contrast to meet WCAG AA in the dark
  tokens (treated as a fix, no decision needed).
- **Gallery.** Every image under `docs/personal` is cleared for use
  (owner, 2026-09-22), including the portraits and the client photos.
  Work: re-add the two originals missing at build (ControlLogix editor
  screenshot, grain-tower install) with their front matter, write a
  caption and alt text per photo, pick the public subset for the tier
  above, then hero-by-track (FR-CNT-12).
- **Gallery captions.** Each photo needs its caption and alt text
  reviewed now that six of them are public and indexed.
- **Ask Roger (Phase 4).** Retrieval, persona, streaming chat with
  citations, guardrails, on the same gateway and decision log as the JD
  reviewer. Sized to the CPX31 with the 4b model.

## 7. Documentation

- Five "(unverified)" markers remain in `docs/FSD.md` (registration
  IP/UA columns, contact-page filters, retrieval-tester UI, whitelist
  auto-approved template and digest, Radix/MDX). Verify each against
  the code and remove the marker.
- Add the event stream and the privacy commitments to the FSD (a
  FR-DATA section) and the privacy page to the route table.
- `SERVICES.md` "last verified" date after each infra change.
- Keep the shared "current state" brief pattern for the next docs
  sweep (one brief, one agent per doc group).

## Standing rules that shape all of the above

- No server spend beyond the CPX31; fit the workload to the box.
- Document every experiment and decision in the same PR
  (`docs/llm-tuning-log.md` for the reviewer, `docs/events/README.md`
  for events, `docs/decision-log.md` for the human-in-the-loop labels).
- No em dashes in user-visible text. Commits authored by Claude so
  Roger can approve his own PRs. `docs/personal` stays gitignored.
