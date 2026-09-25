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
- **D4 golden set and `eval_runs`. SHIPPED 2026-09-22.** `golden_postings`
  holds fixed postings, each asserting only which side of the gate it
  belongs on. That claim is about the world and survives a model change;
  a score would have to be rewritten on every change, and that rewriting
  is how a regression hides. An evaluation scores the whole active set
  through the real pipeline as a job, recording the same provenance a
  pipeline run does, and measures three things: gate accuracy,
  inversions (a posting expected below outscoring one expected above,
  which moving the gate cannot fix), and the margin between the two
  groups, which shrinks before anything visibly breaks. Evaluation
  submissions take the real path but are flagged, so they never reach
  the submission list or an email. `/admin/evals` runs them and reads
  the results; each row links to its derivation.
  **Left over:** the set needs Roger's real calibration postings added,
  which cannot be seeded from the repo because they live in the
  gitignored personal folder. Two placeholders exist locally only.
- **D5 views and `/admin/analytics`. SHIPPED 2026-09-22.** Ten SQL views
  (migration 00028, catalogued in `docs/metrics.md`) define every number
  once; the api selects from them and the page renders what it gets.
  Nothing recomputes a metric in Go or in a page, because two
  definitions of the same number is how a dashboard starts disagreeing
  with itself. The page is arranged by the criteria in the go-to-market
  response: reliability, agreement, time to a result, calibration,
  whether the score predicts anything, reach, model load. A criterion
  with no data says so rather than showing a zero that reads like a
  failure. The read-only console role picks up new views automatically,
  so `/admin/db` queries them directly.
- **Metrics for adapter training** (Roger opted in 2026-09-21). D2 to
  D4 cover the shape; still open are the nightly export of interactions
  and labels to object storage, and a corpus lineage table
  (`content_hash` per document version) so an answer can be replayed
  against the corpus revision that produced it.

## 3. JD reviewer

- **The résumé writer does not tailor to the posting it just scored.**
  Found 2026-09-25 by comparing a generated résumé against one the owner
  wrote himself for the same job, a capital-projects role in distilled
  spirits that the reviewer scored 0.929.

  The scorer understood the posting. The writer did not. It produced a
  résumé headlined "Engineering Executive and Hands-On Platform
  Architect ... Industrial Automation, AI/ML, and Capital Project
  Delivery", with competencies led by MQTT and Unified Namespace, IOF
  and BFO modeling, DataOps and governance-as-code, and AI safety.
  **The posting asked for none of those.** It asked for front-end
  loading, stage-gate delivery, basis of design, total installed cost,
  CAR, project controls, construction management, commissioning and
  CQV, asset turnover and EEM, which is what the owner's own version
  leads with and which appears nowhere in the generated one.

  It also omitted the strongest evidence available to it. The $135M
  greenfield build is in the corpus and in the owner's résumé opening
  line; the generated one does not mention it. Nor the arc flash and
  NFPA 70E work, nor front-end development as a competency at all, on a
  posting that names it twice.

  The pattern is that the writer described the candidate the corpus
  talks about most, rather than the candidate this posting is looking
  for. That is a retrieval and prompt problem rather than a model
  quality one: it has the requirement verdicts in hand, thirteen of
  them met, and does not appear to use them to decide what to lead with.

  Worth fixing in this order:

  - Check whether the writer is actually given the verdicts and the
    requirement text, or only the retrieved chunks. If only the chunks,
    the fix is plumbing rather than prompting.
  - Bias the evidence selection toward the requirements the posting
    actually contains, rather than toward whatever the whole posting
    retrieves. A requirement judged met is a claim the résumé should be
    able to make, and it names its own evidence.
  - Only then change the prompt, and measure with the golden set rather
    than by reading one output and liking it more.

  The owner's version is at `docs/personal/reh-resume-HeavenHill.pdf`
  and is the benchmark worth aiming at: the same facts, ordered by what
  the posting asked for.

- **Requirement extraction drops the words that disambiguate a
  requirement, and the judge then guesses wrong.** Found 2026-09-25 on a
  capital-projects posting the owner rated a very strong match. It
  scored 0.929, thirteen requirements met and one unmet, and the unmet
  one was wrong for an instructive reason.

  The posting asked for "front-end development of future major
  investments, including scope definition, alternatives evaluation,
  preliminary engineering and basis of design, cost and schedule
  development, financial justification, and Early Equipment Management".
  The extractor reduced that to **"Develop front-end development for
  major investments"**, which is ambiguous on its own, and the judge
  read it in the software sense. Its rationale: "The documents focus on
  backend systems, controls, and software architecture rather than
  front-end development for major investments."

  In capital projects, front-end development means early-stage project
  definition. In software it means the user interface. The phrase is the
  same and the corpus is full of software, so the judge resolved the
  ambiguity toward what it had seen most of.

  **This is not a corpus gap and adding documents will not fix it.** It
  will recur on any posting whose industry vocabulary collides with
  software, which for this candidate is a large fraction of them: "the
  platform", "architecture", "deployment", "integration", "pipeline",
  "commissioning" all carry two meanings across his two fields.

  The work, in the order that would settle it:

  - Establish how often it happens before changing anything. Every
    requirement and verdict is in `decision_log` with the evidence; the
    rationales that name the wrong sense are findable by reading them.
  - Then choose between two fixes rather than doing both by reflex.
    Either the extracted requirement keeps enough of its own sentence to
    disambiguate, which lengthens every judge prompt and therefore every
    review, or the judge is shown the requirement's surrounding text as
    context separate from the evidence. The first is simpler; the second
    costs fewer tokens.
  - Whichever is chosen, the golden set decides whether it worked, and
    this posting belongs in it as the case that exposed the fault.

  **Do not let the corpus work absorb this.** The Whiskey House document
  is about to be ingested and this posting re-run. If the score rises to
  1.000, that will look like the document working, and it may instead be
  capital-project evidence masking an extraction bug that is still
  there. The two need opposite fixes and the distinction is worth
  protecting.

- **TOP PRIORITY. The startup probe confirms configuration, not
  capability.** On 2026-09-25, after a reboot, the judge was disabled by
  a failed health probe and the pipeline scored eight golden postings on
  retrieval similarity alone, with no verdicts behind any of them. That
  specific hole is closed: the probe retries, and if the sidecar reports
  no model the scorer is not built, so submissions stay at `received`
  and evaluations refuse to start.

  **The hole next to it is still open.** Verifying the fix showed
  something nobody had checked: stopping Ollama entirely did not change
  the answer. The sidecar reports the model it is *configured* to use,
  not one it has confirmed it can reach. So if Ollama is down, or the
  model was never pulled, or it was deleted, the api still logs "jd
  assessor enabled" and every call fails later, at request time, one
  posting at a time.

  What that failure looks like matters: a submission enters the
  pipeline, the judge call fails, and what the submitter sees depends on
  paths nobody has exercised. The gatekeeper fails open by design. The
  assessor's behaviour on a hard LLM failure has never been tested
  against a real submission, and the last time an untested degradation
  path ran, it produced eight plausible numbers from nothing.

  The work:

  - The sidecar's health should answer "can I serve this model", not "am
    I configured for one". Asking Ollama for its tag list, or a one
    token generation, distinguishes the two.
  - The api should treat "configured but unreachable" the same way it
    now treats "no model": refuse rather than accept work it cannot do.
  - A submission that cannot be judged must fail as a submission, with a
    message saying the reviewer is unavailable, and must never reach a
    score by another route.
  - `/admin/ops` should show whether the model is actually reachable,
    since it is the page built to answer "is it safe to proceed" and it
    currently cannot answer this.

  The principle is already settled everywhere else in this system and is
  simply not enforced here: when the model is absent the code declines,
  it does not substitute something cheaper that resembles the answer.

- **Verify the submitter workflow on prod** end to end with a real
  submission after the next deploy: access request, approval, sign in,
  submit with an apply link, progress modal, email by category with the
  PDF, reopen from the list. Record gaps in
  `docs/jd-submitter-workflow.md`.
- **Member data export and removal.** The privacy page promises both
  by email. Add an admin action that exports a member's rows as JSON
  and one that removes the account with the cascade rules written down
  (sessions end; reviews and events detach; reviews removed on request).
- **Upload a file instead of pasting (owner, 2026-09-24).** Today the
  only reliable path is pasting text, which asks a recruiter to open a
  PDF, select all, and paste, and quietly loses anyone who will not
  bother. Wanted: a browse-your-computer button accepting `.pdf`,
  `.txt` and the usual document formats, converted to plain text before
  anything else happens. Nothing downstream changes: the extractor, the
  gatekeeper, the judge and the scorer all keep taking a string, and
  the conversion is a step in front of them.

  The conversion belongs in the sidecar, which is already Python, is
  already the only thing that touches untrusted bytes, and already
  carries `pypdf` for locking the generated résumé. The same job serves
  the content-management work further down this section, which needs
  identical extraction for articles and documents.

  **This is the attack surface the owner was right to worry about.**
  Parsing a file someone else produced is a different risk from reading
  text they typed, so the work is not finished without:

  - a size ceiling enforced before anything is parsed, and a wall-clock
    timeout on extraction, since a malformed or deliberately hostile
    PDF is a denial-of-service before it is anything else;
  - type decided by sniffing the content, never by the filename, and an
    allow-list rather than a deny-list;
  - extraction that only ever reads: no embedded scripts, no external
    entity resolution, no following links inside the document;
  - the extracted text treated exactly as pasted text is treated now,
    which means the posting-check gatekeeper still decides whether it is
    a job description, and the daily quota still applies.

  **The failure worth designing for is a scanned PDF.** It has no text
  layer, extraction returns nothing or near nothing, and the gatekeeper
  would then refuse it as "not a posting", which is true but useless:
  the visitor uploaded a real job description and is told it is not one.
  That case has to be detected at extraction and answered in its own
  words, either by saying the file has no readable text and asking for
  a paste, or by adding OCR, which is a much larger piece of work and
  should not be assumed.
- **Phone-width check of the progress modal and result page** on a
  real device.

- **Meeting scheduler (`FR-CNT-23`, `FR-ADM-13`, decision `D-22`).**
  Specified as a Must in the FSD and never carried into this backlog,
  which is why it has sat untouched: a public page showing the owner's
  real availability from Google Calendar with busy slots hidden, a
  visitor picks an open slot without needing an account, and the event
  lands on his calendar with both parties invited. The admin surface
  configures the windows, so the hours below are initial values rather
  than anything compiled in.

  **Availability, from the owner 2026-09-24:** Tuesday, Wednesday and
  Thursday only, 09:00 to 12:00 and 14:00 to 16:00. Three days, two
  windows a day, five hours a day.

  Details that decide whether this works or annoys people:

  - **The time zone has to be stated everywhere a time is shown.** The
    windows above have no zone attached yet and that must be settled
    before anything is built. A visitor in another zone reading bare
    times will book the wrong hour and blame the site.
  - **Two windows a day, not one range.** The lunch gap means the
    availability model is a list of ranges per weekday, not a start and
    an end. Building it as a single range and bolting on a break later
    is the usual way this ends up wrong.
  - **Outside the configured windows the page is unavailable whatever
    Google says.** A free Tuesday evening is still not bookable. The
    calendar subtracts from the windows, it does not add to them.
  - **A per-day cap**, so three days a week does not become five
    back-to-back conversations on a Wednesday, and a buffer between
    slots, which the FSD does not mention and which anyone who has
    taken consecutive calls will want.
  - **What happens when the token expires.** OAuth refresh tokens die,
    and the failure has to read as "unavailable" rather than as an
    empty calendar that looks like no availability at all.
  - Costs nothing to run: the Google Calendar API is free at this
    volume, so this does not touch the no-spend constraint.

  Worth doing sooner than its position here suggests. A recruiter who
  has read the reviewer's output and wants fifteen minutes currently
  has to use the contact form and wait for a reply.

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

**5a. A submitted posting can no longer reach private material.
SHIPPED 2026-09-24.** Retrieval had no visibility predicate:
`SearchCorpus` selected across every embedded chunk, and production
holds 21 `corpus_only` documents against 5 public ones. The retrieved
text entered the judge's context and the résumé generator, and the only
thing between it and the submitter was a line in the judge prompt
asking the model not to quote private chunks. A submitted posting is
text the submitter wrote, aimed at that same model, so the prompt was
the wrong place for the control.

The scope now rides the context (`internal/corpusscope`) and the SQL
enforces it. It defaults to public, so code that forgets to widen it
retrieves too little, which someone notices, rather than too much,
which nobody notices. A member's submission is public-only: 52 chunks.
The owner's own submissions and evaluation runs are unrestricted: 239
chunks.

**The cost, stated plainly:** a member's review now draws on five
public documents. Their results will be thinner and less evidenced than
the owner's, and the golden-set numbers describe the owner's path only,
not what a recruiter sees. Closing that gap is a content decision, not
a code one: publish more of the corpus. Nothing should reopen the
retrieval path to do it.

**5a note: two paths are exempt from the corpus scope, on purpose.**
The judge's career facts sheet and the résumé writer's master résumé
are fetched by `ListChunksByKind`, which has no visibility predicate,
so a member's submission sees both although they are `corpus_only`.
Reviewed with the owner on 2026-09-24 and kept as it is. The scope
exists to stop a submitted posting steering retrieval across client and
NDA material; these two are about the candidate rather than a client,
are identical for every posting, and are fetched by kind rather than
retrieved, so nothing in a posting can steer them. Recorded because it
was originally an oversight rather than a decision, and an exemption
nobody wrote down is indistinguishable from a hole.

**5b. A member cannot queue unbounded work. SHIPPED 2026-09-24.** The
existing limiter is keyed on (member, jd_hash), so it stops the same
posting being resubmitted and nothing else: change a word and it is a
different key. One submission is roughly an hour of inference and the
pipeline runs one at a time, so a member pasting distinct postings in a
loop was a queue nobody else could get into. `JD_DAILY_LIMIT` (default
3) caps submissions per member per rolling 24 hours, counted from
`jd_submissions` rather than an in-memory bucket, because the api
container is recreated several times a day and anything counted in
memory is enforced only between deploys. Evaluation rows are excluded.
The admin is exempt. The check fails closed: if the count cannot be
read, the submission is refused. Three rather than five because the cap
has to bound the window it is measured over: five members at five each
is 25 reviews, which at the measured pace is more than a day of
continuous inference, so the queue would outrun the day it fits in.

The number is now an admin setting rather than an environment
variable. `/admin/jd` edits it next to the fit bands, it is stored in
`app_settings` and cached for 15 seconds, so a change is in force for
the next submission without a deploy. `JD_DAILY_LIMIT` seeds the row
the first time it is read and is not consulted again. What the limit
should be depends on how long a review currently takes, which changes
with the model, the corpus and the box, so it had no business being a
value that needs ssh and a restart to alter.

The allowance is also stated rather than merely enforced. The upload
page shows what is left before anyone spends it, every submission
returns the updated count, and running out names the hour the next slot
opens. A limit someone meets without warning reads as a fault, and the
reason for this one is worth saying: a review is close to an hour of
work on one machine.
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
  reviewer.

  **Settled 2026-09-24: it does not run on the current web server.**
  The CPX31 is already strained by the reviewer. Ollama holds 5.2 GB
  of the box's 7, there is one inference lane, a JD review occupies it
  for close to an hour, and an eight-posting evaluation took six and a
  quarter. A chat sharing that box is a form with a queue, not a chat.

  **Where it does run is open.** Running it on the owner's own machine
  was raised the same day as one possibility, and explicitly as a
  suggestion rather than a decision. The viable option gets
  investigated when the phase is actually started. Nothing should be
  built against any particular topology before then.

  What is worth settling early, because it follows from inference being
  somewhere other than the web server rather than from any particular
  choice of where:

  - **Hours of operation.** A machine that is not a server is not
    expected to be up at 3 a.m. The site would have to know when Ask
    Roger is meant to be available and say so.
  - **A heartbeat** between the web server and wherever the model
    runs, so availability is observed rather than assumed. Inside the
    stated hours with no heartbeat is an outage worth seeing; outside
    them it is expected.
  - **A "not currently available" banner** driven by the heartbeat,
    not by a schedule alone. A visitor should never be given an input
    box that will fail.
  - A transport, if the model ends up off the box. A machine behind
    residential Starlink CGNAT takes no inbound connection; the backup
    design hit that and answered it by having the Mac pull, and an
    outbound tunnel held open from the machine is the shape that would
    fit here.
  - Authentication between the ends, and a rule for what the model is
    allowed to be asked for. `internal/corpusscope` already decides
    what a visitor's question may retrieve.
  - What happens to a conversation when the heartbeat drops mid-answer.

## 6a. What the reviewer answers, and what it does not

Settled 2026-09-23. The JD reviewer answers one question: **does the
candidate have the skill set to do this job.** Nothing else.

Whether a role suits the person, salary range, location, full-time
against contract, seniority, is a separate layer and a different kind
of problem. Those are constraints the candidate states once and a
posting either satisfies or does not. They are parameter filters. They
need no model, no evidence, and no judgment, and running them through
an LLM would be slower, dearer and less reliable than a WHERE clause.

Two consequences worth holding on to:

- A posting well below the candidate's level still scores high, because
  he can plainly do it. That is correct, not a bug. The seniority
  mismatch belongs to the filter layer.
- Golden-set labels answer the capability question only. A label that
  means "I would not take this job" would be measuring something the
  reviewer was never asked to judge.

**Not being built now.** The candidate-side constraint filter is
deferred deliberately. When it comes it is a form, a few columns and a
query, and it should stay that way.

## 6b. Evaluation roadmap

From an external review of the go-to-market response
(`docs/personal/review/gtm-response-review01.md`, gitignored; its items
are tagged `RR-NN` and referenced here by that id). Ordered cheapest and
most informative first, which is not the order the review lists them in.

The theme: the reviewer's sharpest observation is that the measurements
built so far can confirm what the owner already believes but cannot
surprise him. Everything below either removes that limitation or makes
an existing claim honest.

**Already true**, noted so nobody builds them twice:

- `RR-19` host and model on every run. Shipped with D2.
- `RR-06` reproducibility fields. `decision_log` already carries model,
  prompt id and version, exact input and output, tokens, latency and
  `run_id`; a second assessor's rows slot into that shape unchanged.
- `RR-13` in part. Ten metric views exist and `/admin/analytics` is
  arranged by the seven criteria. What is missing is the pass/fail
  shape, below.

**1. Random postings in the golden set (`RR-11`, `RR-12`). SHIPPED
2026-09-24.** A posting records how it got into the set, chosen or
random, and where it came from; a random one arrives unlabelled, which
evaluations skip rather than guess at; and the console shows each
posting's full text with a label control, because a posting cannot be
judged without being read.

The set holds eight postings, two chosen and six random, five expected
above the gate and three below. The three on the low side were drawn
from adjacent fields where the requirement that decides it is a
credential: a BSEE and ETAP depth, a PE licence, a PhD with top-venue
publications. Run 2 scored 8 of 8 on the expected side with 0 ordering
violations and a margin of 0.143, in 6h13m45s. The groups did not
interleave: every posting expected above outscored every posting
expected below.

**That margin of 0.143, with 8 of 8 and zero inversions, is the
regression floor.** A later change to a prompt, a model or the corpus
that drops below it is a regression, and the numbers exist to say so.

Calibration reported separately for chosen and random shows no
difference: chosen scored 1.000 and 0.857, random-above scored 0.956,
0.865 and 0.786. The reviewer is not flattering the owner's own picks,
which was the specific worry the `selection` column was added to test.

**Left to do:** grow the set toward ten as real postings arrive, and
keep the low side growing with it, since three rows is thin. See also
`RR-18` on refreshing random postings so the set does not decay into
"postings the model already handles". Originally: the set
holds one posting. The owner chose it, and chose it because he applied
for the job, so its expected outcome is his application decision
restated rather than an independent judgment. A set like that cannot
fail informatively. Add ten postings drawn at random from job boards in
adjacent fields, labelled as usual, and record per posting how it was
selected. Report calibration separately for chosen and random: a gate
that is clean on chosen postings and noisy on random ones is the most
useful thing the set can tell us. Needs nobody else and costs nothing.

**2. Check that a source supports its bullet (`RR-09`). SHIPPED
2026-09-24.** A line is now compared against the text of the chunks it
cites and dropped when it asserts a quantity or a named organisation
that none of them carry. Checked in code rather than by a model call:
one call per line would add twenty minutes to a fifty-minute review,
and fabrication shows up first in the specifics, which is what code can
check. Tuned to miss rather than over-report, since a missed check
costs nothing and a false one costs a true line. `unsupported` and
`unsupported_claims` are recorded on the stored résumé, separately from
`dropped`, because a model citing nothing and a model citing something
that does not support the line need different fixes. **Left to do:** no
generated résumé has been through it yet, so the first real submission
is the evidence; and the counts are queryable through `/admin/db` but
have no place in the console. Originally: today
`jd/resume.go` confirms a cited chunk id was among those offered. It
never checks that the chunk's text supports the sentence. So "verified"
currently means "cites something real", not "is supported by what it
cites", and the honesty criterion is written against the weaker
meaning. An entailment check on each bullet closes the gap between what
the site implies and what it does.

**3. The gate as pass or fail (`RR-13`, `RR-14`). SHIPPED
2026-09-24.** One view per criterion returning `pass, value, target,
as_of, detail`, unioned as `v_gate` (migration 00031), rendered at
`/admin/gate`. `pass` has three states: met, not met, and unanswered,
the last covering both too little data and no target set. An unanswered
criterion is never drawn as a failure, because a young system rendering
as seven red rows is a page nobody opens twice.

On production at the time of writing: calibration passes (8 of 8, no
inversions, margin 0.143), time to a result fails (median 48.9 minutes
against a target of 20), and the other five are unanswerable, four for
sample size and two because no target exists.

**Time to a result: not met, and not being pursued (owner,
2026-09-24).** A run is about 50 minutes: 34 of them are fourteen judge
calls, each evaluating roughly 1,500 tokens of per-requirement evidence
at the 12 to 13 tokens a second this box manages. Prompt-prefix caching
is already working and already counted; the only remaining lever is
sending the judge less evidence, which trades verdict quality for
minutes. The owner's call: leave the pipeline alone, this is a resource
constraint rather than a defect, and further attempts risk the
effectiveness of something that currently works.

The criterion therefore reads red and will keep reading red. That is
truthful rather than a fault to fix, and it is the one row on the gate
whose target was invented before anything had been measured. Whether to
restate it against what the box actually does, so that it detects a
regression instead of restating a known limit, is open and is the
owner's to decide.

Both open questions were settled the same day:

- **Reach is measured, not gated.** Whether strangers find the site is
  demand, not quality; it moves with where a posting was shared and not
  with whether the reviewer improved, and a gate that cannot be moved
  by doing good work teaches nothing. The row says "measured" rather
  than sitting unanswered forever.
- **Model load is gated on the failure rate**, under 2 percent of
  calls. Currently 4 in 363, so it passes, and it turns red if the box
  starts struggling. Latency is deliberately not repeated there, since
  time to a result already gates it and one number gated twice reads as
  two problems.
- **Agreement counts only what the gatekeeper accepted** (migration
  00032). Seven of eleven reviewed verdicts came from a 365-character
  job-search worksheet whose extracted "requirements" were the
  fragments "Data Science" and "Machine Learning". There is no
  assertion in those to judge. Excluding them leaves 4 reviewed, 3
  agreed, and one real disagreement: Blue Origin's "2+ years building
  products that use Large Language Models", judged unmet against the
  owner's met. The hazard is named in the migration: dropping rows that
  make a number look bad is how metrics get gamed, so the gatekeeper
  decides rather than the verdict, and the rule is written in SQL
  rather than applied case by case.

Reliability counts only real attempts to review a posting (migration
00033). Two of the last eight runs were gatekeeper refusals, which
dropped the row to "6 of the last 8" for no fault: a refusal finishes
without anyone stepping in, which is what the criterion asks. Whether
the gatekeeper refuses the right things is a separate question that
belongs to agreement and the decision log, not hidden inside a
reliability number.

**Left to do:** rows do not yet link to the runs behind them, which
`RR-14` asks for. A criterion whose source view returns no rows
disappears from the gate entirely rather than saying it has no data,
because each view groups by tenant and an empty table produces no
group; on an empty database the gate shows one row instead of seven.

**Incident, 2026-09-24.** The first version of migration 00032 dropped
`v_judge_agreement` and its summary. goose runs 00031 first, so
`v_gate_agreement` already depended on the summary by then and Postgres
refused; the api crash-looped and the API was down until a rollback.
The second attempt replaced the view in place but was written from the
00028 definition, and 00029 had since added two columns, so Postgres
refused that too. Both mistakes were invisible in a diff and obvious on
a replay. CI now applies every migration to an empty database in order
and selects from every view.

**4. The second assessor (`RR-05`, `RR-07`, `RR-08`, `RR-10`).** A
stronger model checking every verdict, in two clearly separated modes:
does the cited evidence support this verdict, and separately, is the
verdict right given everything documented. Mixing them in one number
blurs both. The valuable output is the owner-versus-checker
disagreement, which splits into two actionable findings: the documents
understate work that was done, or a verdict leans on something known
but never written down. Calibrate the checker against 30 to 50
owner-labelled entailment judgments before quoting any of its numbers.
**Blocked** until the conflict in item 6 is resolved.

**5. Record the protocol as an ADR (`RR-15`).** Once items 1 to 4 have
settled into something that works, not before: an ADR written ahead of
the practice describes an intention rather than a decision.

**6. Resolve a contradiction in the review before item 4 (`RR-01`
against `RR-10`).** `RR-01` defines `corpus_only` material as never
exported, never used for training, and assigns client and NDA records
to it. `RR-10` then says sending that same material to a hosted model
is acceptable. Sending it to a hosted provider is an export. This needs
an owner decision, and it gates the second assessor: either the checker
sees only `public` material, or `corpus_only` gets a narrower rule that
says what a hosted assessor may read.

**Not a work item, a correction.** The review's draft post states that
a much larger model independently checks whether cited evidence
supports each verdict, and quotes an agreement percentage. Neither
exists: production holds the owner's labels and the small model's
verdicts and nothing else. That post cannot be published until item 4
ships, because it would be a claim the system cannot back, which is the
failure this product exists to prevent.

**Smaller, whenever convenient.** A structured finding log rather than
prose (`RR-17`), a refresh cadence for the random postings so the set
does not decay into "postings the model already handles" (`RR-18`), and
the three-way agreement chart on the public build page once the numbers
are stable (`RR-20`).

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
