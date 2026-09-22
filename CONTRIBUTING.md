# Contributing to career-site

This is a personal portfolio site, but the code is open. Contributions are welcome under the terms below.

## Ground rules

- **Code license:** MIT (see `LICENSE`). By contributing you agree your changes are licensed the same way.
- **Content license:** the site's written content, images, and Ask Roger's answers are *not* MIT — see `LICENSE-CONTENT.md`. Do not contribute résumé text, articles, or personal photos via PRs.
- **Security bugs:** don't file public issues — see `SECURITY.md`.
- **Scope discipline:** if a change alters an RPC, a data-model field, or a user-facing flow, it needs an FSD amendment first. Don't submit a feature PR without either an accepted proposal in an issue or a matching FSD change.

## Ways to contribute

### 1. File a bug or feature request

Open an issue using one of the templates: **Bug report**, **Feature request**, or **Question**. Please search open + closed issues first — duplicates get closed with a link to the original.

### 2. Ask to be added as a repo collaborator

Email `rogerhenley345@gmail.com` with subject `[career-site] contributor request` and your GitHub username, or use the contact form at **https://rogerhenley.dev/contact**. If Roger approves, you're added as a collaborator on the specific repo(s) you asked about. (A "Request contributor access" form inside the site is still on the roadmap and has not shipped.)

### 3. Send a pull request

1. Fork or (if you're a collaborator) create a branch: `git checkout -b feat/<slug>` or `fix/<slug>`.
2. Make the change. Read `AGENTS.md` first — it names the coding conventions per language and the "docs discipline" rules for what to update alongside code.
3. Run the full check suite locally before pushing:
   ```bash
   scripts/dev-push.sh
   ```
   It mirrors what CI runs and refuses to push if anything fails.
4. Push. For `feat/*`, `fix/*` and `claude_dev*` branches a draft PR opens automatically against `main` (see `.github/workflows/auto-pr.yml`); don't race it with a manual `gh pr create`.
5. Flip the PR to Ready-for-review when it's done. The owner reviews and merges.

`main` is protected by the `protect-main` ruleset: a PR is required, the checks **api (go)**, **web (typescript)**, **sidecar (python)**, **proto (lint, breaking, drift)** and **gitleaks** must pass, and force-pushes and branch deletion are blocked. A red check is fixed in the PR, not skipped or excepted.

Merge policy: **Squash and merge** — `main` stays a linear history of shipped features. Your commits on the branch can be as messy as you like; the squash keeps the record clean.

## What good PRs look like

- Small and focused. One PR does one thing. If the change is >~500 lines of hand-written code, propose it in an issue first so the shape is agreed before you write it.
- **Tests included.** New API handlers get unit tests; new UI paths get at least a smoke check.
- **Docs updated.** See "Docs discipline" in `AGENTS.md`. Every RPC change touches the proto and the generated `docs/api/README.md` (`make docs-api`, or CI's drift check fails); every new decision writes an ADR; every new env var lands in `.env.example` (dev), `.env.prod.example` (prod) and the table in `SERVICES.md`; every migration gets a row in `SERVICES.md`. Anything that touches the JD pipeline (prompts, score formula, gate, model, context budget) is logged in `docs/llm-tuning-log.md` in the same PR, with the calibration numbers, so the owner can review it asynchronously.
- **Frontend copy has no em dashes.** Use `,` `.` `:` `·` or `-` in user-visible text.
- **Commit messages** follow Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `ci:`, `db:`, `api:`, `web:`, `sidecar:`, `compose:`) with a one-line summary + body explaining *why*.
- **Do not commit generated files by hand.** `services/api/gen/`, `apps/web/src/gen/`, and `services/sidecar/src/{career,buf}/` are drift-checked; the fix goes in the `.proto` and `make gen`.

## Local development

```bash
# Full stack
docker compose up -d

# Verify
curl http://localhost/api/readyz
```

The default dev stack runs the sidecar with `stub` providers: every interface works but embeddings are meaningless, and the JD assessor refuses the stub unless `LLM_ALLOW_STUB=1` (schema-valid, meaningless verdicts; CI only). To exercise the real pipeline, run Ollama on your host with `nomic-embed-text` and `qwen3:4b-q8_0` pulled and start with `SIDECAR_EMBED_PROVIDER=ollama SIDECAR_LLM_PROVIDER=ollama`; the compose file already points the sidecar at `host.docker.internal:11434`. Generated résumé PDFs need `RESUME_PDF_OWNER_PASSWORD` in your repo-root `.env`.

See `README.md` for standalone-service dev commands and `SERVICES.md` for every env var. See `deploy/README.md` for the production deploy flow (owner only).

## Code of conduct

Be professional. Assume good faith. Disagreements about design decisions are welcome; disagreements about the person are not. The maintainer reserves the right to close or lock any issue or PR that violates this.

## Questions

If you're not sure whether an idea fits: open a **Question** issue with your proposal. That's the low-friction path to a yes/no before you write code.
