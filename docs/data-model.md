# Data Model

## Service Database Ownership

Each microservice owns its database schema exclusively. No service queries another service's database directly: cross-service data access happens via gRPC.

| Service | Database | Access Method | Data Owned |
|---------|----------|---------------|------------|
| Auth Service | PostgreSQL (`auth` schema) | sqlc | Users, credentials, refresh token blacklist |
| Finance Service | PostgreSQL (`finance` schema) | sqlc | Budget periods, default settings, tags, pro-rata schedules |
| Expense Service | immudb | Native Go client | Expense ledger entries |

PostgreSQL runs as a single instance with separate schemas and connection credentials per service. This provides logical isolation with the option to split into separate databases later.

## Auth Schema

### `auth.users`

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID (PK) | Auto-generated |
| `username` | VARCHAR(50) | Unique |
| `email` | VARCHAR(255) | Unique |
| `password_hash` | VARCHAR(255) | bcrypt |
| `role` | VARCHAR(10) | `'user'` or `'admin'` |
| `currency` | VARCHAR(3) | Display currency (default: USD) |
| `has_completed_onboarding` | BOOLEAN | Default: false |
| `tokens_revoked_at` | TIMESTAMPTZ | Set on password change; invalidates older tokens |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

### `auth.refresh_token_blacklist`

| Column | Type | Notes |
|--------|------|-------|
| `jti` | VARCHAR(36) (PK) | Token ID from JWT claims |
| `user_id` | UUID (FK) | References `auth.users` |
| `expires_at` | TIMESTAMPTZ | Natural token expiration (for cleanup) |
| `revoked_at` | TIMESTAMPTZ | When the token was blacklisted |

Periodic cleanup deletes rows where `expires_at < now()`, since blacklisted tokens only matter until their natural expiration.

## Finance Schema

### `finance.budget_periods`

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID (PK) | |
| `user_id` | UUID | References auth.users by convention (no FK) |
| `year` | INTEGER | |
| `month` | INTEGER | 1-12 |
| `budget_amount` | BIGINT | Minor units (cents) |
| `essentials_percent` | INTEGER | 0-100 |
| `desires_percent` | INTEGER | 0-100 |
| `savings_percent` | INTEGER | 0-100 |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

Constraints: unique on `(user_id, year, month)`, check that percentages sum to 100.

### `finance.default_settings`

| Column | Type | Notes |
|--------|------|-------|
| `user_id` | UUID (PK) | |
| `budget_amount` | BIGINT | Default: 0 (not yet configured) |
| `essentials_percent` | INTEGER | Default: 50 |
| `desires_percent` | INTEGER | Default: 30 |
| `savings_percent` | INTEGER | Default: 20 |
| `currency` | VARCHAR(3) | Default: USD |

### `finance.tags`

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID (PK) | |
| `user_id` | UUID | |
| `name` | VARCHAR(50) | Unique per user (case-insensitive) |
| `is_default` | BOOLEAN | Seeded tags cannot be deleted (only renamed) |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

Default tags seeded on onboarding: Bills, Food, Household, Investment, Personal Care, Recreation/Entertainment, Self Investment, Social, Transport, Travel.

### `finance.pro_rata_schedules`

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID (PK) | |
| `user_id` | UUID | |
| `pro_rata_group` | UUID | Groups related installments |
| `name` | VARCHAR(255) | Inherited from original expense |
| `amount` | BIGINT | This installment's amount (cents) |
| `currency` | VARCHAR(3) | |
| `expense_type` | VARCHAR(20) | `essentials`, `desires`, or `savings` |
| `tag_id` | UUID | |
| `target_year` | INTEGER | |
| `target_month` | INTEGER | 1-12 |
| `installment_index` | INTEGER | 1-based |
| `installment_total` | INTEGER | |
| `status` | VARCHAR(10) | `'pending'` or `'applied'` |
| `applied_at` | TIMESTAMPTZ | Null until applied |

## Expense Ledger (immudb)

### `expenses`

| Column | Type | Notes |
|--------|------|-------|
| `id` | VARCHAR(36) (PK) | UUID |
| `user_id` | VARCHAR(36) | |
| `name` | VARCHAR(255) | |
| `amount` | INTEGER | Minor units (cents) |
| `currency` | VARCHAR(3) | ISO 4217 |
| `expense_type` | VARCHAR(20) | `essentials`, `desires`, or `savings` |
| `tag_id` | VARCHAR(36) | References finance.tags by ID |
| `expense_date` | VARCHAR(10) | ISO date string |
| `period_year` | INTEGER | |
| `period_month` | INTEGER | 1-12 |
| `status` | VARCHAR(20) | `'active'` or `'corrected'` |
| `corrects_id` | VARCHAR(36) | ID of the entry this corrects (null for originals) |
| `is_pro_rata` | BOOLEAN | |
| `pro_rata_group` | VARCHAR(36) | Groups related installments |
| `pro_rata_index` | INTEGER | 1-based |
| `pro_rata_total` | INTEGER | |
| `created_at` | VARCHAR(30) | ISO 8601 |

**Design notes:**

- Amounts in cents avoid floating-point precision issues ($12.50 = 1250)
- String dates because immudb's SQL dialect has limited date type support
- Tag referenced by ID: tag renames do not affect historical data
- immudb never truly "updates" rows: under the hood, an UPDATE creates a versioned copy; the SQL interface presents the latest version

### Correction Chain

Expenses are never edited. A correction creates a new entry that references the one it supersedes via `corrects_id`. The original is marked `status='corrected'`. At any point, exactly one entry in a chain has `status='active'`.

The **materialized view** is an application-level abstraction: `SELECT * FROM expenses WHERE status = 'active'`. Downstream services only see current truth.

## Cross-Service References

Services reference each other's data by UUID convention (no foreign key constraints across schemas/databases). Referential integrity is enforced at the application layer:

- The gateway validates `user_id` before forwarding to any service
- The finance service validates `tag_id` before calling the expense service
- Tag deletion checks both expense usage and pending pro-rata schedules via gRPC

### Currency Dual Ownership

Currency appears in both `auth.users.currency` and `finance.default_settings.currency`:

- `auth.users.currency`: display currency, returned by `GET /api/auth/me` for frontend formatting
- `finance.default_settings.currency`: default currency for new expenses

These are kept in sync by the onboarding and settings update endpoints.

## Migration Strategy

**PostgreSQL**: managed by [golang-migrate](https://github.com/golang-migrate/migrate). Files follow `000001_description.up.sql` / `.down.sql` naming. Each service runs its own migrations against its schema.

**immudb**: no migration tool. The expense service creates tables and indexes at startup via `CREATE TABLE IF NOT EXISTS`. Schema evolution is additive only (columns can be added, never modified or removed).
