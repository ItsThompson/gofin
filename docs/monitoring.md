# Monitoring

## Overview

The observability stack runs on Node 4 (monitoring network) and consists of Prometheus (metrics collection), Alertmanager (alert routing), Grafana (visualization), and a custom auth proxy that gates Grafana access to admin users.

## Prometheus

### Scrape Configuration

Prometheus scrapes `/metrics` endpoints from all Go services on the monitoring network:

| Target | Endpoint | Interval |
|--------|----------|----------|
| API Gateway | `api-gateway:8080/metrics` | Default |
| Auth Service | `auth-service:8081/metrics` | Default |
| Expense Service | `expense-service:8082/metrics` | Default |
| Finance Service | `finance-service:8083/metrics` | Default |

Configuration is at `monitoring/prometheus/prometheus.yml`.

### Metrics Exposed by Services

Each Go service exposes standard and custom Prometheus metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | Counter | method, path, status | Total HTTP requests |
| `http_request_duration_seconds` | Histogram | method, path | Request latency |
| `grpc_requests_total` | Counter | method, status | Total gRPC calls |
| `grpc_request_duration_seconds` | Histogram | method | gRPC call latency |
| `db_query_duration_seconds` | Histogram | query_name | Database query latency |
| `active_connections` | Gauge | service | Current DB connection pool usage |
| `expense_entries_total` | Counter | user_id, type | Total expenses created |
| `corrections_total` | Counter | user_id | Total corrections created |
| `token_refresh_total` | Counter | status | Token refresh attempts (success/failure) |

### Access

Prometheus UI: `http://localhost:9090`

## Alerting Rules

Alert rules are defined in `monitoring/prometheus/alerts.yml`:

| Alert | Condition | Severity |
|-------|-----------|----------|
| High error rate | > 5% of requests return 5xx over 5 minutes | Critical |
| Service down | No metrics received for 1 minute | Critical |
| Slow queries | p95 query duration > 1s over 5 minutes | Warning |
| High memory usage | Container memory > 80% of limit | Warning |
| Auth failures spike | > 10 failed logins in 1 minute | Warning |

Alertmanager configuration is at `monitoring/alertmanager/alertmanager.yml`.

## Grafana

### Access

- Local: `http://localhost:3001`
- Remote: via Cloudflare tunnel through the auth proxy (admin-only)

### Pre-Provisioned Dashboards

Dashboards are provisioned automatically from `monitoring/grafana/dashboards/`:

| Dashboard | Panels |
|-----------|--------|
| System Overview | Request rate, error rate, latency p50/p95/p99 per service |
| Auth Service | Login rate, registration rate, token refresh rate, failed auth attempts |
| Expense Service | Expenses created/min, corrections/min, immudb write latency |
| Finance Service | Period creations, aggregation query latency, pro-rata applications |
| Database Health | PostgreSQL connections, query duration, immudb write throughput |

Dashboard JSON files are at `monitoring/grafana/dashboards/`. Datasource provisioning is at `monitoring/grafana/provisioning/`.

### Auth Proxy

The Grafana auth proxy (`monitoring/auth-proxy/`) is a small Go service that:

1. Extracts the `gofin_access` JWT from the request cookie
2. Validates the signature using the shared `JWT_SECRET`
3. Checks that `role === 'admin'`
4. On success: proxies to Grafana with `X-WEBAUTH-USER` header
5. On failure: returns 403

The proxy validates JWTs locally (no gRPC call to Auth Service), keeping the observability node independent of the compute node at runtime.

## Structured Logging

All Go services emit JSON-structured logs to stdout:

```json
{
  "level": "info",
  "timestamp": "2026-05-03T12:00:00Z",
  "service": "finance-service",
  "method": "CreatePeriod",
  "user_id": "abc-123",
  "duration_ms": 45,
  "message": "budget period created"
}
```

| Field | Always Present | Description |
|-------|---------------|-------------|
| `level` | Yes | `info`, `warn`, `error` |
| `timestamp` | Yes | ISO 8601 |
| `service` | Yes | Service name |
| `method` | Yes | Handler or function name |
| `user_id` | When authenticated | User performing the action |
| `assumed_by` | During assumption | Admin who assumed this identity |
| `duration_ms` | For operations | Execution time |
| `error` | On errors | Error message and stack |

Logs are viewable via `just logs <service>` or `docker compose logs -f <service>`. Centralized log aggregation (ELK, Loki) is deferred for MVP.

## Resource Limits

| Container | CPU | Memory |
|-----------|-----|--------|
| MFE (Node.js) | 0.5 | 512 MB |
| API Gateway | 0.25 | 128 MB |
| Auth Service | 0.25 | 128 MB |
| Expense Service | 0.25 | 128 MB |
| Finance Service | 0.5 | 256 MB |
| PostgreSQL | 1.0 | 512 MB |
| immudb | 0.5 | 256 MB |
| Prometheus | 0.5 | 256 MB |
| Grafana | 0.5 | 256 MB |

These are modest for a maximum of 5 concurrent users. Resource limits are defined in `docker-compose.yml`.
