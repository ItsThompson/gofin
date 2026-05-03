# API Reference

## Overview

The API Gateway (Node 2) exposes REST endpoints to the frontend. The Shell App (Node 1) reverse-proxies all `/api/*` requests to the gateway. The gateway validates auth via gRPC and routes to the appropriate downstream service.

Canonical sources for endpoint definitions:

- Gateway routing: `services/gateway/internal/router/`
- Auth handlers: `services/auth/internal/handler/`
- Expense handlers: `services/expense/internal/handler/`
- Finance handlers: `services/finance/internal/handler/`
- gRPC definitions: `services/*/proto/*.proto`

## Gateway Routing

| URL Prefix | Downstream Service | Auth Required |
|------------|-------------------|---------------|
| `/api/auth/*` | Auth Service | Varies (registration, login, and refresh are public) |
| `/api/expenses/*` | Expense Service | Yes |
| `/api/finance/*` | Finance Service | Yes |
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

CRUD operations on the immutable expense ledger: creating expenses, listing materialized (active-only) expenses for a period, viewing single expenses, creating corrections, viewing correction history, and querying pro-rata groups.

List endpoints support pagination, sorting, and filtering via query parameters.

### Finance (`/api/finance/*`)

Budget period lifecycle (get current, create, update, list history), default settings management, onboarding setup, tag CRUD, pro-rata expense creation and scheduling, and all dashboard aggregation endpoints (period summary, spending by tag, cumulative spend, historical comparison).

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

Error codes are defined in each service's handler layer. See the handler source files for the complete set.
