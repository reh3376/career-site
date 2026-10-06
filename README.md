# career-site

Roger Henley's bidirectional-fit career site. A visitor with approved access can upload a job description, get it scored against the owner's career corpus by a local LLM, and, for strong fits, receive a tailored two-page résumé as a locked PDF. A first-person assistant ("Ask Roger") answers from the same corpus and shows its sources. It also hosts **the decision test**, a fifteen-minute research instrument measuring how accuracy and confidence come apart under working-memory load. The writing, the public gallery, the AI disclosure page and the test are open to anyone.

**Live:** https://rogerhenley.dev/ · **Repo:** [github.com/reh3376/career-site](https://github.com/reh3376/career-site) · **Spec:** [`docs/FSD.md`](docs/FSD.md) · **Deploy:** [`deploy/README.md`](deploy/README.md) · **Services:** [`SERVICES.md`](SERVICES.md)

> **Status (2026-10-06):** Phase 4 shipped. Live in production: the public landing (IT editorial mode or OT HMI mode, chosen by a cookie), approval-gated registration + email verify, admin one-click Accept/Decline, sign-in with 30-day sessions, forgot-password (email link + HIBP-checked reset), member home, articles, photo gallery, the meeting scheduler, the JD reviewer (members upload a JD at `/jd-upload`, watch a 0 to 100% progress modal, and reopen the review at `/jd-upload/<id>`), and **Ask Roger** at `/ask` and from a panel on every page: a Q&A bank of the owner's own answers served verbatim, then retrieval over the career corpus plus a career facts sheet, then `qwen3:4b-q8_0`, with published passages cited underneath. The whole pipeline runs on the box's own Ollama with `nomic-embed-text` for retrieval; nothing is sent to a hosted API.
>
> Public without an account: the landing, `/articles` and every article, the public half of `/gallery`, `/how-ask-roger-works`, `/contact`, `/register`, `/login`, `/privacy`, `/terms` and the four decision-test routes. `/decision-test/about` is public deliberately: most participants are anonymous, and gating the debrief would land hardest on the people who helped without wanting anything back. One list, `apps/web/src/lib/public-routes.ts`, drives the proxy, `robots.txt` and the sitemap so the three cannot drift.
>
> Also live: **the decision test** at `/decision-test`, a fifteen-minute instrument open to anyone with or without an account, measuring what happens to a person's accuracy and to their confidence as working memory is loaded. The answer key never crosses the API boundary: questions go out without it, grading happens in the api, and no participant is ever told which items they got wrong. The dataset is the deliverable rather than the page, so every session records the instrument and item-set version it ran under, nothing is deleted, and seven SQL views define every number once. A run ends with one question about method rather than memory, and every participant can read [`/decision-test/about`](https://rogerhenley.dev/decision-test/about) whether or not they left an address. Repeat runs are welcome and are labelled as such: each session stores how many prior runs were visible by account, by address and by cookie, as three separate point-in-time observations, so the attempt number is derived in SQL rather than decided at write time. See [FSD §5.14](docs/FSD.md) and [`docs/fsd-decision-test.md`](docs/fsd-decision-test.md).
>
> Curation is part of the instrument, not an afterthought: `/admin/decision-test` reviews a run or any single block of it, with a note, a reason and a status, and exactly one status (`do_not_use`) removes data from the views that make claims about people. Every judgement is appended to a history, so an exclusion can always be explained. Adding `?synthetic=1` to the test URL marks a run as a test rather than data, says so on every screen, and lets an agent past the tap-along check, which is a motor skill a driven browser cannot perform; synthetic runs are excluded from every aggregate. See [`docs/roadmap-decision-test.md`](docs/roadmap-decision-test.md).
>
> The admin console covers contacts, registrations + member detail, access whitelist, activity, analytics, corpus (reindex and embed sweep as jobs with progress), JD submissions with re-score and owner-editable fit bands, the Q&A bank, decision review with JSONL export, evaluations and the gate, operations, the meeting scheduler, the decision test with its timings, per-run and per-block curation and a CSV export of the dataset, and a read-only SQL surface: fifteen surfaces in all.
>
> Still ahead: adapter training (the decision log is collecting labels, and the Q&A near-miss scores now say which phrasings to add), the FSD 1.0 hardening pass. The front end has source-level tests only (`apps/web/src/**/*.test.ts`) and no component harness, so nothing there renders a component and asserts on the result; see [Tests](#tests) for what each layer does and does not cover. See [FSD §12](docs/FSD.md#12-delivery-roadmap).

## Architecture

Three services behind one contract:

| Component | Path | Language | Role |
|---|---|---|---|
| Web app | `apps/web/` | Next.js 16 App Router / TypeScript | Public landing + members-only UI; `src/proxy.ts` enforces the access policy; calls the API through `connect-es`. Fonts (Fraunces, Inter Tight, JetBrains Mono) are self-hosted from `src/fonts` via `next/font/local`, so builds never touch Google. |
| API | `services/api/` | Go 1.27 | ConnectRPC endpoints, auth, sessions, admin, scheduling, corpus ingest + retrieval, the JD pipeline (requirement extraction, per-requirement judging, score formula in code, grounded résumé) |
| Sidecar | `services/sidecar/` | Python 3.14 | gRPC service: `Embed` (nomic-embed-text via Ollama), `Generate` (LLM gateway, JSON-schema constrained, refuses truncated results), `RenderResume` (Typst PDF locked with pypdf) |

Alongside: Postgres 16 + pgvector (768-dim HNSW cosine), an Ollama container (embeddings and the LLM, one loaded model at a time), and Caddy in front. Everything runs on one Hetzner CPX41 via Docker Compose; that box is the ceiling, so the workload is fitted to it rather than the other way round.

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
  tests/                  UCTS conversion specs + the shared UxTS runner core
  llm-tuning-log.md       every LLM experiment and calibration, by model
  decision-log.md         the decision-review surface and the training-label export
  fsd-decision-test.md    the decision test's own spec, with its roadmap and sprint plan
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
- Python 3.14 with [`uv`](https://docs.astral.sh/uv/) (`pyproject.toml` requires >=3.12; the image ships 3.14)
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

## Tests

There is no aggregate `make test`; each service runs its own, and `scripts/dev-push.sh` runs the CI checks locally and pushes only if they pass.

```bash
scripts/dev-push.sh                                    # what CI will do, before CI does it
(cd services/api && gofmt -l . && go vet ./... && go test ./...)
(cd apps/web && npm run typecheck && npm run lint && npm test && npm run build)
(cd services/sidecar && uv run pytest)
make ucts                                              # document-conversion specs
make tokens versions check-gen                         # design tokens, base images, generated-code drift
```

`npm run build` is part of the list, not an afterthought: it catches Next.js rules that `tsc` and eslint both miss.

**The api tests want a Postgres.** Without `TEST_DATABASE_URL` the database-backed tests skip, so `go test ./...` passes on a laptop with nothing running. CI sets it unconditionally and the tests **fail rather than skip** when `CI` is set and the DSN is missing, because a silent skip is indistinguishable from a pass and that is exactly the hole they were written to close. To run them locally, start a Postgres and replay the migrations in order, the way `.github/workflows/ci.yml` does:

```bash
docker run -d --name dt-test-pg -e POSTGRES_USER=career -e POSTGRES_PASSWORD=career \
  -e POSTGRES_DB=career -p 55432:5432 pgvector/pgvector:pg16

for f in $(ls services/api/internal/db/migrations/*.sql | sort); do
  sed -n '/-- +goose Up/,/-- +goose Down/p' "$f" | grep -v '^-- +goose' \
    | docker exec -i -e PGPASSWORD=career dt-test-pg psql -U career -d career -v ON_ERROR_STOP=1 -q
done

cd services/api && TEST_DATABASE_URL='postgres://career:career@localhost:55432/career?sslmode=disable' \
  go test -count=1 ./...
```

Order matters and `goose` is not used here: the loop replays the `Up` half of every migration in filename order against an empty database, which is also what proves a migration is correct in the order it will actually run.

Two production outages in one evening came from SQL that built, vetted, formatted and shipped: an INSERT listing sixteen columns against fifteen values, which took the decision test down for about an hour and forty minutes, and a parameter Postgres inferred as `text` assigned to a `bigint` column, which made a Save button do nothing at all. Neither was catchable anywhere — Go cannot see inside a string literal, `gofmt` has no opinion about SQL, the migrations job replays the schema but runs no Go. The only thing that catches both is executing the query.

Some tests parse source rather than running behaviour: the events registry, the admin auth gate, the INSERT column counts, the decision test's session guards. That is crude and deliberate. Each one guards an invariant whose failure mode is silence rather than an exception — a method missing an auth check, a literal where a value belongs — and each exists because the thing it checks failed quietly first. They all fail if their own parser stops matching, so a drifting check cannot pass by asserting nothing.

**None of this says a page works.** Every UI defect on this site so far was invisible to the whole suite and obvious to a person using the thing. Deploy it, open it, and press the button.

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
- [Evaluating the JD reviewer](docs/evaluation.md): how to run one, how to read the three numbers, the noise floor, and the failure modes; read before running or interpreting an evaluation
- [Ask Roger](docs/ask-roger.md): the answer pipeline, the Q&A bank, grounding and what is cited
- [The decision test](docs/fsd-decision-test.md): the instrument's own spec — item design, the load conditions, scoring and what each measure claims; with [its roadmap](docs/roadmap-decision-test.md) and [the curation sprint](docs/sprint-decision-test-curation.md)
- [LLM tuning log](docs/llm-tuning-log.md): models tried, prompts, score formula, calibration per model; read before touching the JD pipeline
- [Decision log](docs/decision-log.md): the per-verdict review surface at `/admin/decisions` and the JSONL export for adapter training
- [JD submitter workflow](docs/jd-submitter-workflow.md): the member-facing flow, progress modal, result panel and emails by fit category
- [Local to prod cutover](docs/cutover-local-to-prod.md): how the LLM rollout was sequenced
- [Services](SERVICES.md): every service, port, env var and migration, plus [`deploy/README.md`](deploy/README.md) for the rollout
- [Backlog](docs/backlog.md): open work only, with the reasoning that opened each item
- [Conversion test specs](docs/tests/ucts/README.md): UCTS, the framework that checks `.docx` / `.pdf` / `.md` / PDF conversions rather than eyeballing them
- [Metrics](docs/metrics.md) and [events](docs/events/README.md): what is measured and what each event means
- [Backups](deploy/backup/README.md): nightly dumps, the weekly restore rehearsal, and the corpus snapshot that gives an ingest an undo

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the branch workflow, docs discipline, and how to request repo-collaborator access. `main` is protected: every change lands through a PR with the api, web, sidecar, proto and gitleaks checks green. Bugs and features go through GitHub Issues (templates at `.github/ISSUE_TEMPLATE/`). Security issues follow [`SECURITY.md`](SECURITY.md) — private disclosure via GitHub Security Advisories, not public issues.

## License

- **Code:** MIT — see [`LICENSE`](LICENSE).
- **Content** (writing, images, résumés, Ask Roger persona and answers): © Roger E. Henley II, all rights reserved with non-commercial quotation permitted. See [`LICENSE-CONTENT.md`](LICENSE-CONTENT.md).
