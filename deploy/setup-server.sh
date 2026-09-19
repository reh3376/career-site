#!/usr/bin/env bash
# Bootstraps a fresh Ubuntu 24.04 Hetzner host for career-site.
# Idempotent: safe to re-run (skips work already done).
# Run as root: `bash setup-server.sh`.

set -euo pipefail

REPO_URL="https://github.com/reh3376/career-site.git"
APP_DIR="/opt/career-site"
APP_USER="career"
IMAGE_TAG="${IMAGE_TAG:-latest}"

log() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }

if [ "$(id -u)" -ne 0 ]; then
  echo "run as root (sudo bash $0)"
  exit 1
fi

log "1/6 apt update + prereqs"
apt-get update -qq
apt-get install -y --no-install-recommends \
  ca-certificates curl gnupg git ufw fail2ban unattended-upgrades

log "2/6 docker engine + compose plugin"
if ! command -v docker >/dev/null; then
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
    gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
fi

log "3/6 non-root app user ($APP_USER)"
if ! id "$APP_USER" >/dev/null 2>&1; then
  adduser --disabled-password --gecos "" "$APP_USER"
  usermod -aG docker,sudo "$APP_USER"
  mkdir -p "/home/$APP_USER/.ssh"
  # Reuse root's authorized_keys so the same public key that reached the
  # host can also reach the app user.
  if [ -f /root/.ssh/authorized_keys ]; then
    cp /root/.ssh/authorized_keys "/home/$APP_USER/.ssh/authorized_keys"
    chown -R "$APP_USER:$APP_USER" "/home/$APP_USER/.ssh"
    chmod 700 "/home/$APP_USER/.ssh"
    chmod 600 "/home/$APP_USER/.ssh/authorized_keys"
  fi
fi

log "4/6 clone repo to $APP_DIR"
if [ ! -d "$APP_DIR/.git" ]; then
  git clone "$REPO_URL" "$APP_DIR"
  chown -R "$APP_USER:$APP_USER" "$APP_DIR"
else
  git -C "$APP_DIR" fetch --tags
  git -C "$APP_DIR" pull --ff-only
fi

log "5/6 seed .env.prod template"
if [ ! -f "$APP_DIR/.env.prod" ]; then
  cp "$APP_DIR/.env.prod.example" "$APP_DIR/.env.prod"
  chown "$APP_USER:$APP_USER" "$APP_DIR/.env.prod"
  chmod 600 "$APP_DIR/.env.prod"
  echo
  echo "==> Edit $APP_DIR/.env.prod and fill in the blanks."
  echo "    See deploy/README.md step 6 for what each variable means."
  echo
fi

log "6/6 firewall + auto-updates"
ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp   comment 'ssh'
ufw allow 80/tcp   comment 'http (acme + redirect)'
ufw allow 443/tcp  comment 'https'
ufw --force enable

# unattended-upgrades ships enabled on Ubuntu; nudge it anyway.
dpkg-reconfigure -f noninteractive unattended-upgrades >/dev/null 2>&1 || true

cat <<EOF

------------------------------------------------------------
Server is bootstrapped.

Next:
  1. Edit /opt/career-site/.env.prod (see deploy/README.md step 6).
  2. As the $APP_USER user, first-boot:
       ssh $APP_USER@\$(curl -s ifconfig.me)
       cd $APP_DIR
       docker compose -f docker-compose.yml -f docker-compose.prod.yml pull
       docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
       docker compose logs -f api
------------------------------------------------------------

EOF
