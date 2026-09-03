# API Reference

## Overview

The API Gateway (Node 2) exposes REST endpoints to the frontend. The Shell App (Node 1) reverse-proxies all `/api/*` requests to the gateway. The gateway validates auth via gRPC and routes to the appropriate downstream service.

FX Service is internal-only: it exposes no REST endpoints and has no `/api/fx` gateway prefix. Expense and Finance call it directly over gRPC.

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
| `/api/finance/*` | Finance Service | Mixed: `GET /api/finance/currencies` is Authenticated, the rest is Personal |
| `/api/datarights/*` | Datarights Service | Mixed: `exports*` is Personal, `deletions*` is Admin |
| `/api/admin/*` | Auth Service | Admin |

FX Service has no entry in this table and no proxy prefix. Conversion traffic stays internal gRPC between Expense, Finance, and FX.

### Route-Level Access

The canonical route registry (`services/access/registry.go`) classifies each route, and the gateway resolves each request to the concrete route gin dispatches. Access levels by route:

| Access | Method | Path |
|--------|--------|------|
| `Public` | POST | `/api/auth/register` |
| `Public` | POST | `/api/auth/login` |
| `Public` | POST | `/api/auth/refresh` |
| `Public` | GET | `/health` |
| `Public` | GET | `/readyz` |
| `Public` | GET | `/metrics` |
| `Admin` | GET | `/api/admin/users` |
| `Admin` | POST | `/api/auth/assume` |
| `Admin` | (any) | `/api/datarights/deletions/*` |
| `Personal` | POST | `/api/auth/onboarding-complete` |
| `Personal` | (any) | `/api/finance/*` except `GET /api/finance/currencies` |
| `Personal` | (any) | `/api/expenses/*` |
| `Personal` | (any) | `/api/datarights/exports/*` |
| `Authenticated` | (any) | `/api/auth/me`, `/api/auth/me/password` |
| `Authenticated` | GET | `/api/finance/currencies` |
| `Authenticated` | POST | `/api/auth/logout` |
| `Authenticated` | POST | `/api/auth/restore` |
| `Deny` (default) | (any) | *(any `/api/*` path with no registry entry, e.g. bare `/api/auth`); refused with **403** before any token read* |

The `Personal` routes are `/api/finance/*` (except `GET /api/finance/currencies`), `/api/expenses/*`, `/api/datarights/exports*`, and `POST /api/auth/onboarding-complete`. A direct admin (`role=admin`) receives **403** on all of them; an assumed session carries `role=user` (with an `assumedBy` claim) and passes. `POST /api/auth/restore` and `GET /api/finance/currencies` are `Authenticated`, so an assumed session can always restore and a direct admin can read the currency catalog.

### Auth Middleware Behavior

A single `AccessControl` middleware gates every request. For each one it:

1. Strips client-supplied identity headers (`X-User-ID`, `X-User-Role`, `X-Assumed-By`) so they can never be spoofed
2. Resolves the route's access level from the registry, matching the concrete route gin dispatches; a path with no registry entry falls to the deny-by-default fail-safe (**403**)
3. `Public` routes short-circuit here with no token read; a `Deny` (unclassified) path short-circuits with a **403**, also with no token read
4. Otherwise extracts the `gofin_access` cookie and calls Auth Service gRPC `ValidateToken` (401 on a missing cookie or validation failure; the frontend then handles refresh)
5. Sets `X-User-ID` and `X-User-Role` (and `X-Assumed-By` when the session is assumed) for the downstream service
6. Enforces the level's role: `Authenticated` passes any valid token; `Personal` requires `role == "user"`; `Admin` requires `role == "admin"`. A role mismatch returns 403

Because `Personal` requires `role == "user"`, a direct admin is refused (403) on the Personal finance routes, while an assumed `role=user` session passes. The currency catalog (`GET /api/finance/currencies`) is `Authenticated`, so a direct admin can read it.

## Endpoint Groups

### Auth (`/api/auth/*`)

Handles user registration, login, logout, token refresh, profile management, password changes, and admin identity assumption/restoration. The onboarding-complete endpoint marks a user as onboarded and updates their display currency.

### Admin (`/api/admin/*`)

Admin-only user management endpoints (e.g., listing all registered users).

### Expenses (`/api/expenses/*`)

CRUD operations on the immutable expense ledger: creating expenses, listing materialized (active-only) expenses for a period, viewing single expenses, creating corrections, viewing correction history, querying pro-rata groups, and retrieving ranked autocomplete suggestions.

List endpoints support pagination (`page`, `pageSize`) and period scoping (`year`, `month`) where applicable; sorting and filtering are client-side.

Create and correction requests accept `amount` (minor units) and optional `transactionCurrency`. Saved expense responses carry the money snapshot fields (`transactionAmount`, `transactionCurrency`, `reportingAmount`, `reportingCurrency`, `exchangeRate`, `exchangeRateSource`, `exchangeRateTimestamp`, optional `exchangeRateExpiresAt`). A create for a month with no budget period returns `404 PERIOD_NOT_FOUND` and writes no ledger row. See [Multi-Currency Fields](#multi-currency-fields).

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
| `transactionAmount` | Canonical latest active amount for the name in minor units |
| `transactionCurrency` | Canonical currency from the latest active expense row |
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

Budget period lifecycle (get current, create, update, list history), default settings management, onboarding setup, tag CRUD, pro-rata expense creation and scheduling, and dashboard aggregation endpoints (period summary, spending by tag, cumulative spend, historical comparison, a monthly financial health score, and its multi-month trend). The health score combines four sub-scores: savings achievement, budget adherence, allocation balance, and spending stability. Spending stability needs three or more closed months of history (below that the card shows "building baseline"). Closed months are persisted with the formula version that produced them and are recomputed when that version changes; the current month is computed live and marked provisional. A configure-budget prompt is returned when no budget is set.

Budget periods carry an immutable `reportingCurrency`. Create requests require `reportingCurrency` and `budgetAmount`; update requests reject `reportingCurrency` with `400 REPORTING_CURRENCY_IMMUTABLE`. Default settings currency applies to future periods only. Pro-rata create requires `periodYear` and `periodMonth`. See [Multi-Currency Fields](#multi-currency-fields).

#### GET `/api/finance/currencies`

Returns the supported-currency catalog: code, symbol, name, and minor-unit digits for each currency. The catalog is static reference data owned by `services/shared/currency/` and served by Finance; the same catalog is exposed to internal callers as the `ListSupportedCurrencies` gRPC method. The frontend fetches this endpoint at runtime instead of bundling a currency list.

### Datarights (`/api/datarights/*`)

GDPR data export: creating async export jobs (POST returns 202, runs in background), listing export history with pagination, and retrieving individual job status. The POST endpoint is idempotent (returns existing in-progress job) and rate-limited to one successful export per 30 days (429 with `retryAfter` timestamp). Completed exports are delivered via email as a ZIP of CSV files.

Admin-only GDPR data deletion (`/api/datarights/deletions/*`): creating async deletion jobs that remove or anonymize a user's data across services. The POST endpoint verifies the target user's password, refuses self-deletion, and is idempotent (returns an existing in-progress job).

## Multi-Currency Fields

### Reporting and Transaction Currency

- `reportingCurrency`: the immutable currency of a budget period. Every period-scoped response (period, dashboard summary, history row, health score) carries it; amounts beside it are minor units of that currency.
- `transactionCurrency`: the original charged currency on an expense, correction, suggestion, or pro-rata request; amounts beside it are minor units of that currency.
- `transactionAmount` / `reportingAmount`: minor-unit amounts on enriched expense responses. `reportingAmount` is the expense's contribution to the period budget.
- `exchangeRate`, `exchangeRateSource`, `exchangeRateTimestamp`, `exchangeRateExpiresAt`: conversion facts stored with each expense.

### Transaction Currency Defaults

`transactionCurrency` is optional. On expense and pro-rata creation, an omitted value defaults to the period `reportingCurrency` and the service logs a `transaction_currency_defaulted` event. On correction, an omitted value preserves the original expense's `transactionCurrency` and logs a `correction_currency_preserved` event. Responses always name the currency as `transactionCurrency`; there is no alias or mirror field.

### Period Context Behavior

Public expense creation and correction resolve the target period's context through Finance before any ledger write. The context returns `periodId`, `reportingCurrency`, and `isLocked`. A month with no period returns `404 PERIOD_NOT_FOUND`, and no ledger row is written.

### Correction Currency Behavior

Corrections always convert against the original expense's period `reportingCurrency`. The correction request may send `transactionCurrency`. If it is absent, the active expense's transaction currency is preserved.

### Pro-Rata Period Fields

`POST /api/finance/prorata` requires `periodYear` and `periodMonth` for the schedule creation period. `transactionCurrency` is optional and defaults to the creation period `reportingCurrency`. Finance captures one full provider snapshot before writing the first installment or any future schedule row.

### FX Error Responses

FX is an internal gRPC service. Expense and Finance map its failures to safe REST responses:

| Condition | HTTP | API code |
|-----------|------|----------|
| Unsupported transaction or reporting currency | 400 | `UNSUPPORTED_CURRENCY` |
| Invalid amount or precision | 400 | `VALIDATION_ERROR` |
| Conversion unavailable (provider down, no fresh cache) | 503 | `CONVERSION_UNAVAILABLE` |
| Missing target period | 404 | `PERIOD_NOT_FOUND` |
| Immutable reporting currency update attempt | 400 | `REPORTING_CURRENCY_IMMUTABLE` |
| Captured snapshot missing a needed currency | 409 | `SNAPSHOT_CURRENCY_MISSING` |

FX failures never write partial ledger rows.

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

- `code`: machine-readable error identifier (e.g., `VALIDATION_ERROR`, `UNAUTHORIZED`, `NOT_FOUND`, `CONVERSION_UNAVAILABLE`)
- `message`: safe to display to the user (no stack traces or internal details)
- `fields`: optional, present for field-level validation errors

### Error Categories

| HTTP Status | Meaning | Example Scenarios |
|-------------|---------|-------------------|
| 400 | Validation failure | Missing fields, E/D/S percentages not summing to 100%, weak password, unsupported currency, immutable reporting currency update |
| 401 | Authentication failure | Invalid credentials, expired token, invalid token |
| 403 | Authorization failure | Non-admin accessing admin routes, direct admin accessing personal finance routes, correcting an expense outside the current period |
| 404 | Resource not found | No budget period for the requested month |
| 409 | Conflict | Duplicate email/username, expense already corrected, tag in use, pro-rata snapshot missing a needed currency |
| 429 | Rate limited | Data export requested within 30-day cooldown |
| 503 | Conversion unavailable | FX provider down with no fresh cache; no ledger row is written |

Error codes are defined in each service's handler layer. See the handler source files for the complete set.
