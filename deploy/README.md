# Deploy

career-site deploys as a single-host Docker Compose stack behind Caddy (auto-TLS via Let's Encrypt). Target host today: one Hetzner Cloud CX22 in Ashburn, VA. Everything the deploy needs lives in this directory:

```
deploy/
├── README.md            you are here
├── caddy/
│   ├── Caddyfile        dev (HTTP :80)
│   └── Caddyfile.prod   prod (auto-TLS on ${SITE_DOMAIN})
├── postgres/init/       first-boot SQL (pgvector extension)
└── setup-server.sh      one-shot server provisioning
```

## Sequence

Do these in order. Each step is a hand-off between you and me.

### 1. Register the domain (you)

`rogerhenley.dev` — check availability + register at your registrar of choice (Cloudflare Registrar and Porkbun are both fine). Do **not** enable proxy / CDN yet — Caddy needs to see the ACME HTTP-01 challenge on port 80.

### 2. Provision the Hetzner server (you)

In the Hetzner Cloud console:

1. **Server**: Add server → Location: **Ashburn, VA (US East)** → Image: **Ubuntu 24.04** → Type: **CX22** ($5/mo, 2 vCPU / 4 GB / 40 GB).
2. **SSH key**: pick the ed25519 public key you added earlier (`macbook->macstudio-tb`).
3. **Networking**: IPv4 + IPv6 both on; firewall rules **inbound tcp/22, 80, 443** and nothing else.
4. **Backups**: enable (+$1/mo — worth it).
5. Create.

Note the public IPv4 address. Tell me it, plus the domain you registered.

### 3. Point DNS at the server (you)

At your registrar:
- `A     rogerhenley.dev.    → <server IP>    TTL 300`
- `AAAA  rogerhenley.dev.    → <server IPv6>  TTL 300` (optional but nice)

Wait ~5 minutes and confirm with `dig +short rogerhenley.dev`.

### 4. Bootstrap the server (I run — one command)

I SSH in as root (with your key), run `deploy/setup-server.sh`, which:
- Installs Docker + Docker Compose plugin
- Creates a non-root `career` user with sudo + docker access
- Clones `github.com/reh3376/career-site` into `/opt/career-site`
- Creates `/opt/career-site/.env.prod` with placeholder values
- Sets up a systemd unit so the stack survives reboots

I do **not** paste secrets on the command line — you fill `.env.prod` in step 6.

### 5. Set up Resend (you)

1. Sign up at [resend.com](https://resend.com/) with your gmail. Free tier is 3k emails/month.
2. **Domains** → Add `rogerhenley.dev` → follow the DNS records they give (three CNAMEs for SPF/DKIM/DMARC). Add them at your registrar. Verify (~10 min).
3. **API Keys** → Create a key named `career-site-prod`, Full Access. Save it — you'll paste it in step 6.

### 6. Fill in `.env.prod` on the server (you)

SSH in:

```bash
ssh career@<server-ip>
cd /opt/career-site
sudo vi .env.prod
```

Fill in every blank in `.env.prod` per the template comments. Specifically:
- `POSTGRES_PASSWORD` — generate with `openssl rand -hex 24`
- `MINIO_ROOT_PASSWORD` — same
- `RESEND_API_KEY` — from Resend (starts `re_`)
- `MAIL_FROM` — `"career-site <noreply@rogerhenley.dev>"` (or however you want it to appear)
- `DECISION_TOKEN_SECRET` — generate with `openssl rand -hex 32`
- `ADMIN_USERNAME` — your own admin email (e.g. `rogerhenley345@gmail.com` or a dedicated `admin@rogerhenley.dev`)
- `CAREER_SITE_ADMIN_PW` — a strong password you'll remember (12+ chars)
- `IMAGE_TAG` — leave blank for `latest`, or paste the git SHA I give you (recommended for the very first deploy)

### 7. First boot (I run)

```bash
cd /opt/career-site
docker compose -f docker-compose.yml -f docker-compose.prod.yml pull
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
docker compose logs -f api  # watch migrations + admin bootstrap
```

Verify:
- `https://rogerhenley.dev/` → landing page
- `https://rogerhenley.dev/api/healthz` → `{"status":"ok"}`
- `https://rogerhenley.dev/api/readyz` → `{"status":"ok","checks":{"postgres":true,"sidecar":true}}`
- `https://rogerhenley.dev/login` → sign in as the admin you set

### 8. Verify Resend deliverability (you)

Register a fresh account from a mailbox that isn't in the whitelist. You should:
- Get the "Verify your email" mail in your real inbox (not spam).
- After verifying, receive nothing until you (as admin) approve.
- Sign in as admin, approve yourself (or approve via the email link that landed in `ADMIN_USERNAME`'s inbox).
- The applicant gets "Your access is approved" mail.

If any of those bounce or hit spam, Resend's dashboard flags why (usually a missing DNS record).

## Updating

Every push to `main` builds and pushes images to `ghcr.io/reh3376/career-site-{api,sidecar,web}:{sha,latest}`. To roll out a specific SHA:

```bash
cd /opt/career-site
sudo git pull
sudo sed -i "s/^IMAGE_TAG=.*/IMAGE_TAG=<12-char-sha>/" .env.prod
docker compose -f docker-compose.yml -f docker-compose.prod.yml pull
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

To roll back, re-run with the previous SHA. Postgres data volumes survive; migrations are additive.

## Backups (next)

Not in this deploy. Follow-ups:
- Nightly `pg_dump` to an off-site bucket (FSD §8.10).
- Object-storage snapshot for MinIO.
- Runbook for restore.

## Secrets rotation

- `DECISION_TOKEN_SECRET` — rotating invalidates every unclicked Accept/Decline email. Do it on suspected compromise; low-cost otherwise.
- `POSTGRES_PASSWORD` — requires updating the `.env.prod` and restarting; migrations reconnect automatically.
- `RESEND_API_KEY` — rotate in Resend, update `.env.prod`, restart the api container.
- `CAREER_SITE_ADMIN_PW` — update `.env.prod` and restart; the API's admin-bootstrap re-hashes the password on next start.
