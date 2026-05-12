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
RUN npx turbo build --filter=@gofin/shell...

# Stage 4: Production runtime
FROM node:24-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production

# Copy pruned workspace structure and install production-only deps.
# Then remove packages that are not needed at runtime but get pulled in
# as optional peer dependencies (typescript via @react-router/node).
COPY --from=pruner /app/out/json/ ./
RUN npm ci --omit=dev && \
    rm -rf node_modules/typescript node_modules/@types

# Copy shell build output and entry point
COPY --from=builder /app/apps/shell/build ./apps/shell/build
COPY --from=builder /app/apps/shell/server.js ./apps/shell/server.js

# Copy remote MFE assets where the server expects them.
# Server resolves: path.resolve(__dirname, "../../") from build/server/index.js
# With __dirname = /app/apps/shell/build/server, that yields /app/apps/shell/
COPY --from=builder /app/apps/finance/dist ./apps/shell/finance/dist
COPY --from=builder /app/apps/admin/dist ./apps/shell/admin/dist

WORKDIR /app/apps/shell
EXPOSE 3000
CMD ["node", "server.js"]
