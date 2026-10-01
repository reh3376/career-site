# syntax=docker/dockerfile:1.7

# pnpm arrives in two pieces and only one of them is obvious.
#
# corepack installs the pnpm *package*; pnpm then downloads its own
# native linux-x64 binary the first time it is actually invoked. Left
# alone, that first invocation is the `pnpm install` and `pnpm build`
# lines below, which sit under COPY, so every code change invalidated
# the layer and re-fetched the binary over the network. On 2026-10-01
# that fetch returned a truncated TLS response, undici asserted, and
# the image build died 0.7s in with a stack trace that mentioned
# nothing about the application.
#
# Running `pnpm --version` here forces the download into a layer keyed
# on the pnpm version alone. CI caches layers (type=gha), so an
# ordinary code change now reuses it and makes no network call at all.
FROM node:22-slim AS deps
RUN corepack enable \
 && corepack prepare pnpm@12.4.2 --activate \
 && pnpm --version
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

# Same reasoning as the deps stage: a separate stage has its own
# layers, so the binary has to be warmed here too or `pnpm build`
# fetches it again. That is the exact line that failed.
FROM node:22-slim AS build
RUN corepack enable \
 && corepack prepare pnpm@12.4.2 --activate \
 && pnpm --version
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN pnpm build

# Debian-based runtime — sharp / libvips prebuilt binaries work
# out of the box, which Alpine + musl does not.
FROM node:22-slim AS runtime
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000

RUN groupadd --system --gid 1001 nodejs \
 && useradd --system --uid 1001 --gid 1001 nextjs

COPY --from=build --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=build --chown=nextjs:nodejs /app/public ./public

USER nextjs
EXPOSE 3000
CMD ["node", "server.js"]
