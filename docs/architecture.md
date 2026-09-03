# Architecture

## Overview

gofin is organized as a monorepo with four logical nodes, each representing a group of containers on a shared Docker network. Services communicate via REST (browser-facing) and gRPC (inter-service), with strict network isolation enforced by Docker bridge networks.

## Node Topology

```mermaid
graph LR
    subgraph Node1[Node 1: Edge / DMZ]
        direction TB
        MFE[Shell App<br/>SSR + Static + /api proxy]
        CF1[cloudflared-app]
    end

    subgraph Node2[Node 2: Compute]
        direction TB
        GW[API Gateway]
        AS[Auth Service]
        ES[Expense Service]
        FS[Finance Service]
        FX[FX Service]
        DR[Datarights Service]
    end

    subgraph Node3[Node 3: Data]
        direction TB
        PG[(PostgreSQL)]
        IM[(immudb)]
    end

    subgraph Node4[Node 4: Observability]
        direction TB
        PR[Prometheus]
        AL[Alertmanager]
        GR[Grafana]
        AP[Auth Proxy]
        CF2[cloudflared-grafana]
    end

    OER[Open Exchange Rates]

    CF1 <--> MFE
    MFE -->|edge-net| GW

    GW <-->|compute-net| AS
    GW <-->|compute-net| ES
    GW <-->|compute-net| FS
    GW <-->|compute-net| DR
    FS -->|gRPC| ES
    ES -->|gRPC| FX
    FS -->|gRPC| FX
    DR -->|gRPC| AS
    DR -->|gRPC| ES
    DR -->|gRPC| FS
    FX -->|HTTPS| OER

    AS -->|data-net| PG
    FS -->|data-net| PG
    DR -->|data-net| PG
    ES -->|data-net| IM

    PR -->|monitoring-net| GW
    PR -->|monitoring-net| AS
    PR -->|monitoring-net| ES
    PR -->|monitoring-net| FS
    PR -->|monitoring-net| FX
    PR -->|monitoring-net| DR
    PR --> AL
    GR --> PR
    CF2 --> AP --> GR
```

## Service Responsibilities

### Shell App (Node 1)

The shell is the deepest module in the frontend. It is the only app rendered directly: the finance and admin packages export page components that the shell imports from source and bundles at build time.

The shell owns:

- **SSR**: server-side rendering via React Router for fast initial page loads
- **API Proxy**: reverse proxies all `/api/*` requests to the API Gateway on Node 2, keeping the gateway off the public internet
- **Routing**: the complete route tree; the feature packages export page components, not routed applications
- **Auth Context**: a Zustand store shared across all feature packages, which resolve to one instance inside the shell's bundle
- **Layout**: persistent navbar, admin assumption banner, mobile navigation
- **Auth Guards**: route protection (unauthenticated, authenticated, onboarding, and a direct-admin guard that redirects operators off personal finance routes to `/admin`)

### API Gateway (Node 2)

A lightweight Go/Gin reverse proxy that validates auth and routes requests. A single centralized `AccessControl` middleware backed by the shared `services/access` route registry classifies every route into one of four access levels (Public / Authenticated / Personal / Admin) and enforces it:

1. Strips client-supplied identity headers so they cannot be spoofed
2. Resolves the route's access level from the shared registry, matching the concrete route gin will dispatch to (else the deny-by-default fail-safe: an unclassified path is refused with **403**, so a route or whole prefix is dead until it is added to the registry with an access level)
3. `Public` routes pass with no token read (e.g. `/api/auth/register`, `/api/auth/login`, `/api/auth/refresh`, `/health`, `/metrics`)
4. Otherwise verifies the `gofin_access` cookie via Auth Service gRPC `ValidateToken` (401 on failure) and injects `X-User-ID`, `X-User-Role`, and (when assuming) `X-Assumed-By`
5. Enforces the level's role: `Personal` routes require `role == "user"` and `Admin` routes require `role == "admin"` (403 on mismatch)

Because the personal finance routes are `Personal`, a direct admin is refused there while an assumed regular-user session passes.

The set of proxied prefixes and their downstream services is itself a single source of truth (`services/access.ProxyPrefixes`), from which the gateway derives its proxy wiring; a cross-check test pins it to the registry so every classified route sits under a proxied prefix and every proxied prefix has at least one classified route.

FX Service is deliberately absent from both `ProxyPrefixes` and the route registry. The gateway has no `/api/fx` prefix and never proxies conversion traffic: FX is reachable only over gRPC from Expense and Finance on the compute network.

### Auth Service (Node 2)

Owns user identity, credentials, and token lifecycle:

- User registration and login (bcrypt password hashing)
- JWT access/refresh token generation and validation
- Refresh token rotation with blacklist-based revocation
- Password change with bulk token invalidation (`tokens_revoked_at` timestamp)
- RBAC enforcement (user/admin roles; `admin` is operator-only and owns no finance data)
- Admin identity assumption and restoration

### Expense Service (Node 2)

Owns the immutable expense ledger backed by immudb:

- Append-only expense writes (no updates, no deletes)
- Correction mechanics: new entries supersede originals, forming a correction chain
- Materialized view: filters to `status=active` so downstream consumers see only current truth
- Pro-rata installment tracking (group ID, index, total)
- Money snapshots: every row stores transaction money, reporting money, and the conversion facts (rate, source, timestamp) that connect them
- Period context: public writes resolve the target period's reporting currency from Finance over gRPC before any ledger write
- FX consumption: foreign-currency writes call FX Service over gRPC; same-currency writes use an identity snapshot and bypass FX

### Finance Service (Node 2)

The business logic hub, orchestrating budgets, tags, and dashboard data:

- Budget period lifecycle (creation, missed-month backfill, E/D/S allocation) with an immutable `reportingCurrency` per period
- Default settings that seed only future period creation; changing defaults never mutates existing periods
- Read-only period context API for Expense: period ID, reporting currency, and lock state
- Pro-rata scheduling: stores future installment schedules with captured FX snapshots, creates ledger entries via Expense Service gRPC when periods are created
- Tag CRUD with lazy-seeded defaults
- Dashboard aggregation: period summary, pacing, category breakdowns, cumulative spend, historical comparison, monthly financial health score (persisted per closed month), and its multi-month health-score trend, all computed in the period's reporting currency

### FX Service (Node 2)

An internal-only conversion service. It owns no user ledger data and exposes no browser-facing REST route. Expense and Finance call it over gRPC on the compute network:

- `ConvertAmount`: live source-to-target conversion using an in-memory Open Exchange Rates snapshot cache (default one hour, `FX_CACHE_MAX_AGE`)
- `ConvertWithSnapshot`: derives a source-to-target rate from a previously captured snapshot without calling the provider
- `CaptureRateSnapshot`: returns a full USD-based provider rate map that Finance stores on pro-rata schedules
- Decimal conversion with half-away-from-zero rounding to the target currency's minor units

### Datarights Service (Node 2)

Owns GDPR data export and user data portability (Article 20 compliance):

- Async export job lifecycle (pending → running → completed/failed)
- Data collection from upstream services via gRPC (Auth, Expense, Finance)
- CSV generation per data category with a provider-based extension model
- ZIP assembly and email delivery via Resend
- 30-day rate limiting between successful exports
- In-progress job deduplication (idempotent POST)
- Startup recovery: re-submits non-terminal jobs found in the database
- Bounded concurrency pool with configurable max concurrent exports
- Currency-aware CSV rows: expense rows carry transaction and reporting money columns; period rows carry the reporting currency; amounts format with the shared catalog's per-currency precision
- Admin-only data deletion: async deletion jobs that remove or anonymize a user's data across services (auth, finance, expense ledger), with password verification, self-deletion prevention, and idempotent deduplication

The service coordinates data collection from all other compute services but owns no user data itself: it only stores job metadata (status, timestamps, file size) in its own PostgreSQL schema.

### Databases (Node 3)

- **PostgreSQL**: single instance with three schemas (`auth`, `finance`, `datarights`), each accessed by its owning service via separate credentials. Logical isolation with the option to split later.
- **immudb**: append-only storage for expense ledger entries. SQL interface for queries, native Go client for writes.

FX Service owns no database. Its provider snapshot cache lives in process memory (default one hour, `FX_CACHE_MAX_AGE`); conversion facts are persisted on ledger rows and pro-rata schedules, not in FX.

### Observability Stack (Node 4)

- **Prometheus**: scrapes `/metrics` from each Go service plus cadvisor and node-exporter over the monitoring network
- **Alertmanager**: routes alerts based on Prometheus rules
- **Grafana**: pre-provisioned dashboards for system overview, per-service metrics, and database health
- **Auth Proxy**: a small Go service that validates JWTs and checks admin role before proxying to Grafana

## Network Isolation

Four Docker bridge networks enforce traffic boundaries:

| Network | Connects | Traffic |
|---------|----------|---------|
| `edge-net` | MFE, cloudflared-app, API Gateway | Public-facing traffic only |
| `compute-net` | API Gateway, Auth, Expense, Finance, FX, Datarights | Internal service communication |
| `data-net` | Auth, Finance, Datarights, Expense, PostgreSQL, immudb | Database access only; FX is absent because it owns no user ledger data |
| `monitoring-net` | Prometheus, Grafana, Alertmanager, auth proxy, all Go services, cadvisor, node-exporter | Observability plane |

Databases are never reachable from `edge-net`. The browser only communicates with the shell app, which proxies everything else.

FX Service joins only `compute-net` and `monitoring-net`. It has no `edge-net` membership and no browser-facing REST route, so the browser can never reach it, and the gateway has no proxy prefix for it.

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Single-origin reverse proxy (Node 1 → Node 2) | Avoids cross-origin cookie issues with trycloudflare.com public suffix domains |
| gRPC inter-service, REST browser-facing | gRPC gives binary protocol and generated types between Go services; REST is browser-native |
| immudb for expense ledger | True append-only storage with cryptographic verification; corrections modeled as new entries |
| sqlc for PostgreSQL services | SQL-first, type-safe generated Go code without ORM magic |
| Build-time composition via npm workspace source imports | One browser bundle served from one origin: no runtime remote loading to coordinate, and no shared-singleton config to keep in sync |
| JWT in httpOnly cookies | XSS-proof token storage; automatic inclusion by the browser; works naturally with the reverse proxy |
| Zustand for shared client state | Lightweight state management with no provider tree; a single instance because every feature package is bundled together |
| Finance service orchestrates pro-rata | Simpler than a saga for a 5-user system; finance calls expense directly, rolls back on failure |
| Reporting currency belongs to the budget period | Budgets and dashboard totals are period-scoped; user currency and default settings can change over time |
| Reporting currency is immutable after period creation | Prevents existing expenses, pro-rata schedules, and dashboard totals from changing meaning |
| Internal FX Service over gRPC only | Conversion is a backend responsibility that needs provider credentials; a browser route would expose them |
| Pro-rata captures a full provider snapshot at creation | Future installments preserve the user's schedule intent and never re-rate against live provider rates |
| Just command runner | Polyglot support (Go, Node, Docker) with readable syntax; better than Makefiles for this use case |

## Data Flow: Logging an Expense

```mermaid
sequenceDiagram
    participant B as Browser
    participant S as Shell (Node 1)
    participant G as API Gateway
    participant A as Auth Service
    participant E as Expense Service
    participant F as Finance Service
    participant FX as FX Service
    participant I as immudb

    B->>S: POST /api/expenses
    S->>G: proxy /api/*
    G->>A: gRPC: ValidateToken
    A-->>G: user_id, role
    G->>E: REST: POST /expenses<br/>(+ X-User-ID header)
    E->>F: gRPC: GetPeriodContext
    F-->>E: reportingCurrency
    alt foreign currency
        E->>FX: gRPC: ConvertAmount
        FX-->>E: converted amount + rate
    end
    E->>I: SQL INSERT (money snapshot)
    I-->>E: tx_id
    E-->>G: 201 Created
    G-->>S: 201 Created
    S-->>B: expense data
```

## Data Flow: Pro-rata Expense

```mermaid
sequenceDiagram
    participant B as Browser
    participant G as API Gateway
    participant F as Finance Service
    participant FX as FX Service
    participant E as Expense Service
    participant I as immudb
    participant P as PostgreSQL

    B->>G: POST /api/finance/prorata
    G->>F: route to Finance
    F->>FX: gRPC: CaptureRateSnapshot
    FX-->>F: captured snapshot
    F->>E: gRPC: CreateProRataInstallment<br/>(trusted context + snapshot)
    E->>I: INSERT first installment
    I-->>E: entry_id
    E-->>F: expense
    F->>P: INSERT pro_rata_schedules<br/>(future months + snapshot)
    F-->>G: 201 + expense + schedules
    G-->>B: response
```

## Data Flow: New Month Period Creation

When a new month is accessed and pending pro-rata installments exist:

```mermaid
sequenceDiagram
    participant F as Finance Service
    participant P as PostgreSQL
    participant E as Expense Service
    participant I as immudb

    F->>P: INSERT budget_period
    F->>P: SELECT pending pro_rata_schedules
    P-->>F: [schedule_1, schedule_2]
    loop Each pending schedule
        F->>E: gRPC: CreateProRataInstallment<br/>(trusted context + stored snapshot)
        E->>I: INSERT
        I-->>E: entry_id
        E-->>F: expense
        F->>P: UPDATE schedule status=applied
    end
```

## Data Flow: Data Export

When a user requests a data export, the datarights service collects data from all upstream services concurrently in a background goroutine (an `errgroup` fan-out, with ZIP assembly as the fan-in barrier) and delivers the result via email:

```mermaid
sequenceDiagram
    participant B as Browser
    participant G as API Gateway
    participant DR as Datarights Service
    participant P as PostgreSQL
    participant A as Auth Service
    participant E as Expense Service
    participant F as Finance Service
    participant R as Resend (Email)

    B->>G: POST /api/datarights/exports
    G->>DR: route to Datarights
    DR->>P: INSERT export_job (status=pending)
    DR-->>G: 202 Accepted (job metadata)
    G-->>B: job response

    Note over DR: Async goroutine starts
    DR->>P: UPDATE status=running
    par Providers collect concurrently (errgroup fan-out)
        DR->>A: gRPC: GetUser (profile + email)
        A-->>DR: user profile
    and
        DR->>E: gRPC: StreamAllUserExpenses (server-streaming)
        E-->>DR: stream expense rows (Recv loop, written incrementally)
    and
        DR->>F: gRPC: GetAllUserData (tags, periods, defaults)
        F-->>DR: finance data
    end
    Note over DR: Fan-in barrier: all providers complete → assemble ZIP
    DR->>DR: Generate CSVs + ZIP
    DR->>R: Send email with ZIP attachment
    R-->>DR: delivery confirmed
    DR->>P: UPDATE status=completed
```
