# Architecture decision records

One record per resolved decision from the FSD (`docs/FSD.md` §14). The file number is the decision ID (`D-04` → `0004-…`), so the FSD table and the record always match. Format: context, decision, consequences. A record is required whenever a technology in FSD §8.2 is replaced or a decision is reversed; the superseded record stays and points at its successor.

| ADR | Decision | Status |
|---|---|---|
| [0002](0002-approval-gated-registration.md) | Approval-gated registration | Accepted 2026-09-19 |
| [0004](0004-go-api-with-python-sidecar.md) | Go API with a Python sidecar | Accepted 2026-09-18 |
| [0010](0010-repository-name-and-account.md) | Repository `reh3376/career-site` | Accepted 2026-09-18 |
| [0017](0017-connectrpc-transport.md) | ConnectRPC between browser and API | Accepted 2026-09-18 |
