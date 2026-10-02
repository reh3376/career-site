# services/api

Go ConnectRPC API for career-site. Owns everything on the request path and everything that touches member data (per [ADR 0004](../../docs/adr/0004-go-api-with-python-sidecar.md)).

## Layout

```
services/api/
├── cmd/api/            main — bootstrap, migrations, job registration, shutdown
├── internal/
│   ├── auth/           sessions, password hashing, MFA, Turnstile
│   ├── build/          -ldflags-injected metadata (version, commit, built_at)
│   ├── calendar/       Google Calendar OAuth + event writes
│   ├── chat/           Ask Roger: retrieval, Q&A bank, answer assembly
│   ├── config/         env-driven runtime configuration
│   ├── corpusscope/    how much of the corpus a request may retrieve
│   ├── db/             pool, migrations, query helpers
│   ├── email/          Resend and SMTP providers, templates
│   ├── events/         the product event stream (docs/events/README.md)
│   ├── handlers/       ConnectRPC service implementations
│   ├── ingest/         source text to chunks to embeddings
│   ├── jd/             posting assessment: requirements, evidence, scoring
│   ├── jobs/           in-process runner for admin work that outlives an RPC
│   ├── llm/            gateway to generation; the sidecar owns the provider
│   ├── prompts/        versioned registry of prompts and output schemas
│   ├── ratelimit/      per-IP and per-account limits
│   ├── runid/          which pipeline run the current work belongs to
│   ├── scheduler/      recurring in-process jobs
│   ├── scheduling/     which meeting slots a member may book
│   ├── secrets/        envelope encryption for stored tokens
│   ├── server/         HTTP mux, health endpoints, middleware, lifecycle
│   ├── sidecar/        gRPC client for the Python sidecar
│   ├── tenant/         which site a request belongs to (one today)
│   └── users/          members, JD runs, Q&A bank, decision log
├── gen/                generated Protobuf + ConnectRPC + gRPC sidecar client (do not edit)
└── Dockerfile          distroless multi-stage build
```

Thirteen services are implemented: `System`, `Auth`, `Member`, `Content`, `Home`, `Activity`, `Chat`, `Jd`, `Meeting`, `Download`, `Contact`, `Event`, `Admin`.

`internal/handlers` imports twelve other internal packages and holds the only file over 800 lines, which is why `docs/backlog.md` §4c wants a linter pointed here first.

## Run locally

```bash
go run ./cmd/api                   # binds :8080 by default
API_ADDR=:9090 go run ./cmd/api    # custom port
```

Verify:

```bash
curl -s localhost:8080/api/healthz
curl -s localhost:8080/api/readyz
curl -s -X POST localhost:8080/api/career.v1.SystemService/GetVersion \
  -H 'Content-Type: application/json' \
  -H 'Connect-Protocol-Version: 1' \
  -d '{}'
```

## Test

```bash
go test ./...
```

## Build

```bash
go build -o bin/api ./cmd/api
```

Or via Docker (uses build args to stamp version/commit/built_at into the binary):

```bash
docker build \
  --build-arg VERSION=$(git describe --tags --always) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILT_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -t career-site/api:dev .
```

## Background jobs

Seven recurring jobs run inside this process rather than as cron entries, so they ship and roll back with the binary. `cmd/api/main.go` registers them and `internal/scheduler` runs them:

| Job | Interval | Does |
|---|---|---|
| `expiry-warn` | `EXPIRY_INTERVAL_SECONDS` | warns members whose access is about to lapse |
| `expiry-cut` | `EXPIRY_INTERVAL_SECONDS` | revokes it when it does |
| `auto-decline` | `EXPIRY_INTERVAL_SECONDS` | declines registrations the owner never acted on |
| `calendar-expiry-warn` | 6h | warns the owner before the Google refresh token expires |
| `prompt-warm` | 1m | keeps the LLM KV cache prefix warm |
| `qa-embed` | 5m | embeds newly approved Q&A bank phrasings |
| `events-anonymize` | 24h | drops event identity after `EVENT_IDENTITY_RETENTION_DAYS` |

Separately, startup reconciles work that a restart stranded mid-flight: `FailStrandedJd`, `FailStrandedRuns` and `FailStrandedEvalRuns` close runs left in a non-terminal status, because a process that dies during generation would otherwise leave a row claiming to be generating forever.

## Configuration

Local runs need almost nothing; `DATABASE_URL` and the four below are enough to start the server and serve `/version`.

| Variable | Default | Purpose |
|---|---|---|
| `API_ADDR` | `:8080` | Listen address |
| `API_ENV` | `development` | Environment label for logs |
| `API_READ_TIMEOUT_SECONDS` | `15` | HTTP server read timeout |
| `API_WRITE_TIMEOUT_SECONDS` | `30` | HTTP server write timeout |
| `API_SKIP_MIGRATE` | unset | skip migrations at boot |

`internal/config` reads about forty more, covering the database, email, secrets, the sidecar and the LLM, rate limits and retention. They are not duplicated here: `.env.example` is the list with defaults and `deploy/README.md` is the list with production values and the reasoning behind them. `internal/config/config.go` is the authority if the two ever disagree.
