# API contracts

The `.proto` files here are the single source of truth for every RPC the site exposes and for the internal contract between the Go API and the Python sidecar. Everything else is derived:

| Derived artifact | Where | How |
|---|---|---|
| Go message types and Connect handlers/clients | `services/api/gen/` | `make gen` |
| TypeScript message types and service descriptors for connect-es | `apps/web/src/gen/` | `make gen` |
| Python message types, gRPC stubs, and `buf/validate` descriptors | `services/sidecar/src/{career,buf}/` | `make gen` |
| API reference and endpoint index | `docs/api/README.md`, `docs/api/endpoints.json` | `make docs-api` |
| Request validation at runtime | Connect interceptor using `protovalidate` | reads the `buf.validate` rules in these files |
| Auth enforcement at runtime | Connect interceptor using `career.v1.auth` and related options | reads the method options in these files |
| UATS contract specs | `docs/api/api-spec/uats/` | scaffolded from `docs/api/endpoints.json` |

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
    │                      activity, chat, download, contact, admin, system
    └── sidecar/v1/        internal API ↔ sidecar contract
```

## Rules for editing

1. **Comment everything.** `buf lint` enforces the COMMENTS category: every service, RPC, message, field, oneof, enum, and enum value carries a leading comment. The comments are the API reference, so write them for the frontend developer reading `docs/api/README.md`.
2. **Declare access on every RPC** with `option (career.v1.auth) = AUTH_LEVEL_…;` and, where they apply, `allow_unverified`, `rate_limit_per_minute`, and `mfa_fresh` (see `career/v1/options.proto`). The server refuses to start with a method that has no auth level.
3. **Validate at the edge** with `buf.validate` rules on request fields (lengths, formats, enum `defined_only`, list sizes). The interceptor rejects invalid requests with `invalid_argument` before handlers run, so handlers can assume well-formed input.
4. **Additive changes only** within `career.v1`: add fields, methods, and enum values; never renumber, rename, or change types. `make breaking` (and CI) compares against `main`.
5. **Streaming** is reserved for genuinely incremental responses (chat). Everything else is unary.
6. **Plain HTTP endpoints** (OAuth redirects, downloads, health) are documented in the file-level comment of the service they belong to and in `docs/api/README.md`; they are not RPCs because browsers navigate to them.
7. After any change: `make lint-proto gen docs-api`, review the diff in `docs/api/README.md`, and commit the generated outputs together with the `.proto` change and the matching UATS/UDTS spec.
