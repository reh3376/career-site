# Services

Every service that runs in this project, with type, address, port, and
credentials. **No literal secrets in this file**: credentials appear as
the env-var name that carries the value; the value itself lives in
`.env.prod` (prod, mode 0600, under `/opt/career-site`) or the repo-root
`.env` (dev, gitignored).

The dev DSNs use literal placeholder values (`career_dev_only`,
`minio_dev_only`); they are safe to commit because they are only used by
the dev compose file and never accepted by prod. See
[`docker-compose.yml`](docker-compose.yml) for the base and
[`docker-compose.prod.yml`](docker-compose.prod.yml) for the prod overlay.

Last verified against the running stack: 2026-09-21 (`deploy/live-check.sh`);
text updated 2026-09-22 for the LLM cutover (the JD reviewer now runs on the
box's own Ollama).

---

## Runtime services

| Service | Type / image | Internal address | Host port | Auth (env var) | Notes |
|---|---|---|---|---|---|
| **caddy** | `caddy:2-alpine` | in front of everything | prod: `80`, `443`; dev: `80` | Let's Encrypt via `LETSENCRYPT_EMAIL`, cert for `SITE_DOMAIN` | Routes `/api/*` to `api:8080` and everything else to `web:3000`; carries HSTS, CSP and the other security headers (`deploy/caddy/Caddyfile.prod`). Redirects plain HTTP to HTTPS, including `http://localhost` on the box. |
| **web** | Next.js 16 (App Router), standalone build; `ghcr.io/reh3376/career-site-web:<sha>` | `web:3000` | none | forwards the session cookie to api; `src/proxy.ts` redirects signed-out visitors to `/login` for every route outside the public allow-list | Public: `/`, `/contact`, `/register`, `/login`, `/privacy`, `/terms`, verify / reset / one-click approval. Members: `/home`, `/articles`, `/gallery`, `/jd-upload` (+ `/jd-upload/<id>` reopenable review), `/settings`. Admin: `/admin/*` (Overview, Contact messages, Registrations, Access & whitelist, Activity, Corpus, JD submissions, Decision review, DB query). |
| **api** | Go 1.26 ConnectRPC; `ghcr.io/reh3376/career-site-api:<sha>` | `api:8080` | none (prod); via caddy in dev | session cookie (`career_site_session`) for members and admins; `ADMIN_USERNAME` + `CAREER_SITE_ADMIN_PW` bootstrap the admin row on boot | Health: `GET /api/healthz`, `GET /api/readyz` (checks postgres + sidecar). Runs goose migrations on boot. Plain-HTTP routes besides Connect: `POST /api/admin/decision` (one-click approval token), `GET /api/jd/resume/<id>.pdf?t=<token>` (member session + result token). Distroless image: no shell inside the container. |
| **sidecar** | Python 3.12 gRPC; `ghcr.io/reh3376/career-site-sidecar:<sha>` | `sidecar:50051` | none | none (compose network only) | `Embed` (nomic prefixes `search_document:` / `search_query:`, purpose-aware), `Generate` (LLM gateway, JSON-schema constrained; sends `num_ctx` and refuses truncated results), `RenderResume` (Typst + pypdf, owner-password locked PDF), `Health` reports `embedder_ready`, `llm_ready`, `llm_provider`, `renderer_ready`. Providers are env-selected (`stub` / `ollama`); the LLM can live on a different Ollama host than the embedder (`OLLAMA_LLM_URL`). |
| **ollama** | `ollama/ollama:latest` | `ollama:11434` | none | none | Embedding host (`nomic-embed-text`, ~275 MB, pulled on first boot into the `ollama_models` volume) and, since 2026-09-22, the LLM host (`qwen3:4b-q8_0`). One model resident at a time (`OLLAMA_MAX_LOADED_MODELS=1`; the embedder reloads in seconds), flash attention on, q8_0 KV cache. Always on in prod; dev opt-in via `docker compose --profile embed` (dev usually points the sidecar at the host's own Ollama instead). Memory-capped by `OLLAMA_MEM_LIMIT` (prod `7g`); `OLLAMA_KEEP_ALIVE` unloads idle models. |
| **postgres** | `pgvector/pgvector:pg16` | `postgres:5432` | dev: `5432`; prod: none | app role `career` / `POSTGRES_PASSWORD`; read-only role `career_admin_readonly` / `DB_READONLY_PASSWORD` for `/admin/db` (migration `00005`) | Data volume `postgres_data`. pgvector with `vector(768)` + HNSW cosine index on `corpus_chunks`. |
| **minio** | `quay.io/minio/minio:latest` | `minio:9000` (S3), `minio:9001` (console) | dev: `9000`, `9001`; prod: none | `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` | Provisioned for uploads and object storage; not yet used by the api (résumé PDFs are stored on the submission row for now). |
| **mailpit** | `axllent/mailpit:v1` (dev only) | `mailpit:1025` (SMTP), `mailpit:8025` (web UI) | dev: `1025`, `8025` | none | Prod uses Resend; the service is under the `disabled` profile in the prod overlay. |

### Bind mounts into `api`

| Container path | Host source | Mode | Purpose |
|---|---|---|---|
| `/corpus` | `apps/web/content` (repo checkout) | ro | public corpus (`visibility=public`); `ReindexCorpus scope=public` |
| `/corpus-private` | prod: `/opt/career-site-private/corpus` (fed by `make sync-corpus`); dev: `./.corpus-private` (`make stage-corpus`) | ro | private corpus (`visibility=corpus_only`, never quoted on the site); `ReindexCorpus scope=private` |

### Hosts

| Host | Type | Access | Notes |
|---|---|---|---|
| `5.161.62.205` (rogerhenley.dev) | Hetzner **CPX31** (4 vCPU / 8 GB nominal, 7.7 GB usable / 40 GB, 2 GB swapfile), Ubuntu 24.04, Ashburn VA. Rescaled from CPX11 on 2026-09-21. This box is the ceiling: no further server spend, fit the workload to it. | `ssh career@5.161.62.205` (key auth; `career` has sudo and docker) | Deploy dir `/opt/career-site` (git checkout of `main`); private corpus at `/opt/career-site-private`. The stack returns after a reboot on Docker's `restart: unless-stopped` policy; there is no systemd unit (see `deploy/README.md`). Never `compose stop` before a planned reboot. |
| owner's Mac | local dev | n/a | `docker compose up -d` with the sidecar pointed at the host's Ollama (`host.docker.internal:11434`); `qwen3:4b-q8_0` and `nomic-embed-text` pulled locally. The 14b was dropped (does not fit the box) and the 8b OOM-killed it; the 4b is the model in both places. |

---

## Environment variables (by service)

Every prod variable, with comments, is in [`.env.prod.example`](.env.prod.example). This table is the quick map.

### api

| Var | Purpose | Where set |
|---|---|---|
| `API_ADDR` / `API_ENV` | listen address / environment tag | prod `:8080` / `production` |
| `DATABASE_URL` | app pool DSN | built from `POSTGRES_PASSWORD` |
| `DATABASE_URL_READONLY`, `DB_READONLY_PASSWORD` | `/admin/db` read-only pool; password applied via `ALTER ROLE` on boot | `.env.prod` |
| `SIDECAR_ADDR` | sidecar gRPC target | `sidecar:50051` |
| `WEB_BASE_URL` | absolute URL in outbound email links | `https://${SITE_DOMAIN}` |
| `MAIL_FROM`, `OWNER_CONTACT_EMAIL`, `EMAIL_PROVIDER`, `RESEND_API_KEY` | transactional email (Resend in prod, mailpit in dev) | `.env.prod` |
| `DECISION_TOKEN_SECRET` | HMAC key for one-click Accept / Decline links | `.env.prod` (hex) |
| `COOKIE_SECURE`, `SESSION_TTL_HOURS`, `PWNED_CHECK_ENABLED` | session cookie flags, lifetime (default 30 d), HIBP check | prod `1`, default, `1` |
| `ADMIN_USERNAME`, `CAREER_SITE_ADMIN_PW` | admin bootstrap | `.env.prod`; dev `.env` |
| `CORPUS_ROOT`, `CORPUS_PRIVATE_ROOT` | corpus mounts (see above) | `/corpus`, `/corpus-private` |
| `JD_PIPELINE_TIMEOUT_SECONDS` | wall-clock budget for one JD run (score, assess, résumé, PDF), counted from the moment the run gets the single pipeline slot (one pipeline at a time) | prod `3600` (example file says `1200`; the 4b on 4 vCPU needs 15 to 30 min per JD) |
| `JD_MATCH_THRESHOLD` | seeds the `jd_fit_bands` row in `app_settings` on first read (the "strong" band, which is the résumé gate); after that the bands are edited on `/admin/jd`, not here | prod `0.70` (code default `0.55`) |
| `LLM_NUM_CTX` | context window the api budgets prompts to; compose sets it from `SIDECAR_LLM_NUM_CTX` so the two never drift | prod `8192` |
| `LLM_MONTHLY_CALL_CAP` | LLM gateway calls per calendar month, 0 = unlimited | prod `2000` |
| `LLM_ALLOW_STUB` | let the structured assessor run against the stub provider (CI only) | `0` |
| `RESUME_PDF_OWNER_PASSWORD` | owner password locking edits on generated résumé PDFs; empty skips the PDF | `.env.prod`; dev `.env` (never committed) |

### web

| Var | Purpose | Where set |
|---|---|---|
| `PORT`, `HOSTNAME` | Next.js bind | `3000`, `0.0.0.0` |
| `API_URL` | server-side base URL for `callApi` | `http://api:8080` |
| `NEXT_PUBLIC_API_URL` | client-side base URL | `https://${SITE_DOMAIN}` |
| `LINKEDIN_URL`, `GITHUB_URL` | outbound links, read at request time by the server components (header icons, hero, OT strip, footer, hamburger); empty hides the link. `NEXT_PUBLIC_*` variants are accepted as fallbacks. | `.env.prod` |
| `JD_MATCH_THRESHOLD` | fallback for the gate quoted on the JD pages when the `GetJdReviewConfig` call fails; the live bands come from the api | same value as the api |

### sidecar

| Var | Purpose | Where set |
|---|---|---|
| `SIDECAR_ADDR`, `SIDECAR_MAX_WORKERS` | gRPC bind, thread pool | defaults |
| `SIDECAR_EMBED_PROVIDER` | `stub` or `ollama` | prod `ollama` (since 2026-09-21); dev `ollama` when the host Ollama is up |
| `OLLAMA_URL`, `OLLAMA_EMBED_MODEL`, `SIDECAR_EMBED_DIMENSIONS` | embedder target; dimension is locked to 768 by the schema | `http://ollama:11434`, `nomic-embed-text`, `768` |
| `SIDECAR_LLM_PROVIDER`, `OLLAMA_LLM_MODEL` | LLM gateway; live on prod since 2026-09-22 (`docs/cutover-local-to-prod.md`, `docs/llm-tuning-log.md`) | prod `ollama` / `qwen3:4b-q8_0`; dev the same against the host Ollama |
| `OLLAMA_LLM_URL`, `OLLAMA_API_KEY` | where the LLM runs: the box's ollama, or another Ollama host (e.g. a workstation over a tailnet); the key only for a hosted Ollama endpoint | prod `http://ollama:11434`, key empty |
| `SIDECAR_LLM_NUM_CTX` | context window requested per call; Ollama's default silently truncates, so the sidecar sends `num_ctx` and rejects truncated results | prod `8192` |
| `SIDECAR_LLM_TIMEOUT_SECONDS` | per-call LLM timeout; judge calls take ~100 to 135 s and a résumé ~13 min on the box | prod `3000` (example file says `900`) |

### ollama

| Var | Purpose | Where set |
|---|---|---|
| `OLLAMA_EMBED_MODEL` | model pulled at container start | `nomic-embed-text` |
| `OLLAMA_MEM_LIMIT`, `OLLAMA_KEEP_ALIVE`, `OLLAMA_NUM_PARALLEL` | container memory cap, idle unload, concurrency | prod `7g`, `30m` (read from the box's `.env.prod` 2026-09-22; overlay default `5m`), `1` |
| `OLLAMA_MAX_LOADED_MODELS`, `OLLAMA_FLASH_ATTENTION`, `OLLAMA_KV_CACHE_TYPE` | memory levers for running the LLM next to the embedder: one resident model, flash attention, 8-bit KV cache (about 1 GB off the peak; the first run with both models resident and a full-precision cache OOM-killed llama-server) | prod `1`, `1`, `q8_0` |

### caddy

| Var | Purpose | Where set |
|---|---|---|
| `SITE_DOMAIN`, `LETSENCRYPT_EMAIL` | certificate name + ACME contact | `.env.prod` |

---

## Database schema

Migrations live in [`services/api/internal/db/migrations/`](services/api/internal/db/migrations/) and run automatically on api boot via goose. Current version: **22**.

| # | File | Purpose |
|---|---|---|
| 00001 | `init.sql` | users, sessions, email_tokens, approval_decisions, audit_log |
| 00002 | `users_org_role.sql` | organization + stated_role fields |
| 00003 | `whitelist_and_expiry.sql` | access_grants, users.expires_at, `access_ttl` enum |
| 00004 | `support_messages.sql` | contact form storage |
| 00005 | `admin_readonly_role.sql` | `career_admin_readonly` role + grants |
| 00006 | `user_notification_status.sql` | `last_notification_{kind,at,error}` on users |
| 00007 | `admin_saved_queries.sql` | per-admin saved SQL for `/admin/db` |
| 00008 | `activity_events.sql` | member activity stream |
| 00009 | `jd_submissions.sql` | JD upload requests |
| 00010 | `support_hiring_fields.sql` | hiring-inquiry fields on contact messages |
| 00011 | `ask_roger_corpus.sql` | pgvector, `corpus_documents`, `corpus_chunks` (HNSW cosine) |
| 00012 | `notification_deliveries.sql` | one row per outbound email attempt |
| 00013 | `corpus_visibility.sql` | `corpus_documents.visibility` (public / corpus_only) |
| 00014 | `corpus_chunk_embedder.sql` | `corpus_chunks.embedder_model`, `embedded_at` (drives the embed sweep) |
| 00015 | `llm_usage_and_jd_assessment.sql` | `llm_usage` ledger; `jd_submissions.retrieval_score`, `assessment`, `result_token`, résumé columns |
| 00016 | `jd_resume_json.sql` | verified structured résumé with source chunk ids |
| 00017 | `jd_resume_pdf.sql` | locked PDF bytes + page count on the submission |
| 00018 | `jd_submitter.sql` | `jd_submissions.user_id` (JD upload is members-only) |
| 00019 | `decision_log.sql` | one row per model verdict with the evidence, prompt and response it saw; human verdict + note for `/admin/decisions` and the JSONL training export |
| 00020 | `jd_apply_url.sql` | `jd_submissions.apply_url` (application link the submitter gives; surfaced in the owner's outcome email) |
| 00021 | `jd_progress.sql` | `jd_submissions.progress_pct`, `progress_stage` (written by the pipeline, read by the submitter's progress modal) |
| 00022 | `app_settings.sql` | key/value settings table; holds `jd_fit_bands` (owner-editable on `/admin/jd`, cached 15 s) |

Roles: `career` (app owner, full DML), `career_admin_readonly` (SELECT-only; bounds the `/admin/db` blast radius).

---

## API surface

Generated reference: [`docs/api/README.md`](docs/api/README.md) (kept fresh by `make docs-api`; CI's `check-gen` step fails on drift).

Services shipped:

- **AuthService**: Register / Verify / Login / Logout / ForgotPassword / ResetPassword
- **MemberService**: GetMe / GetHistory (rest still stubbed)
- **ContactService**: SubmitContact (incl. hiring inquiry)
- **ActivityService**: RecordEvents (client beacon)
- **JdService** (members): SubmitJd / GetJdResult / ListMySubmissions / GetJdReviewConfig (the live fit bands); the owning member's session or the result token releases verdicts, résumé and PDF
- **AdminService**: contacts, registrations + member detail (with email delivery history + resend), access whitelist, activity, db console, corpus (ingest, reindex public/private and embed sweep as jobs via RunJob / GetJob with progress), JD submissions + detail + re-score + fit bands (GetJdFitBands / SetJdFitBands), decision log (ListDecisionLog / ReviewDecision / ExportDecisionLog)
- **SystemService**: GetVersion (build info); GetGovernanceStatus is declared in the proto but has no api handler (Unimplemented)
- **SidecarService** (internal gRPC): Embed / Generate / RenderResume / Health; Rerank, Classify, RunJob, GetJob are declared but return UNIMPLEMENTED (the admin jobs run inside the api, not the sidecar)

---

## Operations

- **Deploy**: [`deploy/README.md`](deploy/README.md). Sequence: wait for main CI to publish all three images (check the tag exists for web, api and sidecar) → `git pull` on the box → bump `IMAGE_TAG` → `compose pull` → `compose up -d`.
- **Verify**: `deploy/live-check.sh [--submit] https://rogerhenley.dev` before and after any maintenance. It follows the members-only policy: the JD submission runs as the admin member from the server.
- **Private corpus**: `make sync-corpus` (prod) / `make stage-corpus` (dev) from `docs/personal/corpus-manifest.txt`; then `/admin/corpus` → reindex + embed sweep. Both run as jobs with a progress readout; the page can be left and reopened.
- **Gallery**: `make photos` builds EXIF-stripped WebP derivatives from `docs/personal/images` per `apps/web/content/photos/*.md`.
- **JD reviewer**: one pipeline at a time, 15 to 30 min per JD on the box. A failed assessment marks the row failed (it never falls back to the retrieval score); use Re-score on `/admin/jd`. Fit bands (very strong ≥ 0.85, strong ≥ 0.70 = the résumé gate, possible ≥ 0.55, weak ≥ 0.35) are edited on `/admin/jd`. Every verdict lands in `/admin/decisions` for human review. Calibration and history: [`docs/llm-tuning-log.md`](docs/llm-tuning-log.md); the member-facing flow: [`docs/jd-submitter-workflow.md`](docs/jd-submitter-workflow.md); the review surface: [`docs/decision-log.md`](docs/decision-log.md).
- **LLM cutover** (done 2026-09-22): [`docs/cutover-local-to-prod.md`](docs/cutover-local-to-prod.md).

---

## Where to look next

- **[README.md](README.md)**: high-level architecture + getting started
- **[docs/FSD.md](docs/FSD.md)**: functional spec
- **[docs/adr/](docs/adr/)**: architecture decision records
- **[deploy/README.md](deploy/README.md)**: prod deployment
- **[docs/llm-tuning-log.md](docs/llm-tuning-log.md)**: LLM experiments and calibration per model
- **[docs/decision-log.md](docs/decision-log.md)**: decision review and training export
- **[docs/jd-submitter-workflow.md](docs/jd-submitter-workflow.md)**: the submitter's flow
- **[.env.prod.example](.env.prod.example)**: every prod env var, with comments
