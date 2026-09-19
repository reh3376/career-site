# services/sidecar

Python gRPC sidecar for career-site. Owns embedding, reranking, classification, and batch jobs — everything where Python's libraries are decisive and latency is not on the request-critical path (per [ADR 0004](../../docs/adr/0004-go-api-with-python-sidecar.md)).

## Layout

```
services/sidecar/
├── pyproject.toml
├── src/
│   ├── career_sidecar/       hand-written package (server, config, servicer)
│   ├── career/               generated Protobuf + gRPC stubs (do not edit)
│   └── buf/                  generated buf.validate stubs (do not edit)
├── tests/
└── Dockerfile
```

## Run locally

```bash
uv sync --extra dev
uv run python -m career_sidecar             # binds [::]:50051 by default
SIDECAR_ADDR=[::]:6000 uv run python -m career_sidecar
```

Verify from the Go API tree (once wired) or `grpcurl`:

```bash
grpcurl -plaintext localhost:50051 career.sidecar.v1.SidecarService/Health
```

## Test

```bash
uv run pytest
```

## Build

```bash
docker build -t career-site/sidecar:dev .
```

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `SIDECAR_ADDR` | `[::]:50051` | Listen address |
| `SIDECAR_MAX_WORKERS` | `8` | Thread pool size |
| `SIDECAR_SHUTDOWN_GRACE_SECONDS` | `5` | Graceful stop deadline |
| `SIDECAR_VERSION` | `<package metadata>` | Overrides the version reported by `Health` |

## What is not here yet

Phase 0 scaffold. Only `Health` is functional; every other RPC returns `UNIMPLEMENTED` with a message naming the phase that lights it up:

- **Phase 2** — `RunJob`, `GetJob` (content ingestion CLI wiring)
- **Phase 4** — `Embed`, `Rerank`, `Classify` (Ask Roger)
