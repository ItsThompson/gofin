# gofin

An intentionally overengineered personal finance tracker built as a distributed monorepo with microservice architecture, micro-frontend composition, and an immutable expense ledger.

## What It Does

gofin lets users set monthly budgets with an essentials/desires/savings split, log expenses against those budgets, and track spending via a real-time dashboard. It serves a dual purpose: a functional personal finance tool and a learning platform for distributed systems patterns.

**Key capabilities:**

- Monthly budget periods with configurable E/D/S (essentials/desires/savings) split
- Immutable expense ledger backed by [immudb](https://immudb.io/) with bank-style corrections (no edits, only appends)
- Pro-rata expense spreading across multiple months
- Real-time dashboard with budget pacing, category gauges, and spending charts
- RBAC with admin identity assumption for support and debugging
- Full observability stack: Prometheus, Grafana, Alertmanager
- Micro-frontend architecture: independently built apps composed at runtime via Module Federation 2.0

## High-Level Architecture

```mermaid
graph TB
    subgraph Internet
        Browser[Browser]
        CFApp[Cloudflare Tunnel<br/>app traffic]
        CFGraf[Cloudflare Tunnel<br/>grafana traffic]
    end

    subgraph Node1[Node 1: Edge / DMZ]
        Shell[Shell App<br/><i>Node.js: SSR + API Proxy</i>]
        FinMFE[Finance Remote<br/><i>Dashboard, Expenses, Settings</i>]
        AdminMFE[Admin Remote<br/><i>User list, Identity Assumption</i>]
    end

    subgraph Node2[Node 2: Compute]
        GW[API Gateway<br/><i>Go / Gin</i>]
        Auth[Auth Service<br/><i>JWT, RBAC, Users</i>]
        Expense[Expense Service<br/><i>Immutable Ledger</i>]
        Finance[Finance Service<br/><i>Budgets, Tags, Pro-rata</i>]
    end

    subgraph Node3[Node 3: Data]
        PG[(PostgreSQL)]
        Immudb[(immudb)]
    end

    subgraph Node4[Node 4: Observability]
        AuthProxy[Grafana Auth Proxy]
        Grafana[Grafana]
        Prom[Prometheus]
        Alert[Alertmanager]
    end

    Browser -->|HTTPS| CFApp --> Shell
    Browser -->|HTTPS| CFGraf --> AuthProxy

    Shell -->|Module Federation| FinMFE
    Shell -->|Module Federation| AdminMFE
    Shell -->|/api/* proxy| GW

    GW -->|gRPC: ValidateToken| Auth
    GW -->|REST| Auth
    GW -->|REST| Expense
    GW -->|REST| Finance
    Finance -->|gRPC| Expense

    Auth --> PG
    Finance --> PG
    Expense --> Immudb

    AuthProxy -->|JWT check + proxy| Grafana
    Prom -->|scrape /metrics| GW
    Prom -->|scrape /metrics| Auth
    Prom -->|scrape /metrics| Expense
    Prom -->|scrape /metrics| Finance
    Prom --> Alert
    Grafana --> Prom
```

**Communication patterns:**

| Path | Protocol | Purpose |
|------|----------|---------|
| Browser → Shell | HTTPS (via Cloudflare) | SSR pages, static assets |
| Shell → API Gateway | HTTP reverse proxy | All `/api/*` requests |
| Gateway → Auth Service | gRPC | Token validation on every request |
| Gateway → Services | REST | Routed API calls with trusted user identity |
| Finance → Expense | gRPC | Ledger writes during pro-rata application |
| Services → Databases | SQL / native client | Data persistence |
| Prometheus → Services | HTTP `/metrics` | Metrics collection |

## Quick Start

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- [Just](https://github.com/casey/just) command runner
- [Go](https://go.dev/dl/) (see `services/*/go.mod` for version)
- [Node.js](https://nodejs.org/) (see `frontend/package.json` for version)

### One-Command Start

```bash
# Copy environment config
cp .env.example .env

# Start the full stack
just up

# Seed the admin user
just seed-admin
```

The app is available at `http://localhost:3000`. Grafana dashboards are at `http://localhost:3001`.

### Frontend Only (Mock Mode)

To work on the UI without running any backend services or Docker containers:

```bash
just dev-mock
```

This starts the shell app with [MSW](https://mswjs.io/) (Mock Service Worker) intercepting all `/api/*` requests in the browser. The mock layer returns realistic seed data: an admin user, budget period, expenses, tags, and dashboard aggregations. No Docker, no Go services, no databases required.

Mock mode is useful for:
- Iterating on UI components, layouts, and styling
- Testing frontend state management and error flows
- Onboarding new frontend contributors who don't need the full stack

Mock handlers live in `frontend/apps/shell/mocks/`. See the [Development Guide](docs/development.md) for details.

### Service-Focused Development

For iterating on a single backend service without rebuilding all containers:

```bash
# Start databases and monitoring only
just dev-infra

# In another terminal: start a specific Go service
just dev-service auth

# In another terminal: start the frontend with hot reload
just dev-frontend
```

### With Cloudflare Tunnels

To expose the app via a temporary public URL:

```bash
docker compose --profile tunnels up -d --build
```

## Available Commands

Run `just --list` for the full set of recipes. Key commands:

| Command | Description |
|---------|-------------|
| `just up` | Build and start the full stack |
| `just down` | Stop all containers |
| `just reset` | Stop and remove all volumes (full data reset) |
| `just dev-mock` | Start the frontend with mock API (no backend needed) |
| `just dev-infra` | Start only databases and monitoring |
| `just dev-service <name>` | Run a single Go service locally |
| `just dev-frontend` | Start the Turborepo frontend with hot reload |
| `just test` | Run all backend and frontend tests |
| `just seed-admin` | Create the initial admin user |

## Service Ports

See `docker-compose.yml` for the full port mapping. Default local access:

| Service | URL | Purpose |
|---------|-----|---------|
| App | `http://localhost:3000` | Frontend (SSR + API proxy) |
| Grafana | `http://localhost:3001` | Dashboards and visualization |
| Prometheus | `http://localhost:9090` | Metrics collection |

## Deployment

gofin is designed to run on a single VPS with Cloudflare Tunnels for ingress. No container registry or orchestrator required.

1. **Tunnel setup** (one-time, interactive): follow the [Tunnel Setup](docs/tunnel-setup.md) runbook to create named Cloudflare tunnels and DNS records
2. **Deploy** (automated, re-runnable): `scripts/deploy.sh <server-ip>` bootstraps a fresh VPS, copies tunnel credentials, builds and starts the stack. Designed to be run from a CI/CD pipeline on push to main.

See [`.env.example`](.env.example) for the full list of production environment variables.

## Further Reading

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | Node topology, service boundaries, data flow, and design decisions |
| [Auth System](docs/auth.md) | JWT lifecycle, RBAC model, identity assumption, Grafana auth proxy |
| [Data Model](docs/data-model.md) | Database schemas, service ownership, cross-service references |
| [API Reference](docs/api.md) | REST endpoint catalog with request/response shapes |
| [Development Guide](docs/development.md) | Local workflow, environment variables, code generation |
| [Testing](docs/testing.md) | Test strategy, patterns, and how to run each layer |
| [Monitoring](docs/monitoring.md) | Prometheus metrics, Grafana dashboards, alerting rules |
| [Tunnel Setup](docs/tunnel-setup.md) | One-time Cloudflare tunnel creation and DNS routing |
