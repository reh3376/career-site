#!/usr/bin/env bash
# One rollout, start to finish: wait for the images, deploy, verify,
# prune. Run it from a checkout on the owner's machine, not on the box.
#
#   deploy/rollout.sh                    # deploy the current origin/main
#   deploy/rollout.sh <12-char-sha>      # deploy a specific build
#   deploy/rollout.sh --rollback         # go back to the previous tag
#   deploy/rollout.sh --only web [sha]   # one service, leaving the rest
#   deploy/rollout.sh --force            # deploy even if the site is busy
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
# --force skips the quiet check below. It exists for an outage, where
# shipping a fix beats protecting a run, and for nothing else.
FORCE=""
if [ "${1:-}" = "--force" ]; then
  FORCE=1
  shift
fi

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

# 0. Is anybody using the site right now?
#
# A rollout recreates the api and web containers. A participant part way
# through the decision test loses the run, and there is no resume by
# design, so the cost is fifteen unbroken minutes of a volunteer's
# attention and one row from a sample that had two independent
# participants in it. Volunteers are the scarce resource here; server
# time and my time are not.
#
# Added 2026-10-07 after a day of deploying repeatedly while volunteers
# were being recruited. The rule was "check first"; a rule that lives
# only in somebody's head is a rule that gets skipped on the busy day,
# so the script checks.
#
# A session sitting at `running` for hours is abandoned, not active, so
# this asks about recent starts rather than about status alone.
quiet_check() {
  [ -n "$FORCE" ] && { say "skipping the quiet check (--force)"; return 0; }
  say "checking the site is quiet"
  local sql out
  # The eval_runs clause is bounded by 8 hours on purpose: that is
  # jobTimeout for an evaluation (handlers/admin_jobs.go), so a row
  # older than that cannot still be running and must be a stranded one
  # the next boot has not yet closed. Without the bound, one crashed run
  # would block every deploy until somebody noticed why.
  sql="SELECT
         (SELECT count(*) FROM dt_sessions
           WHERE status = 'running'
             AND NOT is_synthetic
             AND started_at > now() - interval '25 minutes'),
         (SELECT count(*) FROM events
           WHERE occurred_at > now() - interval '10 minutes'),
         (SELECT count(*) FROM eval_runs
           WHERE status = 'running'
             AND started_at > now() - interval '8 hours');"
  out=$(S "cd $REMOTE && $CS exec -T postgres psql -U career -d career -Atc \"$sql\"" 2>/dev/null | tr -d '\r')
  local in_test recent evals
  in_test=$(echo "$out" | cut -d'|' -f1)
  recent=$(echo "$out" | cut -d'|' -f2)
  evals=$(echo "$out" | cut -d'|' -f3)
  if [ -z "$in_test" ]; then
    say "could not read the database to check; treating that as not quiet"
    die "quiet check failed. Re-run with --force only if you know the site is idle"
  fi
  echo "  decision tests in flight: $in_test"
  echo "  events in the last 10 min: $recent"
  echo "  evaluations in flight: $evals"
  if [ "$in_test" -gt 0 ]; then
    die "$in_test decision test run(s) started in the last 25 minutes. A rollout \
would end them and there is no resume. Wait, or --force if you are certain"
  fi
  # FR-EVAL-06: an evaluation shall not be silently destroyed by a
  # deploy. It was, repeatedly: 6 of the first 15 production runs ended
  # at status 'failed' with partial scores, several of them hours in,
  # because the job runner holds everything in memory and recreating the
  # api container kills the goroutine. Nothing in this script looked.
  #
  # This does not make a run survive a deploy; it makes the deploy
  # refuse. Resume is the stronger thing and is not built; see the FSD
  # row and docs/fsd-items/FR-EVAL-06-eval-durability.md.
  if [ "$evals" -gt 0 ]; then
    die "$evals evaluation(s) in flight. A rollout recreates the api container, \
which kills the run: the job runner holds it in memory and there is no resume, \
so the hours already spent are lost and only the postings already scored survive.
  Wait for it to finish (/admin/ops shows progress), or if this change only
  touches the front end, deploy just that: deploy/rollout.sh --only web
  leaves the api container, and the run, untouched.
  --force overrides, and ends the run."
  fi
  if [ "$recent" -gt 20 ]; then
    die "$recent events in the last 10 minutes: the site is busy. Wait, or --force"
  fi
  echo "  ok, quiet"
}
quiet_check

# Settings the new image needs in .env.prod before it is pulled.
#
# The api refuses to boot without TURNSTILE_SECRET_KEY, so a missing
# value would otherwise be discovered by the readiness check, after the
# containers have already been recreated. Catching it here costs one ssh
# and leaves production untouched.
#
# TURNSTILE_SITE_KEY is the quieter half and the reason this check takes
# a pair rather than a list: with the secret set and the site key empty,
# everything boots, every page renders, no widget appears, and every
# registration is rejected for a missing token. Nothing fails loudly, so
# the script has to.
env_preflight() {
  say "checking .env.prod has what the new image needs"
  local missing=""
  for key in TURNSTILE_SECRET_KEY TURNSTILE_SITE_KEY; do
    if ! S "cd $REMOTE && grep -qE '^${key}=.+' .env.prod"; then
      missing="$missing $key"
    fi
  done
  if [ -n "$missing" ]; then
    die "missing from .env.prod:$missing
  The api will not boot without TURNSTILE_SECRET_KEY, and an empty
  TURNSTILE_SITE_KEY hides the widget while the api still demands a
  token. Create the widget at https://dash.cloudflare.com -> Turnstile,
  put both values in $REMOTE/.env.prod, then re-run."
  fi
  echo "  ok, both Turnstile keys present"
}
env_preflight

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
#
#    The Caddyfile is bind-mounted, so the pull puts the new file on the
#    box but caddy keeps serving the old one until it is restarted. That
#    is silent: every page still renders, and only whatever the new
#    config was for stops working. Hash it either side of the pull so
#    the restart happens when, and only when, the file changed.
CADDY_CONF="$REMOTE/deploy/caddy/Caddyfile.prod"
CADDY_BEFORE=$(S "md5sum $CADDY_CONF 2>/dev/null | cut -d' ' -f1" || true)
say "updating the checkout"
S "cd $REMOTE && git pull -q --ff-only && git log -1 --format='%h %s'"
CADDY_AFTER=$(S "md5sum $CADDY_CONF 2>/dev/null | cut -d' ' -f1" || true)

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

# Caddy last, and only if its config moved. Restarting it as part of a
# whole-stack `up -d` is what strands web's DNS entry for `api`
# (EAI_AGAIN), so web and api are already up and ready by this point and
# caddy comes back on its own.
if [ -n "$CADDY_AFTER" ] && [ "$CADDY_BEFORE" != "$CADDY_AFTER" ]; then
  say "Caddyfile changed, restarting caddy"
  S "cd $REMOTE && $CS restart caddy"
  # A bad config takes the site off the internet, so prove it answers
  # before the live check reports on anything else.
  for attempt in $(seq 1 15); do
    code=$(curl -s -o /dev/null -w '%{http_code}' "$SITE/" 2>/dev/null || echo 000)
    [ "$code" = "200" ] && { echo "  ok, caddy serving ($code)"; break; }
    [ "$attempt" = 15 ] && die "caddy did not serve after restart (last $code); \
check $CADDY_CONF and run: $CS logs caddy --tail 50"
    sleep 2
  done
else
  echo "  Caddyfile unchanged, leaving caddy alone"
fi

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
