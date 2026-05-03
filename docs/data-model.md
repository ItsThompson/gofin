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

Canonical source: `services/auth/db/migrations/`

### `auth.users`

Stores user accounts with credentials and profile data. Key design points:

- `password_hash`: bcrypt-hashed password
- `role`: supports `'user'` and `'admin'` (checked via RBAC at every layer)
- `currency`: the user's display currency, returned by `GET /api/auth/me` for frontend formatting
- `has_completed_onboarding`: gates the onboarding redirect flow
- `tokens_revoked_at`: set on password change; any token with `iat` before this timestamp is rejected, forcing re-login on all other sessions

### `auth.refresh_token_blacklist`

Tracks revoked refresh tokens by their `jti` claim. Entries include the token's natural expiration so a periodic cleanup job can delete rows that no longer matter.

## Finance Schema

Canonical source: `services/finance/db/migrations/`

### `finance.budget_periods`

One row per user per calendar month. Stores the budget amount and E/D/S percentage split. Constrained so percentages always sum to 100 and each user has at most one period per month.

### `finance.default_settings`

One row per user. Stores the default budget amount, E/D/S split, and currency applied when a new month's period is created. A budget amount of 0 means "not yet configured" (user skipped onboarding).

### `finance.tags`

User-defined expense categories. Tag names are unique per user (case-insensitive). Default tags are seeded during onboarding and flagged with `is_default` (they can be renamed but not deleted).

### `finance.pro_rata_schedules`

Tracks future installments of pro-rata expenses. Each row represents a single installment for a specific target month. Rows have a `status` of `'pending'` until the finance service applies them during budget period creation, at which point they transition to `'applied'`.

All installments in a pro-rata group share a `pro_rata_group` UUID, enabling queries that retrieve the full set of related installments.

## Expense Ledger (immudb)

The expense service creates its schema at startup (`CREATE TABLE IF NOT EXISTS`). Schema evolution is additive only: columns can be added but never modified or removed, consistent with immudb's immutability guarantees.

### `expenses`

The immutable expense ledger. Key design points:

- **Amounts in cents** (integer): avoids floating-point precision issues. $12.50 is stored as `1250`.
- **String dates**: immudb's SQL dialect has limited date type support, so dates are stored as ISO strings and parsed at the application layer.
- **Tag by ID**: tags live in PostgreSQL (finance schema). The ledger stores the tag UUID, not the tag name, so tag renames do not affect historical data.
- **Currency per expense**: even though MVP is single-currency, each expense carries its own currency field for future multi-currency support.

### Correction Chain

Expenses are never edited or deleted. A correction creates a new entry that references the one it supersedes via `corrects_id`. The original is marked `status='corrected'`. At any point in a chain, exactly one entry has `status='active'`: the most recent correction (or the original if never corrected).

The **materialized view** is an application-level abstraction: queries filter to `status='active'`. Downstream services (finance engine) only see current truth and never deal with correction mechanics.

Note: immudb does not truly "update" rows. Under the hood, an UPDATE creates a new versioned copy. The SQL interface presents the latest version, but both the original and corrected states are preserved in immudb's internal versioned storage.

## Cross-Service References

Services reference each other's data by UUID convention (no foreign key constraints across schemas or databases). Referential integrity is enforced at the application layer:

- The gateway validates `user_id` before forwarding to any service
- The finance service validates `tag_id` before calling the expense service
- Tag deletion checks both expense usage and pending pro-rata schedules via gRPC

### Currency Dual Ownership

Currency appears in both `auth.users.currency` and `finance.default_settings.currency`:

- `auth.users.currency`: display currency, returned by `GET /api/auth/me` for frontend formatting
- `finance.default_settings.currency`: default currency for new expenses

These are kept in sync by the onboarding and settings update endpoints. The finance service is the source of truth; the auth copy exists solely so the frontend can display currency symbols without an extra call to the finance service.

## Migration Strategy

**PostgreSQL**: managed by [golang-migrate](https://github.com/golang-migrate/migrate). Files follow `000001_description.up.sql` / `.down.sql` naming. Each service runs its own migrations against its schema.

**immudb**: no migration tool. The expense service creates tables and indexes at startup via `CREATE TABLE IF NOT EXISTS`. Schema evolution is additive only.
