#!/usr/bin/env bash
# One rollout, start to finish: wait for the images, deploy, verify,
# prune. Run it from a checkout on the owner's machine, not on the box.
#
#   deploy/rollout.sh                    # deploy the current origin/main
#   deploy/rollout.sh <12-char-sha>      # deploy a specific build
#   deploy/rollout.sh --rollback         # go back to the previous tag
#   deploy/rollout.sh --only web [sha]   # one service, leaving the rest
#
# --only exists because all three services shared one IMAGE_TAG, so
# `compose up -d` recreated the api for a CSS change and killed any
# evaluation in flight. Use it for a front-end fix while a run is going;
# use the full rollout for anything else, which also clears the
# override.
#
# Every step prints what it is doing and stops on the first failure, so
# a half-finished rollout is visible rather than silent. The runbook in
# deploy/README.md explains what each step is for; this script is that
# runbook, executable, so the steps cannot be done in the wrong order
# or forgotten (the image prune used to be, and filled the disk).
set -euo pipefail

SERVER=${SERVER:-career@5.161.62.205}
REMOTE=${REMOTE:-/opt/career-site}
SITE=${SITE:-https://rogerhenley.dev}
REPO_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

S() { ssh -o ConnectTimeout=20 -o ServerAliveInterval=15 "$SERVER" "$@"; }
CS='docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml'
say() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
die() { printf '\nFAILED: %s\n' "$*" >&2; exit 1; }

current_tag() { S "grep '^IMAGE_TAG=' $REMOTE/.env.prod | cut -d= -f2"; }

# --only <service> deploys one service without touching the others.
#
# All three share IMAGE_TAG, so an ordinary rollout recreates the api
# whatever changed, and the job runner keeps evaluations in memory. A
# one-line front-end fix therefore waited on a five-hour run four times
# on 2026-09-29. With --only web the api is left alone and the run
# survives.
#
# It sets that service's own tag variable and leaves IMAGE_TAG alone, so
# .env.prod still names what the untouched services are running and
# --rollback and live-check keep telling the truth. A full rollout after
# one of these clears the override.
ONLY=""
if [ "${1:-}" = "--only" ]; then
  ONLY="${2:?--only needs a service: web, api or sidecar}"
  case "$ONLY" in
    web|api|sidecar) ;;
    *) die "--only takes web, api or sidecar, not '$ONLY'" ;;
  esac
  shift 2
fi

if [ "${1:-}" = "--rollback" ]; then
  PREV=$(S "cat $REMOTE/.previous-image-tag 2>/dev/null || true")
  [ -n "$PREV" ] || die "no previous tag recorded on the server"
  TAG=$PREV
  say "rolling back to $TAG"
else
  TAG=${1:-}
  if [ -z "$TAG" ]; then
    git -C "$REPO_DIR" fetch -q origin
    TAG=$(git -C "$REPO_DIR" rev-parse origin/main | cut -c1-12)
  fi
fi

PREV_TAG=$(current_tag)
say "deploying $TAG (currently $PREV_TAG)"
[ "$TAG" != "$PREV_TAG" ] || say "note: already on $TAG, redeploying"

# 1. The three images must all exist. Prod compose has build: !reset, so
#    a missing tag fails the `up` rather than quietly building on the box.
say "waiting for images"
for repo in ${ONLY:-api sidecar web}; do
  for attempt in $(seq 1 60); do
    if S "docker manifest inspect ghcr.io/reh3376/career-site-$repo:$TAG >/dev/null 2>&1"; then
      echo "  ok   $repo"
      break
    fi
    [ "$attempt" = 60 ] && die "image career-site-$repo:$TAG never appeared"
    sleep 20
  done
done

# 2. The checkout matters: the Caddyfile, the public corpus and the
#    backup scripts are read from it, not from the image.
say "updating the checkout"
S "cd $REMOTE && git pull -q --ff-only && git log -1 --format='%h %s'"

if [ -n "$ONLY" ]; then
  VAR=$(echo "$ONLY" | tr '[:lower:]' '[:upper:]')_IMAGE_TAG
  say "pointing $VAR at $TAG (IMAGE_TAG stays $PREV_TAG)"
  S "cd $REMOTE && sed -i '/^${VAR}=/d' .env.prod && echo '${VAR}=$TAG' >> .env.prod && grep -E '^(IMAGE_TAG|${VAR})=' .env.prod"
else
  say "pointing .env.prod at $TAG"
  # A full rollout clears any per-service override, so the stack goes
  # back to one tag and .env.prod cannot describe a mixture nobody
  # intended.
  S "cd $REMOTE && echo '$PREV_TAG' > .previous-image-tag && sed -i -E '/^(WEB|API|SIDECAR)_IMAGE_TAG=/d' .env.prod && sed -i 's/^IMAGE_TAG=.*/IMAGE_TAG=$TAG/' .env.prod && grep '^IMAGE_TAG=' .env.prod"
fi

# Only the three images this repository builds.
#
# A bare `compose pull` pulls everything in the file, including
# postgres, ollama and minio, none of which a career-site release
# changes. That put a third party in the middle of every deploy, and on
# 2026-09-30 one of them stepped on it: quay.io began refusing
# anonymous pulls of minio/minio, the pull step failed 401, and the
# rollout only survived because the three images that mattered had
# already come down and minio kept its cached copy.
#
# Verified at the time: quay issues an anonymous token and still
# refuses the manifest, so this is a policy change rather than a blip
# and pinning a tag would not have helped.
#
# Naming the services means a release can only be broken by an image
# this repository actually publishes. Third-party images still update,
# deliberately, by pulling them by hand.
PULL=${ONLY:-api sidecar web}
say "pulling ($PULL)"
for attempt in 1 2 3; do
  S "cd $REMOTE && $CS pull ${PULL} 2>&1 | tail -3" && break
  [ "$attempt" = 3 ] && die "image pull kept failing"
  echo "  retrying"; sleep 10
done

say "starting"
S "cd $REMOTE && $CS up -d ${ONLY} 2>&1 | tail -8"

say "waiting for readiness"
ok=no
for _ in $(seq 1 30); do
  body=$(curl -s --max-time 10 "$SITE/api/readyz" || true)
  case "$body" in
    *'"status":"ok"'*) echo "  $body"; ok=yes; break ;;
    *) sleep 10 ;;
  esac
done
[ "$ok" = yes ] || die "readyz never came good; roll back with deploy/rollout.sh --rollback"

# The version endpoint reports the commit the api binary was built from,
# so this catches an image that did not actually change. It speaks for
# the api only, so on --only web or sidecar it should still be the tag
# the api was already on.
got=$(curl -s --max-time 10 "$SITE/api/readyz" | sed -n 's/.*"version":"\([^"]*\)".*/\1/p')
case "$ONLY" in
  web|sidecar)
    [ "$got" = "$PREV_TAG" ] || echo "  warning: readyz reports '$got'; the api was not deployed and should still be '$PREV_TAG'"
    ;;
  *)
    [ "$got" = "$TAG" ] || echo "  warning: readyz reports version '$got', expected '$TAG'"
    ;;
esac

say "migrations"
S "cd $REMOTE && $CS logs api --since 5m 2>&1 | grep -iE 'goose|migrat' | tail -5 || echo '  none run'"

say "pruning old images"
S "KEEP=\" $TAG $PREV_TAG latest \$(docker ps --format '{{.Image}}' | sed 's/.*://' | tr '\\n' ' ') \"; n=0
   for img in \$(docker images --format '{{.Repository}}:{{.Tag}}' | grep '^ghcr.io/reh3376/career-site-'); do
     t=\${img##*:}
     case \"\$KEEP\" in *\" \$t \"*) ;; *) docker rmi \"\$img\" >/dev/null 2>&1 && n=\$((n+1)) ;; esac
   done
   docker builder prune -af >/dev/null 2>&1 || true
   docker image prune -f >/dev/null 2>&1 || true
   echo \"  removed \$n stale tags\"; df -h / | tail -1"

# The live check is the gate, not a postscript: if it fails the rollout
# failed, whatever the containers say. Piping it would hide the exit
# status, so capture first and print after.
say "live check"
out=$("$REPO_DIR/deploy/live-check.sh" "$SITE" 2>&1) && rc=0 || rc=$?
echo "$out" | tail -12
[ "$rc" -eq 0 ] || die "live check failed; roll back with deploy/rollout.sh --rollback"

say "done: $SITE on $TAG (previous $PREV_TAG, roll back with deploy/rollout.sh --rollback)"
