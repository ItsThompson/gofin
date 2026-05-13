# Monitoring

## Overview

The observability stack runs on Node 4 (monitoring network) and consists of Prometheus (metrics collection), Alertmanager (alert routing), Grafana (visualization), and a custom auth proxy that gates Grafana access to admin users.

## Prometheus

### Scrape Configuration

Prometheus scrapes `/metrics` endpoints from all Go services on the monitoring network. Targets and intervals are defined in `monitoring/prometheus/prometheus.yml`.

### Metrics Exposed by Services

Each Go service exposes standard HTTP/gRPC metrics and service-specific counters via `/metrics`. The general categories:

- **HTTP request metrics**: total count, latency histograms (by method, path, status)
- **gRPC request metrics**: total count, latency histograms (by method, status)
- **Database query metrics**: latency histograms (by query name)
- **Connection pool metrics**: active database connections
- **Business metrics**: expense creation counts, correction counts, token refresh counts, export job lifecycle

Exact metric names and labels are defined in the `services/metrics/` package (shared) and `services/datarights/internal/metrics/` (datarights-specific).

#### Datarights Service Metrics

Custom business metrics for data export monitoring:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `export_jobs_created_total` | Counter | — | Export jobs submitted via the API |
| `export_jobs_completed_total` | Counter | `status` (completed/failed) | Jobs reaching terminal state |
| `export_rate_limit_rejections_total` | Counter | — | Requests rejected by 30-day cooldown |
| `export_job_duration_seconds` | Histogram | — | End-to-end job duration (pending to terminal) |
| `export_data_collection_duration_seconds` | Histogram | `provider` | Per-provider data collection latency |
| `export_email_send_duration_seconds` | Histogram | — | Email delivery latency via Resend |
| `export_zip_size_bytes` | Histogram | — | Size of generated ZIP files |
| `export_pool_active_jobs` | Gauge | — | Currently running export goroutines |
| `export_pool_queued_jobs` | Gauge | — | Jobs waiting for a pool slot |

### Access

Prometheus UI: `http://localhost:9090`

## Alerting Rules

Alert rules are defined in `monitoring/prometheus/alerts.yml`. The rules cover:

- **High error rate**: elevated 5xx response ratio over a sliding window
- **Service down**: no metrics received from a scrape target
- **Slow queries**: p95 query duration exceeding a threshold
- **Auth failures spike**: unusual volume of failed login attempts
- **Export job failure rate**: more than 50% of export jobs failed in the last hour
- **Export job stuck**: active jobs exist but no completions in 10 minutes

### Datarights Alert Rules

| Alert | Condition | Severity | Description |
|-------|-----------|----------|-------------|
| `ExportJobFailureRate` | >50% of jobs failed in 1h window | warning | Indicates systemic issue with upstream services or email delivery |
| `ExportJobStuck` | Active jobs with no completions for 10m | warning | Likely a hung goroutine or upstream service timeout |

Alertmanager configuration (notification channels and routing) is at `monitoring/alertmanager/alertmanager.yml`.

## Grafana

### Access

- Local: `http://localhost:3001`
- Remote: via Cloudflare tunnel through the auth proxy (admin-only)

### Pre-Provisioned Dashboards

Dashboards are provisioned automatically from `monitoring/grafana/dashboards/` and cover system-wide request/error/latency metrics, per-service breakdowns (including datarights export job metrics), and database health. Datasource provisioning is at `monitoring/grafana/provisioning/`.

### Auth Proxy

The Grafana auth proxy (`monitoring/auth-proxy/`) is a small Go service that:

1. Extracts the `gofin_access` JWT from the request cookie
2. Validates the signature using the shared `JWT_SECRET`
3. Checks that the user has the admin role
4. On success: proxies to Grafana with `X-WEBAUTH-USER` header
5. On failure: returns 403

The proxy validates JWTs locally (no gRPC call to Auth Service), keeping the observability node independent of the compute node at runtime.

## Structured Logging

All Go services emit JSON-structured logs to stdout with a consistent format:

- `level`: info, warn, error
- `timestamp`: ISO 8601
- `service`: service name
- `method`: handler or function name
- `user_id`: present for authenticated requests
- `assumed_by`: present during identity assumption
- `duration_ms`: present for timed operations
- `error`: present on errors

Logs are viewable via `just logs <service>` or `docker compose logs -f <service>`. Centralized log aggregation (ELK, Loki) is deferred for MVP.

## Resource Limits

Container CPU and memory limits are defined in `docker-compose.yml` under each service's `deploy.resources` section. Current limits are modest, sized for a maximum of 5 concurrent users.
