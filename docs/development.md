# Development Guide

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Docker](https://docs.docker.com/get-docker/) | 24+ | Container runtime |
| [Just](https://github.com/casey/just) | 1.0+ | Command runner |
| [Go](https://go.dev/dl/) | 1.22+ | Backend services |
| [Node.js](https://nodejs.org/) | 20+ | Frontend and tooling |
| [protoc](https://grpc.io/docs/protoc-installation/) | 3.x | Protobuf compilation |
| [golang-migrate](https://github.com/golang-migrate/migrate) | 4.x | Database migrations |
| [sqlc](https://sqlc.dev/) | 1.x | Type-safe SQL code generation |

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

### Shared

| Variable | Description | Default |
|----------|-------------|---------|
| `JWT_SECRET` | Shared JWT signing key | (required) |
| `LOG_LEVEL` | Structured logging level | `info` |
| `ENVIRONMENT` | Runtime environment | `development` |

### Auth Service

| Variable | Description | Default |
|----------|-------------|---------|
| `AUTH_DB_URL` | PostgreSQL connection string (auth schema) | (required) |
| `BCRYPT_COST` | Password hashing cost factor | `12` (dev override: `4`) |
| `ADMIN_USERNAME` | Admin seed username | (seed-admin only) |
| `ADMIN_EMAIL` | Admin seed email | (seed-admin only) |
| `ADMIN_PASSWORD` | Admin seed password | (seed-admin only) |

### Finance Service

| Variable | Description | Default |
|----------|-------------|---------|
| `FINANCE_DB_URL` | PostgreSQL connection string (finance schema) | (required) |
| `EXPENSE_SERVICE_ADDR` | gRPC address of expense service | (required) |

### Expense Service

| Variable | Description | Default |
|----------|-------------|---------|
| `IMMUDB_ADDR` | immudb connection address | (required) |
| `IMMUDB_USERNAME` | immudb credentials | (required) |
| `IMMUDB_PASSWORD` | immudb credentials | (required) |

### API Gateway

| Variable | Description | Default |
|----------|-------------|---------|
| `AUTH_SERVICE_ADDR` | Auth service gRPC address | (required) |
| `AUTH_SERVICE_REST` | Auth service REST address | (required) |
| `EXPENSE_SERVICE_REST` | Expense service REST address | (required) |
| `FINANCE_SERVICE_REST` | Finance service REST address | (required) |

### MFE (Shell)

| Variable | Description | Default |
|----------|-------------|---------|
| `API_GATEWAY_URL` | Internal URL for API proxy target | (required) |
| `COOKIE_SECURE` | Require HTTPS for cookies | `false` (dev), `true` (prod) |

### Grafana Auth Proxy

| Variable | Description | Default |
|----------|-------------|---------|
| `JWT_SECRET` | Same signing key as auth service | (required) |
| `GRAFANA_URL` | Internal Grafana URL | (required) |

## Admin Bootstrap

The first admin user must be created before the admin panel, identity assumption, or Grafana access can function:

```bash
just seed-admin
```

This runs the auth service's `seed-admin` CLI subcommand, which reads `ADMIN_USERNAME`, `ADMIN_EMAIL`, and `ADMIN_PASSWORD` from environment variables. The command is idempotent: if an admin already exists, it exits successfully.

The admin then logs in through the normal UI and completes onboarding like any other user.
