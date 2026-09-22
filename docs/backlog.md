# Backlog

Open work, reviewed 2026-09-22 after the event stream (data layer D1)
landed. Shipped items are dropped from this file; their history is in
`docs/llm-tuning-log.md`, `docs/decision-log.md` and the PR list.
Items are grouped, and ordered within a group by how much they unblock.
Nothing here names a member or carries a credential.

## 1. Owner decisions waiting on Roger

These block or shape work below; each needs a yes, a no, or a number.

- **Postgres app role split.** Run the api as `career_api` with only
  the grants it needs; keep `career_admin_readonly` for `/admin/db`.
  Recommendation: yes, one migration plus a `.env.prod` change.
- **Backups.** Nothing backs up the database today. Recommendation: a
  nightly `pg_dump` pulled to the Mac over ssh (zero server spend), with
  a restore rehearsal documented in `deploy/README.md`.
- **Admin TOTP.** Second factor on the admin account. Yes or later.
- **Registration friction.** Whether to keep the organization and
  stated-role fields required, and whether verified but unapproved
  users should see anything beyond the "pending" page.
- **Pre-gate landing evidence.** How much of the practice, timeline and
  writing to show anonymously before the access gate.
- **OT mode contrast tokens.** The dark theme's body and muted text
  contrast; the review flagged it, the fix is a token change.
- **Résumé length.** Two pages is the current target; confirm.
- **Facts sheet.** Whether the "robotics and vision" line belongs in
  `docs/personal/career-facts.md` (it steers the judge on those
  requirements).
- **Gallery.** Are the three `reh-shot0x` portraits real or generated;
  which `whk-*` / `bbc-*` client photos have permission; captions for
  the personal and event photos.

## 2. Data layer (Roger's stated priority)

D1 shipped 2026-09-22 (`docs/events/README.md`). Remaining, in order:

- **D1 follow-ups.**
  - Set `EVENT_IP_SALT` in `.env.prod` before the deploy that carries
    migration 00023, or the api hashes with the development salt.
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
- **D2 `jd_runs`.** One row per pipeline run with `run_id`, model,
  prompt hashes, corpus fingerprint, timings and outcome; `run_id` on
  `llm_usage` and `decision_log` so a score can be traced to exactly
  what produced it. Rescore then makes a new run, not an overwrite.
- **D3 feedback and outcomes.** Submitter thumbs on a result, owner
  outcome on a submission (interview, offer, no reply), and the
  `admin.decision_reviewed` labels, all joined by `run_id`.
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

- **OT mode.** First pass shipped (PR 86); check on a real phone,
  then the contrast tokens once Roger decides.
- **Gallery.** Re-add the two originals that were missing at build
  (ControlLogix editor screenshot, grain-tower install) and their front
  matter; hero-by-track (FR-CNT-12) after the portrait and permission
  questions are answered.
- **Shared public-path constant** used by the proxy, the sitemap and
  the header nav, so a new public page is added once.
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
