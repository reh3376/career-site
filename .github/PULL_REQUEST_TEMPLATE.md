<!--
Auto-opened by .github/workflows/auto-pr.yml on push to a feature branch,
OR manually via `gh pr create`. Either way, fill this out before flipping
draft → ready-for-review.
-->

## What's in this PR

<!-- One or two sentences: what changed and why. -->

## Docs discipline checklist

<!-- Per AGENTS.md → "Docs discipline". Tick each that applies; delete
lines that don't. If none apply, that's the answer — leave the list empty. -->

- [ ] `.proto` change → `make gen docs-api` run and generated files committed
- [ ] New RPC / new field → `FR-*` in `docs/FSD.md` amended
- [ ] Resolved open decision (`D-NN`) → ADR at `docs/adr/NNNN-…md` written; FSD §14 marked resolved
- [ ] New env var → `.env.example` (and `.env.prod.example` if prod-only) **and** the table in `SERVICES.md`
- [ ] New migration → row added to `SERVICES.md`
- [ ] Change to deploy sequence → `deploy/README.md` updated
- [ ] JD or chat pipeline touched (prompt, score formula, gate, model, context budget, retrieval) → `docs/llm-tuning-log.md` updated in this PR, with the calibration numbers
- [ ] New product event → `docs/events/README.md` updated
- [ ] Something in `docs/in-flight.md` landed or was abandoned → entry deleted
- [ ] A status claim in a doc is now false → corrected, not left for later

## Not a docs task, but checked anyway

- [ ] `scripts/dev-push.sh` passed (it runs what CI runs, in the same order)
- [ ] UI change → opened on a deployed build and reviewed, not just built locally

## Test plan

<!-- What did you verify? For a UI change include a screenshot or gif.
For an API change include the curl or test that proved it works. -->

## Roll-out notes

<!-- If this needs anything beyond "merge to main, wait for CI to publish
images, git pull + docker compose up on the server" — say what. -->
