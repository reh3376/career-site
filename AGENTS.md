# AGENTS.md

Conventions for AI-assisted development sessions in this repository (Claude Code, Cursor, Copilot, etc.). Human contributors follow the same rules; the acronym is just what the tools call this file.

## Read first, in this order

1. **`docs/FSD.md`** — the authoritative specification. Version, change log, personas, requirements, architecture, data model, and delivery roadmap. When the code and the FSD disagree, amend the FSD (via PR) first, then change the code. Current version: **0.4.0**, audited against the running system 2026-10-01.
2. **`docs/adr/`** — one Architecture Decision Record per resolved decision (`D-NN` → `NNNN-…`). Read these before proposing a change that would revisit a decision.
3. **`proto/README.md`** — the rules for editing the API contracts. Every RPC has an auth level, every request field has `buf.validate` rules, and `career.v1` is additive-only.
4. **`README.md`** — a fast tour of the project and how to run it locally.

## Repository shape

```
proto/                 API contracts — SINGLE source of truth
services/api/          Go ConnectRPC API (owns request path, member data, streaming)
services/sidecar/      Python gRPC sidecar (embed, rerank, classify, batch jobs)
apps/web/              Next.js frontend (App Router, RSC, connect-es)
docs/                  FSD, ADRs, generated API reference, backlog, tuning log, UCTS specs
packages/schema/       JSON Schema for a content model nothing reads yet (see its README)
deploy/                Caddy, postgres init, and the production deploy runbook
```

Generated code trees — **never edit by hand**:

- `services/api/gen/**` (Go + Connect + gRPC sidecar client)
- `apps/web/src/gen/**` (TypeScript for connect-es)
- `services/sidecar/src/{career,buf}/**` (Python protobuf + gRPC stubs)

## Standing rules

- **Spec-first.** Edit `.proto` → `make gen docs-api` → commit generated outputs with the `.proto` change. If your change alters behavior, amend the FSD first (`docs/FSD.md` + changelog + a new/updated ADR when a decision moves).
- **Never edit generated files.** They are drift-checked in CI (`make check-gen`); a hand-edit fails the merge.
- **Additive-only in `career.v1`.** Add fields, methods, and enum values; never renumber, rename, or change types. `make breaking` and the `proto` CI job enforce this.
- **Every RPC declares its access policy** with `option (career.v1.auth) = AUTH_LEVEL_…;`. The server refuses to start with a method that has no auth level.
- **Every request field carries validation** via `buf.validate`.
- **Frontend copy has no em dashes.** Use `,` `.` `:` `·` or `-` in user-visible text.
- **Approval-gated registration is the default** (FSD 0.3.3 D-02, ADR-0002). A verified user enters `pending_approval`; the admin approves or declines via a signed one-click email link. Do not reintroduce open-with-verification.
- **`.env` is gitignored; `.env.example` is tracked**. Real secrets go in `.env` only. `docs/personal/` is also gitignored (FSD A-05 — private material never in the repo).
- **Commits use Conventional Commits** (`feat:`, `fix:`, `docs:`, `chore:`, `ci:`, `db:`, `api:`, `web:`, `sidecar:`, `compose:`). Reference the FSD ID (`FR-AUTH-14`, `NFR-SEC-02`, …) when applicable.

## Local development

```bash
docker compose up -d               # caddy, web, api, sidecar, postgres, mailpit, ollama, minio
curl http://localhost/api/readyz   # {"status":"ok","checks":{"postgres":true,"sidecar":true}}
docker compose down
```

Two things the first run gets wrong without warning:

- **`minio` will fail to pull.** `quay.io/minio/minio` is no longer anonymously pullable; this is a verified policy change, not a blip, and pinning a tag does not help (`docs/backlog.md`). Nothing in the current stack reads object storage, so `docker compose up -d` minus that service is a working stack.
- **Both sidecar providers default to `stub`.** Every interface works and every embedding is meaningless. The JD assessor refuses a stub unless `LLM_ALLOW_STUB=1`. For the real pipeline, see CONTRIBUTING.md's local-development section.

Standalone:

```bash
# API
cd services/api && go run ./cmd/api

# Sidecar
cd services/sidecar && uv run python -m career_sidecar

# Web
cd apps/web && pnpm dev
```

## Coding conventions

- **Go.** `gofmt`, `go vet`, `go test -race`. Structured logs with `log/slog`. Errors wrapped with `%w`. No package-level state that a test can't override.
- **Python.** `ruff check`, `pyright` (standard mode), `pytest`. Type hints required on new code. Prefer `dataclass(frozen=True)` for value types.
- **TypeScript.** `tsc --noEmit` (strict), `eslint`. Prefer React Server Components; client boundaries marked with `"use client"`. Server-side API calls go through `src/lib/api.ts`.
- **SQL.** Migrations under `services/api/internal/db/migrations/`, goose format, `-- +goose StatementBegin/End` blocks. Every mutable table has a `set_updated_at` trigger. Every foreign key has an explicit `ON DELETE`.
- **Comments.** Explain the *why*, not the *what*. If a well-named identifier already says what, delete the comment.

## Delivery phase

The phases are no longer sequential and the FSD says so: 4 (Ask Roger) and 5 (admin) are substantially live while 1 (content sections) and 3 (personalization) are not. See FSD §12 for the per-phase state, which is the authority. Do not reorder scope further without amending that table in the same PR.

## Branch workflow

**Never work directly on `main`.** All changes land through a PR from a feature branch.

- Owner-facing work by Claude Code: **`claude_dev01`** (single long-lived branch).
- Human contributor work: **`feat/<slug>`** or **`fix/<slug>`** branches.
- `main` is protected — direct pushes refused; PRs require CI green.

**Pushing a change:**

```bash
scripts/dev-push.sh
```

That script runs every check CI runs (`buf lint`, `make check-gen`, `gofmt`, `go vet`, `go test -race`, `ruff`, `pyright`, `pytest`, `pnpm typecheck`, `pnpm lint`, `pnpm test`, `pnpm build`) in the same order, then pushes. `pnpm build` is not redundant with `typecheck`: it is the only step that applies the App Router's own rules, so a client/server boundary violation fails there and nowhere else. On push, GitHub Actions runs CI + security against the new SHA and the `auto-pr.yml` workflow opens (or updates) a draft PR against main.

Flip the PR from Draft → Ready-for-review when the work is complete. Merge via **Squash and merge** so `main`'s history stays a linear record of shipped features. Deploys run from `main`; see `deploy/README.md`.

## Docs discipline

**Every behavior change touches its spec.** Before pushing, if the change:

- **Modifies an RPC** — regenerate (`make gen docs-api`) and commit the diff.
- **Adds a new field, table, or column** — add or amend the FSD `FR-*` entry.
- **Resolves an open decision (`D-NN`)** — write an ADR at `docs/adr/NNNN-…md` and mark the FSD §14 row resolved.
- **Adds a config env var** — add it to `.env.example` (and `.env.prod.example` when it's prod-only) with a comment explaining what reads it.
- **Adds a new dependency** — note it in the module manifest (`go.mod`, `pyproject.toml`, `package.json`); no separate doc.
- **Changes the deploy sequence** — update `deploy/README.md`.
- **Adds a new env var** — `.env.example` is not enough on its own: the var also belongs in the table in `SERVICES.md`, and in `.env.prod.example` when it is prod-only.
- **Adds a migration** — give it a row in `SERVICES.md`.
- **Touches the JD or chat pipeline** (prompts, score formula, gate, model, context budget, retrieval) — log it in `docs/llm-tuning-log.md` in the same PR, with the calibration numbers. The owner reviews asynchronously and cannot reconstruct the reasoning later.
- **Adds a product event** — add it to `docs/events/README.md`. An event nobody documented is an event nobody queries.
- **Lands or abandons something in `docs/in-flight.md`** — delete the entry. That file's whole value is that an entry in it means work is genuinely underway.

The FSD is the source of truth for what the system *should* do; when it and the code disagree about intent, amend the FSD first and then change the code.

That rule does not license leaving a status claim wrong. The FSD also records what is *built*, and those notes rot: the 0.4.0 audit found §5.5 asserting Ask Roger was not built while every requirement row beneath it said *Shipped*. A status line is a factual claim about the running system, so it is corrected to match reality, not defended because it is in the spec.

## When you're stuck

- Cannot find where something lives? `rg --hidden --glob '!node_modules'` from the repo root.
- Not sure whether a change needs a spec update? If it changes an RPC signature, an auth level, a data-model field, or the user-facing flow — yes.
- CI failing on `check-gen`? You edited a `.proto` and did not commit the regenerated code. Run `make gen docs-api` and add the diff.
- CI failing on `breaking`? You made a non-additive change to `career.v1`. Revert or bump the package version (last-resort; ADR required).
