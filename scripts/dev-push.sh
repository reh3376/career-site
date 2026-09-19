#!/usr/bin/env bash
# scripts/dev-push.sh — mirror what CI runs locally, then push.
#
# Runs the same checks the CI matrix runs, in the same order. Exits at
# the first failure so a broken change never reaches origin. Push happens
# only if every check passes.
#
# Usage:
#   scripts/dev-push.sh             # runs checks + git push
#   scripts/dev-push.sh --dry-run   # runs checks, prints what would push
#   scripts/dev-push.sh --skip-web  # useful when you know only Go changed
#   scripts/dev-push.sh --skip-api
#   scripts/dev-push.sh --skip-sidecar

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

DRY_RUN=false
SKIP_API=false
SKIP_SIDECAR=false
SKIP_WEB=false
SKIP_PROTO=false

for arg in "$@"; do
  case "$arg" in
    --dry-run)     DRY_RUN=true ;;
    --skip-api)    SKIP_API=true ;;
    --skip-sidecar) SKIP_SIDECAR=true ;;
    --skip-web)    SKIP_WEB=true ;;
    --skip-proto)  SKIP_PROTO=true ;;
    *) echo "unknown flag: $arg"; exit 2 ;;
  esac
done

log() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
ok()  { printf '\033[1;32m✓ %s\033[0m\n' "$*"; }

# ─── Branch guard ─────────────────────────────────────────────────────────
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$BRANCH" = "main" ]; then
  echo "REFUSING: you are on main. Feature branches only (claude_dev*, feat/*, fix/*)."
  echo "  git checkout -b claude_dev01"
  exit 1
fi

# ─── Proto (buf lint + drift check) ────────────────────────────────────────
if [ "$SKIP_PROTO" = false ]; then
  log "proto: buf lint + check-gen"
  BUF="$(command -v buf)"
  if [ -z "$BUF" ]; then
    echo "buf not on PATH — install with: brew install bufbuild/buf/buf"
    exit 1
  fi
  BUF="$BUF" make lint-proto
  BUF="$BUF" make check-gen
  ok "proto"
fi

# ─── API (Go) ─────────────────────────────────────────────────────────────
if [ "$SKIP_API" = false ]; then
  log "api (go): gofmt drift + vet + test -race + build"
  (
    cd services/api
    drift=$(gofmt -l .)
    if [ -n "$drift" ]; then
      echo "gofmt drift:"; echo "$drift"; exit 1
    fi
    go vet ./...
    go test -race -count=1 ./...
    go build ./...
  )
  ok "api"
fi

# ─── Sidecar (Python) ─────────────────────────────────────────────────────
if [ "$SKIP_SIDECAR" = false ]; then
  log "sidecar (python): sync + ruff + pyright + pytest"
  (
    cd services/sidecar
    uv sync --frozen --extra dev >/dev/null
    uv run ruff check .
    uv run pyright
    uv run pytest -q
  )
  ok "sidecar"
fi

# ─── Web (TypeScript) ─────────────────────────────────────────────────────
if [ "$SKIP_WEB" = false ]; then
  log "web (typescript): install + typecheck + lint + audit + build"
  (
    cd apps/web
    pnpm install --frozen-lockfile >/dev/null
    pnpm typecheck
    pnpm lint
    pnpm audit --prod --audit-level=high || echo "  (audit failed; not a hard block locally — CI enforces)"
    pnpm build
  )
  ok "web"
fi

# ─── Push ─────────────────────────────────────────────────────────────────
log "push $BRANCH -> origin/$BRANCH"
if [ "$DRY_RUN" = true ]; then
  echo "  (dry-run; skipping git push)"
  git log --oneline origin/"$BRANCH"..HEAD 2>/dev/null || git log --oneline -5
else
  git push -u origin "$BRANCH"
  ok "pushed"
  echo
  echo "GitHub Actions will:"
  echo "  1. Run CI + security workflows against the pushed SHA"
  echo "  2. Open (or update) a draft PR against main via auto-pr.yml"
  echo
  echo "Watch: gh pr list --head $BRANCH"
fi
