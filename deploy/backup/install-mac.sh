#!/usr/bin/env bash
# Install the daily backup pull on this Mac. Idempotent.
#
#   deploy/backup/install-mac.sh
set -euo pipefail

REPO=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
LABEL=dev.rogerhenley.career-backup-pull
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
LOCAL_DIR="$HOME/backups/career-site"

[ "$(uname)" = Darwin ] || { echo "this installer is for the owner's Mac"; exit 1; }

mkdir -p "$LOCAL_DIR" "$HOME/Library/LaunchAgents"
chmod 700 "$LOCAL_DIR"
chmod +x "$REPO"/deploy/backup/*.sh

sed -e "s|__REPO__|$REPO|g" -e "s|__HOME__|$HOME|g" \
  "$REPO/deploy/backup/launchd/$LABEL.plist" > "$PLIST"

launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
launchctl bootstrap "gui/$(id -u)" "$PLIST"
launchctl enable "gui/$(id -u)/$LABEL"

echo "installed $LABEL"
echo "  pulls to:  $LOCAL_DIR"
echo "  log:       $LOCAL_DIR/pull.log"
echo "  run now:   launchctl kickstart -p gui/$(id -u)/$LABEL"
echo "  status:    launchctl print gui/$(id -u)/$LABEL | head -20"
