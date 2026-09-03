# Monitoring

## Overview

The observability stack runs on Node 4 (monitoring network) and consists of Prometheus (metrics collection), Alertmanager (alert routing), Grafana (visualization), and a custom auth proxy that gates Grafana access to admin users.

## Prometheus

### Scrape Configuration

Prometheus scrapes `/metrics` endpoints from all Go services on the monitoring network. Targets and intervals are defined in `monitoring/prometheus/prometheus.yml`.

### Metrics Exposed by Services

Each Go service exposes standard HTTP/gRPC metrics and service-specific counters via `/metrics`. The general categories:

- **HTTP request metrics**: total count, latency histograms (by method, path, status)
- **gRPC request metrics**: total count, latency histograms (by method, status), for unary and server-streaming methods alike
- **Recovered panics**: total count by recovery site
- **Business metrics**: expense creation counts, correction counts, token refresh counts, export job lifecycle

Exact metric names and labels are defined in the `services/metrics/` package (shared) and `services/datarights/internal/metrics/` (datarights-specific).

#### gRPC Stream Metrics

`services/metrics` provides a stream server interceptor beside the unary one, and `serverkit.NewGRPCServer` installs it inside the recovery stream interceptor, matching the unary chain. `StreamAllUserExpenses` (the one streaming RPC, consumed by datarights to build a user data export) therefore records `grpc_requests_total` and `grpc_request_duration_seconds` like every unary method.

A server interceptor can only time the whole stream, so the recorded duration is the stream's lifetime from first message to terminal status, not per-message latency. `job_method:grpc_request_duration_seconds:p95` is per method, so an export that runs for a minute does not move any other method's p95.

#### Recovered Panic Counter

`recovered_panics_total{site}` is incremented in `serverkit.LogRecoveredPanic`, the sole recovery reporter for every recovery site in the tree: both gRPC interceptors, the HTTP middleware, the datarights job runner and its provider fan-out, the expense stream's page producer, the finance fan-outs, the auth cleanup run, and the gateway readiness probe. Counting there rather than in an interceptor is what makes the coverage complete; an interceptor sees only the two request-scoped gRPC paths.

`site` carries the same values as the Sentry group keys without their `panic.` prefix, so its cardinality is fixed at compile time. There is no `service` label, because Prometheus adds `job` from the scrape config.

A dead client connection (`EPIPE`, `ECONNRESET`, `http.ErrAbortHandler`) does not increment the counter. Recovery classifies that case and returns before reporting, so the counter holds defects only, which is what lets `RecoveredPanic` page on a single increment (see Alerting Rules).

Request metrics record nothing for a panicking call. Recovery sits outside the metrics interceptor and the metrics middleware on purpose, because a panic raised in the metrics layer itself has to be caught too, and both record after the handler returns. A panicking call therefore leaves no `grpc_requests_total` or `http_requests_total` observation, and `HighErrorRate` cannot see it: `recovered_panics_total` is the signal that pages, and the panic value and its stack are in the log record and in Sentry (see Structured Logging).

#### Datarights Service Metrics

Custom business metrics for data export monitoring:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `export_jobs_created_total` | Counter | — | Export jobs submitted via the API |
| `export_jobs_completed_total` | Counter | `status` (completed/failed) | Jobs reaching terminal state |
| `export_rate_limit_rejections_total` | Counter | — | Requests rejected by 30-day cooldown |
| `export_currency_formatting_fallback_total` | Counter | - | Legacy default-settings amounts rendered with the two-decimal fallback because the stored currency is unsupported |
| `export_job_duration_seconds` | Histogram | — | End-to-end job duration (pending to terminal) |
| `export_data_collection_duration_seconds` | Histogram | `provider` | Per-provider data collection latency |
| `export_email_send_duration_seconds` | Histogram | — | Email delivery latency via Resend |
| `export_zip_size_bytes` | Histogram | — | Size of generated ZIP files |
| `export_pool_active_jobs` | Gauge | — | Currently running export goroutines |
| `export_pool_queued_jobs` | Gauge | — | Jobs waiting for a pool slot |

#### FX Service Metrics

FX exposes conversion, provider, and cache counters from `services/fx/internal/metrics/metrics.go`:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `fx_conversion_requests_total` | Counter | `source_currency`, `target_currency`, `result` (`success`/`failure`) | Conversion attempts by pair and outcome |
| `fx_provider_requests_total` | Counter | `result`, `status_code` | Open Exchange Rates requests by outcome |
| `fx_cache_hits_total` | Counter | - | Fresh provider snapshot cache hits |
| `fx_cache_misses_total` | Counter | - | Provider snapshot cache misses |
| `fx_conversion_latency_seconds` | Histogram | `source_currency`, `target_currency` | Conversion latency |
| `fx_provider_latency_seconds` | Histogram | `result` | Open Exchange Rates request latency |

`fx_provider_requests_total` uses `result` values `success`, `error`, `auth_failed`, `retryable_error`, and `invalid`. FX logs carry the currency pair and the failure reason (error text, or provider HTTP status on auth failure) only: no request IDs, cache status, expense names, user emails, or provider credentials.

#### Known gaps

**Database query and connection-pool metrics do not exist.** No service exports `db_query_duration_seconds` or `active_connections`, and a test in `services/metrics` asserts that `active_connections` is never exported. Every consumer of those two names is therefore permanently empty: the `SlowQueries` alert, which cannot fire at all; the `job_query:db_query_duration_seconds:p95` and `query:db_query_duration_seconds:p95_expense` recording rules; all three Database Health panels; and the Expense Service "immudb Write Latency" panel. Closing this needs the metrics written, not the consumers retired.

### Access

Prometheus UI: `http://localhost:9090`

## Alerting Rules

Alert rules are defined in `monitoring/prometheus/alerts.yml`. The rules cover:

- **High error rate**: per-job 5xx ratio above a threshold, gated by a minimum per-job request rate
- **Service down**: no metrics received from a scrape target
- **Recovered panic**: any panic recovered by a Go service, by recovery site
- **Slow queries**: p95 query duration exceeding a threshold. This rule cannot fire, because no service exports `db_query_duration_seconds` (see Known gaps)
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

- A job serving 2 requests per minute or fewer never fires this alert, whatever its error ratio. That is the deliberate cost of the floor, and no container that answers its healthcheck is in that state: see the next bullet before acting on this one. `ServiceDown` covers such a service going away entirely.
- Every job carries about 12 requests per minute before any user traffic, because each Go service's Docker healthcheck polls its own `/health` every 5 seconds and `/health` is counted (only `/metrics` is excluded). On an idle stack that healthcheck is the job's entire request rate. Two consequences follow.
  - The floor never excludes a service whose container is answering, so it guards far less than the arithmetic above suggests.
  - Those healthy 200s dilute the ratio inside the job. A job needs more than about 0.6 failed requests per minute before it can reach 5%, and a single failed request in the window computes to about 1.75%.
- A job with no traffic in the window produces no alert. Its ratio is `0/0`, which is `NaN`, and `NaN` fails every comparison, so the rule yields no series rather than a false page.
- Both sides are summed by `job`, so two services failing at once produce two alert instances. `group_by: ["alertname", "job"]` then sends one Discord message per service, and both Discord titles name the job.
- `mfe` exposes no `http_requests_total`, so it is absent from this alert.
- Alerts that keep a `job` label from their exporter, such as `ContainerHighMemory` and `HostDiskAlmostFull`, show that exporter in the Discord title: `· cadvisor` and `· node-exporter`. The failing container or mountpoint is named in the message body. This is the accepted cost of putting the job in the title, which is what names the service for `HighErrorRate`.

### RecoveredPanic

`RecoveredPanic` fires when `sum by (job, site) (increase(recovered_panics_total[5m]))` exceeds 0, at `critical`, with no `for` delay.

| Setting | Value | Meaning |
|---------|-------|---------|
| Threshold | `> 0` | Any recovered panic in the window. Every increment is a defect: an aborted client connection is classified before the counter is touched, so there is nothing benign to tolerate |
| `for` duration | `0s` | Pages on the first evaluation after the panic. There is no flap source to debounce |
| Window | `5m` | The rule reads the increase, not the counter, so a panic stops paging about five minutes after the last one. A service that panicked last week does not page forever |
| Grouping | `job`, `site` | `site` names where the panic was recovered, so the Discord message says which path failed. `instance` is summed away |

During triage: the counter says a panic happened and where, and nothing else. Read the service's log stream for the `panic` and `stack` attributes of that record, or open the Sentry issue under the matching `panic.` group key. Two sites failing in one service produce one Discord message with two lines, because Alertmanager groups by `alertname` and `job`.

### Testing Alert Rules

`monitoring/prometheus/tests/` holds `promtool` unit tests for the alert rules. They evaluate the rule files against synthetic series, with no Prometheus instance and no scraped data. Run both commands from the repository root:

```bash
# Rule syntax, via the scrape config that references every rule file
docker run --rm -v "$PWD/monitoring:/monitoring" -w /monitoring/prometheus \
  --entrypoint promtool prom/prometheus:v3.11.3 check config prometheus.yml

# Rule behavior: firing cases, non-firing cases, rendered annotations
docker run --rm -v "$PWD/monitoring:/monitoring" -w /monitoring/prometheus/tests \
  --entrypoint promtool prom/prometheus:v3.11.3 test rules \
  high_error_rate_test.yml recovered_panic_test.yml
```

`promtool` ships inside the `prom/prometheus` image, so no local install is needed. Use the image tag that `docker-compose.yml` pins for Prometheus, so the tests run on the same evaluation engine as production. `just test-monitoring` runs both commands.

When reproducing `HighErrorRate` by hand against a running stack, drive at least 3 requests per minute at the target service. Below the floor the rule stays silent, which looks identical to a broken rule.

### Datarights Alert Rules

| Alert | Condition | Severity | Description |
|-------|-----------|----------|-------------|
| `ExportJobFailureRate` | >50% of jobs failed in 1h window | warning | Indicates systemic issue with upstream services or email delivery |
| `ExportJobStuck` | Active jobs with no completions for 10m | warning | Likely a hung goroutine or upstream service timeout |

### FX and Multi-Currency Alert Context

FX Service is a scrape target in `monitoring/prometheus/prometheus.yml`, so the existing `ServiceDown` rule (`up == 0`, critical) covers it going down: no separate FX-down rule is needed. The shared rule pages at critical for every service, so FX inherits that.

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

Logs are viewable via `just logs <service>` or `docker compose logs -f <service>`. There is no centralized log aggregation (ELK, Loki).

### Recovered panics

Every recovered panic writes one panic record wherever it happened: the
HTTP middleware, either gRPC interceptor, the expense stream's page producer,
the finance fan-out tasks (spending trends, historical comparison, health-score
desires, and the three export reads), the datarights job runner and its provider
fan-out, the auth blacklist cleanup run, and the gateway readiness fan-out. Each
carries two shared attributes, so one query returns them all:

- `panic`: the panic value, wrapped into an error when it is not one already
- `stack`: the stack captured at recovery, rooted at the frame that panicked

A record from a fan-out that runs once per period also carries a `period`
attribute, so a panic in one month's read is distinguishable from another's.

Reporting the panic writes a second error-level record beside it, the ordinary
one `errkit` writes for every reported failure. It carries `error_kind`,
`operation` and `domain` instead of `panic` and `stack`, so a query on the
taxonomy returns panics alongside every other reported error. `operation` is the
panic site, matching the Sentry group key.

A dead client connection (`EPIPE`, `ECONNRESET`, `http.ErrAbortHandler`) is not
a service defect and is recorded at warn level with no stack. It is neither
reported nor counted: nothing is wrong with the service, and `RecoveredPanic`
pages on a single increment of `recovered_panics_total`.

### Bounded reports

Two error paths fire per request or per heartbeat rather than per incident. Each
is recorded every time and reported at most once an hour:

- The gateway's `downstream service unreachable`, once per hour per target. A
  downstream that is down while one browser tab polls produces about 1,440
  records an hour, and `ServiceDown` already pages for the same outage.
- expense's `immudb reconnection failed`, once per hour. The SDK heartbeat drives
  it about once a minute for as long as immudb is unreachable, and a session error
  reaches the same line per request.

Both collapse into a single Sentry issue per class, so a long outage is one issue
with a low event count rather than a flood. Read the record count for volume, not
the event count.

The gateway's request logger reports nothing. It holds a status code and no error
value, and the service that failed has already reported that failure with its own
stack, so one failing request is one event rather than three.

## Resource Limits

Container CPU and memory limits are defined in `docker-compose.yml` under each service's `deploy.resources` section. Current limits are modest, sized for a maximum of 5 concurrent users.
