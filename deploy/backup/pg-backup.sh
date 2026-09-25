#!/usr/bin/env bash
# Nightly Postgres backup, run on the production server by
# career-backup.timer. See deploy/backup/README.md.
#
# Writes a compressed custom-format dump plus the cluster globals (the
# roles, which a bare-metal rebuild needs), verifies the dump is
# readable, then prunes old copies. Every step is logged to the journal;
# a non-zero exit marks the systemd unit failed, which is what the
# weekly restore check and `systemctl list-timers` surface.
set -euo pipefail

BACKUP_DIR=${BACKUP_DIR:-/opt/career-site-backups}
CONTAINER=${CONTAINER:-career-site-postgres-1}
DB=${DB:-career}
DB_USER=${DB_USER:-career}

# Retention, in that order of precedence: everything for two weeks,
# Sunday copies for two months, first-of-month copies for half a year.
# At a couple of megabytes a dump this costs almost nothing and covers
# the realistic failure, which is noticing a bad migration or a bad
# delete days later.
KEEP_DAILY_DAYS=${KEEP_DAILY_DAYS:-14}
KEEP_WEEKLY_DAYS=${KEEP_WEEKLY_DAYS:-56}
KEEP_MONTHLY_DAYS=${KEEP_MONTHLY_DAYS:-190}

log() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
die() { log "ERROR: $*"; exit 1; }

command -v docker >/dev/null || die "docker not on PATH"
docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null | grep -q true \
  || die "container $CONTAINER is not running"

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

# The backups live on a separate volume, reached through a symlink at
# /opt/career-site-backups. If that volume ever fails to mount, the
# mountpoint is still an empty directory on the root disk, mkdir -p
# happily creates the path inside it, and every dump after that
# "succeeds" onto the disk the backups exist to survive. Nothing would
# look wrong until the day it mattered.
#
# So: refuse unless the target sits on a different device from the root
# filesystem. Set BACKUP_REQUIRE_SEPARATE_DEVICE=0 to allow a
# same-device location deliberately, which is the right setting on a
# machine that has no second volume.
if [ "${BACKUP_REQUIRE_SEPARATE_DEVICE:-1}" = "1" ]; then
  # -L matters: BACKUP_DIR is a symlink onto the volume, and without
  # dereferencing, stat reports the device of the symlink itself, which
  # is the root disk. That made this refuse the very arrangement it
  # exists to protect.
  backup_dev=$(stat -L -c %d "$BACKUP_DIR" 2>/dev/null || echo "")
  root_dev=$(stat -L -c %d / 2>/dev/null || echo "")
  if [ -n "$backup_dev" ] && [ "$backup_dev" = "$root_dev" ]; then
    die "refusing to write backups to $BACKUP_DIR: it is on the root filesystem, which usually means the backup volume is not mounted. Set BACKUP_REQUIRE_SEPARATE_DEVICE=0 if that is intended."
  fi
fi

stamp=$(date -u +%Y-%m-%dT%H-%M-%SZ)
dump="$BACKUP_DIR/career-$stamp.dump"
globals="$BACKUP_DIR/globals-$stamp.sql"
tmp="$dump.part"

# The dump carries member names, email addresses and the generated
# résumé PDFs, so it is written private from the first byte rather than
# chmod'ed afterwards.
umask 077

log "dumping $DB from $CONTAINER"
if ! docker exec "$CONTAINER" pg_dump -U "$DB_USER" -d "$DB" --format=custom --compress=9 > "$tmp"; then
  rm -f "$tmp"
  die "pg_dump failed"
fi
mv "$tmp" "$dump"

# Globals are the roles, including career_admin_readonly. Without them a
# restore onto an empty cluster fails on the first GRANT.
if ! docker exec "$CONTAINER" pg_dumpall -U "$DB_USER" --globals-only > "$globals.part"; then
  rm -f "$globals.part"
  die "pg_dumpall --globals-only failed"
fi
mv "$globals.part" "$globals"

size=$(stat -c%s "$dump")
[ "$size" -gt 10240 ] || die "dump is implausibly small ($size bytes)"

# Structural verification: pg_restore reads the whole archive's table of
# contents, so a truncated or corrupt file fails here rather than on the
# night it is needed. The file goes into the container because
# pg_restore needs a seekable path and the host has no client tools.
log "verifying archive"
docker cp "$dump" "$CONTAINER:/tmp/verify.dump" >/dev/null
if ! docker exec "$CONTAINER" pg_restore --list /tmp/verify.dump > /dev/null; then
  docker exec "$CONTAINER" rm -f /tmp/verify.dump || true
  die "pg_restore --list rejected the archive"
fi
entries=$(docker exec "$CONTAINER" pg_restore --list /tmp/verify.dump | grep -c '^[0-9]' || true)
docker exec "$CONTAINER" rm -f /tmp/verify.dump || true
[ "${entries:-0}" -gt 20 ] || die "archive lists only ${entries:-0} objects"

log "ok: $(basename "$dump") $(numfmt --to=iec "$size" 2>/dev/null || echo "$size bytes"), $entries objects"

# Prune. Age is taken from the filename so a copied or touched file is
# not accidentally kept forever.
now=$(date -u +%s)
pruned=0
shopt -s nullglob
for f in "$BACKUP_DIR"/career-*.dump "$BACKUP_DIR"/globals-*.sql; do
  base=$(basename "$f")
  day=${base#*-}            # 2026-09-22T03-15-00Z.dump
  day=${day%%T*}            # 2026-09-22
  [[ $day =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || continue
  when=$(date -u -d "$day" +%s 2>/dev/null) || continue
  age=$(( (now - when) / 86400 ))
  dow=$(date -u -d "$day" +%u)   # 7 = Sunday
  dom=$(date -u -d "$day" +%d)

  keep=no
  if   [ "$age" -le "$KEEP_DAILY_DAYS" ];                                then keep=yes
  elif [ "$dow" = "7" ] && [ "$age" -le "$KEEP_WEEKLY_DAYS" ];           then keep=yes
  elif [ "$dom" = "01" ] && [ "$age" -le "$KEEP_MONTHLY_DAYS" ];         then keep=yes
  fi

  if [ "$keep" = no ]; then
    rm -f "$f"
    pruned=$((pruned + 1))
  fi
done

kept=$(ls -1 "$BACKUP_DIR"/career-*.dump 2>/dev/null | wc -l)
log "retention: $kept dumps kept, $pruned files pruned, $(du -sh "$BACKUP_DIR" | cut -f1) on disk"
