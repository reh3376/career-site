# apps/web

Next.js 16 (App Router, React Server Components, TypeScript `strict`) frontend for career-site. Uses `@connectrpc/connect-web` against the Go API's ConnectRPC surface. Same-origin in production, so no CORS.

## Layout

```
apps/web/
├── package.json
├── tsconfig.json
├── next.config.mjs
├── eslint.config.mjs
├── vitest.config.ts
├── .npmrc                    strict-dep-builds off (see below)
├── public/
├── content/                  MDX articles, served from the filesystem
├── src/
│   ├── proxy.ts              access policy + the anonymous event id cookie
│   ├── app/
│   │   ├── layout.tsx        root layout, header, footer, Ask Roger panel
│   │   ├── page.tsx          landing, IT or OT depending on saved mode
│   │   ├── globals.css       Tailwind layer + design tokens
│   │   ├── robots.ts         ├─ both generated from lib/public-routes.ts
│   │   ├── sitemap.ts        ┘  so the three cannot disagree
│   │   ├── ask/              Ask Roger full page
│   │   ├── jd-upload/        posting submission + progress
│   │   ├── actions/          review results and fit categories
│   │   ├── admin/            console: members, corpus, Q&A bank, evals
│   │   ├── meetings/         booking and cancellation
│   │   ├── articles/ gallery/ home/ settings/
│   │   ├── login/ register/ verify/ forgot-password/ reset-password/
│   │   ├── contact/ privacy/ terms/ how-ask-roger-works/
│   │   └── version/page.tsx  server-rendered API version — Phase 0 smoke
│   ├── components/
│   │   ├── landing/          IT and OT landings, core actions, podcasts
│   │   ├── ask/              chat panel, thread, proposed actions
│   │   └── site-header, hamburger-menu, mode-toggle, ...
│   ├── lib/                  Connect client factory, session, access tiers
│   ├── fonts/                self-hosted woff2 (CSP has no font CDN)
│   └── gen/                  generated Connect-ES types (do not edit)
└── Dockerfile
```

## Run locally

```bash
pnpm install
pnpm dev
```

The dev server listens on `http://localhost:3000`. The `/version` page calls the Go API server-side; set `API_URL` (or `NEXT_PUBLIC_API_URL`) to point at it:

```bash
API_URL=http://localhost:8080 pnpm dev
```

## Test / lint / build

```bash
pnpm typecheck
pnpm lint
pnpm test
pnpm build
```

Run all four before pushing. `pnpm build` is not redundant with `typecheck`: it is the only one of the four that applies the App Router's own rules, so a client component importing a server-only module, or a `use client` boundary violation, fails here and nowhere else.

`pnpm test` is vitest. Coverage is deliberately narrow rather than broad: `lib/public-routes.test.ts` covers the access policy, because a mistake there either serves a members-only page to anyone or 404s a real one, and the same table drives the proxy, `robots.ts` and `sitemap.ts`. `docs/backlog.md` §4c names the next three worth writing.

`pnpm build` emits a Next.js standalone bundle under `.next/standalone/` that the Docker runtime stage copies.

## Docker

```bash
docker build -t career-site/web:dev .
```

## `.npmrc` note

`strict-dep-builds=false` is set to keep `pnpm install` non-interactive. pnpm 10+ errors out when a transitive dep (here: `unrs-resolver`, pulled in via `eslint-config-next`) wants to run a postinstall script that hasn't been explicitly approved. Turning off the strict check lets `pnpm install` succeed in CI and Docker; the resolver falls back to a JS implementation when the native binary isn't built.

## What is not here yet

Everything this section used to list as ahead has shipped, so what is left is a shorter list than the history suggests. As of 2026-10-01 the app serves 41 pages: the IT and OT landings, registration and the whole verify / approve / sign-in path, members-only articles and gallery, JD upload with live progress, review results and fit categories, Ask Roger as both a full page and a panel on every page, meeting booking and cancellation, settings, the admin console, and the public policy and explainer pages.

Still open, and both tracked in `docs/backlog.md` rather than here:

- **Test coverage beyond the access policy** (§4c). One test file is not a suite. The three named next targets are the booking flow's slot arithmetic, the JD upload form's states, and the admin scheduler's save-whole-or-refuse behaviour.
- **Dead-export detection** (§4c) — `knip` or `ts-prune`, to stop the generated and hand-written surfaces drifting apart.

Two constraints worth knowing before adding anything, because both have caught work late:

- **The CSP (`deploy/caddy/Caddyfile.prod`) has no `frame-src`,** so `default-src 'self'` governs frames and third-party embeds do not render. This is why the podcast section links out to Spotify instead of embedding its player. Anything external either links out or needs a deliberate CSP change. Fonts are self-hosted in `src/fonts/` even though the CSP would permit `fonts.gstatic.com`, so no page depends on a font CDN being reachable.
- **Member-facing routes must be added to `lib/public-routes.ts`**, not just created. That one table is what the proxy, `robots.ts` and `sitemap.ts` all read, and `MEMBER_PATHS` and the public list are disjoint by construction.
