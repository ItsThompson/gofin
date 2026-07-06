# Development Guide

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- [Just](https://github.com/casey/just) command runner
- [Go](https://go.dev/dl/) (see `services/*/go.mod` for required version)
- [Node.js](https://nodejs.org/) (see `frontend/package.json` for required version)
- [protoc](https://grpc.io/docs/protoc-installation/) (for protobuf compilation)
- [golang-migrate](https://github.com/golang-migrate/migrate) (for database migrations)
- [sqlc](https://sqlc.dev/) (for type-safe SQL code generation)

## Environment Setup

```bash
# Clone and configure
cp .env.example .env
# Edit .env with your values (defaults work for local development)
```

Key environment variables are documented in `.env.example`. See the [full variable reference](#environment-variables) below.

## Development Workflows

### Full Stack

```bash
just up          # Build and start everything
just seed-admin  # Create the initial admin user
# App: http://localhost:3000
# Grafana: http://localhost:3001
```

### Service-Focused

When iterating on a single backend service, avoid rebuilding all containers:

```bash
just dev-infra              # Start databases + monitoring
just dev-service auth       # Run the auth service locally (hot reload with air)
just dev-frontend           # Start the Turborepo frontend with HMR
```

### With Dev Overrides

`docker-compose.dev.yml` adds volume mounts (source code hot reload), debug ports (Delve debugger on 4000x), relaxed security (no HTTPS for cookies), and reduced bcrypt cost:

```bash
just up-dev
```

### Frontend Only (Mock Mode)

To work on the UI without running any backend services or Docker containers:

```bash
just dev-mock
```

This starts the shell dev server with the `VITE_MOCK_API=true` flag, which activates [MSW](https://mswjs.io/) (Mock Service Worker) in the browser. MSW intercepts all `fetch` calls to `/api/*` before they reach the express proxy, so the backend never needs to be running.

The mock layer provides:

- An operator (admin) user, auto-logged in by default (lands on `/admin`; personal finance routes are not reachable as a direct admin)
- A regular user (`alex`) available for identity assumption and for exercising the finance UI
- A current-month budget period ($3,000, 50/30/20 split)
- Seven sample expenses across different categories and tags
- All eleven default tags plus one custom tag
- Dashboard aggregation data (summary, pacing, tag spending, cumulative chart, historical comparison)
- An upcoming pro-rata installment

Mock data is defined in `frontend/apps/shell/mocks/data.ts`. Request handlers are in `frontend/apps/shell/mocks/handlers.ts`. The default mock user is the operator (admin), who lands on `/admin` and sees only the admin panel plus Settings (Profile and Password); a direct admin who opens a personal finance route by URL gets a 403 page, so the budget/expenses/dashboard fixtures are not reachable while acting as the admin. To exercise the personal finance UI, set `currentMockUser` to the regular user (`regularUser`, "alex") in `data.ts`, or log in as the admin and assume that user from the admin panel.

**How it works:**

1. The shell's `entry.client.tsx` checks `import.meta.env.VITE_MOCK_API`
2. When `"true"`, it starts the MSW browser service worker before React hydration
3. The service worker intercepts `fetch` calls matching handler routes and returns mock responses
4. Unmatched requests (static assets, HMR) pass through normally

## Code Generation

### Protobuf / gRPC

Each Go service with a `proto/` directory has gRPC definitions. After modifying `.proto` files:

```bash
just proto auth       # Generate for a specific service
just proto-all        # Generate for all services
```

### sqlc

Auth and Finance services use sqlc for type-safe database queries. After modifying SQL queries in `db/queries/`:

```bash
just sqlc auth
just sqlc finance
```

### Database Migrations

Migrations use golang-migrate with SQL files in each service's `db/migrations/` directory:

```bash
just migrate auth up      # Run auth service migrations
just migrate finance down  # Roll back finance service migrations
```

In Docker Compose, migrations run automatically: PostgreSQL's init scripts execute migration files on first boot.

## Go Workspace

The `services/` directory uses a [Go workspace](https://go.dev/doc/tutorial/workspaces) (`go.work`) to link all service modules. This allows cross-module imports during development while maintaining independent `go.mod` files per service.

```bash
cd services
go work sync    # Sync the workspace after dependency changes
```

## Frontend (Turborepo)

The `frontend/` directory is a Turborepo monorepo with three apps and three shared packages:

**Apps:**
- `shell`: MF host (SSR, routing, auth, layout, API proxy)
- `finance`: MF remote (dashboard, expense log, expense form, settings)
- `admin`: MF remote (admin panel, user list, identity assumption)

**Packages:**
- `@gofin/ui`: shared shadcn/ui components with Tailwind
- `@gofin/types`: shared TypeScript types (API contracts, domain models)
- `@gofin/config`: shared configs (tailwind, tsconfig, eslint)

```bash
cd frontend
npx turbo dev     # Start all apps in dev mode
npx turbo build   # Build all apps
npx turbo lint    # Lint all apps and packages
npx turbo test    # Run all frontend tests
```

## Environment Variables

All environment variables are documented in `.env.example` with sensible defaults for local development:

```bash
cp .env.example .env
```

Key variable groups:

- **Shared**: `JWT_SECRET`, `LOG_LEVEL`, `ENVIRONMENT`
- **Auth Service**: PostgreSQL connection, bcrypt cost, admin seed credentials
- **Finance Service**: PostgreSQL connection, expense service gRPC address
- **Expense Service**: immudb connection credentials
- **Datarights Service**: PostgreSQL connection, Resend API key, email sender config
- **API Gateway**: service addresses (gRPC and REST) for auth, expense, finance, and datarights
- **MFE (Shell)**: API gateway URL, cookie security flag
- **Grafana Auth Proxy**: JWT secret, Grafana URL

See `.env.example` for the complete list with descriptions and example values.

## Admin Bootstrap

The first admin user must be created before the admin panel, identity assumption, or Grafana access can function:

```bash
just seed-admin
```

This runs the auth service's `seed-admin` CLI subcommand, which reads `ADMIN_USERNAME`, `ADMIN_EMAIL`, and `ADMIN_PASSWORD` from environment variables. The command is idempotent: if an admin already exists, it exits successfully. It also marks the admin's onboarding complete server-side for data consistency, but that flag is not the exemption mechanism: admins are structurally exempt from onboarding via their role and the route access metadata (see below), not because the flag is set.

The admin is an operator-only identity: it owns no finance data and does not use the onboarding or budget flows (`POST /api/auth/onboarding-complete` is a `Personal` route and returns 403 to a direct admin). After logging in through the normal UI, a direct admin lands on `/admin`; opening a personal finance route by URL renders a 403 page, because the route's access metadata rejects a direct operator.

## Datarights Service

The datarights service runs on port **8084** (REST) and **9084** (gRPC, reserved). It handles GDPR data export: collecting user data from upstream services, assembling CSVs into a ZIP, and emailing the result.

### Local Development

```bash
just dev-service datarights
```

By default, `EMAIL_ENABLED=false` in `.env.example`. In this mode, the service logs email content to stdout instead of sending via Resend:

```
{"level":"INFO","msg":"email delivery disabled: logging email content","to":"user@example.com","zip_size_bytes":24576}
```

To test real email delivery locally, set `EMAIL_ENABLED=true` and provide a valid `RESEND_API_KEY` in your `.env`.

### Brand Tokens

The service reads `tokens/brand.json` for email template styling (colors, logo URL). In Docker, this is volume-mounted from the repo root `tokens/` directory. When running locally via `just dev-service datarights`, set `BRAND_TOKENS_PATH=./tokens/brand.json` (relative to repo root).

### Key Environment Variables

| Variable | Default | Purpose |
|----------|---------|-------|
| `DATARIGHTS_DB_URL` | (required) | PostgreSQL connection with `search_path=datarights` |
| `EMAIL_ENABLED` | `false` (via .env.example) | When false, emails log to stdout. If unset, defaults to `true` |
| `RESEND_API_KEY` | (empty) | Required when `EMAIL_ENABLED=true` |
| `EMAIL_FROM` | `gofin <noreply@usegofin.com>` | Sender address |
| `BRAND_TOKENS_PATH` | `/app/tokens/brand.json` | Path to brand token JSON file |
| `EXPORT_MAX_CONCURRENT` | `5` | Max parallel export goroutines |
| `EXPORT_TIMEOUT_SECONDS` | `300` | Per-job timeout (5 minutes) |
