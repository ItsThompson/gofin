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

#### Known gaps

Two classes of request are absent from the metrics above. Both are deliberate.

- **Server-streaming RPCs are unmeasured.** `services/metrics` provides a unary
  gRPC interceptor only, so `StreamAllUserExpenses` (the one streaming RPC,
  consumed by datarights to build a user data export) contributes nothing to
  `grpc_requests_total` or `grpc_request_duration_seconds`. The stream
  interceptor chain carries panic recovery alone.
- **A recovered panic is counted nowhere.** Recovery sits outside the metrics
  interceptor and the metrics middleware, both of which record after the handler
  returns, so a panic unwinds past them before they observe anything. Recovery
  is outside on purpose: a panic raised in the metrics layer itself has to be
  caught too. Panics are queryable in the log stream instead (see Structured
  Logging).

  For unary gRPC this changes which alert fires. A panic used to kill the
  process, so `up` went to 0 and `ServiceDown` paged within a minute. The
  process now survives, `grpc_requests_total` records nothing, and
  `HighErrorRate` reads `http_requests_total` only: a service panicking on
  every unary call therefore fires **no alert at all**, and the log record is
  the sole signal. A live process is still the right trade, and error reporting
  restores the signal, but treat "counted nowhere" as "not paged" until then.
  HTTP has no such change: `gin.Recovery()` was already outermost, so
  `HTTPMetrics` never saw a panicking request before this either.

### Access

Prometheus UI: `http://localhost:9090`

## Alerting Rules

Alert rules are defined in `monitoring/prometheus/alerts.yml`. The rules cover:

- **High error rate**: per-job 5xx ratio above a threshold, gated by a minimum per-job request rate
- **Service down**: no metrics received from a scrape target
- **Slow queries**: p95 query duration exceeding a threshold
- **Auth failures spike**: unusual volume of failed login attempts
- **Export job failure rate**: more than 50% of export jobs failed in the last hour
- **Export job stuck**: active jobs exist but no completions in 10 minutes

### HighErrorRate Thresholds

`HighErrorRate` compares 5xx requests to total requests **per job**, and requires a minimum per-job request rate before it can fire. A global ratio hides a fully failing low-traffic service behind healthy traffic elsewhere: 3 requests per minute all returning 5xx, beside 200 requests per minute of healthy traffic, is a global ratio of 1.5%. A per-job ratio without a floor has the opposite problem, because per-service ratios are spiky at this traffic level.

| Setting | Value | Meaning |
|---------|-------|---------|
| Error ratio threshold | `> 0.05` | More than 5% of that job's requests returned 5xx over the 5-minute window |
| Request-rate floor | `> 2 / 60` req/s | More than 2 requests per minute for that job, so more than about 10 requests in the 5-minute window |
| `for` duration | `5m` | Both conditions hold continuously for 5 minutes before the alert fires |

Consequences worth knowing during triage:

- A job serving 2 requests per minute or fewer never fires this alert, whatever its error ratio. That is the deliberate cost of the floor. `ServiceDown` still covers such a service going away entirely.
- A job with no traffic in the window produces no alert. Its ratio is `0/0`, which is `NaN`, and `NaN` fails every comparison, so the rule yields no series rather than a false page.
- Both sides are summed by `job`, so two services failing at once produce two alert instances. `group_by: ["alertname", "job"]` then sends one Discord message per service, and both Discord titles name the job.
- `mfe` exposes no `http_requests_total`, so it is absent from this alert.
- Alerts that keep a `job` label from their exporter, such as `ContainerHighMemory` and `HostDiskAlmostFull`, now show that exporter in the Discord title: `· cadvisor` and `· node-exporter`. The failing container or mountpoint is named in the message body. Accepted cost of putting the job in the title, which is what names the service for `HighErrorRate`.

### Testing Alert Rules

`monitoring/prometheus/tests/` holds `promtool` unit tests for the alert rules. They evaluate the rule files against synthetic series, with no Prometheus instance and no scraped data. Run both commands from the repository root:

```bash
# Rule syntax, via the scrape config that references every rule file
docker run --rm -v "$PWD/monitoring:/monitoring" -w /monitoring/prometheus \
  --entrypoint promtool prom/prometheus:v3.11.3 check config prometheus.yml

# Rule behavior: firing cases, non-firing cases, rendered annotations
docker run --rm -v "$PWD/monitoring:/monitoring" -w /monitoring/prometheus/tests \
  --entrypoint promtool prom/prometheus:v3.11.3 test rules high_error_rate_test.yml
```

`promtool` ships inside the `prom/prometheus` image, so no local install is needed. Use the image tag that `docker-compose.yml` pins for Prometheus, so the tests run on the same evaluation engine as production. `just test-monitoring` runs both commands.

When reproducing `HighErrorRate` by hand against a running stack, drive at least 3 requests per minute at the target service. Below the floor the rule stays silent, which looks identical to a broken rule.

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

### Recovered panics

Every recovered panic writes one error-level record wherever it happened: the
HTTP middleware, either gRPC interceptor, the expense stream's page producer,
the finance fan-out tasks (spending trends, historical comparison, health-score
desires, and the three export reads), the datarights job runner and its provider
fan-out, the auth blacklist cleanup run, and the gateway readiness fan-out. Each
carries two shared attributes, so one query returns them all:

- `panic`: the panic value, wrapped into an error when it is not one already
- `stack`: the stack captured at recovery, rooted at the frame that panicked

A record from a fan-out that runs once per period also carries a `period`
attribute, so a panic in one month's read is distinguishable from another's.

A dead client connection (`EPIPE`, `ECONNRESET`, `http.ErrAbortHandler`) is not
a service defect and is recorded at warn level with no stack.

## Resource Limits

Container CPU and memory limits are defined in `docker-compose.yml` under each service's `deploy.resources` section. Current limits are modest, sized for a maximum of 5 concurrent users.
