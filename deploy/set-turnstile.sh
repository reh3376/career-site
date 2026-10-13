#!/usr/bin/env bash
# Put the two Cloudflare Turnstile keys into production's .env.prod.
#
# Run it, paste each value when prompted. The keys are typed into your
# own terminal and sent straight to the server over ssh: they are never
# echoed, never written to shell history, and never pasted into a chat
# transcript.
#
#   ./deploy/set-turnstile.sh
#
# Get the two values from https://dash.cloudflare.com -> Turnstile, by
# adding a widget for rogerhenley.dev. Cloudflare calls them "Site Key"
# (public, goes in the page) and "Secret Key" (server only).
#
# Re-running it replaces whatever is there, so a mistyped key is fixed
# by running it again rather than by editing the file by hand.
set -euo pipefail

REMOTE=${REMOTE:-/opt/career-site}
HOST=${HOST:-career@5.161.62.205}

bold() { printf '\n\033[1m%s\033[0m\n' "$*"; }
die() { printf '\n\033[31mFAILED: %s\033[0m\n' "$*" >&2; exit 1; }

bold "== Cloudflare Turnstile keys for production"
echo "Find both at https://dash.cloudflare.com -> Turnstile -> your widget."
echo "Nothing you type here is shown on screen or saved locally."

# -s so neither value is echoed. The site key is public, but treating
# both the same way means there is no habit of pasting one in the clear
# and then doing it with the other by mistake.
printf '\nSite Key   (public, starts 0x): '
read -rs SITE
echo
printf 'Secret Key (server, starts 0x): '
read -rs SECRET
echo

# Catch the mistakes that otherwise boot cleanly and fail every
# submission: the two swapped, a stray quote from a copy, or a value
# that is obviously not a key.
[ -n "$SITE" ] || die "site key is empty"
[ -n "$SECRET" ] || die "secret key is empty"
[ "$SITE" != "$SECRET" ] || die "both values are identical; you pasted the same one twice"
case "$SITE$SECRET" in
  *[\"\'\ ]*) die "one of the values contains a quote or a space. Copy just the key itself." ;;
esac
[ ${#SECRET} -ge 20 ] || die "secret key is ${#SECRET} characters, shorter than any real one. Did it copy fully?"
[ ${#SITE} -ge 15 ] || die "site key is ${#SITE} characters, shorter than any real one."
# Cloudflare's secret is the longer of the two. If that is reversed,
# they are almost certainly swapped, which boots fine and then refuses
# every registration.
if [ ${#SECRET} -lt ${#SITE} ]; then
  die "the secret is shorter than the site key, which usually means they are swapped.
  The Secret Key is the longer one, and the one Cloudflare hides behind a reveal control."
fi

bold "== writing to $HOST:$REMOTE/.env.prod"
# Printed on stdin rather than passed as arguments, so neither value
# appears in the remote process list.
printf '%s\n%s\n' "$SITE" "$SECRET" | ssh "$HOST" "cd $REMOTE && \
  cp .env.prod .env.prod.bak-turnstile-\$(date +%s) && \
  read -r site && read -r secret && \
  sed -i -E '/^TURNSTILE_(SITE|SECRET)_KEY=/d' .env.prod && \
  printf 'TURNSTILE_SITE_KEY=%s\nTURNSTILE_SECRET_KEY=%s\n' \"\$site\" \"\$secret\" >> .env.prod && \
  chmod 600 .env.prod"

bold "== checking"
got=$(ssh "$HOST" "cd $REMOTE && grep -cE '^TURNSTILE_(SITE|SECRET)_KEY=.+' .env.prod")
[ "$got" = "2" ] || die "expected 2 keys in .env.prod, found $got"
echo "  both keys present"
echo
echo "Next: deploy/rollout.sh"
