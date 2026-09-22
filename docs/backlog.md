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

## 4b. Server operations

- **Disk. Cleared 2026-09-22**, from 93 percent full to 68 percent
  (2.7 GB free to 12 GB), by dropping the build cache and 159 stale
  deploy image tags; the current and previous tags were kept so a
  rollback is still a local `up`, and all seven containers stayed up.
  The prune is now the last step of the rollout in `deploy/README.md`.
  Roger is also enlarging the Hetzner volume. Left to do: make the
  prune automatic rather than a step someone remembers, either in a
  deploy script or a weekly timer on the box.
- **Backups**, per the decision in section 1: nightly `pg_dump` on the
  server with a week kept locally, a launchd job on the Mac pulling over
  ssh, and a restore rehearsal recorded in `deploy/README.md`. Disk
  cleanup comes first, since the dumps need room.

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

- **Anonymous access, the shape agreed 2026-09-22.** Registration stays
  required for anything that costs compute or reveals contact detail;
  evidence that helps a hiring manager decide to register moves in
  front of the gate. Always public: the landing page in full, the
  practice and timeline detail, the articles list and reader, the
  contact form, the legal pages. Always gated: the JD reviewer and its
  results, résumé PDFs, `/home`, the admin console, anything naming a
  member. The open call is the middle tier, in ascending order of how
  much is given away:
  **DECIDED 2026-09-22: the middle tier.** Public becomes the full
  landing with practice and timeline detail, the articles list and
  reader, and a gallery subset of six to eight photos carrying no
  client marks. The reviewer, the résumé PDFs, the rest of the gallery,
  `/home` and the admin console stay behind the gate. The landing also
  needs one line naming what registering buys: submit a posting and get
  a résumé written against it.
- **OT mode.** First pass shipped (PR 86); check on a real phone, and
  raise the body and muted text contrast to meet WCAG AA in the dark
  tokens (treated as a fix, no decision needed).
- **Gallery.** Every image under `docs/personal` is cleared for use
  (owner, 2026-09-22), including the portraits and the client photos.
  Work: re-add the two originals missing at build (ControlLogix editor
  screenshot, grain-tower install) with their front matter, write a
  caption and alt text per photo, pick the public subset for the tier
  above, then hero-by-track (FR-CNT-12).
- **Articles public.** Move `/articles` and `/articles/[slug]` out of
  the gated set, drop the server session check, index them, and put an
  access call to action at the foot of each. Watch that the gallery
  and JD pages stay gated when the public-path list changes.
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
