# Monitoring

## Overview

The observability stack uses Grafana Cloud for dashboards, alerting, and long-term metric storage. On the VPS, Grafana Alloy scrapes local metrics and remote-writes to Grafana Cloud. Supporting exporters (cadvisor, node-exporter) run on the monitoring network.

## Architecture

```
VPS (monitoring-net)              Grafana Cloud (SaaS)
┌──────────────────┐              ┌──────────────────┐
│ cadvisor         │──┐           │ Dashboards       │
│ node-exporter    │──┼──▶ Alloy ──▶ Alerting        │
│ Go services      │──┘           │ Metric storage   │
│   /metrics       │              │ (14d retention)  │
└──────────────────┘              └──────────────────┘
```

## Grafana Alloy

Alloy (`monitoring/alloy/config.alloy`) is the sole metric collection agent. It:

- Scrapes all Go service `/metrics` endpoints every 15s
- Scrapes cadvisor and node-exporter every 15s
- Remote-writes all metrics to Grafana Cloud

Configuration uses HCL syntax with `prometheus.scrape` and `prometheus.remote_write` blocks. Credentials are injected via environment variables: `GRAFANA_REMOTE_WRITE_URL`, `GRAFANA_REMOTE_WRITE_USER`, `GRAFANA_REMOTE_WRITE_KEY`.

## Metrics Exposed by Services

Each Go service exposes standard HTTP/gRPC metrics and service-specific counters via `/metrics`. The general categories:

- **HTTP request metrics**: total count, latency histograms (by method, path, status)
- **gRPC request metrics**: total count, latency histograms (by method, status)
- **Database query metrics**: latency histograms (by query name)
- **Connection pool metrics**: active database connections
- **Business metrics**: expense creation counts, correction counts, token refresh counts, export job lifecycle

Exact metric names and labels are defined in the `services/metrics/` package (shared) and `services/datarights/internal/metrics/` (datarights-specific).

### Datarights Service Metrics

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

## Alerting

Alerting is managed entirely in Grafana Cloud. Alert rules are defined in `monitoring/grafana-cloud/alerts/` and cover:

- **High error rate**: elevated 5xx response ratio over a sliding window
- **Service down**: no metrics received from a scrape target
- **Slow queries**: p95 query duration exceeding a threshold
- **Auth failures spike**: unusual volume of failed login attempts
- **Export job failure rate**: more than 50% of export jobs failed in the last hour
- **Export job stuck**: active jobs exist but no completions in 10 minutes
- **Host disk filling up**: predicted to fill within 24h
- **Container high CPU/memory**: exceeding 90% of limits

Notifications route to a Discord webhook configured as a contact point in Grafana Cloud.

## Dashboards

Dashboards are managed in Grafana Cloud. JSON exports are stored in `monitoring/grafana-cloud/dashboards/` for version control and disaster recovery.

## Grafana Cloud Access

Access Grafana Cloud directly at your organization's Grafana Cloud URL. Authentication is handled by Grafana Cloud's built-in user management.

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
