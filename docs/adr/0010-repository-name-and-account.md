# ADR 0010 — Repository `reh3376/career-site`

**Status:** Accepted, 2026-09-18. Partially resolves FSD D-10 (the domain remains open).

## Context

The repository is public and is itself part of the portfolio. The owner's résumé, LinkedIn profile, and open-source projects (Forge, MDEMG, NextTrend, BOSC IMS, ann-research, PLC-GBT) all point at the `reh3376` GitHub account. A first repository was created under a different account.

## Decision

The repository lives at `github.com/reh3376/career-site`, alongside the rest of the portfolio, and the Go module path is `github.com/reh3376/career-site/services/api`.

## Consequences

- One account for everything an employer will see.
- The site's domain is still to be chosen (D-10); nothing in the code depends on it.

## Status update (2026-09-22)

- The domain is chosen and live: https://rogerhenley.dev/ (see `README.md`). Container images are published to `ghcr.io/reh3376/career-site-{web,api,sidecar}` under the same account. The open half of D-10 is closed.
