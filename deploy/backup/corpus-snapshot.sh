#!/usr/bin/env bash
# Snapshot and revert the Ask Roger corpus tables.
#
# There is no delete in the corpus API. ListCorpusDocuments,
# IngestCorpusText, ReindexCorpus and SweepCorpusEmbeddings are the
# whole surface, so a document ingested through /admin/corpus stays
# ingested. That is fine for material anyone is sure about and
# uncomfortable for a first run against live data.
#
# This gives that first run an undo. Take a snapshot, ingest, and if
# the result is wrong, revert.
#
# Two revert paths, because they fail differently:
#
#   revert-since  deletes documents above a recorded id. Ingestion only
#                 ever inserts, so removing what was added restores the
#                 previous state exactly while never touching a row
#                 that was already there. This is the one to reach for.
#
#   restore       truncates both tables and reloads the dump. Exact, and
#                 correct even if something did update rows in place,
#                 but it discards anything ingested after the snapshot,
#                 including work you meant to keep.
#
# Note on `docker exec -i`: it is used only where data is piped in, on
# the pg_restore line. Anywhere else it reads the caller's stdin and
# swallows the confirmation `read` is waiting for, which reads as the
# operator declining a revert they in fact approved.
#
# corpus_chunks references corpus_documents ON DELETE CASCADE and
# nothing else references either table, so deleting a document takes
# its chunks with it and leaves the rest of the schema alone.
#
# Usage, from the server (or with DOCKER_HOST set):
#   deploy/backup/corpus-snapshot.sh snapshot
#   deploy/backup/corpus-snapshot.sh revert-since <max_document_id>
#   deploy/backup/corpus-snapshot.sh restore <dump-file>
#   deploy/backup/corpus-snapshot.sh status

set -euo pipefail

CONTAINER=${CONTAINER:-career-site-postgres-1}
DB=${DB:-career}
DB_USER=${DB_USER:-career}
SNAP_DIR=${SNAP_DIR:-/opt/career-site-backups/corpus}

die() { echo "error: $*" >&2; exit 1; }

psql_q() { docker exec "$CONTAINER" psql -U "$DB_USER" -d "$DB" -tAc "$1"; }

require_container() {
  command -v docker >/dev/null || die "docker not on PATH"
  docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null | grep -q true \
    || die "container $CONTAINER is not running"
}

cmd_status() {
  require_container
  local docs chunks maxid
  docs=$(psql_q "SELECT count(*) FROM corpus_documents")
  chunks=$(psql_q "SELECT count(*) FROM corpus_chunks")
  maxid=$(psql_q "SELECT coalesce(max(id), 0) FROM corpus_documents")
  echo "documents:       $docs"
  echo "chunks:          $chunks"
  echo "max document id: $maxid"
  echo
  echo "by visibility:"
  docker exec "$CONTAINER" psql -U "$DB_USER" -d "$DB" -c \
    "SELECT visibility, count(*) AS documents, sum(c.n) AS chunks
       FROM corpus_documents d
       LEFT JOIN (SELECT document_id, count(*) n FROM corpus_chunks GROUP BY 1) c
         ON c.document_id = d.id
      GROUP BY visibility ORDER BY visibility"
}

cmd_snapshot() {
  require_container
  mkdir -p "$SNAP_DIR"
  chmod 700 "$SNAP_DIR"
  local stamp maxid docs chunks dump meta
  stamp=$(date -u +%Y%m%dT%H%M%SZ)
  dump="$SNAP_DIR/corpus-$stamp.dump"
  meta="$SNAP_DIR/corpus-$stamp.txt"

  maxid=$(psql_q "SELECT coalesce(max(id), 0) FROM corpus_documents")
  docs=$(psql_q "SELECT count(*) FROM corpus_documents")
  chunks=$(psql_q "SELECT count(*) FROM corpus_chunks")

  # Custom format so pg_restore can load it selectively, data only so
  # the restore never fights the migrations over table definitions.
  docker exec "$CONTAINER" pg_dump -U "$DB_USER" -d "$DB" \
    --format=custom --compress=9 --data-only \
    -t corpus_documents -t corpus_chunks > "$dump" \
    || { rm -f "$dump"; die "pg_dump failed"; }

  [ -s "$dump" ] || { rm -f "$dump"; die "pg_dump produced an empty file"; }

  {
    echo "taken_at=$stamp"
    echo "max_document_id=$maxid"
    echo "documents=$docs"
    echo "chunks=$chunks"
  } > "$meta"

  echo "snapshot written"
  echo "  dump:            $dump"
  echo "  documents:       $docs"
  echo "  chunks:          $chunks"
  echo "  max document id: $maxid"
  echo
  echo "to undo an ingest made after this point:"
  echo "  $0 revert-since $maxid"
}

cmd_revert_since() {
  local watermark=${1:-}
  [ -n "$watermark" ] || die "usage: $0 revert-since <max_document_id>"
  [[ "$watermark" =~ ^[0-9]+$ ]] || die "watermark must be a number"
  require_container

  local n
  n=$(psql_q "SELECT count(*) FROM corpus_documents WHERE id > $watermark")
  if [ "$n" = "0" ]; then
    echo "nothing to revert: no documents above id $watermark"
    return 0
  fi

  echo "documents that would be deleted:"
  docker exec "$CONTAINER" psql -U "$DB_USER" -d "$DB" -c \
    "SELECT id, source_kind, source_path, visibility, ingested_at
       FROM corpus_documents WHERE id > $watermark ORDER BY id"
  echo
  printf 'delete these %s documents and their chunks? type yes: ' "$n"
  read -r reply
  [ "$reply" = "yes" ] || die "aborted"

  psql_q "DELETE FROM corpus_documents WHERE id > $watermark" >/dev/null
  echo "deleted $n documents; chunks removed by cascade"
  cmd_status
}

cmd_restore() {
  local dump=${1:-}
  [ -n "$dump" ] || die "usage: $0 restore <dump-file>"
  [ -f "$dump" ] || die "no such file: $dump"
  require_container

  echo "this truncates corpus_documents and corpus_chunks and reloads"
  echo "$dump, discarding everything ingested since it was taken."
  printf 'type yes to continue: '
  read -r reply
  [ "$reply" = "yes" ] || die "aborted"

  docker exec "$CONTAINER" psql -U "$DB_USER" -d "$DB" \
    -c "TRUNCATE corpus_chunks, corpus_documents RESTART IDENTITY"
  docker exec -i "$CONTAINER" pg_restore -U "$DB_USER" -d "$DB" --data-only < "$dump" \
    || die "pg_restore failed; the corpus is now empty, restore by hand before anything else"
  echo "restored"
  cmd_status
}

case "${1:-}" in
  snapshot)     shift; cmd_snapshot "$@" ;;
  revert-since) shift; cmd_revert_since "$@" ;;
  restore)      shift; cmd_restore "$@" ;;
  status)       shift; cmd_status "$@" ;;
  *) die "usage: $0 {snapshot|revert-since <id>|restore <dump>|status}" ;;
esac
