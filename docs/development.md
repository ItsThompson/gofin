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
- **API Gateway**: service addresses (gRPC and REST) for auth, expense, and finance
- **MFE (Shell)**: API gateway URL, cookie security flag
- **Grafana Auth Proxy**: JWT secret, Grafana URL

See `.env.example` for the complete list with descriptions and example values.

## Admin Bootstrap

The first admin user must be created before the admin panel, identity assumption, or Grafana access can function:

```bash
just seed-admin
```

This runs the auth service's `seed-admin` CLI subcommand, which reads `ADMIN_USERNAME`, `ADMIN_EMAIL`, and `ADMIN_PASSWORD` from environment variables. The command is idempotent: if an admin already exists, it exits successfully.

The admin then logs in through the normal UI and completes onboarding like any other user.
