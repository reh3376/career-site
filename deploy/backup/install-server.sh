#!/usr/bin/env bash
# Install the nightly dump and the weekly restore test on the production
# server. Idempotent: safe to re-run after every deploy, which is how
# unit changes in the repo reach the box.
#
#   ssh career@<server> 'cd /opt/career-site && sudo deploy/backup/install-server.sh'
set -euo pipefail

BACKUP_DIR=${BACKUP_DIR:-/opt/career-site-backups}
REPO=${REPO:-/opt/career-site}
OWNER=${OWNER:-career}

[ "$(id -u)" -eq 0 ] || { echo "run with sudo"; exit 1; }
[ -d "$REPO/deploy/backup/systemd" ] || { echo "run from the deploy checkout at $REPO"; exit 1; }

install -d -o "$OWNER" -g "$OWNER" -m 700 "$BACKUP_DIR"
chmod +x "$REPO"/deploy/backup/*.sh

for unit in career-backup.service career-backup.timer \
            career-restore-check.service career-restore-check.timer; do
  install -m 644 "$REPO/deploy/backup/systemd/$unit" "/etc/systemd/system/$unit"
done

systemctl daemon-reload
systemctl enable --now career-backup.timer career-restore-check.timer

echo
systemctl list-timers --no-pager 'career-*' || true
echo
echo "installed. first dump runs at the next 03:15 UTC; run one now with:"
echo "  sudo systemctl start career-backup.service && journalctl -u career-backup -n 20 --no-pager"
