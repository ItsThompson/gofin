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

| URL Prefix | Downstream Service | Auth Required |
|------------|-------------------|---------------|
| `/api/auth/*` | Auth Service | Varies (registration, login, and refresh are public) |
| `/api/expenses/*` | Expense Service | Yes |
| `/api/finance/*` | Finance Service | Yes |
| `/api/datarights/*` | Datarights Service | Yes |
| `/api/admin/*` | Auth Service | Yes (admin only) |

### Auth Middleware Behavior

For every incoming request (except public routes):

1. Extract the `gofin_access` cookie
2. Call Auth Service gRPC `ValidateToken`
3. On success: set `X-User-ID` and `X-User-Role` headers, forward to downstream service
4. On 401: return 401 to the frontend (frontend handles refresh)
5. Admin-only routes: additionally verify `X-User-Role == admin`, return 403 if not

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
| 403 | Authorization failure | Non-admin accessing admin routes, correcting an expense outside the current period |
| 404 | Resource not found | No budget period for the requested month |
| 409 | Conflict | Duplicate email/username, expense already corrected, tag in use |
| 429 | Rate limited | Data export requested within 30-day cooldown |

Error codes are defined in each service's handler layer. See the handler source files for the complete set.
