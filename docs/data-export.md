# Data Export

## Overview

The datarights service provides GDPR Article 20 data portability: users can request a complete export of their personal data. The export runs asynchronously, collects data from all upstream services via gRPC, packages it as a ZIP of CSV files, and delivers it via email.

For architecture details, see [Architecture: Data Flow](architecture.md#data-flow-data-export). For API endpoints, see [API Reference: Datarights](api.md#datarights-apidatarights).

## Resend Configuration

Email delivery uses [Resend](https://resend.com) with the verified domain `usegofin.com`.

### Domain Setup

1. In the Resend dashboard: **Domains** → **Add Domain** → enter `usegofin.com`
2. Resend provides 3 DKIM CNAME records. Add them in Cloudflare (usegofin.com zone) with proxy **OFF** (DNS only)
3. Add SPF TXT record: `v=spf1 include:amazonses.com ~all` on `@`
4. Add DMARC TXT record: `v=DMARC1; p=none;` on `_dmarc`
5. Click **Verify** in Resend dashboard (Cloudflare propagation is near-instant)

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `EMAIL_ENABLED` | No | `false` (via .env.example) | When false, emails log to stdout (dev mode). Note: if unset, code defaults to `true` |
| `RESEND_API_KEY` | When `EMAIL_ENABLED=true` | — | Resend API key with sending access |
| `EMAIL_FROM` | No | `gofin <noreply@usegofin.com>` | Sender address |
| `BRAND_TOKENS_PATH` | No | `/app/tokens/brand.json` | Path to brand styling tokens |

### Development Mode

With `EMAIL_ENABLED=false` (the default), the service uses a `LogSender` that writes email metadata to stdout instead of calling Resend. This allows full pipeline testing without network access or API keys:

```
{"level":"INFO","msg":"email delivery disabled: logging email content","to":"user@example.com","zip_size_bytes":24576}
```

## CSV Format Specification

The export ZIP contains 5 CSV files, one per data provider. All CSVs use UTF-8 encoding with a header row.

### `profile.csv`

Single row with account information.

| Column | Type | Notes |
|--------|------|-------|
| `username` | string | |
| `email` | string | |
| `currency` | string | ISO 4217 code (e.g., `USD`) |
| `role` | string | `user` or `admin` |
| `account_created_at` | timestamp | ISO 8601 |

### `expenses.csv`

All expense ledger entries (both active and corrected) in chronological order.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID | |
| `name` | string | Expense description |
| `amount` | decimal | Dollars with 2 decimal places (e.g., `12.50`) |
| `currency` | string | ISO 4217 code |
| `expense_type` | string | `essentials`, `desires`, or `savings` |
| `tag_name` | string | Resolved tag name (not ID). `Unknown` if tag was deleted |
| `expense_date` | string | ISO date |
| `period_year` | int | Budget period year |
| `period_month` | int | Budget period month (1-12) |
| `status` | string | `active` or `corrected` |
| `corrects_id` | UUID | ID of the expense this corrects (empty if original) |
| `is_pro_rata` | boolean | `true` or `false` |
| `pro_rata_group` | UUID | Shared across installments (empty if not pro-rata) |
| `pro_rata_index` | int | Installment number (empty if not pro-rata) |
| `pro_rata_total` | int | Total installments (empty if not pro-rata) |
| `created_at` | timestamp | ISO 8601 |

### `tags.csv`

All user-defined expense categories.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID | |
| `name` | string | |
| `is_default` | boolean | `true` for system-seeded tags |
| `created_at` | timestamp | ISO 8601 |

### `budget_periods.csv`

All budget periods (one per month configured).

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID | |
| `year` | int | |
| `month` | int | 1-12 |
| `budget_amount` | decimal | Dollars with 2 decimal places |
| `essentials_percent` | int | 0-100 |
| `desires_percent` | int | 0-100 |
| `savings_percent` | int | 0-100 |
| `created_at` | timestamp | ISO 8601 |

### `default_settings.csv`

Single row with current budget defaults (empty if not configured).

| Column | Type | Notes |
|--------|------|-------|
| `budget_amount` | decimal | Dollars with 2 decimal places |
| `essentials_percent` | int | 0-100 |
| `desires_percent` | int | 0-100 |
| `savings_percent` | int | 0-100 |
| `currency` | string | ISO 4217 code |

## Adding a Data Provider

The export engine uses a provider registry pattern. Each provider is responsible for one CSV file.

### 1. Implement the `DataProvider` interface

```go
// services/datarights/internal/engine/provider.go
type DataProvider interface {
    Name() string                                          // CSV filename (without .csv)
    Headers() []string                                     // Column headers
    Collect(ctx context.Context, userID string) ([][]string, error) // Data rows
}
```

### 2. Create the provider

Create a new file in `services/datarights/internal/engine/providers/`:

```go
package providers

type MyProvider struct {
    client somepb.SomeServiceClient
}

func NewMyProvider(client somepb.SomeServiceClient) *MyProvider {
    return &MyProvider{client: client}
}

func (p *MyProvider) Name() string        { return "my_data" }
func (p *MyProvider) Headers() []string   { return []string{"col1", "col2"} }
func (p *MyProvider) Collect(ctx context.Context, userID string) ([][]string, error) {
    // Fetch data via gRPC, transform to string slices
    return rows, nil
}
```

### 3. Register in `cmd/main.go`

```go
registry.Register(providers.NewMyProvider(myClient))
```

The engine handles everything else: CSV writing, ZIP assembly, email delivery, error handling, and metrics emission per provider.

### Formatting helpers

`services/datarights/internal/engine/providers/format.go` provides:

- `formatCentsToDollars(cents int64)`: converts cents to `"12.50"` format
- `formatBool(b bool)`: renders `"true"` / `"false"`
- `resolveTagName(tagID, tagMap)`: looks up tag names with `"Unknown"` fallback
- `formatOptionalInt(value, condition)`: renders empty string when condition is false

## Rate Limiting

- One successful export allowed per 30-day rolling window
- Failed exports do not consume the limit (users can retry immediately)
- If a pending/running job exists, POST returns the existing job (200) instead of creating a duplicate
- The `retryAfter` field in the 429 response tells the client when the next export is allowed

## Troubleshooting

### Job stuck in "running"

**Symptoms**: `export_pool_active_jobs` gauge stays elevated, `ExportJobStuck` alert fires.

**Likely causes**:
- Upstream service unreachable (auth, expense, or finance service down)
- Expense pagination taking too long (user with very large dataset)
- Network timeout between datarights and upstream services

**Resolution**: Check datarights service logs for gRPC errors. Verify upstream services are healthy. The job will time out after `EXPORT_TIMEOUT_SECONDS` (default 5 minutes) and transition to failed.

### Email not received

**Likely causes**:
- `EMAIL_ENABLED=false` (check service logs for "email delivery disabled")
- Invalid `RESEND_API_KEY` (check Resend dashboard for API key status)
- Recipient email address rejected by Resend

**Resolution**: Check the Resend dashboard **Logs** tab for delivery status. Verify the API key has sending access for `usegofin.com`.

### SPF/DKIM/DMARC failure

**Likely causes**:
- DNS records not matching what Resend dashboard shows
- CNAME records proxied through Cloudflare (must be DNS-only / grey cloud)
- Stale DNS cache

**Resolution**: Compare records in Cloudflare with Resend dashboard. Use `dig` to verify propagation:
```bash
dig CNAME resend._domainkey.usegofin.com
dig TXT _dmarc.usegofin.com
dig TXT usegofin.com  # should include amazonses.com SPF
```

### Rate limited unexpectedly

**Likely cause**: A completed (not failed) export exists within the last 30 days.

**Resolution**: Check via `GET /api/datarights/exports` for recent jobs with `status: "completed"`. The `retryAfter` field in the 429 response indicates when the next export is allowed.

### Startup recovery re-submitting old jobs

On startup, the service queries for non-terminal jobs (pending/running) and re-submits them. This is expected behavior after a restart or deployment. If a job was interrupted mid-collection, it restarts from scratch.
