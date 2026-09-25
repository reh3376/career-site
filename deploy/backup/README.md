# Backups

The database is the only thing here that cannot be rebuilt from the
repo. It holds the members, the approval decisions, the JD submissions
with their assessments and generated résumé PDFs, the corpus chunks
with their embeddings, the decision log that becomes training data, and
the event stream. Losing it loses all of that. Everything else on the
box, the images, the models, the checkout, comes back with a deploy.

## The shape

| | what | where | how often | kept |
|---|---|---|---|---|
| dump | `pg_dump` custom format, plus the cluster roles | `/opt/career-site-backups` on the server, a symlink onto the 15 GB volume | nightly, 03:15 UTC | 14 daily, 8 weekly, 6 monthly |
| copy | the same files, pulled down | `~/backups/career-site` on the owner's Mac | daily, or on the next wake | 365 days |
| proof | newest dump restored into a scratch database and counted | on the server | weekly, Sunday 04:30 UTC | n/a |

Two copies on two machines in two places. The server copy covers the
ordinary case, a bad migration or a mistaken delete noticed days later.
The Mac copy covers the loss of the server itself.

The Mac pulls; the server never pushes. Starlink gives the Mac no
inbound address, so a push would need the Mac reachable, which it is
not. A laptop that is asleep or off simply misses a day and catches up
on the next wake, which is why the server keeps its own copies too.

The database is about 13 MB, so a compressed dump is a few megabytes
and the whole retention above costs well under a gigabyte. There was no
reason to trade retention for space.

## What is in a dump, and how to treat it

Member names and email addresses, the full text of every submitted job
description, and the generated résumé PDFs. Treat a dump exactly like
the database: private mode on every file, never in the repo, never in a
shared folder, never attached to anything.

The dumps are not encrypted at rest. On the server that would buy
nothing, because anyone who can read the dump can already read the
database it came from. On the Mac they sit in a private directory on an
encrypted disk. If the copies ever move somewhere less trusted, encrypt
them at that point.

The pull also takes `.env.prod` to `env.prod.copy`. That file is the
difference between restoring a database and restoring a working site:
it carries the decision token key, the event salt, and the mail and
storage credentials. Without it, every approval link already sent is
dead and nobody can sign in. It is secrets, so it lives beside the
dumps under the same rules and never enters git. Set `PULL_ENV=0` to
skip it.

Not backed up, on purpose: the private corpus under
`/opt/career-site-private`, whose source of truth is `docs/personal` on
the Mac and which is re-synced with `make sync-corpus`; MinIO, which
holds 104 KB and nothing that matters yet; the Ollama models, which are
a re-pull.

## Install

On the server, after a deploy that carries changes to these files:

```bash
ssh career@5.161.62.205 'cd /opt/career-site && sudo deploy/backup/install-server.sh'
```

That creates the backup directory, installs the two systemd timers, and
enables them. It is idempotent, so re-running it after a deploy is how
changes to the scripts or the schedule take effect.

On the Mac, once:

```bash
deploy/backup/install-mac.sh
```

## Check it is working

```bash
# server: when the timers last ran and next run
ssh career@5.161.62.205 'systemctl list-timers --no-pager "career-*"'

# server: the last dump and the last restore test
ssh career@5.161.62.205 'journalctl -u career-backup -n 15 --no-pager'
ssh career@5.161.62.205 'journalctl -u career-restore-check -n 30 --no-pager'

# mac: what is actually on this machine
ls -lh ~/backups/career-site | tail -5
tail -20 ~/backups/career-site/pull.log
```

The pull script exits non-zero if the newest dump it can see is more
than two days old, so a pull job that quietly stopped working shows up
as a failed launchd run rather than as silence.

## Restore

### One table, or one row, from last night

Restore into a scratch database first and copy across. Never restore
over the live database to recover one thing.

```bash
ssh career@5.161.62.205
cd /opt/career-site-backups
docker cp career-<stamp>.dump career-site-postgres-1:/tmp/r.dump
docker exec career-site-postgres-1 psql -U career -d postgres -c 'CREATE DATABASE scratch'
docker exec career-site-postgres-1 pg_restore -U career -d scratch --no-owner /tmp/r.dump
docker exec -it career-site-postgres-1 psql -U career -d scratch   # look around
# copy what you need across, then:
docker exec career-site-postgres-1 psql -U career -d postgres -c 'DROP DATABASE scratch WITH (FORCE)'
```

### The whole database, onto this same server

Stop the api first so nothing writes while the restore runs.

```bash
cd /opt/career-site
CS='docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml'
$CS stop api
docker cp /opt/career-site-backups/career-<stamp>.dump career-site-postgres-1:/tmp/r.dump
docker exec career-site-postgres-1 psql -U career -d postgres -c 'DROP DATABASE career WITH (FORCE)'
docker exec career-site-postgres-1 psql -U career -d postgres -c 'CREATE DATABASE career OWNER career'
docker exec career-site-postgres-1 pg_restore -U career -d career --no-owner /tmp/r.dump
$CS start api
$CS logs api --tail 30          # migrations run on boot and should be a no-op
deploy/live-check.sh https://rogerhenley.dev
```

### A new server, from nothing

1. Build the box with `deploy/setup-server.sh` and clone the repo to
   `/opt/career-site`.
2. Put `env.prod.copy` back as `.env.prod`, mode 600. Without it the
   site comes up unable to verify any token it ever issued.
3. Bring up Postgres alone: `$CS up -d postgres`.
4. Load the roles, then the data:
   ```bash
   docker cp globals-<stamp>.sql career-site-postgres-1:/tmp/g.sql
   docker exec career-site-postgres-1 psql -U career -d postgres -f /tmp/g.sql
   docker cp career-<stamp>.dump career-site-postgres-1:/tmp/r.dump
   docker exec career-site-postgres-1 pg_restore -U career -d career --no-owner /tmp/r.dump
   ```
5. `$CS up -d`, then `deploy/live-check.sh`.
6. Re-sync the private corpus from the Mac with `make sync-corpus`, and
   re-pull the Ollama models, which the sidecar does on first use.

Point the DNS last, once the live check passes.

## When this changes

Adding a table changes nothing here; the dump is whole-database. Adding
a new store, an object bucket that actually holds files, or a second
volume, does change it, and the table at the top of this file is the
place to say so.
