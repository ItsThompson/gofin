# Dashboard cache rollout

Use this runbook only with an authenticated representative deployment and access to its service containers, Prometheus, Grafana, and browser test account. This procedure records evidence. It does not approve rollout.

## Preconditions

Set the deployment URL and the tested user before starting:

```bash
export APP_URL="https://<deployment-host>"
export TEST_IMMUDB_ADDR="<immudb-host>:3322"
export TEST_USER="<test-user>"
```

Record these values outside the repository:

- deployment version or image digests
- expense and finance container IDs
- expense and finance memory limits
- deployed timezone
- tested user, period, dataset size, and active-row count
- desktop and mobile viewport dimensions
- browser cache and DevTools settings

Do not use a browser `Cache-Control: no-cache` setting for warm-visit evidence. Use it only for explicit bypass checks.

## Gate 1: verify the deployed read and transport path

Run the repository gates before touching cache configuration:

```bash
for service in auth expense finance fx gateway; do
  (cd services/$service && go test ./...)
done
for service in expense finance metrics; do
  (cd services/$service && GOWORK=off go build ./...)
done
(cd services/expense && TEST_IMMUDB_ADDR="$TEST_IMMUDB_ADDR" go test -tags=integration ./internal/repository -run 'TestGetActiveExpensesByPeriodAfter_IntegrationTransportDecision|TestCompletePeriodReadGRPCMessageSize_Integration')
```

The integration test creates 123 active rows, checks bounded keyset pages and transport size, and measures concurrent expense-process memory. Treat a skipped test or missing immudb as **BLOCKED**, not as a pass.

Verify the deployed API and metric endpoints:

```bash
curl --fail "$APP_URL/api/health"
curl --fail "$APP_URL/metrics" > /tmp/gofin-gateway.metrics
```

Capture `/metrics` from expense and finance through the deployment's monitoring path. Confirm named operation labels only. Reject any label containing a user ID, period, expense name, or raw URL.

## Gate 2: collect the uncached baseline

Set both caches off through explicit deployment configuration. Restart only the affected services, then wait for healthy checks:

```bash
export EXPENSE_READ_CACHE_ENABLED=false
export FINANCE_RESULT_CACHE_ENABLED=false
docker compose up -d --force-recreate expense-service finance-service

docker compose ps expense-service finance-service
```

For each device cohort, run five navigations after a service restart. Record one row per navigation:

| run | cohort | viewport | period | users | active rows | browser cache | process state | first usable | all visible | errors |
|---:|---|---|---|---:|---:|---|---|---|---|---|
| 1 | desktop/mobile | width x height | YYYY-MM | count | count | on/off | cold/warm | timestamp | timestamp | names |

Use the same period, dataset, viewport, and visible-section set for all five runs in one cohort. Report median and range for first usable and all visible. Do not report a TTI unless the measured event is explicitly defined.

Capture service source timings, request counts, and process memory for the same runs. This is the pre-cache comparison. Do not compare it with the supplied HAR as a baseline.

## Gate 3: verify rollback behavior with caches off

With both caches disabled, verify the following for the tested user:

1. Complete totals include more than 100 active rows.
2. Public expense pagination keeps its existing page size and response shape.
3. Create, correction, deletion, and pro-rata writes produce fresh results.
4. A failed revision check returns an error and does not render cached totals.
5. Personal successes and errors include `Cache-Control: no-store`.
6. Mobile progressive rendering, section Retry, and Refresh all remain available.

Mark the gate **BLOCKED** if any check cannot run against the deployed version.

## Gate 4: enable one cache and collect warm evidence

Enable only the measured service cache through explicit configuration. Keep the other cache disabled. Preserve these bounds in the deployment configuration:

```text
expense: 256 MB process limit
finance: 512 MB process limit
cache upper age: about 48 hours
entry and byte budgets: record exact values
callback deadline: record exact value
freshness-check timeout: configure `FINANCE_RESULT_CACHE_VALIDATION_TIMEOUT` below the validation lease, then record the exact value
```

Restart the changed service, verify health, and repeat the five-run cold-after-restart and warm-process matrix. Record cache hits, misses, single-flight joins, capacity bypasses, evictions, freshness checks and failures, callback outcomes, source timings, and peak process memory.

Repeat with only the other service cache enabled. A regression disables that cache and returns to Gate 3.

## Gate 5: obtain approval and run the rollback smoke check

A human reviewer must approve the evidence before both caches are enabled. Record the approver, date, deployment version, evidence location, and decision.

Immediately after approval, disable each cache independently and run a rollback smoke check:

```bash
export EXPENSE_READ_CACHE_ENABLED=false
export FINANCE_RESULT_CACHE_ENABLED=true
docker compose up -d --force-recreate expense-service finance-service
# repeat the direct-read, privacy, progressive-rendering, Retry, and Refresh checks

export EXPENSE_READ_CACHE_ENABLED=true
export FINANCE_RESULT_CACHE_ENABLED=false
docker compose up -d --force-recreate expense-service finance-service
# repeat the same checks
```

The rollback passes only when direct reads remain complete and normal error behavior, privacy headers, progressive rendering, and Retry remain intact. If any live gate, environment, access, or approval is unavailable, mark the gate **BLOCKED** and do not claim rollout complete.
