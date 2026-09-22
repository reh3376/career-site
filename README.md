# career-site

Roger Henley's bidirectional-fit career site: a members-only portfolio where a visitor with approved access can upload a job description, get it scored against the owner's career corpus by a local LLM, and, for strong fits, receive a tailored two-page résumé as a locked PDF. A first-person conversational assistant ("Ask Roger") grounded in the same corpus is next.

**Live:** https://rogerhenley.dev/ · **Repo:** [github.com/reh3376/career-site](https://github.com/reh3376/career-site) · **Spec:** [`docs/FSD.md`](docs/FSD.md) · **Deploy:** [`deploy/README.md`](deploy/README.md) · **Services:** [`SERVICES.md`](SERVICES.md)

> **Status (2026-09-22):** Phase 4 in progress. Live in production: the public landing (IT editorial mode or OT HMI mode, chosen by a cookie), approval-gated registration + email verify, admin one-click Accept/Decline, sign-in with 30-day sessions, forgot-password (email link + HIBP-checked reset), member home, articles, photo gallery, and the JD reviewer (members upload a JD at `/jd-upload`, watch a 0 to 100% progress modal, and reopen the review at `/jd-upload/<id>`; the pipeline runs on the box's own Ollama with `qwen3:4b-q8_0` and `nomic-embed-text`). The admin console covers contacts, registrations + member detail, access whitelist, activity, corpus (reindex and embed sweep as jobs with progress), JD submissions with re-score and owner-editable fit bands, decision review with JSONL export, and a read-only SQL surface. Everything outside the landing, `/contact`, `/register`, `/login`, `/privacy` and `/terms` is members-only. Still ahead: Ask Roger chat, adapter training (the decision log is collecting labels), the FSD 1.0 hardening pass. See [FSD §12](docs/FSD.md#12-delivery-roadmap).

## Architecture

Three services behind one contract:

| Component | Path | Language | Role |
|---|---|---|---|
| Web app | `apps/web/` | Next.js 16 App Router / TypeScript | Public landing + members-only UI; `src/proxy.ts` enforces the access policy; calls the API through `connect-es`. Fonts (Fraunces, Inter Tight, JetBrains Mono) are self-hosted from `src/fonts` via `next/font/local`, so builds never touch Google. |
| API | `services/api/` | Go 1.26 | ConnectRPC endpoints, auth, sessions, admin, scheduling, corpus ingest + retrieval, the JD pipeline (requirement extraction, per-requirement judging, score formula in code, grounded résumé) |
| Sidecar | `services/sidecar/` | Python 3.12 | gRPC service: `Embed` (nomic-embed-text via Ollama), `Generate` (LLM gateway, JSON-schema constrained, refuses truncated results), `RenderResume` (Typst PDF locked with pypdf) |

Alongside: Postgres 16 + pgvector (768-dim HNSW cosine), an Ollama container (embeddings and the LLM, one loaded model at a time), and Caddy in front. Everything runs on one Hetzner CPX31 via Docker Compose; that box is the ceiling, so the workload is fitted to it rather than the other way round.

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
  src/fonts/              self-hosted WOFF2 fonts (see its README)
  content/                public corpus (articles, photo captions); bind-mounted into the api as /corpus
deploy/                 Caddyfiles, server bootstrap, corpus sync, live-check.sh
docs/
  FSD.md                  functional spec — authoritative product + architecture doc
  adr/                    architecture decision records
  api/                    generated API reference (README.md + endpoints.json)
  llm-tuning-log.md       every LLM experiment and calibration, by model
  decision-log.md         the decision-review surface and the training-label export
  jd-submitter-workflow.md  what a member sees from upload to result email
  cutover-local-to-prod.md  how the LLM went from the owner's machine to the box
scripts/dev-push.sh     runs the CI checks locally, pushes only if they pass
scripts/gen_api_docs.py generates docs/api/ from the compiled buf image
Makefile                single entry point for local + CI automation
```

Generated code is committed and drift-checked; never edit it by hand. See [`proto/README.md`](proto/README.md) for the rules on editing contracts.

## Getting started

### Prerequisites

- Go (current stable)
- Python 3.12+ with [`uv`](https://docs.astral.sh/uv/)
- Node.js + pnpm (for the web app)
- Docker + Compose for the full stack (`docker compose up -d`); for real embeddings and LLM calls in dev, point the sidecar at an Ollama on the host with `SIDECAR_EMBED_PROVIDER=ollama SIDECAR_LLM_PROVIDER=ollama` (see [`SERVICES.md`](SERVICES.md))

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
- [LLM tuning log](docs/llm-tuning-log.md): models tried, prompts, score formula, calibration per model; read before touching the JD pipeline
- [Decision log](docs/decision-log.md): the per-verdict review surface at `/admin/decisions` and the JSONL export for adapter training
- [JD submitter workflow](docs/jd-submitter-workflow.md): the member-facing flow, progress modal, result panel and emails by fit category
- [Local to prod cutover](docs/cutover-local-to-prod.md): how the LLM rollout was sequenced
- [Services](SERVICES.md): every service, port, env var and migration, plus [`deploy/README.md`](deploy/README.md) for the rollout

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the branch workflow, docs discipline, and how to request repo-collaborator access. `main` is protected: every change lands through a PR with the api, web, sidecar, proto and gitleaks checks green. Bugs and features go through GitHub Issues (templates at `.github/ISSUE_TEMPLATE/`). Security issues follow [`SECURITY.md`](SECURITY.md) — private disclosure via GitHub Security Advisories, not public issues.

## License

- **Code:** MIT — see [`LICENSE`](LICENSE).
- **Content** (writing, images, résumés, Ask Roger persona and answers): © Roger E. Henley II, all rights reserved with non-commercial quotation permitted. See [`LICENSE-CONTENT.md`](LICENSE-CONTENT.md).
