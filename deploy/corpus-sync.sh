#!/usr/bin/env bash
# Sync the curated private corpus to the server.
#
# Reads a manifest of `source|kind` lines (paths relative to
# docs/personal, kinds from ingest.KnownKinds) and rsyncs only text
# sources (.md / .txt) into <host>:<dest>/<kind>/. The manifest lives
# in docs/personal so it is gitignored with the material it lists;
# see deploy/corpus-manifest.example.txt for the shape.
#
# Nothing is deleted on the server. To retire a document, remove it
# there by hand (or from the manifest and then the server) and hit
# "Reindex private corpus" so the admin list reflects reality.
#
# Usage:  deploy/corpus-sync.sh [manifest] [user@host] [dest]
set -euo pipefail

MANIFEST="${1:-docs/personal/corpus-manifest.txt}"
HOST="${2:-${CORPUS_HOST:-career@5.161.62.205}}"
DEST="${3:-${CORPUS_DEST:-/opt/career-site-private/corpus}}"
SRC_ROOT="docs/personal"

if [[ ! -f "$MANIFEST" ]]; then
  echo "manifest not found: $MANIFEST" >&2
  echo "copy deploy/corpus-manifest.example.txt to $MANIFEST and edit it" >&2
  exit 1
fi

ssh "$HOST" "mkdir -p '$DEST'"

# Manifest is read on fd 3: ssh and rsync inside the loop would otherwise
# consume the rest of the manifest from stdin.
synced=0
while IFS= read -r -u 3 line || [[ -n "$line" ]]; do
  line="${line%%#*}"
  line="$(echo "$line" | sed -e 's/[[:space:]]*$//')"
  [[ -z "$line" ]] && continue
  src="${line%%|*}"
  kind="${line##*|}"
  src="$(echo "$src" | sed -e 's/[[:space:]]*$//')"
  kind="$(echo "$kind" | sed -e 's/^[[:space:]]*//')"
  if [[ "$src" == "$line" || -z "$kind" ]]; then
    echo "skip (need 'source|kind'): $line" >&2
    continue
  fi
  if [[ ! "$kind" =~ ^[a-z_]+$ ]]; then
    echo "skip (bad kind '$kind'): $line" >&2
    continue
  fi
  path="$SRC_ROOT/$src"
  if [[ ! -e "$path" ]]; then
    echo "skip (missing): $path" >&2
    continue
  fi
  ssh "$HOST" "mkdir -p '$DEST/$kind'"
  if [[ -d "$path" ]]; then
    rsync -a --prune-empty-dirs \
      --include='*/' --include='*.md' --include='*.txt' --exclude='*' \
      "$path/" "$HOST:$DEST/$kind/$(basename "$src")/"
  else
    case "$path" in
      *.md|*.txt) rsync -a "$path" "$HOST:$DEST/$kind/" ;;
      *) echo "skip (not text; extraction lands later): $path" >&2; continue ;;
    esac
  fi
  echo "synced $src -> $kind/"
  synced=$((synced + 1))
done 3< "$MANIFEST"

echo "done: $synced manifest entries synced to $HOST:$DEST"
echo "next: /admin/corpus -> Reindex private corpus"
