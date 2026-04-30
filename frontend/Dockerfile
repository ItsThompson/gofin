# Stage 1: Build all Turborepo apps
FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json turbo.json ./
COPY apps/ apps/
COPY packages/ packages/
RUN npm ci
RUN npx turbo build

# Stage 2: Runtime
FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/apps/shell/build ./build
COPY --from=builder /app/apps/shell/server.ts ./
COPY --from=builder /app/apps/finance/dist ./public/remotes/finance
COPY --from=builder /app/apps/admin/dist ./public/remotes/admin
COPY --from=builder /app/node_modules ./node_modules
EXPOSE 3000
CMD ["node", "build/server.js"]
