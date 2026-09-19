# Security policy

## Reporting a vulnerability

**Please do not open a public GitHub issue for security bugs.** Use one of the two private channels below.

### Preferred: private vulnerability report

1. Open **https://github.com/reh3376/career-site/security/advisories/new** (requires a GitHub account).
2. Describe the issue, the affected component, and a reproducer.
3. GitHub will notify the maintainer; expect an acknowledgement within **48 hours**.

### Alternative: email

`rogerhenley345@gmail.com` with subject prefix `[career-site security]`. Include the same information a GitHub advisory would.

## What to include

- **Where** — file path, endpoint, or component
- **What** — the bug and its impact
- **How** — reproducer (curl, code snippet, or step-by-step)
- **Who** — your name and how you'd like to be credited (optional; anonymous is fine)

Please **do not** include working exploits for third-party systems or user data taken during discovery — describe the class of issue instead.

## Timeline

| Stage | Target |
|---|---|
| Acknowledgement of report | 48 hours |
| Initial triage + severity | 5 days |
| Fix landed in `main` | 30 days for High/Critical, best-effort for others |
| Public disclosure | after fix is deployed and users are notified where applicable |

Fixes for Critical / High issues typically ship faster; the numbers above are ceilings, not targets.

## Scope

Everything in this repository is in scope: the Go API, Python sidecar, Next.js frontend, Protobuf contracts, Docker compose files, deploy scripts, and generated code (the fix goes in the .proto or generator, not the generated file).

## Out of scope

- Denial-of-service via volumetric attacks or targeted resource exhaustion of the single production host. That is a hosting concern, not a code defect.
- Findings against a fork or an outdated deploy — please reproduce against the current `main` before reporting.
- Issues in transitive dependencies with a known-safe usage pattern in this codebase.

## Automated scanning

CI runs on every PR:
- **govulncheck** against the Go module (`security.yml`)
- **pip-audit** against the Python sidecar's runtime deps
- **pnpm audit --prod --audit-level=high** against the web app
- **gitleaks** on the full history
- **CodeQL** SAST across Go, TypeScript, and Python

## Credit

Reporters who follow this policy will be credited by name (with your permission) in the release notes for the fix and in a running `SECURITY-CREDITS.md` once the file first exists.
