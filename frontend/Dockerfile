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
# The Sentry upload target. Slugs, not secrets, so an ARG is appropriate; the
# auth token below is not, because ARG values are recorded in the image history.
ARG SENTRY_ORG
ARG SENTRY_PROJECT
RUN npx turbo build --filter=@gofin/shell...

# `sourcemaps inject` writes a debug ID into every .js file and the matching
# .map, and Sentry pairs a deployed bundle to its maps by that ID. So the upload
# has to happen in the build that ships: a second build on the CI runner would
# key the maps to a bundle nobody downloads, and symbolication would fall back
# to the fragile release-plus-filename path or fail outright.
#
# apps/shell/build/client only. build/server holds the SSR bundle, and adding it
# would double the upload and mix two artifact sets under one release name.
#
# No install step: @sentry/cli is already here as a dependency of
# @sentry/react-router, and the installer stage ran npm ci with devDependencies.
# The leading --version proves it resolves from /app rather than trusting npm to
# keep hoisting it, and fails at that boundary instead of inside the upload.
#
# The token arrives on a secret mount, which leaves no trace in the image. An
# absent secret skips the upload, so a local `docker compose build` and the CI
# and E2E builds all still succeed.
RUN --mount=type=secret,id=sentry_auth_token \
    if [ -s /run/secrets/sentry_auth_token ]; then \
      export SENTRY_AUTH_TOKEN="$(cat /run/secrets/sentry_auth_token)" && \
      npx sentry-cli --version && \
      npx sentry-cli sourcemaps inject apps/shell/build/client && \
      npx sentry-cli sourcemaps upload apps/shell/build/client \
        --org "$SENTRY_ORG" --project "$SENTRY_PROJECT" \
        --release "${VITE_SENTRY_RELEASE:?the upload keys the maps to a release name, so one is required}"; \
    else \
      echo "No sentry_auth_token secret supplied: skipping the source-map upload."; \
    fi

# Stage 4: Production runtime
FROM node:24-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production

# Copy pruned workspace structure and install production-only deps.
# Then remove packages that are not needed at runtime but get pulled in
# as optional peer dependencies (typescript via @react-router/node).
# lucide-react (39MB) is bundled into the SSR output via ssr.noExternal
# so only the ~12 used icons are included; the full package is unnecessary.
#
# SENTRYCLI_SKIP_DOWNLOAD belongs on this stage and only on this stage.
# @sentry/cli's postinstall downloads its binary from downloads.sentry-cdn.com
# and exits non-zero on failure, so the flag removes a network call and a build
# failure mode from an image that never runs the CLI. The installer stage must
# keep the download path available, because the source-map step needs a working
# binary.
#
# The strip takes the two binary paths and nothing more. @sentry/react-router's
# node entry statically re-exports sentryOnBuildEnd and sentryReactRouter, which
# import @sentry/cli and @sentry/vite-plugin, so removing either JS tree makes
# the server SDK throw ERR_MODULE_NOT_FOUND on the first import of server.js.
# The JS resolves its binary lazily, and getBinaryPath() prefers
# @sentry/cli/sentry-cli over the platform package, which is why both go.
COPY --from=pruner /app/out/json/ ./
RUN SENTRYCLI_SKIP_DOWNLOAD=1 npm ci --omit=dev && \
    rm -rf node_modules/typescript node_modules/@types \
           node_modules/lucide-react \
           node_modules/@sentry/cli-linux-x64 \
           node_modules/@sentry/cli/sentry-cli

# Copy shell build output and entry point
COPY --from=builder /app/apps/shell/build ./apps/shell/build
COPY --from=builder /app/apps/shell/server.js ./apps/shell/server.js
# The unbundled server graph: server.js imports instrument.server.mjs, which
# imports sentry.options.mjs. Nothing compiles or bundles these, so both must
# arrive verbatim or the container fails to boot on the first import.
COPY --from=builder /app/apps/shell/instrument.server.mjs ./apps/shell/instrument.server.mjs
COPY --from=builder /app/apps/shell/sentry.options.mjs ./apps/shell/sentry.options.mjs

# Source maps carry the original TypeScript. They are uploaded to Sentry from
# the builder stage and must not ship. Rooted at /app rather than the build
# directory, so the maps some production dependencies ship are covered too.
RUN find /app -name "*.map" -delete

WORKDIR /app/apps/shell
EXPOSE 3000
CMD ["node", "server.js"]
