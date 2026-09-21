# Services

Every service that runs in this project, with type, address, port, and
credentials. **No literal secrets in this file** — credentials appear
as the env-var name that carries the value; the value itself lives in
`.env.prod` (prod, mode 0600) or `.env` / `.env.local` (dev).

The dev DSNs use literal placeholder values (`career_dev_only`,
`minio_dev_only`) — these are safe to commit because they are only
loaded when `NODE_ENV=development` and never accepted by prod. See
[`docker-compose.yml`](docker-compose.yml) for the base and
[`docker-compose.prod.yml`](docker-compose.prod.yml) for prod overlay.

---

## Runtime services

| Service | Type / image | Internal address | Host port (dev) | Auth (env var) | Notes |
|---|---|---|---|---|---|
| **api** | Go 1.26 ConnectRPC — `ghcr.io/reh3376/career-site-api:<sha>` | `api:8080` | `8080` | session cookie for members; `ADMIN_USERNAME` + `CAREER_SITE_ADMIN_PW` bootstrap the admin | Health: `GET /api/healthz`, `GET /api/readyz`. Runs migrations on boot (goose, `services/api/internal/db/migrations/`). |
| **web** | Next.js 16 (App Router) — `ghcr.io/reh3376/career-site-web:<sha>` | `web:3000` | `3000` | forwards session cookie to api | Renders public + gated surfaces; talks to api via `NEXT_PUBLIC_API_URL`. |
| **sidecar** | Python 3.12 gRPC — `ghcr.io/reh3376/career-site-sidecar:<sha>` | `sidecar:50051` | not exposed | none (internal only) | Embedding, rerank, ingest jobs, `career-cli`. See `services/sidecar/`. |
| **caddy** | `caddy:2-alpine` (prod) | in front of everything | prod: `80`, `443` | Let's Encrypt via `LETSENCRYPT_EMAIL` | Only reverse-proxy in prod; also carries the security headers (HSTS, COOP, CSP). |
| **postgres** | `pgvector/pgvector:pg16` | `postgres:5432` | `5432` (dev only) | app role: `career` / `POSTGRES_PASSWORD`; read-only admin role for `/admin/db`: `career_admin_readonly` / `DB_READONLY_PASSWORD` (see migration `00005`) | Not port-exposed in prod. |
| **minio** | `quay.io/minio/minio:latest` | `minio:9000` (S3), `minio:9001` (console) | dev: `9000`, `9001` | `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` | Object storage for résumés + generated PDFs. Not port-exposed in prod. |
| **mailpit** | `axllent/mailpit:v1` (dev only) | `mailpit:1025` (SMTP), `mailpit:8025` (web UI) | dev: `1025`, `8025` | none | Prod uses Resend instead — profile `disabled` in `docker-compose.prod.yml`. |

Prod host: `career-site-prod-01` (Hetzner CX22), user `career`,
SSH `ssh career@5.161.62.205`. Deploy dir `/opt/career-site`. See
[`deploy/README.md`](deploy/README.md) for the full workflow.

---

## Environment variables (by service)

### api

| Var | Purpose | Where set |
|---|---|---|
| `API_ADDR` | listen address | prod: `:8080` (default) |
| `API_ENV` | environment tag for logs | prod: `production` |
| `DATABASE_URL` | app pool DSN | built from `POSTGRES_PASSWORD` |
| `DATABASE_URL_READONLY` | `/admin/db` read-only pool | built from `DB_READONLY_PASSWORD` (empty → falls back to write pool with warn) |
| `DB_READONLY_PASSWORD` | ALTER ROLE on boot | prod-only, in `.env.prod` |
| `SIDECAR_ADDR` | sidecar gRPC target | `sidecar:50051` |
| `WEB_BASE_URL` | absolute URL used in outbound email links | `https://${SITE_DOMAIN}` |
| `MAIL_FROM` | From: header | `.env.prod` |
| `OWNER_CONTACT_EMAIL` | admin approval + escalation recipient | `.env.prod` |
| `EMAIL_PROVIDER` | `resend` (prod) or `smtp` (dev via mailpit) | prod: `resend` |
| `RESEND_API_KEY` | Resend API key | `.env.prod` |
| `DECISION_TOKEN_SECRET` | HMAC key for one-click Accept/Decline URLs | `.env.prod` (hex) |
| `COOKIE_SECURE` | Set-Cookie `Secure` flag | prod: `1` |
| `PWNED_CHECK_ENABLED` | HIBP range check on register/reset | prod: `1` |
| `ADMIN_USERNAME` / `CAREER_SITE_ADMIN_PW` | admin bootstrap; ensures the row exists in `role=admin, status=active` on every boot | `.env.prod` |
| `SESSION_TTL_HOURS` | session cookie lifetime | default 720 (30d) |

### web

| Var | Purpose | Where set |
|---|---|---|
| `PORT` | Next.js listen port | `3000` |
| `HOSTNAME` | Next.js bind | `0.0.0.0` |
| `API_URL` | server-side base URL for callApi | `http://api:8080` (compose DNS) |
| `NEXT_PUBLIC_API_URL` | client-side base URL | `https://${SITE_DOMAIN}` |
| `NEXT_PUBLIC_LINKEDIN_URL` | outbound LinkedIn link (hidden if empty) | prod: `.env.prod` |
| `NEXT_PUBLIC_GITHUB_URL` | outbound GitHub link | default: `https://github.com/reh3376` |

### sidecar

| Var | Purpose | Where set |
|---|---|---|
| (none required today) | | |

### caddy

| Var | Purpose | Where set |
|---|---|---|
| `SITE_DOMAIN` | ACME certificate name + `Host` matcher | `.env.prod` |
| `LETSENCRYPT_EMAIL` | ACME contact | `.env.prod` |

---

## Database — Postgres schema

Migrations live in [`services/api/internal/db/migrations/`](services/api/internal/db/migrations/) and run automatically on api boot via goose. Current version: **8**.

| # | File | Purpose |
|---|---|---|
| 00001 | `init.sql` | users, sessions, email_tokens, approval_decisions, audit_log |
| 00002 | `users_org_role.sql` | organization + stated_role fields |
| 00003 | `whitelist_and_expiry.sql` | access_grants table, users.expires_at, `access_ttl` enum |
| 00004 | `support_messages.sql` | contact form storage |
| 00005 | `admin_readonly_role.sql` | `career_admin_readonly` role + grants |
| 00006 | `user_notification_status.sql` | `last_notification_{kind,at,error}` columns on users |
| 00007 | `admin_saved_queries.sql` | per-admin saved SQL for /admin/db |
| 00008 | `activity_events.sql` | member activity stream (server + client) |

Roles: `career` (app owner, full DML), `career_admin_readonly` (SELECT-only, USAGE on schema — bounds `/admin/db` blast radius).

---

## API surface

Generated reference: [`docs/api/README.md`](docs/api/README.md) (kept fresh by `make docs-api`; CI's `check-gen` step fails on drift).

Services shipped:

- **AuthService** — Register / Verify / Login / Logout / ForgotPassword / ResetPassword
- **MemberService** — GetMe / GetHistory (rest still stubbed)
- **ContactService** — SubmitContact
- **ActivityService** — RecordEvents (client beacon)
- **AdminService** — the full admin console (contacts, registrations + detail, access whitelist, activity, db)
- **SystemService** — version + build info

---

## Where to look next

- **[README.md](README.md)** — high-level architecture + getting-started
- **[docs/FSD.md](docs/FSD.md)** — functional spec / architecture
- **[docs/adr/](docs/adr/)** — architecture decision records
- **[deploy/README.md](deploy/README.md)** — prod deployment
- **[.env.prod.example](.env.prod.example)** — every prod env var, with comments
