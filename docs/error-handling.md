# Error Handling and Logging

This page describes how a gofin backend failure becomes visible: which layer records it, which layer reports it to Sentry, and which layer stays silent. Use it to decide, for any new error path, what to call and where.

Canonical source: `services/errkit` (reporting), `services/apierr` (wire contract), `services/httpx` (REST gate).

## The single-owner rule

Every server-side failure has exactly one reporting owner:

- **A returned error is reported by the handler.** The handler is the last layer that sees the error, and it gates on the rendered status: a 5xx is reported, a 4xx is not. A service-level `errkit.Report` beside a returned error would be a second event for one failure.
- **A swallowed error (fire-and-forget) is reported by the service**, at the point of failure, where the context and metadata are richest. Nothing downstream will ever see the error, so the service is the only possible reporter.

The rule exists because Sentry's event allowance is shared across the whole organization, and duplicate events both waste it and distort issue counts.

## The two handler gates

REST and gRPC handlers do not call `errkit.Report` directly. Two shared gates wrap it, and both gate on `apierr.IsServerError(err)` (true exactly when the rendered response is a 5xx), so a typed 4xx can never consume error quota:

- **REST: `httpx.RespondError(c, err, meta)`** (`services/httpx/respond.go`). Reports when the response is a 5xx, then writes the shared apierr wire body. It fills `meta.Op` from the shared route registry when the caller omits it, so the operation tag matches the route without per-handler strings.
- **gRPC: a local `reportServerFailure(ctx, err, meta)` helper** (finance and expense have the reference copies; auth mirrors them). Same gate: report only when `apierr.IsServerError(err)` is true, then return the `codes.Internal` status the handler already returned.

Because the gate is the rendered status rather than a list of codes, it stays correct as the code set grows: a validation or not-found error reaching an internal exit is client input, not a service defect, and is not billed.

## apierr: the wire contract, never a reporter

`apierr` is the single renderer of HTTP error responses (and the typed error type services return). It must not import `errkit`: apierr is imported by every service, so the dependency would make the Sentry SDK unavoidable everywhere and couple wire formatting to monitoring. If apierr ever needs to report, it takes a reporter function installed from `main` rather than an import.

## errkit: Report, Ignore, Limiter

`services/errkit` owns the single reporting path. `errkit.Report(ctx, err, meta)` writes one structured slog record (through the package-level default logger) and captures one Sentry event on the hub carried by `ctx`. It returns `err` unchanged, so `return errkit.Report(ctx, err, meta)` keeps `errors.Is` working.

- **`Report`**: the default. Use for every failure the service owns (swallowed errors, background jobs, and the gated handler sites above).
- **`Ignore(ctx, err, meta, expected...)`**: use when one operation can fail with both an expected sentinel and a real failure. An `errors.Is` match against an expected sentinel logs at info and skips Sentry. Note the limitation: `*apierr.Error` values compare by pointer identity, so `Ignore` cannot match apierr sentinels; expected 4xx must be classified before the report instead.
- **`Limiter`**: bounds one reporting site to one report per window (one hour by convention). Use when a failure path fires per request or per heartbeat rather than per incident. See Bounded reports below.

Errors can implement `errkit.DataCarrier` (`ReportData() map[string]any`) so any report of that error type automatically carries structured detail. Example: expense's `SnapshotIntegrityError` contributes `expense_id` and `missing_fields` to every event that reports it; the call site adds nothing.

## Meta and the tag vocabulary

Every event carries three taxonomy tags, derived from `errkit.Meta`:

- **`operation`** (`Meta.Op`): the logical operation, dot notation, e.g. `expense.create`, `finance.prorata_apply`. Must come from a bounded set: never interpolate an identifier, which would create one Sentry issue per record.
- **`domain`** (`Meta.Domain`): the business area. The closed set:

| Domain | Services | Business area |
|--------|----------|---------------|
| `auth` | auth | Accounts, sessions, tokens |
| `budgets` | finance | Periods, tags, pro-rata, health score, dashboards |
| `expenses` | expense | The expense ledger and corrections |
| `datarights` | datarights | Export and deletion jobs |
| `fx` | fx | Provider fetch and conversion |
| `platform` | gateway | Proxying and access control |

- **`error_kind`** (`Meta.Kind`): the low-cardinality failure class (`internal`, `database`, `upstream`, `timeout`, `validation`, ...). The set is closed in `services/errkit/kind.go`; adding a value widens the query vocabulary of both Sentry projects.

Grouping: every event carries the fingerprint `{"{{ default }}", op/kind}`, which refines Sentry's own grouping. `GroupKey` + `GroupExact: true` replaces grouping entirely with one key; use it only for a generic failure whose stack varies but whose meaning is singular (see Bounded reports).

`Meta.Data` is arbitrary structured metadata, sent as the Sentry context block `gofin` (capped at 8 kB): identifiers, amounts, the endpoint. Put identifiers here, never in tags.

## The three call-site categories

For any new `logger.Error` / `logger.Warn` / `errkit.Report` decision:

1. **Server failure returned to a handler**: do not record at the service. The handler's gate (`httpx.RespondError` or `reportServerFailure`) reports it once.
2. **Server failure swallowed by the service** (fire-and-forget, background job, best-effort side write): call `errkit.Report` at the point of failure. Keep the existing control flow (return nil, continue, proceed); only the recording changes.
3. **Expected 4xx in a handler**: record nothing. The 4xx response is the record; a report would bill quota for ordinary client input.

A fourth shape exists deliberately: **advisories**, warn-level records that are not failures and stay on `slog`. See the keep-list below.

## Bounded reports

Some failure paths fire per request or per heartbeat rather than per incident. An unbounded report there spends the whole event allowance on one already-alerting outage. The documented shape:

```go
reports := errkit.NewLimiter(time.Hour)
// at the failure site:
logger.Error("site record", ...)        // unconditional: per-occurrence, shows volume
if reports.Allow() {
    _ = errkit.Report(ctx, err, errkit.Meta{GroupKey: "class.key", GroupExact: true, ...})
}
```

The record is per occurrence (it is the durable artifact; log volume is not the constrained resource); the report is once per window, and `GroupExact` collapses the class into one stable Sentry issue. One Limiter per site: the window belongs to the site it guards, so two sites sharing an instance would suppress each other's first report.

Current bounded sites:

- gateway proxy, `gateway.downstream_unreachable` (per target; Prometheus pages for the same outage)
- expense immudb reconnect, once per hour (heartbeat-driven)
- fx provider, two classes: `fx.provider_unreachable` (network + 5xx, exhausted at the terminal retry exit) and `fx.provider_auth_failed` (401/403), each with its own Limiter

## Background jobs

Background work (job pools, cleanup tickers, startup recovery) has no request hub on its context. `errkit.Report` falls back to `sentry.CurrentHub().Clone()`, which is the documented correct path for background jobs: the event keeps its tags and context; it simply lacks request/trace data. Pass the most specific live context available (the job's own context while it is still live, `context.Background()` when it may have expired), not a request context borrowed from elsewhere.

## Keep-list: warn/error slog sites that are not failures

These sites stay on `slog` deliberately. They are advisories, startup diagnostics, or per-attempt retry notes, not server failures a Sentry event would help:

| Site | Reason |
|------|--------|
| Every `cmd/main.go` sentry-init error | Sentry is not initialized when it fires; `errkit` cannot capture it |
| `gateway/internal/access/control.go` 401/403 warns | Access decisions with no error value; middleware choices, not service failures |
| `gateway/internal/config/config.go` oversized-timeout warn | Startup config advisory |
| `gateway/internal/readiness/readiness.go` probe warn | Health probe result |
| `fx/cmd/main.go` empty-API-key warn | Startup advisory (dev rates in use) |
| `expense/cmd/immudb.go` connection retry warns | Transient per-attempt retries |
| `expense/cmd/immudb_prod.go` heartbeat/session-loss warns | Session diagnostics beside the bounded reconnect report (dual pattern) |
| `auth/internal/service/auth.go` refresh-token replay warn | Security advisory (replay detection) |
| `finance/internal/service/prorata.go` schedule-marked-failed warn | Status-transition advisory beside the failure reports |
| `datarights/internal/deletion/engine.go` per-attempt provider warn | Transient per-attempt retry |
| `datarights/internal/handler/rest.go` rate-limit warn | 429 decision, not a failure |
| `expense/internal/repository/schema.go` index-creation warn | Idempotent startup advisory |
| `datarights/internal/engine/engine.go` export failure record | Dual pattern: user-facing record beside the errkit report |
| `gateway/internal/proxy/proxy.go` unreachable record | Dual pattern: site record beside the bounded report |
| `fx/internal/provider/openrates.go` fetch/auth site records | Dual pattern: per-occurrence records beside the bounded reports |
| `serverkit/recover.go` dead-client-connection warn | Client went away; not a service defect (see monitoring.md) |

Two more dual-pattern records join this list by construction: any new bounded-report site keeps its unconditional per-occurrence record beside the gated report.

## Known follow-ups

- **The expense REST handler reports the fx outage per request.** When the fx provider is down, each failed expense REST conversion surfaces a typed 503 and `httpx.RespondError` reports it, so a long outage emits one event per request. Bounding it (gateway-proxy style) is a separate change; the provider's bounded reports are unaffected.
- **fx `ErrorProviderResponseInvalid` paths stay dark on the swallowed surface.** Unparseable bodies, malformed payloads, and non-200 non-5xx statuses render as a 503 the expense REST handler reports, but on the expense gRPC / pro-rata surface (where conversion errors are swallowed) nothing reports them. They signal a provider contract defect rather than an outage and are rare; accepted for now.
