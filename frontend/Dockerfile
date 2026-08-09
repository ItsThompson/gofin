# =============================================================================
# MFE Multi-Stage Build (Turbo Prune)
#
# 4-stage build using turbo prune --docker to install only @gofin/shell's
# dependency tree. Key optimization: the installer stage (npm ci) is cached
# across source-only changes because out/json/ only contains package.json
# files and the lockfile.
# =============================================================================

# Stage 1: Prune workspace for shell app only
FROM node:24-alpine AS pruner
WORKDIR /app
COPY package.json package-lock.json turbo.json ./
COPY apps/ apps/
COPY packages/ packages/
RUN npx turbo prune @gofin/shell --docker

# Stage 2: Install dependencies (cached across source-only changes)
FROM node:24-alpine AS installer
WORKDIR /app
COPY --from=pruner /app/out/json/ ./
RUN npm ci

# Stage 3: Build
FROM node:24-alpine AS builder
WORKDIR /app
COPY --from=installer /app/ ./
COPY --from=pruner /app/out/full/ ./
# Vite bakes VITE_* values into the browser bundle, so the DSN and the release
# have to arrive at build time. An absent DSN produces a bundle that initializes
# nothing, which is what keeps the CI and E2E stack from sending events.
ARG VITE_SENTRY_DSN
ARG VITE_SENTRY_RELEASE
RUN npx turbo build --filter=@gofin/shell...

# Stage 4: Production runtime
FROM node:24-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production

# Copy pruned workspace structure and install production-only deps.
# Then remove packages that are not needed at runtime but get pulled in
# as optional peer dependencies (typescript via @react-router/node).
# lucide-react (39MB) is bundled into the SSR output via ssr.noExternal
# so only the ~12 used icons are included; the full package is unnecessary.
COPY --from=pruner /app/out/json/ ./
RUN npm ci --omit=dev && \
    rm -rf node_modules/typescript node_modules/@types \
           node_modules/lucide-react

# Copy shell build output and entry point
COPY --from=builder /app/apps/shell/build ./apps/shell/build
COPY --from=builder /app/apps/shell/server.js ./apps/shell/server.js
# The unbundled server graph: server.js imports instrument.server.mjs, which
# imports sentry.options.mjs. Nothing compiles or bundles these, so both must
# arrive verbatim or the container fails to boot on the first import.
COPY --from=builder /app/apps/shell/instrument.server.mjs ./apps/shell/instrument.server.mjs
COPY --from=builder /app/apps/shell/sentry.options.mjs ./apps/shell/sentry.options.mjs

WORKDIR /app/apps/shell
EXPOSE 3000
CMD ["node", "server.js"]
