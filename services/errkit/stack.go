package errkit

import (
	"runtime"

	"github.com/getsentry/sentry-go"
)

// maxStackDepth matches the depth sentry.NewStacktrace captures, so a carrier
// stack is never shallower than the one the SDK would have synthesized.
const maxStackDepth = 100

// maxChainDepth bounds the wrap-chain walk at the SDK's own MaxErrorDepth
// default, so a self-referential chain cannot spin forever.
const maxChainDepth = 100

// withStack carries program counters the Sentry SDK reads through its
// StackTrace() []uintptr interface, so a captured error's frames point at the
// code that failed rather than at the reporting helper.
//
// Unwrap is mandatory. The SDK's chain walker recurses only through Unwrap and
// Cause and types each exception entry from reflect.TypeOf, so a carrier without
// Unwrap terminates the walk and flattens the whole %w chain into one exception
// named after this type. That is the degenerate grouping this package exists to
// prevent. The cost is that the carrier shows up as the outermost exception
// entry and can prefix the issue title, which is cosmetic and accepted.
type withStack struct {
	err error
	pcs []uintptr
}

func (w *withStack) Error() string         { return w.err.Error() }
func (w *withStack) StackTrace() []uintptr { return w.pcs }
func (w *withStack) Unwrap() error         { return w.err }

// WithStack returns err unchanged when err or anything it wraps already carries
// a stack the Sentry SDK can read. Otherwise it returns a carrier holding the
// caller's program counters, so a captured error's frames point at the code that
// failed rather than at this package.
func WithStack(err error) error {
	return attachStack(err, 1)
}

// attachStack is the shared implementation behind WithStack and Report. skip is
// the number of frames between attachStack's caller and the frame the stack
// should start at, so both entry points pass 1 to skip themselves and the first
// recorded frame is the code that decided to report.
func attachStack(err error, skip int) error {
	if err == nil || hasStack(err) {
		return err
	}

	pcs := make([]uintptr, maxStackDepth)
	// 2 covers runtime.Callers itself and attachStack; skip covers the caller
	// that asked for the stack on its own caller's behalf.
	n := runtime.Callers(skip+2, pcs)

	return &withStack{err: err, pcs: pcs[:n]}
}

// hasStack reports whether err or anything it wraps carries a stack the SDK can
// read. sentry.ExtractStacktrace is the detector rather than a local type switch
// so the answer is the SDK's own: it accepts StackTrace() []uintptr, the
// pkg/errors StackTrace() errors.StackTrace shape, go-errors' StackFrames, and
// the xerrors frame field. The walk mirrors the SDK's recursion cases in the
// same order, because a link the SDK never visits cannot supply the stack it
// looks for.
func hasStack(err error) bool {
	pending := []error{err}

	for visited := 0; len(pending) > 0 && visited < maxChainDepth; visited++ {
		link := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		if link == nil {
			continue
		}
		if sentry.ExtractStacktrace(link) != nil {
			return true
		}

		switch v := link.(type) {
		case interface{ Unwrap() []error }:
			pending = append(pending, v.Unwrap()...)
		case interface{ Unwrap() error }:
			pending = append(pending, v.Unwrap())
		case interface{ Cause() error }:
			pending = append(pending, v.Cause())
		}
	}

	return false
}
