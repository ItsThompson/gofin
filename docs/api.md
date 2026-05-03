# API Reference

The API Gateway (Node 2) exposes REST endpoints to the frontend. The Shell App (Node 1) reverse-proxies all `/api/*` requests to the gateway. The gateway validates auth via gRPC and routes to the appropriate downstream service.

## Gateway Routing

| URL Prefix | Downstream Service | Auth Required |
|------------|-------------------|---------------|
| `/api/auth/*` | Auth Service | Varies (register/login/refresh are public) |
| `/api/expenses/*` | Expense Service | Yes |
| `/api/finance/*` | Finance Service | Yes |
| `/api/admin/*` | Auth Service | Yes (admin only) |

## Auth Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/auth/register` | No | Create account. Body: `{ username, email, password }`. Sets auth cookies. |
| `POST` | `/api/auth/login` | No | Authenticate. Body: `{ email, password }`. Sets auth cookies. |
| `POST` | `/api/auth/logout` | Yes | Blacklist refresh token, clear cookies. |
| `POST` | `/api/auth/refresh` | No* | Rotate tokens. Uses refresh cookie (not access token). |
| `GET` | `/api/auth/me` | Yes | Get current user profile. |
| `PUT` | `/api/auth/me` | Yes | Update profile. Body: `{ username?, email? }`. |
| `POST` | `/api/auth/me/password` | Yes | Change password. Body: `{ currentPassword, newPassword }`. Revokes old tokens. |
| `POST` | `/api/auth/assume` | Admin | Assume user identity. Body: `{ userId }`. |
| `POST` | `/api/auth/restore` | Yes | Restore admin's original session. |
| `POST` | `/api/auth/onboarding-complete` | Yes | Mark onboarding done. Body: `{ currency }`. |

## Admin Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/admin/users` | Admin | List all registered users. |

## Expense Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/expenses` | Yes | Create expense. Body: `{ name, amount, currency, expenseType, tagId, expenseDate, periodYear, periodMonth }`. Pro-rata fields optional. |
| `GET` | `/api/expenses` | Yes | List materialized expenses. Query: `year, month, page?, pageSize?, sort?, filter?`. |
| `GET` | `/api/expenses/:id` | Yes | Get single expense. |
| `POST` | `/api/expenses/:id/correct` | Yes | Create correction. Body: `{ name?, amount?, expenseType?, tagId?, expenseDate? }`. |
| `GET` | `/api/expenses/:id/history` | Yes | Get full correction chain. |
| `GET` | `/api/expenses/prorata/:groupId` | Yes | Get all installments in a pro-rata group. |

## Finance Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/finance/periods/current` | Yes | Get current budget period. Query: `year, month`. Returns 404 `PERIOD_NOT_FOUND` if none exists. |
| `POST` | `/api/finance/periods` | Yes | Create budget period. Body: `{ year, month, budgetAmount, essentialsPercent, desiresPercent, savingsPercent }`. Applies pending pro-rata. |
| `PUT` | `/api/finance/periods/:id` | Yes | Update period settings. |
| `GET` | `/api/finance/periods` | Yes | List historical periods. Query: `page?, pageSize?`. |
| `GET` | `/api/finance/defaults` | Yes | Get user's default budget settings. |
| `PUT` | `/api/finance/defaults` | Yes | Update defaults. |
| `POST` | `/api/finance/onboarding` | Yes | Save onboarding settings and seed default tags. |
| `GET` | `/api/finance/tags` | Yes | List all user tags (lazy-seeds defaults if none exist). |
| `POST` | `/api/finance/tags` | Yes | Create tag. Body: `{ name }`. |
| `PUT` | `/api/finance/tags/:id` | Yes | Rename tag. Body: `{ name }`. |
| `DELETE` | `/api/finance/tags/:id` | Yes | Delete tag (blocked if in use). |
| `POST` | `/api/finance/prorata` | Yes | Create pro-rata expense. Body: `{ name, totalAmount, currency, expenseType, tagId, expenseDate, months }`. |
| `GET` | `/api/finance/prorata/upcoming` | Yes | Get pending pro-rata for next month. |
| `GET` | `/api/finance/summary` | Yes | Dashboard: period summary with pacing. Query: `year, month`. |
| `GET` | `/api/finance/spending/by-tag` | Yes | Dashboard: spending by tag. Query: `year, month`. |
| `GET` | `/api/finance/spending/cumulative` | Yes | Dashboard: cumulative spend chart data. Query: `year, month`. |
| `GET` | `/api/finance/spending/comparison` | Yes | Dashboard: current vs. historical spending. Query: `year, month`. |

## Response Shapes

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

### Common Error Codes

| Code | HTTP Status | When |
|------|-------------|------|
| `VALIDATION_ERROR` | 400 | Missing/invalid fields, E/D/S not summing to 100% |
| `WEAK_PASSWORD` | 400 | Password does not meet strength requirements |
| `INVALID_CREDENTIALS` | 401 | Wrong email or password |
| `TOKEN_EXPIRED` | 401 | Access token expired (triggers client refresh) |
| `UNAUTHORIZED` | 401 | Invalid token |
| `FORBIDDEN` | 403 | Non-admin accessing admin routes |
| `PERIOD_LOCKED` | 403 | Attempting to correct an expense outside the current period |
| `PERIOD_NOT_FOUND` | 404 | No budget period exists for the requested month |
| `DUPLICATE_EMAIL` | 409 | Email already registered |
| `DUPLICATE_USERNAME` | 409 | Username already taken |
| `ALREADY_CORRECTED` | 409 | Expense has already been corrected |
| `TAG_IN_USE` | 409 | Tag is referenced by expenses and cannot be deleted |
