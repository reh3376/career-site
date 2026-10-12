# API contracts

The `.proto` files here are the single source of truth for every RPC the site exposes and for the internal contract between the Go API and the Python sidecar. Everything else is derived:

| Derived artifact | Where | How |
|---|---|---|
| Go message types and Connect handlers/clients | `services/api/gen/` | `make gen` |
| TypeScript message types and service descriptors for connect-es | `apps/web/src/gen/` | `make gen` |
| Python message types, gRPC stubs, and `buf/validate` descriptors | `services/sidecar/src/{career,buf}/` | `make gen` |
| API reference and endpoint index | `docs/api/README.md`, `docs/api/endpoints.json` | `make docs-api` |
| Auth enforcement at runtime | Connect interceptor in `services/api/internal/authpolicy` | reads `career.v1.auth` from the method options in these files |

UATS contract specs are **planned, not built**: `docs/FSD.md` §10 describes them and this table used to list them as a derived artifact, but there is no `docs/api/api-spec/` directory and no `make test-uats`. The only UxTS framework actually running here is UCTS, for document conversion (`docs/tests/ucts/`), which is unrelated to the proto surface.

CI runs `make check-gen`, which regenerates everything and fails if a committed output differs. Never edit generated files; change the `.proto` and regenerate.

## Layout

```
proto/
├── buf.yaml               module config; lint uses STANDARD + COMMENTS, breaking uses FILE
├── buf.lock               pinned dependency (buf.build/bufbuild/protovalidate)
├── buf.gen.yaml           generation for the public API (career.v1) → Go, TS, Python
├── buf.gen.sidecar.yaml   generation for the internal contract (career.sidecar.v1) → Go, Python
└── career/
    ├── v1/                public API: options, common, auth, member, content, home,
    │                      activity, chat, jd, meetings, events, download,
    │                      contact, admin, system
    └── sidecar/v1/        internal API ↔ sidecar contract
```

## Rules for editing

1. **Comment everything.** `buf lint` enforces the COMMENTS category: every service, RPC, message, field, oneof, enum, and enum value carries a leading comment. The comments are the API reference, so write them for the frontend developer reading `docs/api/README.md`.
2. **Declare access on every RPC** with `option (career.v1.auth) = AUTH_LEVEL_…;` (see `career/v1/options.proto`). **The declaration is the enforcement**, as of 2026-10-11: the interceptor in `services/api/internal/authpolicy` reads it from the descriptor and refuses every caller below the level before the handler runs, and the server will not start if any method omits it. Handlers no longer carry their own checks; `requireAdmin`, `requireMember` and `requireChatAdmin` were deleted, and a handler that needs the caller reads it with `handlers.RequireCaller(ctx)`.

   This section said the opposite until 2026-10-11 ("`auth` is enforced by hand in each handler, not by an interceptor"), which was true when written. The sprint it pointed at, `docs/sprint-auth-interceptor.md`, has landed.

   `allow_unverified` is enforced too, and only means something alongside `AUTH_LEVEL_MEMBER`: it drops the active-account requirement and keeps the session requirement. The server refuses to start if it appears at any other level. `mfa_fresh` and `rate_limit_per_minute` are deprecated and declared on nothing; do not add them to a method.
3. **Validate at the edge** with `buf.validate` rules on request fields (lengths, formats, enum `defined_only`, list sizes). **They are not enforced at runtime.** There is no `protovalidate` interceptor and `protovalidate-go` is not a dependency: the only `protovalidate` entry in `services/api/go.mod` is the generated descriptor types, which exist because these files import `buf/validate/validate.proto`. The rules document the contract and feed the API reference; **a handler must validate its own input** and every one of them does, returning `invalid_argument`. This item used to say the interceptor rejected invalid requests "so handlers can assume well-formed input", which was never true and is the kind of advice that gets a check skipped.
4. **Additive changes only** within `career.v1`: add fields, methods, and enum values; never renumber, rename, or change types. `make breaking` (and CI) compares against `main`.
5. **Streaming** is reserved for genuinely incremental responses (chat). Everything else is unary.
6. **Plain HTTP endpoints** (downloads, the one-click admin decision link, calendar ICS, health) are documented in the file-level comment of the service they belong to and in `docs/api/README.md`; they are not RPCs because a browser, mail client or calendar app navigates to them. They also sit outside the auth interceptor, which only sees Connect procedures, so each one carries its own token-as-auth check. This list used to begin "OAuth redirects"; social sign-in does not exist.
7. After any change: `make lint-proto gen docs-api`, review the diff in `docs/api/README.md`, and commit the generated outputs together with the `.proto` change. Running `docs-api` is not optional even for a one-word comment edit, because `check-gen` regenerates it in CI and fails on any difference.
8. **Pin every plugin.** `buf.gen.yaml` and `buf.gen.sidecar.yaml` give each remote plugin an explicit version. An unpinned plugin means the generated output depends on when you ran it, which turns `check-gen` into a test of the current date.
