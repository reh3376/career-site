# ADR 0017 — ConnectRPC between browser and API

**Status:** Accepted, 2026-09-18 (owner decision). Resolves FSD D-17.

## Context

The browser-facing API needs a contract the frontend can build against, streaming for the assistant, request validation, and a documented surface that also feeds the UATS contract specs. The alternatives were ConnectRPC (Protobuf services served over plain HTTP) and REST with an OpenAPI description and server-sent events for chat.

## Decision

- All browser-facing RPCs are Protobuf services in `proto/career/v1/`, served by the Go API with `connect-go` under `/api/{package}.{Service}/{Method}` and called from the Next.js app through the generated `connect-es` client.
- JSON encoding by default (switchable to binary without a server change); `ChatService.SendMessage` uses server streaming.
- Method-level options (`career.v1.auth`, `allow_unverified`, `rate_limit_per_minute`, `mfa_fresh`) declare the access policy; a Connect interceptor enforces it and `protovalidate` enforces the field rules, both before handlers run.
- `docs/api/README.md` and `docs/api/endpoints.json` are generated from the compiled descriptors (`make docs-api`) so the reference cannot drift from the contract.
- OAuth redirects, downloads, and health probes stay plain HTTP `GET` handlers because browsers navigate to them.

## Consequences

- One contract for three languages; streaming without a second mechanism; the `Connect-Protocol-Version` header gives cross-site request forgery no same-origin path.
- UATS specs address methods as `POST` with JSON bodies; streaming and internal gRPC methods are covered by UDTS instead.
- `curl` calls need the protocol header; the reference includes the template.
- Breaking changes to `career.v1` are blocked by `buf breaking` in CI; the package version only changes for an incompatible redesign.
