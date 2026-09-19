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

Sign in at **https://rogerhenley.dev/**, open the "Contribute to my code" page (Phase 2), and use the "Request contributor access" form. It emails Roger with your GitHub username; if he approves, you're added as a collaborator on the specific repo(s) you asked about.

(Until that form ships, email `rogerhenley345@gmail.com` with subject `[career-site] contributor request` and your GitHub username.)

### 3. Send a pull request

1. Fork or (if you're a collaborator) create a branch: `git checkout -b feat/<slug>` or `fix/<slug>`.
2. Make the change. Read `AGENTS.md` first — it names the coding conventions per language and the "docs discipline" rules for what to update alongside code.
3. Run the full check suite locally before pushing:
   ```bash
   scripts/dev-push.sh
   ```
   It mirrors what CI runs and refuses to push if anything fails.
4. Push. A draft PR opens automatically against `main` (see `.github/workflows/auto-pr.yml`).
5. Flip the PR to Ready-for-review when it's done. CI must be green; the owner reviews.

Merge policy: **Squash and merge** — `main` stays a linear history of shipped features. Your commits on the branch can be as messy as you like; the squash keeps the record clean.

## What good PRs look like

- Small and focused. One PR does one thing. If the change is >~500 lines of hand-written code, propose it in an issue first so the shape is agreed before you write it.
- **Tests included.** New API handlers get unit tests; new UI paths get at least a smoke check.
- **Docs updated.** See "Docs discipline" in `AGENTS.md`. Every RPC change touches the proto and the generated `docs/api/README.md`; every new decision writes an ADR; every new env var lands in `.env.example`.
- **Commit messages** follow Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `ci:`, `db:`, `api:`, `web:`, `sidecar:`, `compose:`) with a one-line summary + body explaining *why*.
- **Do not commit generated files by hand.** `services/api/gen/`, `apps/web/src/gen/`, and `services/sidecar/src/{career,buf}/` are drift-checked; the fix goes in the `.proto` and `make gen`.

## Local development

```bash
# Full stack
docker compose up -d

# Verify
curl http://localhost/api/readyz
```

See `README.md` for standalone-service dev commands. See `deploy/README.md` for the production deploy flow (owner only).

## Code of conduct

Be professional. Assume good faith. Disagreements about design decisions are welcome; disagreements about the person are not. The maintainer reserves the right to close or lock any issue or PR that violates this.

## Questions

If you're not sure whether an idea fits: open a **Question** issue with your proposal. That's the low-friction path to a yes/no before you write code.
