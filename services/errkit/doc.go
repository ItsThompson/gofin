// Package errkit owns the single error-reporting path for all gofin Go
// services: it writes one structured slog record and captures one Sentry issue
// for a failure, enriched with the low-cardinality tag vocabulary the two
// gofin Sentry projects share.
//
// Report takes context.Context rather than *gin.Context, so one call shape
// serves REST and gRPC alike and no shared code depends on gin. The context
// must be the request context: it carries the per-request Sentry hub, and every
// scope mutation happens inside that hub so concurrent requests cannot read one
// another's tags.
//
// Report returns the error it was given, unchanged, so a call site can write
// `return errkit.Report(ctx, err, meta)` and errors.Is still matches a sentinel
// through the result. The slog record is written before the hub is resolved and
// does not depend on a DSN, because the container log stream is the durable
// record: Sentry retains events for 30 days.
//
// Grouping is the reason this package exists rather than a bare
// sentry.CaptureException at each call site. A shared helper becomes the top
// stack frame of every error it reports, and the Sentry server excludes an
// exception message from grouping whenever a stack is present, so a helper-rooted
// stack collapses every backend error into one issue. Two mitigations prevent
// that and both are load-bearing: every event carries the fingerprint
// {"{{ default }}", <logical key>} derived from Meta.Op and Meta.Kind, and
// WithStack gives an error that carries no stack one rooted at the reporting
// call site instead of inside this package.
//
// The apierr package must not import this one. apierr is the single renderer of
// HTTP error responses and every service imports it, so the dependency would make
// the Sentry SDK unavoidable everywhere and couple wire formatting to monitoring.
// If apierr ever needs to report, it takes a reporter function installed from
// main rather than an import.
package errkit
