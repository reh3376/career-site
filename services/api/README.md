# services/api

Go ConnectRPC API for career-site. Owns everything on the request path and everything that touches member data (per [ADR 0004](../../docs/adr/0004-go-api-with-python-sidecar.md)).

## Layout

```
services/api/
├── cmd/api/            main — server bootstrap, signals, graceful shutdown
├── internal/
│   ├── build/          -ldflags-injected build metadata (version, commit, built_at)
│   ├── config/         env-driven runtime configuration
│   ├── handlers/       ConnectRPC service implementations
│   └── server/         HTTP mux, health endpoints, middleware, lifecycle
├── gen/                generated Protobuf + ConnectRPC + gRPC sidecar client (do not edit)
└── Dockerfile          distroless multi-stage build
```

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

## What is not here yet

Phase 0 scaffold; everything else lives in later phases:

- **Phase 1** (identity + approval-gated registration): `AuthService`, `MemberService`, sessions, Turnstile, email delivery, admin approval workflow, minimal *Pending approvals* admin surface.
- **Phase 2+**: `ContentService`, `HomeService`, `ActivityService`, `ChatService`, `DownloadService`, `ContactService`, `AdminService`, sidecar client wiring, in-process scheduler.

The generated stubs for every service are in `gen/`; handlers are added service-by-service under `internal/handlers/`.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `API_ADDR` | `:8080` | Listen address |
| `API_ENV` | `development` | Environment label for logs |
| `API_READ_TIMEOUT_SECONDS` | `15` | HTTP server read timeout |
| `API_WRITE_TIMEOUT_SECONDS` | `30` | HTTP server write timeout |
