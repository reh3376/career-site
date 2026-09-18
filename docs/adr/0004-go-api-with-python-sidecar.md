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
