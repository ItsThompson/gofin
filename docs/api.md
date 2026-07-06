# API Reference

## Overview

The API Gateway (Node 2) exposes REST endpoints to the frontend. The Shell App (Node 1) reverse-proxies all `/api/*` requests to the gateway. The gateway validates auth via gRPC and routes to the appropriate downstream service.

Canonical sources for endpoint definitions:

- Gateway routing: `services/gateway/internal/router/`
- Auth handlers: `services/auth/internal/handler/`
- Expense handlers: `services/expense/internal/handler/`
- Finance handlers: `services/finance/internal/handler/`
- Datarights handlers: `services/datarights/internal/handler/`
- gRPC definitions: `services/*/proto/*.proto`

## Gateway Routing

The gateway applies one centralized access-control policy: every route resolves to exactly one of four access levels, enforced by the single `AccessControl` middleware. Resolution matches the concrete route gin will dispatch to; a path that no registry entry classifies falls to the deny-by-default fail-safe and is refused with **403** (an unclassified `/api/*` path is not a real route).

| Level | Meaning | Token required | Role check |
|-------|---------|----------------|------------|
| `Public` | Reachable with no token | No | None |
| `Authenticated` | Any valid token | Yes | None |
| `Personal` | Valid token acting as a regular user | Yes | `role == "user"` |
| `Admin` | Valid token acting as an operator | Yes | `role == "admin"` |

### Route Groups

This prefix→service mapping is the single source of truth in `services/access.ProxyPrefixes`, from which the gateway derives its reverse-proxy wiring (a cross-check test pins it against the route registry).

| URL Prefix | Downstream Service | Access Level |
|------------|-------------------|--------------|
| `/api/auth/*` | Auth Service | Mixed (see route-level table): Public login/register/refresh, Personal onboarding-complete, Admin assume, Authenticated for the rest |
| `/api/expenses/*` | Expense Service | Personal |
| `/api/finance/*` | Finance Service | Personal |
| `/api/datarights/*` | Datarights Service | Mixed: `exports*` is Personal, `deletions*` is Admin |
| `/api/admin/*` | Auth Service | Admin |

### Route-Level Access

The canonical route registry (`services/access/registry.go`) classifies each route, and the gateway resolves each request to the concrete route gin dispatches. Access levels by route:

| Access | Method | Path |
|--------|--------|------|
| `Public` | POST | `/api/auth/register` |
| `Public` | POST | `/api/auth/login` |
| `Public` | POST | `/api/auth/refresh` |
| `Public` | GET | `/health` |
| `Public` | GET | `/metrics` |
| `Admin` | GET | `/api/admin/users` |
| `Admin` | POST | `/api/auth/assume` |
| `Admin` | (any) | `/api/datarights/deletions/*` |
| `Personal` | POST | `/api/auth/onboarding-complete` |
| `Personal` | (any) | `/api/finance/*` |
| `Personal` | (any) | `/api/expenses/*` |
| `Personal` | (any) | `/api/datarights/exports/*` |
| `Authenticated` | (any) | `/api/auth/me`, `/api/auth/me/password` |
| `Authenticated` | POST | `/api/auth/logout` |
| `Authenticated` | POST | `/api/auth/restore` |
| `Deny` (default) | (any) | *(any `/api/*` path with no registry entry, e.g. bare `/api/auth`); refused with **403** before any token read* |

The `Personal` routes are `/api/finance/*`, `/api/expenses/*`, `/api/datarights/exports*`, and `POST /api/auth/onboarding-complete`. A direct admin (`role=admin`) receives **403** on all of them; an assumed session carries `role=user` (with an `assumedBy` claim) and passes. `POST /api/auth/restore` is `Authenticated`, so an assumed session can always restore.

### Auth Middleware Behavior

A single `AccessControl` middleware gates every request. For each one it:

1. Strips client-supplied identity headers (`X-User-ID`, `X-User-Role`, `X-Assumed-By`) so they can never be spoofed
2. Resolves the route's access level from the registry, matching the concrete route gin dispatches; a path with no registry entry falls to the deny-by-default fail-safe (**403**)
3. `Public` routes short-circuit here with no token read; a `Deny` (unclassified) path short-circuits with a **403**, also with no token read
4. Otherwise extracts the `gofin_access` cookie and calls Auth Service gRPC `ValidateToken` (401 on a missing cookie or validation failure; the frontend then handles refresh)
5. Sets `X-User-ID` and `X-User-Role` (and `X-Assumed-By` when the session is assumed) for the downstream service
6. Enforces the level's role: `Authenticated` passes any valid token; `Personal` requires `role == "user"`; `Admin` requires `role == "admin"`. A role mismatch returns 403

Because `Personal` requires `role == "user"`, a direct admin is refused (403) on the personal finance APIs, while an assumed `role=user` session passes.

## Endpoint Groups

### Auth (`/api/auth/*`)

Handles user registration, login, logout, token refresh, profile management, password changes, and admin identity assumption/restoration. The onboarding-complete endpoint marks a user as onboarded and updates their display currency.

### Admin (`/api/admin/*`)

Admin-only user management endpoints (e.g., listing all registered users).

### Expenses (`/api/expenses/*`)

CRUD operations on the immutable expense ledger: creating expenses, listing materialized (active-only) expenses for a period, viewing single expenses, creating corrections, viewing correction history, querying pro-rata groups, and retrieving ranked autocomplete suggestions.

List endpoints support pagination, sorting, and filtering via query parameters.

#### GET `/api/expenses/suggestions`

Returns ranked active historical expense suggestions for the authenticated user.

Query parameters:

| Parameter | Default | Rules |
|-----------|---------|-------|
| `page` | `1` | One-based positive integer |
| `pageSize` | `50` | Positive integer, maximum `100` |

Response fields:

| Field | Description |
|-------|-------------|
| `data` | Array of suggestion records sorted by `frecencyScore DESC`, `lastUsedAt DESC`, then `name ASC` |
| `total` | Total ranked suggestions before pagination |
| `page` | Current one-based page |
| `pageSize` | Applied page size |
| `hasMore` | `true` when another page exists |

Suggestion record fields:

| Field | Description |
|-------|-------------|
| `name` | Exact active expense name used as the aggregation key |
| `amount` | Latest active amount for the name in minor units |
| `currency` | Currency from the latest active expense row |
| `expenseType` | Latest active expense type: `essentials`, `desires`, or `savings` |
| `tagId` | Latest active tag ID |
| `frequency` | Usage count after pro-rata group de-duplication |
| `lastUsedAt` | Latest active usage timestamp |
| `recencyBucket` | `today`, `last_7_days`, `last_30_days`, or `older` |
| `frecencyScore` | Frequency multiplied by the recency bucket weight |

Pagination rules: sorting is applied before pagination. Corrected rows are excluded from ranking, frequency, and latest-value selection. Active rows in the same non-empty pro-rata group count as one frequency event.

Error codes:

| HTTP Status | Code | Condition |
|-------------|------|-----------|
| 400 | `validation_error` | Invalid `page`, invalid `pageSize`, or `pageSize > 100` |
| 401 | `unauthorized` | Missing or invalid authenticated user |
| 500 | `internal_server_error` | Repository or unexpected service failure |

### Finance (`/api/finance/*`)

Budget period lifecycle (get current, create, update, list history), default settings management, onboarding setup, tag CRUD, pro-rata expense creation and scheduling, and all dashboard aggregation endpoints (period summary, spending by tag, cumulative spend, historical comparison).

### Datarights (`/api/datarights/*`)

GDPR data export: creating async export jobs (POST returns 202, runs in background), listing export history with pagination, and retrieving individual job status. The POST endpoint is idempotent (returns existing in-progress job) and rate-limited to one successful export per 30 days (429 with `retryAfter` timestamp). Completed exports are delivered via email as a ZIP of CSV files.

## Response Contracts

### Paginated Response

All list endpoints that support pagination return:

```json
{
  "data": [],
  "total": 100,
  "page": 1,
  "pageSize": 20,
  "hasMore": true
}
```

### Error Response

All API errors follow a consistent shape:

```json
{
  "code": "VALIDATION_ERROR",
  "message": "Human-readable message",
  "fields": {
    "email": "Email already in use"
  }
}
```

- `code`: machine-readable error identifier (e.g., `VALIDATION_ERROR`, `UNAUTHORIZED`, `NOT_FOUND`)
- `message`: safe to display to the user (no stack traces or internal details)
- `fields`: optional, present for field-level validation errors

### Error Categories

| HTTP Status | Meaning | Example Scenarios |
|-------------|---------|-------------------|
| 400 | Validation failure | Missing fields, E/D/S percentages not summing to 100%, weak password |
| 401 | Authentication failure | Invalid credentials, expired token, invalid token |
| 403 | Authorization failure | Non-admin accessing admin routes, direct admin accessing personal finance routes, correcting an expense outside the current period |
| 404 | Resource not found | No budget period for the requested month |
| 409 | Conflict | Duplicate email/username, expense already corrected, tag in use |
| 429 | Rate limited | Data export requested within 30-day cooldown |

Error codes are defined in each service's handler layer. See the handler source files for the complete set.
