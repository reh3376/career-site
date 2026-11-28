#!/usr/bin/env bash
# Roll the v2c translator (served at /v2c) without touching the rest of
# the stack. Run it from a checkout on the owner's machine, like
# deploy/rollout.sh.
#
#   deploy/rollout-v2c.sh                 # deploy v2c-translator's current main
#   deploy/rollout-v2c.sh <12-char-sha>   # deploy a specific build
#   deploy/rollout-v2c.sh --rollback      # go back to the previous v2c tag
#
# The translator is built and published by its own repository
# (github.com/reh3376/v2c-translator), so its tag moves independently of
# IMAGE_TAG and an ordinary career-site rollout never pulls it. This
# script is the only thing that changes V2C_IMAGE_TAG, and it recreates
# only the v2c container: the api, its in-memory evaluations and any
# decision test in flight are untouched, so there is no quiet check.
#
# The Caddy route and the compose service arrive with a career-site
# rollout (deploy/rollout.sh), which also restarts caddy when the
# Caddyfile changed. Do that first, once; after that, this script alone.
set -euo pipefail

SERVER=${SERVER:-career@5.161.62.205}
REMOTE=${REMOTE:-/opt/career-site}
SITE=${SITE:-https://rogerhenley.dev}
IMAGE=${V2C_IMAGE:-ghcr.io/reh3376/v2c-translator}

S() { ssh -o ConnectTimeout=20 -o ServerAliveInterval=15 "$SERVER" "$@"; }
CS='docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml'
say() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
die() { printf '\nFAILED: %s\n' "$*" >&2; exit 1; }

current_tag() { S "grep '^V2C_IMAGE_TAG=' $REMOTE/.env.prod | cut -d= -f2" || true; }

if [ "${1:-}" = "--rollback" ]; then
  TAG=$(S "cat $REMOTE/.previous-v2c-image-tag 2>/dev/null || true")
  [ -n "$TAG" ] || die "no previous v2c tag recorded on the server"
  say "rolling v2c back to $TAG"
else
  TAG=${1:-}
  if [ -z "$TAG" ]; then
    TAG=$(git ls-remote https://github.com/reh3376/v2c-translator refs/heads/main | cut -c1-12)
    [ -n "$TAG" ] || die "could not read v2c-translator's main"
  fi
fi

# The translator refuses to boot in hosted mode without the Turnstile
# secret, and renders no robot check without the site key; both are the
# api's pair, already in .env.prod.
say "checking .env.prod has the Turnstile pair"
for key in TURNSTILE_SECRET_KEY TURNSTILE_SITE_KEY; do
  S "cd $REMOTE && grep -qE '^${key}=.+' .env.prod" || die "$key is missing from .env.prod"
done
echo "  ok"

PREV_TAG=$(current_tag)
say "deploying v2c $TAG (currently ${PREV_TAG:-unset})"

say "waiting for the image"
for attempt in $(seq 1 60); do
  if S "docker manifest inspect $IMAGE:$TAG >/dev/null 2>&1"; then
    echo "  ok   v2c-translator:$TAG"
    break
  fi
  [ "$attempt" = 60 ] && die "image v2c-translator:$TAG never appeared"
  sleep 20
done

say "pointing V2C_IMAGE_TAG at $TAG"
S "cd $REMOTE && echo '${PREV_TAG}' > .previous-v2c-image-tag && sed -i '/^V2C_IMAGE_TAG=/d' .env.prod && echo 'V2C_IMAGE_TAG=$TAG' >> .env.prod && grep '^V2C_IMAGE_TAG=' .env.prod"

say "pulling v2c"
for attempt in 1 2 3; do
  S "cd $REMOTE && $CS pull v2c 2>&1 | tail -2" && break
  [ "$attempt" = 3 ] && die "image pull kept failing"
  echo "  retrying"; sleep 10
done

say "starting v2c"
S "cd $REMOTE && $CS up -d v2c 2>&1 | tail -4"

say "waiting for /v2c to answer"
ok=no
for _ in $(seq 1 30); do
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$SITE/v2c/api/healthz" || true)
  if [ "$code" = "200" ]; then ok=yes; break; fi
  sleep 5
done
[ "$ok" = yes ] || die "$SITE/v2c/api/healthz never returned 200. Roll back with: deploy/rollout-v2c.sh --rollback"
echo "  ok   healthz"

code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "$SITE/v2c/api/readyz" || true)
[ "$code" = "200" ] || die "/v2c/api/readyz -> $code: the translator or its index is not ready"
echo "  ok   readyz"

# -L: the translator redirects /v2c/ to /v2c (308); the page and its
# policy are on the response after the redirect.
page=$(curl -sL --max-time 20 "$SITE/v2c/")
echo "$page" | grep -q 'sent over HTTPS to this server' || die "/v2c/ is not showing the hosted privacy copy: is V2C_MODE=hosted reaching the container?"
echo "  ok   hosted privacy copy"
SITE_KEY=$(S "grep '^TURNSTILE_SITE_KEY=' $REMOTE/.env.prod | cut -d= -f2-")
echo "$page" | grep -qF "$SITE_KEY" || die "/v2c/ does not carry TURNSTILE_SITE_KEY: the robot check cannot render"
echo "  ok   Turnstile site key on the page"
csp=$(curl -sIL --max-time 20 "$SITE/v2c/" | grep -i '^content-security-policy' | tail -1 | tr -d '\r')
echo "$csp" | grep -q 'challenges.cloudflare.com' || die "/v2c/ policy does not allow Turnstile"
echo "$csp" | grep -q 'api.pwnedpasswords.com' && die "/v2c/ is receiving the site's CSP, not its own: check the @site matcher in Caddyfile.prod and that caddy restarted"
echo "  ok   the translator's own CSP"

say "pruning old v2c images"
S "docker image prune -f --filter 'label=org.opencontainers.image.source=https://github.com/reh3376/v2c-translator' >/dev/null 2>&1 || true"

say "done: v2c $TAG is live at $SITE/v2c/"
echo "First deploy: open it in a browser and check the robot-check box appears (v2c-translator web/TESTING.md, step 59)."
