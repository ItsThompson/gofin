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
	return report(ctx, err, m, 1)
}

// Ignore reports err only when it is not an expected condition. An expected error
// is logged at info level and never reaches Sentry, so a 4xx-class failure cannot
// consume error quota. Anything else falls through to Report's behavior.
func Ignore(ctx context.Context, err error, m Meta, expected ...error) error {
	if err != nil && isExpected(err, expected) {
		slog.Log(ctx, slog.LevelInfo, m.message(), logAttrs(m, err)...)
		return err
	}
	return report(ctx, err, m, 1)
}

// report is the shared body behind Report and Ignore. skip is the number of
// frames between report's caller and the call site the recorded stack should
// start at, so every public entry point declares its own distance and the stack
// never roots inside this package regardless of which one was used.
func report(ctx context.Context, err error, m Meta, skip int) error {
	if err == nil {
		return nil
	}

	level := m.level()
	m.Data = mergeData(m.Data, err)

	// Logged before the hub is resolved and unconditionally, because the log
	// record is the durable artifact: it must not depend on a DSN being set.
	slog.Log(ctx, slogLevel(level), m.message(), logAttrs(m, err)...)

	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		// A clone, never the global hub itself, so the scope stack this report
		// pushes onto cannot be observed by a concurrent report.
		hub = sentry.CurrentHub().Clone()
	}

	reported := attachStack(err, skip+1)
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
// flattened alongside the derived attributes so the log stream carries the same
// detail as the Sentry context block, which matters when Sentry is unreachable or
// the event has aged out. A Data key that would duplicate a derived attribute is
// dropped: slog emits both, and a parser that keeps the last would discard the
// error message rather than the caller's copy of it.
func logAttrs(m Meta, err error) []any {
	derived := make([]slog.Attr, 0, 4)
	derived = append(derived,
		slog.String("error", err.Error()),
		slog.String(tagErrorKind, string(m.Kind.resolve())),
	)

	if m.Op != "" {
		derived = append(derived, slog.String(tagOperation, m.Op))
	}
	if m.Domain != "" {
		derived = append(derived, slog.String(tagDomain, m.Domain))
	}

	attrs := make([]any, 0, len(derived)+len(m.Data))
	taken := make(map[string]struct{}, len(derived))
	for _, attr := range derived {
		attrs = append(attrs, attr)
		taken[attr.Key] = struct{}{}
	}

	for key, value := range m.Data {
		if _, duplicate := taken[key]; duplicate {
			continue
		}
		attrs = append(attrs, slog.Any(key, value))
	}

	return attrs
}
