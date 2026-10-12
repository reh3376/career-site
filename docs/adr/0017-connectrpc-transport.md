# ADR 0017 — ConnectRPC between browser and API

**Status:** Accepted, 2026-09-18 (owner decision). Resolves FSD D-17.

## Context

The browser-facing API needs a contract the frontend can build against, streaming for the assistant, request validation, and a documented surface that also feeds the UATS contract specs. The alternatives were ConnectRPC (Protobuf services served over plain HTTP) and REST with an OpenAPI description and server-sent events for chat.

## Decision

- All browser-facing RPCs are Protobuf services in `proto/career/v1/`, served by the Go API with `connect-go` under `/api/{package}.{Service}/{Method}` and called from the Next.js app through the generated `connect-es` client.
- JSON encoding by default (switchable to binary without a server change); `ChatService.SendMessage` uses server streaming.
- Method-level options (`career.v1.auth`, `allow_unverified`, `rate_limit_per_minute`, `mfa_fresh`) declare the access policy; a Connect interceptor enforces it and `protovalidate` enforces the field rules, both before handlers run.
  - **Built as decided, 2026-10-11.** `internal/authpolicy` builds the policy map from the registered descriptors and the server refuses to start if any method omits a level; one Connect interceptor enforces `auth` and `allow_unverified` before any handler runs, and the handler-side gates (`requireAdmin`, `requireMember`, `requireChatAdmin`) were deleted in the same change. `docs/sprint-auth-interceptor.md` is the full record.
  - Two of the four options named above are gone rather than enforced: `mfa_fresh` and `rate_limit_per_minute` were removed from every method on 2026-10-09 and are deprecated, declared on nothing. There is no second factor and none is planned. Rate limiting is real but hand-wired in `internal/ratelimit`, not driven from the contract.
  - `protovalidate` is **not** wired. The field rules in these files are not enforced at runtime; `protovalidate-go` is not a dependency and no interceptor applies the `buf.validate` rules, so each handler validates its own input. This clause of the decision was never implemented and is tracked in `docs/backlog.md`.
  - A note here between 2026-10-09 and 2026-10-11 said the interceptor "was never written". That was true when written and is kept visible because this ADR is read as evidence: the record of an intent the code did not follow, and then did.
- `docs/api/README.md` and `docs/api/endpoints.json` are generated from the compiled descriptors (`make docs-api`) so the reference cannot drift from the contract.
- Downloads, the one-click admin decision link, calendar ICS and health probes stay plain HTTP handlers because a browser, mail client or calendar app navigates to them; each carries its own token-as-auth check, and the Connect interceptor does not see them. This line used to begin "OAuth redirects": social sign-in does not exist, and the only OAuth in the service is Google Calendar for the owner's own calendar, which is a ConnectRPC flow driven from a Next.js page.

## Consequences

- One contract for three languages; streaming without a second mechanism; the `Connect-Protocol-Version` header gives cross-site request forgery no same-origin path.
- UATS specs address methods as `POST` with JSON bodies; streaming and internal gRPC methods are covered by UDTS instead.
- `curl` calls need the protocol header; the reference includes the template.
- Breaking changes to `career.v1` are blocked by `buf breaking` in CI; the package version only changes for an incompatible redesign.

## Status update (2026-09-22)

- The transport decision stands and now carries `JdService` (JD upload, own-submissions list, `GetJdReviewConfig` for the live fit bands) and a much larger `AdminService` (jobs, decision log, fit bands, DB query, access grants) on the same contract.
- `ChatService` with the streaming `SendMessage` is built and mounted (`internal/server/server.go`, with its own write-deadline wrapper), so streaming is exercised on the browser path. This line said the handler was not registered and Ask Roger was not built; both became false once the chat shipped.
- `make docs-api` output is drift-checked in CI by `make check-gen`, as this record intended; the proto check also runs `buf lint` and `buf breaking`.
