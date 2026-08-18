# Data Model

## Multi-Currency Model

Reporting currency is period-scoped, not user-scoped. Each budget period stores an immutable `reportingCurrency`, and every period-scoped money value is expressed in that currency.

- **Period reporting currency**: chosen at period creation, immutable after creation.
- **Default settings currency**: seeds future periods only. Changing it never mutates existing periods.
- **Expense money snapshots**: each post-cutover ledger row stores transaction money, reporting money, and the conversion facts that connect them.
- **Legacy migration snapshots**: pre-epic rows missing a transaction amount are backfilled at startup with an identity migration snapshot in the legacy currency, with rate `1` and source `migration`.
- **Pro-rata captured snapshots**: schedules created after this epic store a full USD-based provider rate map so future installments never re-rate against live rates.

The shared currency catalog (`shared/currency/catalog.json`) is the source of truth for supported codes and minor-unit digits. See [Architecture](architecture.md) for service boundaries.

## Service Database Ownership

Each microservice owns its database schema exclusively. No service queries another service's database directly: cross-service data access happens via gRPC.

| Service | Database | Access Method | Data Owned |
|---------|----------|---------------|------------|
| Auth Service | PostgreSQL (`auth` schema) | sqlc | Users, credentials, refresh token blacklist |
| Finance Service | PostgreSQL (`finance` schema) | sqlc | Budget periods, default settings, tags, pro-rata schedules |
| Datarights Service | PostgreSQL (`datarights` schema) | pgx | Export job records |
| Expense Service | immudb | Native Go client | Expense ledger entries |
| FX Service | none | n/a | No persistent state; one-hour provider snapshot in process memory |

PostgreSQL runs as a single instance with separate schemas and connection credentials per service. This provides logical isolation with the option to split into separate databases later.

## Datarights Schema

Canonical source: `services/datarights/db/migrations/`

> **Operator-only admin:** data export is a `Personal` operation, so `datarights.export_jobs` holds only regular-user rows and never admin-owned rows.

### `datarights.export_jobs`

Stores export job metadata. The service tracks job lifecycle but does not persist user data: collected data exists only transiently during ZIP assembly. Key design points:

- **No foreign key to `auth.users`**: cross-schema FKs are avoided project-wide; referential integrity enforced at the application layer via gateway auth validation
- **Nullable terminal fields**: `error`, `file_size_bytes`, and `completed_at` are NULL while a job is in-progress, populated only on completion/failure
- **Indexes**: `(user_id, status)` for rate limit and deduplication checks; `(user_id, created_at DESC)` for paginated listing

```sql
CREATE TABLE datarights.export_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID         NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    error           TEXT,
    file_size_bytes BIGINT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
```

### Job State Machine

| From | To | Trigger | Side Effects |
|------|----|---------|--------------|
| (new) | pending | POST /exports | Row inserted, async goroutine queued |
| pending | running | Goroutine acquires pool slot | `updated_at` set |
| running | completed | All data collected + email sent | `completed_at`, `file_size_bytes` set |
| running | failed | Any unrecoverable error | `completed_at`, `error` set |

Only `completed` jobs consume the 30-day rate limit. Failed exports do not block retries.

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

- `reporting_currency`: immutable after period creation. Set from the create request, or defaulted from `finance.default_settings.currency` when the request omits it.
- `budget_amount`: minor units of the period's reporting currency.
- Historical periods backfilled during migration inherit the current default, user, or app fallback currency, in that precedence order.

### `finance.default_settings`

One row per user. Stores the default budget amount, E/D/S split, and default reporting currency applied when a future month's period is created. A budget amount of 0 means "not yet configured" (user skipped onboarding).

`currency` is future-scoped: it seeds newly created periods only. Updating it never changes the reporting currency of an existing period.

### `finance.tags`

User-defined expense categories. Tag names are unique per user (case-insensitive). Default tags are seeded during onboarding and flagged with `is_default` (they can be renamed but not deleted).

### `finance.pro_rata_schedules`

Tracks future installments of pro-rata expenses. Each row represents a single installment for a specific target month. Rows have a `status` of `pending` until the finance service applies them during budget period creation, at which point they transition to `applied`. Deterministic failures transition them to `failed` with a typed `failure_reason`.

Post-epic rows also store:

- `transaction_amount` and `transaction_currency`: the installment's original charged money.
- `creation_reporting_currency`: the reporting currency of the period where the schedule was created.
- `captured_rate_snapshot`: a full USD-based provider snapshot captured once at schedule creation. Future installments derive target-period reporting amounts from this snapshot, never from live rates.
- `failure_reason`: one of `missing_target_period`, `missing_captured_rate_snapshot`, `expense_write_failed`, or `snapshot_currency_missing`.

All installments in a pro-rata group share a `pro_rata_group` UUID, enabling queries that retrieve the full set of related installments.

### `finance.health_scores`

One row per user per closed month, keyed by `(user_id, year, month)`. Stores the full computed health score as `score` (JSONB) plus denormalized `total`, `band`, and `formula_version` scalar columns for cheap trend reads. Closed months are written lazily on read (compute-and-upsert on a miss or a stale formula version); the current provisional month is never stored. The JSONB is the single source of truth and shares the shape of the versioned golden snapshots under `services/finance/internal/service/testdata/healthscore/`.

## Expense Ledger (immudb)

The expense service creates its schema at startup (`CREATE TABLE IF NOT EXISTS`). Schema evolution is additive only: columns can be added but never modified or removed, consistent with immudb's immutability guarantees.

### `expenses`

The immutable expense ledger. Key design points:

- **Amounts in minor units** (integer): avoids floating-point precision issues. $12.50 is stored as `1250`; ¥1250 is stored as `1250`.
- **String dates**: immudb's SQL dialect has limited date type support, so dates are stored as ISO strings and parsed at the application layer.
- **Tag by ID**: tags live in PostgreSQL (finance schema). The ledger stores the tag UUID, not the tag name, so tag renames do not affect historical data.
- **Money snapshots**: every row carries `transaction_amount`, `transaction_currency`, `reporting_amount`, `reporting_currency`, `exchange_rate`, `exchange_rate_source`, `exchange_rate_timestamp`, and optional `exchange_rate_expires_at`.
- **Legacy migration backfill**: rows missing a transaction amount are backfilled at startup with an identity migration snapshot from the legacy `amount` and `currency`, with `exchangeRate = "1"` and `exchangeRateSource = "migration"`. The legacy `currency` column is obsolete metadata.

Snapshot sources:

| Source | Meaning |
|--------|---------|
| `identity` | Same-currency write or correction; rate `1`, no provider call |
| `open_exchange_rates` | Foreign-currency write or correction through FX Service |
| `migration` | Legacy row backfilled at startup; rate `1` in the legacy currency |

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
- `shared/currency/catalog.json`: static source of truth for supported codes, symbols, and minor-unit digits. Frontend and Go services consume generated artifacts from this file; Open Exchange Rates does not expand the supported set.

## Migration Strategy

**PostgreSQL**: managed by [golang-migrate](https://github.com/golang-migrate/migrate). Files follow `000001_description.up.sql` / `.down.sql` naming. Each service runs its own migrations against its schema.

**immudb**: no migration tool. The expense service creates tables and indexes at startup via `CREATE TABLE IF NOT EXISTS`. Schema evolution is additive only. The multi-currency migration adds nullable snapshot columns to the existing table; legacy rows keep their legacy `amount` and `currency` columns and resolve through the migration snapshot synthesis described above.

### Historical Migration Semantics

Historical periods inherit their reporting currency silently from the current default settings currency, then the user's auth profile currency, then the configured app fallback. Historical expenses are treated as already denominated in the migrated period reporting currency: no provider conversion is performed, and legacy `currency` is obsolete metadata used only for mismatch telemetry.
