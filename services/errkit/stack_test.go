package errkit_test

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit"
)

// errkitPackagePath is compared exactly, not by substring: the external test
// package is "…/services/errkit_test", which contains the production path.
const errkitPackagePath = "github.com/ItsThompson/gofin/services/errkit"

// callWithStack exists so the frame the assertion names is a stable, unique
// function rather than a test body or a closure.
func callWithStack(err error) error {
	return errkit.WithStack(err)
}

// pcStackError exposes the StackTrace() []uintptr shape the SDK reads directly,
// which is also the shape errkit's own carrier uses.
type pcStackError struct{ pcs []uintptr }

func (e *pcStackError) Error() string         { return "pc stack" }
func (e *pcStackError) StackTrace() []uintptr { return e.pcs }

// pkgFrame and pkgStackTrace mirror github.com/pkg/errors: a named slice whose
// element type is uintptr-kinded. Reproduced here rather than imported so errkit
// gains no dependency on pkg/errors to be tested against its shape.
type pkgFrame uintptr

type pkgStackTrace []pkgFrame

type pkgStackError struct{ frames pkgStackTrace }

func (e *pkgStackError) Error() string             { return "pkg stack" }
func (e *pkgStackError) StackTrace() pkgStackTrace { return e.frames }

// causeError exposes the third chain link the SDK walks, alongside the two
// Unwrap shapes.
type causeError struct{ cause error }

func (e *causeError) Error() string { return "cause: " + e.cause.Error() }
func (e *causeError) Cause() error  { return e.cause }

// nilUnwrapError ends its own chain with a nil link, which the walk must skip
// rather than dereference.
type nilUnwrapError struct{}

func (e *nilUnwrapError) Error() string { return "no cause" }
func (e *nilUnwrapError) Unwrap() error { return nil }

// driverError stands in for a concrete driver error so an exception entry's Type
// is distinguishable from *errors.errorString.
type driverError struct{}

func (e *driverError) Error() string { return "duplicate key value violates unique constraint" }

func realPCs() []uintptr {
	pcs := make([]uintptr, 8)
	n := runtime.Callers(1, pcs)
	return pcs[:n]
}

func newPCStackError() *pcStackError { return &pcStackError{pcs: realPCs()} }

func newPkgStackError() *pkgStackError {
	pcs := realPCs()
	frames := make(pkgStackTrace, 0, len(pcs))
	for _, pc := range pcs {
		frames = append(frames, pkgFrame(pc))
	}
	return &pkgStackError{frames: frames}
}

// topInAppFrame returns the newest in-app frame. Sentry orders frames oldest
// first, so the frame where the failure happened is the last one.
func topInAppFrame(t *testing.T, stacktrace *sentry.Stacktrace) sentry.Frame {
	t.Helper()
	require.NotNil(t, stacktrace, "expected a stacktrace")

	for i := len(stacktrace.Frames) - 1; i >= 0; i-- {
		if stacktrace.Frames[i].InApp {
			return stacktrace.Frames[i]
		}
	}

	t.Fatalf("no in-app frame in %d frames", len(stacktrace.Frames))
	return sentry.Frame{}
}

func TestWithStack_NilStaysNil(t *testing.T) {
	assert.NoError(t, errkit.WithStack(nil))
}

func TestWithStack_RootsTheStackAtItsCaller(t *testing.T) {
	got := callWithStack(errors.New("boom"))

	frame := topInAppFrame(t, sentry.ExtractStacktrace(got))
	assert.Equal(t, "callWithStack", frame.Function)
}

// An off-by-one skip depth is the defect this guards: it would make every
// captured error's stack start inside errkit and therefore look identical.
func TestWithStack_RecordsNoErrkitFrame(t *testing.T) {
	got := callWithStack(errors.New("boom"))

	stacktrace := sentry.ExtractStacktrace(got)
	require.NotNil(t, stacktrace)

	for _, frame := range stacktrace.Frames {
		assert.NotEqual(t, errkitPackagePath, frame.Module, "errkit frame %q leaked into the stack", frame.Function)
	}
}

// Unwrap on the carrier is mandatory: without it the SDK's chain walk terminates
// at the carrier and every wrapped error collapses into one exception entry.
func TestWithStack_CarrierUnwrapsToTheOriginal(t *testing.T) {
	sentinel := errors.New("sentinel")
	wrapped := fmt.Errorf("insert expense: %w", sentinel)

	got := errkit.WithStack(wrapped)

	require.NotSame(t, wrapped, got, "expected the error to be wrapped in a carrier")
	assert.Same(t, wrapped, errors.Unwrap(got))
	assert.ErrorIs(t, got, sentinel)
	assert.Equal(t, wrapped.Error(), got.Error())
}

func TestWithStack_LeavesAnAlreadyStackedErrorUnchanged(t *testing.T) {
	tests := []struct {
		name  string
		build func() error
	}{
		{
			name:  "outermost carries []uintptr",
			build: func() error { return newPCStackError() },
		},
		{
			name:  "outermost carries a pkg/errors shaped StackTrace",
			build: func() error { return newPkgStackError() },
		},
		{
			name:  "Unwrap() error reaches a stacked link",
			build: func() error { return fmt.Errorf("insert expense: %w", newPCStackError()) },
		},
		{
			name:  "Unwrap() []error reaches a stacked link",
			build: func() error { return errors.Join(errors.New("first"), newPkgStackError()) },
		},
		{
			name:  "Cause() reaches a stacked link",
			build: func() error { return &causeError{cause: newPCStackError()} },
		},
		{
			name:  "a carrier from a previous WithStack call",
			build: func() error { return errkit.WithStack(errors.New("boom")) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.build()

			assert.Same(t, err, errkit.WithStack(err))
		})
	}
}

func TestWithStack_WrapsWhenNoLinkCarriesAStack(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "a plain %w chain",
			err:  fmt.Errorf("insert expense: %w", fmt.Errorf("query: %w", &driverError{})),
		},
		{
			name: "a chain ending in a nil link",
			err:  fmt.Errorf("insert expense: %w", &nilUnwrapError{}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := errkit.WithStack(tc.err)

			assert.NotSame(t, tc.err, got)
			assert.NotNil(t, sentry.ExtractStacktrace(got))
		})
	}
}
