#!/usr/bin/env bash
# One rollout, start to finish: wait for the images, deploy, verify,
# prune. Run it from a checkout on the owner's machine, not on the box.
#
#   deploy/rollout.sh                 # deploy the current origin/main
#   deploy/rollout.sh <12-char-sha>   # deploy a specific build
#   deploy/rollout.sh --rollback      # go back to the previous tag
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
for repo in api sidecar web; do
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

say "pointing .env.prod at $TAG"
S "cd $REMOTE && echo '$PREV_TAG' > .previous-image-tag && sed -i 's/^IMAGE_TAG=.*/IMAGE_TAG=$TAG/' .env.prod && grep '^IMAGE_TAG=' .env.prod"

say "pulling"
for attempt in 1 2 3; do
  S "cd $REMOTE && $CS pull 2>&1 | tail -3" && break
  [ "$attempt" = 3 ] && die "image pull kept failing"
  echo "  retrying"; sleep 10
done

say "starting"
S "cd $REMOTE && $CS up -d 2>&1 | tail -8"

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

# The version endpoint reports the commit the binary was built from, so
# this catches an image that did not actually change.
got=$(curl -s --max-time 10 "$SITE/api/readyz" | sed -n 's/.*"version":"\([^"]*\)".*/\1/p')
[ "$got" = "$TAG" ] || echo "  warning: readyz reports version '$got', expected '$TAG'"

say "migrations"
S "cd $REMOTE && $CS logs api --since 5m 2>&1 | grep -iE 'goose|migrat' | tail -5 || echo '  none run'"

say "pruning old images"
S "KEEP=' $TAG $PREV_TAG latest '; n=0
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
