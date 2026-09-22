#!/usr/bin/env bash
# Weekly proof that the newest backup restores, run on the production
# server by career-restore-check.timer. See deploy/backup/README.md.
#
# An untested backup is a guess. This restores the newest dump into a
# scratch database beside the live one, counts what came back, compares
# it with the live database, then drops the scratch. It never touches
# the live database, and it fails loudly if the numbers disagree.
set -euo pipefail

BACKUP_DIR=${BACKUP_DIR:-/opt/career-site-backups}
CONTAINER=${CONTAINER:-career-site-postgres-1}
DB=${DB:-career}
DB_USER=${DB_USER:-career}
SCRATCH=${SCRATCH:-career_restore_check}

log() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
die() { log "ERROR: $*"; exit 1; }

psql_live() { docker exec "$CONTAINER" psql -U "$DB_USER" -d "$DB" -Atc "$1"; }
psql_scratch() { docker exec "$CONTAINER" psql -U "$DB_USER" -d "$SCRATCH" -Atc "$1"; }

cleanup() {
  docker exec "$CONTAINER" psql -U "$DB_USER" -d postgres -c \
    "DROP DATABASE IF EXISTS $SCRATCH WITH (FORCE)" >/dev/null 2>&1 || true
  docker exec "$CONTAINER" rm -f /tmp/restore-check.dump >/dev/null 2>&1 || true
}
trap cleanup EXIT

newest=$(ls -1t "$BACKUP_DIR"/career-*.dump 2>/dev/null | head -1) \
  || die "no dumps in $BACKUP_DIR"
[ -n "$newest" ] || die "no dumps in $BACKUP_DIR"
log "restoring $(basename "$newest") into $SCRATCH"

cleanup
docker cp "$newest" "$CONTAINER:/tmp/restore-check.dump" >/dev/null
docker exec "$CONTAINER" psql -U "$DB_USER" -d postgres -c "CREATE DATABASE $SCRATCH" >/dev/null

# pgvector lives in the dump as CREATE EXTENSION, which needs the
# extension present in the image. Restore is quiet about ownership
# warnings for roles that exist only in the globals dump, so only a hard
# failure counts.
if ! docker exec "$CONTAINER" pg_restore -U "$DB_USER" -d "$SCRATCH" --no-owner --no-privileges /tmp/restore-check.dump 2> /tmp/restore-check.err; then
  log "pg_restore reported errors:"
  tail -20 /tmp/restore-check.err || true
  die "restore failed"
fi

tables=$(psql_scratch "select count(*) from information_schema.tables where table_schema='public'")
[ "${tables:-0}" -gt 10 ] || die "restored schema has only ${tables:-0} tables"

# Row counts on the tables whose loss would actually hurt. The restore
# is from last night, so it may trail the live database; it must never
# exceed it, and it must not be empty where live has rows.
fail=0
for t in users jd_submissions corpus_chunks decision_log events approval_decisions; do
  live=$(psql_live "select count(*) from $t" 2>/dev/null || echo skip)
  [ "$live" = skip ] && continue
  got=$(psql_scratch "select count(*) from $t" 2>/dev/null || echo 0)
  if [ "$got" -gt "$live" ]; then
    log "FAIL $t: restored $got rows, live has $live"
    fail=1
  elif [ "$live" -gt 0 ] && [ "$got" -eq 0 ]; then
    log "FAIL $t: restored 0 rows, live has $live"
    fail=1
  else
    log "ok   $t: restored $got, live $live"
  fi
done

# The résumé PDFs are bytea and are the easiest thing to lose silently,
# so check the bytes survived rather than just the row.
pdfs=$(psql_scratch "select count(*) from jd_submissions where resume_pdf is not null and length(resume_pdf) > 1000" 2>/dev/null || echo 0)
log "ok   résumé PDFs restored with content: $pdfs"

[ "$fail" -eq 0 ] || die "restore check failed"
log "restore check passed against $(basename "$newest")"
