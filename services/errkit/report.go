package errkit

import (
	"context"
	"errors"
	"log/slog"

	"github.com/getsentry/sentry-go"
)

// Report logs err through the default slog logger and captures it as a Sentry
// issue on the hub bound to ctx. It returns err unchanged, so a call site can
// write `return errkit.Report(ctx, err, meta)` and errors.Is still matches a
// sentinel through the result.
//
// ctx must be the request context: it carries the per-request Sentry hub
// installed by sentrygin or sentrygrpc. Passing context.Background() from a
// request handler falls back to a clone of the global hub and loses the request,
// trace, and user data, which is correct for a background job and a defect in a
// handler.
func Report(ctx context.Context, err error, m Meta) error {
	if err == nil {
		return nil
	}

	level := m.level()

	// Logged before the hub is resolved and unconditionally, because the log
	// record is the durable artifact: it must not depend on a DSN being set.
	slog.Log(ctx, slogLevel(level), m.message(), logAttrs(m, err)...)

	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		// A clone, never the global hub itself, so the scope stack this report
		// pushes onto cannot be observed by a concurrent report.
		hub = sentry.CurrentHub().Clone()
	}

	reported := attachStack(err, 1)
	tags := m.tags()
	fingerprint := m.fingerprint()

	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(level)
		scope.SetTags(tags)
		scope.SetFingerprint(fingerprint)
		if len(m.Data) > 0 {
			scope.SetContext(contextBlockName, m.Data)
		}
		hub.CaptureException(reported)
	})

	// The original error, not the stacked carrier: a monitoring call must not
	// change the concrete type a caller gets back.
	return err
}

// Ignore reports err only when it is not an expected condition. An expected error
// is logged at info level and never reaches Sentry, so a 4xx-class failure cannot
// consume error quota. Anything else falls through to Report.
func Ignore(ctx context.Context, err error, m Meta, expected ...error) error {
	if err != nil && isExpected(err, expected) {
		slog.Log(ctx, slog.LevelInfo, m.message(), logAttrs(m, err)...)
		return err
	}
	return Report(ctx, err, m)
}

// isExpected reports whether err matches any of expected. errors.Is walks the
// wrap chain, so a sentinel wrapped through %w still matches.
func isExpected(err error, expected []error) bool {
	for _, candidate := range expected {
		if errors.Is(err, candidate) {
			return true
		}
	}
	return false
}

// logAttrs builds the structured attributes of the log record. Meta.Data is
// flattened alongside them so the log stream carries the same detail as the
// Sentry context block, which matters when Sentry is unreachable or the event has
// aged out.
func logAttrs(m Meta, err error) []any {
	attrs := make([]any, 0, len(m.Data)+4)
	attrs = append(attrs,
		slog.String("error", err.Error()),
		slog.String(tagErrorKind, string(m.Kind.resolve())),
	)

	if m.Op != "" {
		attrs = append(attrs, slog.String(tagOperation, m.Op))
	}
	if m.Domain != "" {
		attrs = append(attrs, slog.String(tagDomain, m.Domain))
	}

	for key, value := range m.Data {
		attrs = append(attrs, slog.Any(key, value))
	}

	return attrs
}
