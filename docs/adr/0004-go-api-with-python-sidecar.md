# ADR 0004 — Go API with a Python sidecar

**Status:** Accepted, 2026-09-18 (owner decision). Resolves FSD D-04.

## Context

The site needs a serving path (auth, sessions, personalization, streaming chat, admin, scheduling) and a set of content/ML jobs (parsing, chunking, embeddings, reranking, résumé generation, evaluation). The owner's stated practice is Go for backend efficiency with a Python sidecar for scripting, and JS/TS for the front end; his production systems (Forge, BOSC IMS) use the same split over gRPC/Protobuf.

## Decision

- **Go** (current stable) owns everything on the request path and everything that touches member data: the ConnectRPC API, argon2id/session/TOTP/OIDC auth, personalization, chat orchestration and streaming, admin, audit, retention, exports, and the in-process scheduler. Data access through `pgx` + `sqlc`; migrations with `goose` embedded in the binary.
- **Python 3.12+** (`uv`, `Ruff`, `pyright`) owns the sidecar: a gRPC service (`Embed`, `Rerank`, `Classify`, `RunJob`, `GetJob`, `Health`) and the `career-cli` for batch jobs.
- The boundary is the Protobuf contract in `proto/career/sidecar/v1/`. The API calls the sidecar with short deadlines and fallbacks so a sidecar outage degrades retrieval quality rather than availability.
- Generated code lands inside each consumer (`services/api/gen`, `apps/web/src/gen`, `services/sidecar/src`) so each toolchain's import rules are natural; all outputs are committed and drift-checked.

## Consequences

- Three toolchains in one repository (mitigated by one `Makefile` entry point, Compose parity, and `AGENTS.md`; FSD R-14).
- The site itself becomes a Go work sample and a live demonstration of the owner's gRPC/Protobuf hub-and-spoke pattern.
- The Python ecosystem stays available for the ML-heavy work without putting Python on the latency-critical path.

## Status update (2026-09-22)

- The sidecar contract (`proto/career/sidecar/v1/`) grew two RPCs beyond the list above: `Generate` (schema-constrained LLM calls through the gateway) and `RenderResume` (Typst PDF render plus pypdf edit-lock). The sidecar is Python 3.12 and talks to an `ollama` container for both embeddings (`nomic-embed-text`) and the LLM (`qwen3:4b-q8_0`).
- `RunJob` / `GetJob` also exist on the api's `AdminService` (`proto/career/v1/admin.proto`): corpus reindex and the embed sweep run as api-side jobs with progress (PR 88).
- "Short deadlines and fallbacks" applies to retrieval. The JD pipeline is asynchronous and long-running on purpose (`SIDECAR_LLM_TIMEOUT_SECONDS=3000`, `JD_PIPELINE_TIMEOUT_SECONDS=3600` on prod); with the assessor wired, an LLM failure marks the submission `failed` for Re-score rather than degrading to the retrieval score. See `docs/cutover-local-to-prod.md`.
