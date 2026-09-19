# apps/web

Next.js 15 (App Router, React Server Components, TypeScript `strict`) frontend for career-site. Uses `@connectrpc/connect-web` against the Go API's ConnectRPC surface. Same-origin in production, so no CORS.

## Layout

```
apps/web/
├── package.json
├── tsconfig.json
├── next.config.mjs
├── eslint.config.mjs
├── .npmrc                    strict-dep-builds off (see below)
├── public/
├── src/
│   ├── app/
│   │   ├── layout.tsx        root layout
│   │   ├── page.tsx          placeholder home (real landing lands Phase 1)
│   │   └── version/page.tsx  server-rendered API version — Phase 0 smoke
│   ├── lib/api.ts            Connect client factory
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
pnpm build
```

`pnpm build` emits a Next.js standalone bundle under `.next/standalone/` that the Docker runtime stage copies.

## Docker

```bash
docker build -t career-site/web:dev .
```

## `.npmrc` note

`strict-dep-builds=false` is set to keep `pnpm install` non-interactive. pnpm 10+ errors out when a transitive dep (here: `unrs-resolver`, pulled in via `eslint-config-next`) wants to run a postinstall script that hasn't been explicitly approved. Turning off the strict check lets `pnpm install` succeed in CI and Docker; the resolver falls back to a JS implementation when the native binary isn't built.

## What is not here yet

Phase 0 scaffold. Everything visual, functional, and gated lands later:

- **Phase 1** — the real landing page + "coming soon — late 2026" screen; the registration form; the "your request is with the owner" post-verify screen; the gated under-construction page after approval; Tailwind + design tokens; global layout + header/footer.
- **Phase 2+** — content sections, personalized home, Ask Roger UI, member profile, admin console.
