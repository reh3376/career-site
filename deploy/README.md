# Deploy

career-site deploys as a single-host Docker Compose stack behind Caddy (auto-TLS via Let's Encrypt). Target host today: one Hetzner Cloud CPX31 (4 vCPU / 8 GB) in Ashburn, VA, at `5.161.62.205`, deploy dir `/opt/career-site`. That box is the ceiling: there is no budget for a bigger one, so the workload (embeddings and the LLM included) is sized to fit it. Everything the deploy needs lives in this directory:

```
deploy/
├── README.md                    you are here
├── caddy/
│   ├── Caddyfile                dev (HTTP :80)
│   └── Caddyfile.prod           prod (auto-TLS on ${SITE_DOMAIN}, security headers)
├── postgres/init/               first-boot SQL (pgvector extension)
├── setup-server.sh              one-shot server provisioning
├── corpus-sync.sh               rsync the private corpus to the box (`make sync-corpus`)
├── corpus-manifest.example.txt  shape of the gitignored docs/personal/corpus-manifest.txt
└── live-check.sh                functional check of a running deployment
```

## Sequence

Do these in order. Each step is a hand-off between you and me.

### 1. Register the domain (you)

`rogerhenley.dev` — check availability + register at your registrar of choice (Cloudflare Registrar and Porkbun are both fine). Do **not** enable proxy / CDN yet — Caddy needs to see the ACME HTTP-01 challenge on port 80.

### 2. Provision the Hetzner server (you)

In the Hetzner Cloud console:

1. **Server**: Add server → Location: **Ashburn, VA (US East)** → Image: **Ubuntu 24.04** → Type: **CPX31** (4 vCPU / 8 GB / 40 GB). The site started on a CPX11 and was rescaled on 2026-09-21 when the LLM landed (Rescale → keep disk, needs a power-off in the console); a fresh install should go straight to the CPX31. Add a 2 GB swapfile as the OOM safety net.
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
- Relies on Docker's `restart: unless-stopped` policy (and an enabled
  docker.service) to bring the whole stack back after a reboot; there is
  no separate systemd unit. Never `docker compose stop` before a planned
  reboot, or those containers stay down afterwards; a plain `poweroff`
  is the right move.

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
- `DB_READONLY_PASSWORD`: `openssl rand -hex 24`, different from `POSTGRES_PASSWORD`; the api applies it to the `career_admin_readonly` role at boot
- `RESUME_PDF_OWNER_PASSWORD`: the fixed owner password that locks editing on generated résumé PDFs; empty skips PDF rendering
- `LINKEDIN_URL`, `GITHUB_URL`: outbound links, read at request time; empty hides the link
- `IMAGE_TAG`: the 12-char git SHA of a `main` build whose three images exist on GHCR; never `latest`
- The model settings for the box (these are what prod runs today):
  `SIDECAR_EMBED_PROVIDER=ollama`, `SIDECAR_LLM_PROVIDER=ollama`, `OLLAMA_LLM_URL=http://ollama:11434`, `OLLAMA_LLM_MODEL=qwen3:4b-q8_0`, `SIDECAR_LLM_NUM_CTX=8192`, `SIDECAR_LLM_TIMEOUT_SECONDS=3000`, `JD_PIPELINE_TIMEOUT_SECONDS=3600`, `JD_MATCH_THRESHOLD=0.70`, `OLLAMA_MEM_LIMIT=7g`, `OLLAMA_MAX_LOADED_MODELS=1`, `OLLAMA_FLASH_ATTENTION=1`, `OLLAMA_KV_CACHE_TYPE=q8_0`. The example file still carries the pre-cutover defaults for a few of these (timeouts, memory cap); the values above are the ones the box runs. See "Embeddings and the LLM" below.

### 7. First boot (I run)

```bash
cd /opt/career-site
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml pull
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml logs -f api
```

Compose only auto-loads `.env`; the extra `--env-file .env.prod` flag is
what tells it to read the prod overlay's variable references from your
prod secrets file. Alias it if you're going to run these a lot:

```bash
alias cs='docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml'
# then just:  cs pull    cs up -d    cs logs -f api    cs down
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

`main` is protected (ruleset `protect-main`): changes land only through a PR with the api, web, sidecar, proto and gitleaks checks green; the auto-pr workflow opens the draft from `claude_dev*`, `feat/*` and `fix/*` branches and the owner merges. Every merge to `main` then builds and pushes images to `ghcr.io/reh3376/career-site-{api,sidecar,web}:{sha,latest}`. Before bumping, confirm the tag exists for **all three** images; prod compose has `build: !reset`, so a missing tag fails the `up` loudly instead of building on the box. To roll out a specific SHA:

```bash
cd /opt/career-site
deploy/live-check.sh https://rogerhenley.dev          # baseline, from your machine
git pull
sed -i "s/^IMAGE_TAG=.*/IMAGE_TAG=<12-char-sha>/" .env.prod
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml pull
docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d
deploy/live-check.sh https://rogerhenley.dev          # compare with the baseline
```

`git pull` matters: `apps/web/content` is bind-mounted into the api as the public corpus and the Caddyfile is read from the checkout. To roll back, re-run with the previous SHA. Postgres data volumes survive; migrations (goose, currently up to `00022`) are additive and run on api boot. Pulling the sidecar image can take longer than the default compose timeout on a slow pull; if `pull` times out, run it again.

## Private corpus

The Ask Roger corpus has two mounts inside the api container:

| Mount | Host source | Visibility | How it gets there |
|---|---|---|---|
| `/corpus` | `/opt/career-site/apps/web/content` (repo checkout) | `public` | ships with every deploy |
| `/corpus-private` | `/opt/career-site-private/corpus` (outside the checkout) | `corpus_only` | `make sync-corpus` from the owner's machine |

`docs/personal` is gitignored and never reaches the server on its own. To feed
the private corpus, keep a manifest at `docs/personal/corpus-manifest.txt`
(gitignored; shape in `deploy/corpus-manifest.example.txt`) listing which
files or folders to sync and the `source_kind` each lands under, then:

```bash
make sync-corpus          # rsync .md/.txt sources to <host>:/opt/career-site-private/corpus/<kind>/
```

Then in the admin console: `/admin/corpus` → **Reindex private corpus**. The
reindex runs as a job with a progress readout; you can leave the page and
come back. Everything under the private mount is ingested as
`visibility=corpus_only`; front matter cannot loosen that, and corpus_only
text is never quoted on the site. The manifest kinds include `profile`, the
owner's career facts sheet that the JD judge reads first on every call. The
sync never deletes on the server.

## Embeddings and the LLM (Ollama)

The example env file ships with both providers on `stub`, which keeps every
interface exercised but produces meaningless vectors and refuses to assess
JDs (unless `LLM_ALLOW_STUB=1`, CI only). Prod has run real embeddings since
2026-09-21 and the real LLM since 2026-09-22, both on the box's own `ollama`
container, which pulls `nomic-embed-text` into a volume on first boot. The
LLM model is pulled by hand once:

```bash
cs exec ollama ollama pull qwen3:4b-q8_0     # cs = the compose alias from step 7
```

**Sizing on the CPX31.** `OLLAMA_MEM_LIMIT=7g` caps the container. The
levers that make the 4b fit next to the embedder are
`OLLAMA_MAX_LOADED_MODELS=1` (one resident model; the embedder reloads in
seconds when needed), `OLLAMA_FLASH_ATTENTION=1` and
`OLLAMA_KV_CACHE_TYPE=q8_0`, with `SIDECAR_LLM_NUM_CTX=8192`. The first
attempt with both models resident and a full-precision KV cache OOM-killed
llama-server. Larger models are off the table: `qwen3:8b` OOM-killed the
box and `qwen3:14b` does not fit at all (the owner's decision is to stay on
the 4b rather than buy a bigger box). If a bigger model is ever wanted,
`OLLAMA_LLM_URL` can point the LLM at another Ollama host (a workstation
over a tailnet, or a hosted endpoint with `OLLAMA_API_KEY`) while
embeddings stay on the box.

**What to expect.** One JD pipeline runs at a time. Each requirement judge
call takes about 100 to 135 s and a résumé about 13 min, so a whole
submission is 15 to 30 min; `JD_PIPELINE_TIMEOUT_SECONDS=3600` and
`SIDECAR_LLM_TIMEOUT_SECONDS=3000` are sized for that. The submitter sees a
progress modal, and a failed assessment marks the row failed rather than
silently falling back to the retrieval score; **Re-score** on `/admin/jd`
runs it again.

**After a model or provider change.** Restart `ollama` and `sidecar`, then
`/admin/corpus` → **Embed sweep** (a job with progress; rerun until
`remaining` reads 0). The sweep re-embeds any chunk whose recorded embedder
differs from the live one, so nothing needs deleting. A new judge model also
needs recalibrating: the fit bands live in `app_settings` (edited on
`/admin/jd`; `JD_MATCH_THRESHOLD` only seeds them), and every calibration
run is recorded per model in
[`docs/llm-tuning-log.md`](../docs/llm-tuning-log.md). The original rollout
order is in [`docs/cutover-local-to-prod.md`](../docs/cutover-local-to-prod.md).

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
- `DB_READONLY_PASSWORD`: update `.env.prod` and restart the api; it re-applies the role password at boot.
- `RESUME_PDF_OWNER_PASSWORD`: only PDFs rendered after the change use the new value; PDFs already stored on submission rows keep the password they were locked with.
