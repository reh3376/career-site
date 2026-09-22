# ADR 0027 — Docs discipline and public-repo maturity

**Status:** Accepted, 2026-09-19 (owner decision). Resolves FSD D-27.

## Context

The site's first deploy landed with the code shipping ahead of the documentation. `README.md` still described "early scaffolding" days after the live site started taking real registrations; the FSD lagged behind the spec changes the owner had requested mid-implementation; there was no `LICENSE` file even though the code license (MIT) had been decided in D-09; there was no `SECURITY.md`, no `CONTRIBUTING.md`, no PR template, no CodeQL, no branch protection.

That worked for a private working session. It stops working the moment the repository is public and other people can read the code, file issues, or propose changes. The owner asked (explicitly) to codify a docs-and-hygiene bar that matches the site's public-facing posture.

Three related decisions came in together and are resolved together:

1. **Docs stay in lockstep with code.** Every behavior change ships its documentation in the same commit.
2. **The repo carries the standard public-project files.** LICENSE, SECURITY, CONTRIBUTING, CODEOWNERS, PR + issue templates, Dependabot, CodeQL.
3. **`main` is protected.** No direct pushes; PRs from feature branches with CI green; squash-merge only.

## Decision

- **Docs discipline (NFR-DOC-01/02/03).** The PR template carries the docs-discipline checklist: `.proto` change → regenerate + commit `docs/api/*`; new / changed RPC → update the matching `FR-*` row in `docs/FSD.md`; resolving an open `D-NN` → write an ADR at `docs/adr/NNNN-…md`; new env var → `.env.example` (and `.env.prod.example` if prod-only); deploy-sequence change → `deploy/README.md`. The FSD is the source of truth; when code and FSD disagree, the FSD wins and the code changes. The generated API reference (`docs/api/README.md` + `docs/api/endpoints.json`) is drift-checked in CI via `make check-gen`.
- **Public-repo hygiene (NFR-SUPP-01/02/03).** The repository carries `LICENSE` (MIT, per D-09), `LICENSE-CONTENT.md` (All rights reserved with quote-with-attribution), `SECURITY.md` (private disclosure via GitHub Security Advisories; email fallback), `CONTRIBUTING.md` (branch workflow + docs discipline + contributor-request path), `CODEOWNERS` (owner reviews everything), PR + issue templates under `.github/`, `.github/dependabot.yml` (weekly Monday updates grouped per ecosystem), and `.github/workflows/codeql.yml` (Go / TypeScript / Python SAST). Repo-level secret scanning + push protection + private vulnerability reporting are enabled in Settings.
- **Branch workflow (NFR-SUPP-04, §9.3).** `main` is protected — no direct pushes; PRs from feature branches; CI must be green; merges are squash-only. AI-assisted work uses the single long-lived branch **`claude_dev01`**; human contributor work uses `feat/<slug>` or `fix/<slug>`. All three patterns run CI on push and open a draft PR against main via `.github/workflows/auto-pr.yml`. `scripts/dev-push.sh` mirrors the CI matrix locally so a broken change is caught before push.

## Consequences

- **PR overhead is small but non-zero.** Every behavior-changing PR now touches at least one doc file. The PR template makes the checklist unavoidable; reviewers refuse PRs that leave rows unticked without explanation. The alternative — accumulating doc debt on a public repo — is worse.
- **Direct-to-main is gone.** Every change routes through a feature branch → PR → CI → review → squash-merge. The very first commits on `claude_dev01` proved the shape works; the auto-PR workflow needs one owner setting toggle (**Settings → Actions → General → "Allow GitHub Actions to create and approve pull requests"**) to open PRs automatically. Branch protection on `main` is a separate one-time Settings-side toggle (**Settings → Branches → Add rule** for `main` → require PR + CI green + no force-push).
- **Dependabot grouping.** One PR per ecosystem per week instead of a dozen. Security updates ship immediately regardless of the interval.
- **CodeQL cost.** SAST across three languages adds ~5–10 minutes to CI per push. Acceptable for the security signal; may parallelize or scope to `main`-only later if it becomes a bottleneck.
- **The `AGENTS.md` file is now load-bearing** for AI-assisted contributors, not just aspirational. It carries the branch workflow, the docs-discipline checklist, and the coding conventions per language.

## Status update (2026-09-22)

- **Branch protection is a ruleset.** `main` is protected by the GitHub ruleset `protect-main`: PR required, required checks `api` (Go), `web` (TypeScript), `sidecar` (Python), `proto` (lint, breaking, drift) and `gitleaks`, no force push, no deletion. The auto-pr workflow opens the PR from `claude_dev01`; the owner merges. The Settings-side "Add rule" step described above is therefore done, in ruleset form.
- **Docs discipline covers more than the FSD, ADRs and API reference.** In practice every LLM-path PR also updates, in the same PR: `docs/llm-tuning-log.md` (every experiment, measurement and decision, dated), `docs/decision-log.md` (what the decision log records and how it is reviewed), `docs/jd-submitter-workflow.md` (what the submitter sees) and `docs/cutover-local-to-prod.md` (the runbook). The PR template checklist and `AGENTS.md` do not yet name these four files; the requirement is enforced by review, not by the template.
- The rest of the record (LICENSE, SECURITY, CONTRIBUTING, CODEOWNERS, templates, Dependabot, CodeQL, squash-only merges) stands.
