# 0033. Run a separately built service behind the site's proxy, at a path

Status: Accepted 2026-11-27

## Context

The v2c translator ([reh3376/v2c-translator](https://github.com/reh3376/v2c-translator))
reads a veteran's DD-214, DD-215 or VMET and returns civilian job
titles, skills and a keyword string. It is its own application: a
Next.js front end over a Python command-line tool with OCR and an O*NET
index, built and published as one image by its own repository.

Until now every service in this stack was built here. `api`, `web` and
`sidecar` share one `IMAGE_TAG`, CI in this repository publishes all
three, and `deploy/rollout.sh` moves them together. The translator does
not fit that shape, and the owner made two decisions about how it is
offered:

- **Address:** `rogerhenley.dev/v2c`, a path on this site, not a
  subdomain.
- **Access:** anyone, with no account. Every upload is guarded by
  Cloudflare Turnstile and a per-visitor rate limit.

Three facts decide the rest.

**A Caddy `header` directive with a value replaces the upstream
header.** The site sets one Content-Security-Policy for every response.
Left as it was, the browser would receive this site's policy on `/v2c`
and the translator's own policy would be discarded.

**The api keeps evaluations in memory.** A rollout that recreates the
api ends any evaluation or decision-test run in flight, which is why
`rollout.sh` checks the site is quiet first. A translator release has
nothing to do with the api and should not be able to cost a run.

**The upload is a service record.** It carries a name, dates, and often
a Social Security number. Nothing about it may be kept.

## Decision

1. **The translator runs as one more compose service, `v2c`, in the
   prod overlay only,** from the image its own repository publishes.
   This repository does not build it and holds none of its code.
2. **Caddy routes `/v2c` and `/v2c/*` to it** on the internal network.
   The container publishes no host port.
3. **The site's Content-Security-Policy is scoped away from those
   paths** with a matcher, its value unchanged, so the translator's own
   narrower policy is the one the browser receives.
4. **Caddy sets `X-Real-IP` on the route,** replacing anything the
   client sent, because the translator's per-visitor rate limit keys on
   it. This is the same reasoning as the api's `ClientIP`.
5. **The service has its own tag, `V2C_IMAGE_TAG`, and its own
   rollout, `deploy/rollout-v2c.sh`,** which recreates only the `v2c`
   container. `rollout.sh` never pulls it. Because it cannot touch the
   api, it has no quiet check.
6. **It reuses the api's Turnstile key pair.** The hostname is the
   same, so the widget is the same.
7. **Uploads never touch the disk.** They are read in a memory-only
   tmpfs and deleted when the report is written; the root filesystem is
   read-only; the container logs no file names, contents or report
   data. The `/privacy` page says so.
8. **The container is capped** in memory and CPU so that it cannot
   starve the model that reads job postings on the same host.

## Consequences

- A second repository can now change what is served under this domain.
  The tag is pinned to a sha by `rollout-v2c.sh` rather than left on
  `main`, and `deploy/live-check.sh` checks the translator's health,
  readiness, hosted privacy text and policy on every run.
- The Turnstile secret is present in two containers instead of one.
- `/v2c` is not a Next.js route in this repository. It is absent from
  `public-routes.ts`, so it is in neither this site's proxy allow-list
  nor its sitemap. Requests to it never reach the web container.
  Whether it should be listed for crawlers is the owner's to decide.
- A translation competes for CPU with an evaluation running at the same
  time. The cap bounds that; it does not remove it.
- The privacy claims on `/privacy` are true only while the translator's
  own repository keeps them true. That repository tests them; this one
  cannot.
- Removing the service is one revert, one rollout and one
  `docker compose rm`.

## Alternatives considered

- **A subdomain.** Cleaner isolation, since a separate origin needs no
  policy scoping. The owner chose the path so the tool sits with the
  rest of his work.
- **Vendoring the translator into this repository.** One build and one
  tag, at the cost of coupling a Python OCR toolchain to this site's CI
  and of every translator change needing a full rollout and its quiet
  check.
- **Letting `rollout.sh` move it.** Simpler to operate, but every
  translator release would then recreate the api.
