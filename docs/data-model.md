# Data Model

## Multi-Currency Model

Reporting currency is period-scoped, not user-scoped. Each budget period stores an immutable `reportingCurrency`, and every period-scoped money value is expressed in that currency.

- **Period reporting currency**: chosen at period creation, immutable after creation.
- **Default settings currency**: seeds future periods only. Changing it never mutates existing periods.
- **Expense money snapshots**: each ledger row stores transaction money, reporting money, and the conversion facts that connect them.
- **Legacy migration snapshots**: rows with `exchange_rate_source = migration` carry rate `1` and source `migration`. No code path writes migration snapshots; the datarights export normalizes these rows to the period reporting currency and emits them with `exchange_rate_source = identity`.
- **Pro-rata captured snapshots**: schedules store a full USD-based provider rate map so future installments never re-rate against live rates.

The shared currency catalog (`services/shared/currency/catalog.go`) is the source of truth for supported codes and minor-unit digits. The finance service serves it to the frontend through `GET /api/finance/currencies`. See [Architecture](architecture.md) for service boundaries.

## Service Database Ownership

Each microservice owns its database schema exclusively. No service queries another service's database directly: cross-service data access happens via gRPC.

| Service | Database | Access Method | Data Owned |
|---------|----------|---------------|------------|
| Auth Service | PostgreSQL (`auth` schema) | sqlc | Users, credentials, refresh token blacklist |
| Finance Service | PostgreSQL (`finance` schema) | sqlc | Budget periods, default settings, tags, pro-rata schedules, health scores, period reporting currency migration audit |
| Datarights Service | PostgreSQL (`datarights` schema) | pgx | Export job records, deletion job records |
| Expense Service | immudb | Native Go client | Expense ledger entries |
| FX Service | none | n/a | No persistent state; provider snapshot cache in process memory (default one hour, `FX_CACHE_MAX_AGE`) |

PostgreSQL runs as a single instance with separate schemas and connection credentials per service. This provides logical isolation with the option to split into separate databases later.

## Datarights Schema

Canonical source: `services/datarights/db/migrations/`

> **Operator-only admin:** data export is a `Personal` operation, so `datarights.export_jobs` holds only regular-user rows and never admin-owned rows.

### `datarights.export_jobs`

Stores export job metadata. The service tracks job lifecycle but does not persist user data: collected data exists only transiently during ZIP assembly. Key design points:

- **No foreign key to `auth.users`**: cross-schema FKs are avoided project-wide; referential integrity enforced at the application layer via gateway auth validation
- **Nullable terminal fields**: `error`, `file_size_bytes`, and `completed_at` are NULL while a job is in-progress, populated only on completion/failure
- **Indexes**: `(user_id, status)` for rate limit and deduplication checks; `(user_id, created_at DESC)` for paginated listing

Canonical DDL: `services/datarights/db/migrations/000001_create_export_jobs.up.sql`.

### Job State Machine

| From | To | Trigger | Side Effects |
|------|----|---------|--------------|
| (new) | pending | POST /exports | Row inserted, async goroutine queued |
| pending | running | Goroutine acquires pool slot | `updated_at` set |
| running | completed | All data collected + email sent | `completed_at`, `file_size_bytes` set |
| running | failed | Any unrecoverable error | `completed_at`, `error` set |

Only `completed` jobs consume the 30-day rate limit. Failed exports do not block retries.

### `datarights.deletion_jobs`

Tracks admin-initiated GDPR deletion jobs. Like export jobs, it stores only job metadata (status, timestamps); the deletion engine fans out across services to remove or anonymize the target user's data. Key design points:

- **No foreign key to `auth.users`**: referential integrity enforced at the application layer
- **Self-deletion prevention**: the service rejects a deletion job whose target is the acting admin
- **Idempotent deduplication**: a non-terminal deletion job for the user short-circuits to the existing in-progress job
- **Indexes**: `(user_id, status)` and `(status)` for pending-job discovery

Canonical DDL: `services/datarights/db/migrations/000002_create_deletion_jobs.up.sql`.

## Auth Schema

Canonical source: `services/auth/db/migrations/`

### `auth.users`

Stores user accounts with credentials and profile data. Key design points:

- `password_hash`: bcrypt-hashed password
- `role`: supports `'user'` and `'admin'` (checked via RBAC at every layer). `admin` is an operator-only identity and owns no finance data (see the Finance Schema note below).
- `currency`: display-only profile currency, still returned by `GET /api/auth/me` for frontend display. It does not control period reporting; each budget period owns its reporting currency.
- `has_completed_onboarding`: gates the onboarding redirect flow
- `tokens_revoked_at`: set on password change; any token with `iat` before this timestamp is rejected, forcing re-login on all other sessions

### `auth.refresh_token_blacklist`

Tracks revoked refresh tokens by their `jti` claim. Entries include the token's natural expiration so a periodic cleanup job can delete rows that no longer matter.

## Finance Schema

Canonical source: `services/finance/db/migrations/`

> **Operator-only admin:** `admin` accounts own no rows in any finance table. Admin is an operator identity (authentication, admin panel, identity assumption, user deletion, Grafana) and never goes through the onboarding or budget flows. Every table below is scoped per `user_id` and only ever holds regular-user (`role=user`) data.

### `finance.budget_periods`

One row per user per calendar month. Stores the budget amount in the period's reporting currency and an E/D/S percentage split. Constrained so percentages always sum to 100 and each user has at most one period per month.

- `reporting_currency`: immutable after period creation. Required in the create request (`binding:"required"`, validated against the catalog). Periods auto-created for skipped months use the user's default settings, including `default_settings.currency`.
- `budget_amount`: minor units of the period's reporting currency.
- Historical periods were backfilled by migration `000006_add_period_reporting_currency.up.sql` with precedence `default_settings.currency` -> `auth.users.currency` -> `USD`, recording audit rows in `finance.period_reporting_currency_migration_report`. There is no fallback reporting currency in the live create path.

### `finance.default_settings`

One row per user. Stores the default budget amount, E/D/S split, and default reporting currency applied when a future month's period is created. A budget amount of 0 means "not yet configured" (user skipped onboarding).

`currency` is future-scoped: it seeds newly created periods only. Updating it never changes the reporting currency of an existing period.

### `finance.tags`

User-defined expense categories. Tag names are unique per user (case-insensitive). Default tags are seeded during onboarding and flagged with `is_default` (they can be renamed but not deleted).

### `finance.pro_rata_schedules`

Tracks future installments of pro-rata expenses. Each row represents a single installment for a specific target month. Rows have a `status` of `pending` until the finance service applies them during budget period creation, at which point they transition to `applied`. Deterministic failures transition them to `failed` with a typed `failure_reason`.

The schedule also stores:

- `transaction_amount` and `transaction_currency`: the installment's original charged money.
- `creation_reporting_currency`: the reporting currency of the period where the schedule was created.
- `captured_rate_snapshot`: a full USD-based provider snapshot captured once at schedule creation. Future installments derive target-period reporting amounts from this snapshot, never from live rates.
- `failure_reason`: the only typed deterministic reason stored today is `SNAPSHOT_CURRENCY_MISSING` (set when a schedule has no captured snapshot, or the snapshot cannot derive a required currency). Transient write failures log `transient_write_failure` and leave the row `pending` for retry; they are not stored as `failure_reason`. See `services/finance/internal/service/prorata.go` (`applyOneProRataSchedule`).

All installments in a pro-rata group share a `pro_rata_group` UUID, enabling queries that retrieve the full set of related installments.

### `finance.health_scores`

One row per user per closed month, keyed by `(user_id, year, month)`. Stores the full computed health score as `score` (JSONB) plus denormalized `total`, `band`, and `formula_version` scalar columns for cheap trend reads. Closed months are written lazily on read (compute-and-upsert on a miss or a stale formula version); the current provisional month is never stored. The JSONB is the single source of truth and shares the shape of the versioned golden snapshots under `services/finance/internal/service/testdata/healthscore/`.

## Expense Ledger (immudb)

The expense service creates its schema at startup (`CREATE TABLE IF NOT EXISTS`). Schema evolution is additive by policy: the startup reconcile adds nullable columns to pre-existing tables. As a temporary cleanup, it also drops the legacy `amount` and `currency` columns once prod has booted with the new schema (see `services/expense/internal/repository/immudb.go`, `InitSchema`).

### `expenses`

The immutable expense ledger. Key design points:

- **Amounts in minor units** (integer): avoids floating-point precision issues. $12.50 is stored as `1250`; ¥1250 is stored as `1250`.
- **String dates**: immudb's SQL dialect has limited date type support, so dates are stored as ISO strings and parsed at the application layer.
- **Tag by ID**: tags live in PostgreSQL (finance schema). The ledger stores the tag UUID, not the tag name, so tag renames do not affect historical data.
- **Money snapshots**: every row carries `transaction_amount`, `transaction_currency`, `reporting_amount`, `reporting_currency`, `exchange_rate`, `exchange_rate_source`, `exchange_rate_timestamp`, and optional `exchange_rate_expires_at`.
- **Legacy migration snapshots**: rows with `exchange_rate_source = migration` carry rate `1`. A row missing a required snapshot field fails the read with a `SnapshotIntegrityError` (logged as `expense_snapshot_integrity_error`); the read is not synthesized.

Snapshot sources:

| Source | Meaning |
|--------|---------|
| `identity` | Same-currency write or correction; rate `1`, no provider call |
| `open_exchange_rates` | Foreign-currency write or correction through FX Service |
| `migration` | Legacy row with no provider conversion; rate `1`. No code path writes this source. The datarights export normalizes these rows to the period reporting currency and emits them as `identity`. |

### Correction Chain

Expenses are never edited or deleted. A correction creates a new entry that references the one it supersedes via `corrects_id`. The original is marked `status='corrected'`. At any point in a chain, exactly one entry has `status='active'`: the most recent correction (or the original if never corrected).

The **materialized view** is an application-level abstraction: queries filter to `status='active'`. Downstream services (finance engine) only see current truth and never deal with correction mechanics.

Note: immudb does not truly "update" rows. Under the hood, an UPDATE creates a new versioned copy. The SQL interface presents the latest version, but both the original and corrected states are preserved in immudb's internal versioned storage.

## Cross-Service References

Services reference each other's data by UUID convention (no foreign key constraints across schemas or databases). Referential integrity is enforced at the application layer:

- The gateway validates `user_id` before forwarding to any service
- The finance service validates `tag_id` before calling the expense service
- Tag deletion checks both expense usage and pending pro-rata schedules via gRPC

### Currency Ownership

Currency has one reporting authority: the budget period. The shared catalog is the single source of supported codes.

- `finance.budget_periods.reporting_currency`: immutable reporting currency for a period; this controls dashboard totals, history rows, and health scores.
- `finance.default_settings.currency`: default for future period creation only.
- `auth.users.currency`: display-only profile copy, returned by `GET /api/auth/me` for frontend display. It does not control reporting output.
- `services/shared/currency/catalog.go`: Go catalog, the single source of truth for supported codes, symbols, and minor-unit digits. The finance service serves it through `GET /api/finance/currencies`; the frontend loads it at runtime (no built-in catalog, no generated artifacts). Open Exchange Rates does not expand the supported set.

## Migration Strategy

**PostgreSQL**: managed by [golang-migrate](https://github.com/golang-migrate/migrate). Files follow `000001_description.up.sql` / `.down.sql` naming. Each service runs its own migrations against its schema.

**immudb**: no migration tool. The expense service creates tables and indexes at startup via `CREATE TABLE IF NOT EXISTS`. The startup schema reconcile adds the nullable money-snapshot columns to existing tables. Rows with `exchange_rate_source = migration` carry rate `1`; reads of rows missing required snapshot fields fail with a `SnapshotIntegrityError` instead of synthesizing values.

### Historical Migration Semantics

Historical periods were backfilled by migration `000006_add_period_reporting_currency.up.sql`: `reporting_currency` was set with precedence `default_settings.currency` -> `auth.users.currency` -> `USD`, and audit rows were recorded in `finance.period_reporting_currency_migration_report` for the auth-fallback and USD-fallback cases. The column is `NOT NULL` with a `CHECK` limiting it to the ten catalog codes. Historical expenses carry `migration`/`identity` snapshots with rate `1`, so no provider conversion is performed; the datarights export normalizes legacy rows to the period reporting currency and emits them as `identity`.
