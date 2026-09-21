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

Last verified against the running stack: 2026-09-21 (`deploy/live-check.sh`).

---

## Runtime services

| Service | Type / image | Internal address | Host port | Auth (env var) | Notes |
|---|---|---|---|---|---|
| **caddy** | `caddy:2-alpine` | in front of everything | prod: `80`, `443`; dev: `80` | Let's Encrypt via `LETSENCRYPT_EMAIL`, cert for `SITE_DOMAIN` | Routes `/api/*` to `api:8080` and everything else to `web:3000`; carries HSTS, CSP and the other security headers (`deploy/caddy/Caddyfile.prod`). Redirects plain HTTP to HTTPS, including `http://localhost` on the box. |
| **web** | Next.js 16 (App Router), standalone build; `ghcr.io/reh3376/career-site-web:<sha>` | `web:3000` | none | forwards the session cookie to api; `src/proxy.ts` redirects signed-out visitors to `/login` for every route outside the public allow-list | Public: `/`, `/contact`, `/register`, `/login`, legal pages, verify / reset / one-click approval. Members: `/home`, `/articles`, `/gallery`, `/jd-upload`, `/settings`. Admin: `/admin/*`. |
| **api** | Go 1.26 ConnectRPC; `ghcr.io/reh3376/career-site-api:<sha>` | `api:8080` | none (prod); via caddy in dev | session cookie (`career_site_session`) for members and admins; `ADMIN_USERNAME` + `CAREER_SITE_ADMIN_PW` bootstrap the admin row on boot | Health: `GET /api/healthz`, `GET /api/readyz` (checks postgres + sidecar). Runs goose migrations on boot. Plain-HTTP routes besides Connect: `POST /api/admin/decision` (one-click approval token), `GET /api/jd/resume/<id>.pdf?t=<token>` (member session + result token). Distroless image: no shell inside the container. |
| **sidecar** | Python 3.12 gRPC; `ghcr.io/reh3376/career-site-sidecar:<sha>` | `sidecar:50051` | none | none (compose network only) | `Embed` (nomic prefixes, purpose-aware), `Generate` (LLM gateway, JSON-schema constrained), `RenderResume` (Typst + pypdf, owner-password locked PDF), `Health` reports `embedder_ready`, `llm_ready`, `llm_provider`, `renderer_ready`. Providers are env-selected (`stub` / `ollama`). |
| **ollama** | `ollama/ollama:latest` | `ollama:11434` | none | none | Embedding host (`nomic-embed-text`, ~275 MB, pulled on first boot into the `ollama_models` volume) and, once flipped, the LLM host (`OLLAMA_LLM_MODEL`). Always on in prod; dev opt-in via `docker compose --profile embed` (dev usually points the sidecar at the host's own Ollama instead). Memory-capped by `OLLAMA_MEM_LIMIT`; `OLLAMA_KEEP_ALIVE` unloads idle models. |
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
| `5.161.62.205` (rogerhenley.dev) | Hetzner **CPX31** (4 vCPU / 8 GB / 40 GB, 2 GB swapfile), Ubuntu 24.04, Ashburn VA. Rescaled from CPX11 on 2026-09-21. | `ssh career@5.161.62.205` (key auth; `career` has sudo and docker) | Deploy dir `/opt/career-site` (git checkout of `main`); private corpus at `/opt/career-site-private`. The stack returns after a reboot on Docker's `restart: unless-stopped` policy; there is no systemd unit (see `deploy/README.md`). Never `compose stop` before a planned reboot. |
| owner's Mac | local dev | n/a | `docker compose up -d` with the sidecar pointed at the host's Ollama (`host.docker.internal:11434`); `qwen3:14b` and `nomic-embed-text` pulled locally. |

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
| `JD_PIPELINE_TIMEOUT_SECONDS` | wall-clock budget for one JD run (score, assess, résumé, PDF) | prod `1200` |
| `LLM_MONTHLY_CALL_CAP` | LLM gateway calls per calendar month, 0 = unlimited | prod `2000` |
| `LLM_ALLOW_STUB` | let the structured assessor run against the stub provider (CI only) | `0` |
| `RESUME_PDF_OWNER_PASSWORD` | owner password locking edits on generated résumé PDFs; empty skips the PDF | `.env.prod`; dev `.env` (never committed) |

### web

| Var | Purpose | Where set |
|---|---|---|
| `PORT`, `HOSTNAME` | Next.js bind | `3000`, `0.0.0.0` |
| `API_URL` | server-side base URL for `callApi` | `http://api:8080` |
| `NEXT_PUBLIC_API_URL` | client-side base URL | `https://${SITE_DOMAIN}` |
| `NEXT_PUBLIC_LINKEDIN_URL`, `NEXT_PUBLIC_GITHUB_URL` | outbound links (LinkedIn hidden when empty) | `.env.prod` |

### sidecar

| Var | Purpose | Where set |
|---|---|---|
| `SIDECAR_ADDR`, `SIDECAR_MAX_WORKERS` | gRPC bind, thread pool | defaults |
| `SIDECAR_EMBED_PROVIDER` | `stub` or `ollama` | prod `ollama` (since 2026-09-21); dev `ollama` when the host Ollama is up |
| `OLLAMA_URL`, `OLLAMA_EMBED_MODEL`, `SIDECAR_EMBED_DIMENSIONS` | embedder target; dimension is locked to 768 by the schema | `http://ollama:11434`, `nomic-embed-text`, `768` |
| `SIDECAR_LLM_PROVIDER`, `OLLAMA_LLM_MODEL`, `SIDECAR_LLM_TIMEOUT_SECONDS` | LLM gateway; prod stays `stub` until the local evaluation passes (`docs/cutover-local-to-prod.md`) | prod `stub`; local `ollama` / `qwen3:14b` |

### ollama

| Var | Purpose | Where set |
|---|---|---|
| `OLLAMA_EMBED_MODEL` | model pulled at container start | `nomic-embed-text` |
| `OLLAMA_MEM_LIMIT`, `OLLAMA_KEEP_ALIVE`, `OLLAMA_NUM_PARALLEL` | container memory cap, idle unload, concurrency | prod `2g`, `10m`, `1` |

### caddy

| Var | Purpose | Where set |
|---|---|---|
| `SITE_DOMAIN`, `LETSENCRYPT_EMAIL` | certificate name + ACME contact | `.env.prod` |

---

## Database schema

Migrations live in [`services/api/internal/db/migrations/`](services/api/internal/db/migrations/) and run automatically on api boot via goose. Current version: **18**.

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

Roles: `career` (app owner, full DML), `career_admin_readonly` (SELECT-only; bounds the `/admin/db` blast radius).

---

## API surface

Generated reference: [`docs/api/README.md`](docs/api/README.md) (kept fresh by `make docs-api`; CI's `check-gen` step fails on drift).

Services shipped:

- **AuthService**: Register / Verify / Login / Logout / ForgotPassword / ResetPassword
- **MemberService**: GetMe / GetHistory (rest still stubbed)
- **ContactService**: SubmitContact (incl. hiring inquiry)
- **ActivityService**: RecordEvents (client beacon)
- **JdService** (members): SubmitJd / GetJdResult, with the result token gating the résumé
- **AdminService**: contacts, registrations + member detail (with email delivery history + resend), access whitelist, activity, db console, corpus (ingest, reindex public/private, embed sweep), JD submissions + detail + re-score
- **SystemService**: version + build info
- **SidecarService** (internal gRPC): Embed / Generate / RenderResume / Health; Rerank, Classify, RunJob, GetJob are declared but unimplemented

---

## Operations

- **Deploy**: [`deploy/README.md`](deploy/README.md). Sequence: wait for main CI to publish images → `git pull` on the box → bump `IMAGE_TAG` → `compose pull` → `compose up -d`.
- **Verify**: `deploy/live-check.sh [--submit] https://rogerhenley.dev` before and after any maintenance.
- **Private corpus**: `make sync-corpus` (prod) / `make stage-corpus` (dev) from `docs/personal/corpus-manifest.txt`; then `/admin/corpus` → reindex + embed sweep.
- **Gallery**: `make photos` builds EXIF-stripped WebP derivatives from `docs/personal/images` per `apps/web/content/photos/*.md`.
- **LLM cutover**: [`docs/cutover-local-to-prod.md`](docs/cutover-local-to-prod.md).

---

## Where to look next

- **[README.md](README.md)**: high-level architecture + getting started
- **[docs/FSD.md](docs/FSD.md)**: functional spec
- **[docs/adr/](docs/adr/)**: architecture decision records
- **[deploy/README.md](deploy/README.md)**: prod deployment
- **[.env.prod.example](.env.prod.example)**: every prod env var, with comments
