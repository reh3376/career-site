# career-site

Roger Henley's interactive career portfolio: a gated, personalized site with a first-person conversational assistant ("Ask Roger") grounded in the owner's own writing, résumés, and repositories.

> **Status:** Early scaffolding. The API contracts and generated code are in place; consumer builds (Go API, Next.js app, Python sidecar) have not been scaffolded yet. See [`docs/FSD.md`](docs/FSD.md) §12 for the delivery roadmap.

## Architecture

Three services behind one contract:

| Component | Path | Language | Role |
|---|---|---|---|
| Web app | `apps/web/` | Next.js 15 / TypeScript | Public landing + gated member UI; calls the API through `connect-es` |
| API | `services/api/` | Go (stable) | ConnectRPC endpoints, auth, sessions, personalization, chat orchestration, admin, scheduling |
| Sidecar | `services/sidecar/` | Python 3.12+ | gRPC service for embedding, rerank, classification, and batch jobs; hosts the `career-cli` |

The `.proto` files in `proto/` are the single source of truth. Everything else — Go, TypeScript, and Python stubs plus the API reference — is generated from them. See [ADR 0004](docs/adr/0004-go-api-with-python-sidecar.md) for the split and [ADR 0017](docs/adr/0017-connectrpc-transport.md) for the transport choice.

## Repository layout

```
proto/                  API contracts (career.v1 + career.sidecar.v1)
services/api/           Go ConnectRPC API
  gen/                    generated Go + connect handlers (committed)
services/sidecar/       Python gRPC sidecar
  src/                    generated Python + buf/validate stubs (committed)
apps/web/               Next.js frontend
  src/gen/                generated TypeScript + connect-es clients (committed)
docs/
  FSD.md                  functional spec — authoritative product + architecture doc
  adr/                    architecture decision records
  api/                    generated API reference (README.md + endpoints.json)
scripts/gen_api_docs.py generates docs/api/ from the compiled buf image
Makefile                single entry point for local + CI automation
```

Generated code is committed and drift-checked; never edit it by hand. See [`proto/README.md`](proto/README.md) for the rules on editing contracts.

## Getting started

### Prerequisites

- Go (current stable)
- Python 3.12+ with [`uv`](https://docs.astral.sh/uv/)
- Node.js + pnpm (for the web app, once scaffolded)

### Install tooling and regenerate

```bash
make tools        # installs buf into $GOBIN
make gen          # regenerates Go, TypeScript, and Python from proto/
make docs-api     # regenerates docs/api/README.md and docs/api/endpoints.json
```

`make docs-api` depends on `make gen` — the doc generator imports Python stubs from `services/sidecar/src/`.

### Common tasks

```bash
make help         # list all targets
make lint-proto   # buf lint (STANDARD + COMMENTS)
make breaking     # buf breaking against main
make check-gen    # regenerate everything and fail if anything drifted
```

## Workflow for changing the API

1. Edit the `.proto` file in `proto/career/v1/` (or `proto/career/sidecar/v1/`).
2. `make lint-proto gen docs-api`.
3. Review the diff in `docs/api/README.md` — that's the contract from the frontend's point of view.
4. Commit the `.proto` change together with every generated file it touched.
5. `make breaking` must pass; `career.v1` is additive-only.

Full rules: [`proto/README.md`](proto/README.md).

## Documentation

- [Functional Specification](docs/FSD.md) — product, requirements, architecture, roadmap
- [API reference](docs/api/README.md) — generated from the contracts
- [Endpoint index](docs/api/endpoints.json) — machine-readable, feeds tooling
- [Architecture Decision Records](docs/adr/README.md)

## License

TBD — private working repository at present.
