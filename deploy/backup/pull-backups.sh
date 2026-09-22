#!/usr/bin/env bash
# Pull the server's backups down to this machine. Runs on the owner's
# Mac, because Starlink gives it no inbound address, so the server can
# never push. Scheduled by the launchd job beside this script; safe to
# run by hand any time.
#
#   deploy/backup/pull-backups.sh
set -euo pipefail

SERVER=${SERVER:-career@5.161.62.205}
REMOTE_DIR=${REMOTE_DIR:-/opt/career-site-backups}
LOCAL_DIR=${LOCAL_DIR:-$HOME/backups/career-site}
# Off-box copies are kept longer than the server's, since the point of
# this copy is surviving the loss of the server itself.
KEEP_DAYS=${KEEP_DAYS:-365}
# The environment file holds the secrets a rebuild needs: the decision
# token key, the event salt, the mail and storage credentials. Without
# it a restored database is a site nobody can sign into and every
# outstanding approval link is dead. Pulled by default; set to 0 to skip.
PULL_ENV=${PULL_ENV:-1}

log() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }

umask 077
mkdir -p "$LOCAL_DIR"
chmod 700 "$LOCAL_DIR"

if ! ssh -o ConnectTimeout=20 -o BatchMode=yes "$SERVER" true 2>/dev/null; then
  log "server unreachable, nothing pulled"
  exit 0   # a sleeping laptop or a down link is not a failure
fi

log "pulling dumps from $SERVER"
rsync -az --partial --chmod=F600 \
  -e 'ssh -o ConnectTimeout=20 -o BatchMode=yes' \
  "$SERVER:$REMOTE_DIR/" "$LOCAL_DIR/"

if [ "$PULL_ENV" = "1" ]; then
  log "pulling .env.prod (secrets; stays out of the repo)"
  rsync -az --chmod=F600 \
    -e 'ssh -o ConnectTimeout=20 -o BatchMode=yes' \
    "$SERVER:/opt/career-site/.env.prod" "$LOCAL_DIR/env.prod.copy"
fi

# Local retention, by filename date like the server's.
now=$(date -u +%s)
pruned=0
shopt -s nullglob
for f in "$LOCAL_DIR"/career-*.dump "$LOCAL_DIR"/globals-*.sql; do
  base=$(basename "$f")
  day=${base#*-}; day=${day%%T*}
  [[ $day =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || continue
  # BSD date on macOS, GNU date elsewhere.
  when=$(date -j -f %Y-%m-%d "$day" +%s 2>/dev/null || date -u -d "$day" +%s 2>/dev/null) || continue
  age=$(( (now - when) / 86400 ))
  if [ "$age" -gt "$KEEP_DAYS" ]; then rm -f "$f"; pruned=$((pruned + 1)); fi
done

count=$(ls -1 "$LOCAL_DIR"/career-*.dump 2>/dev/null | wc -l | tr -d ' ')
newest=$(ls -1t "$LOCAL_DIR"/career-*.dump 2>/dev/null | head -1)
log "$count dumps here, newest $(basename "${newest:-none}"), $pruned pruned, $(du -sh "$LOCAL_DIR" | cut -f1) on disk"

# Loud about staleness: a pull job that silently stopped working is the
# usual way backups are discovered missing.
if [ -n "${newest:-}" ]; then
  mtime=$(stat -f %m "$newest" 2>/dev/null || stat -c %Y "$newest")
  age_h=$(( (now - mtime) / 3600 ))
  if [ "$age_h" -gt 48 ]; then
    log "WARNING: newest local dump is ${age_h}h old; check career-backup.timer on the server"
    exit 1
  fi
fi
