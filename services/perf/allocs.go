package perf

import "testing"

// allocRunsPerAssertion is the number of measured runs passed to
// testing.AllocsPerRun. AllocsPerRun does one warmup run plus this many measured
// runs and returns the integer-floored average, so a value well above 1 averages
// out incidental background allocations (GC bookkeeping, other goroutines) that
// would otherwise inflate a deterministic per-run count.
const allocRunsPerAssertion = 100

// allocReporter is the subset of testing.TB that AssertMaxAllocs needs to report
// a failure. testing.TB satisfies it; tests supply a fake to exercise the
// failure path without failing the enclosing test (testing.TB is sealed and
// cannot be implemented outside the testing package).
type allocReporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// AssertMaxAllocs fails t if fn allocates more than max times per run, reporting
// observed vs allowed allocations.
//
// max is an arch-stable upper bound with headroom, NOT a committed absolute
// alloc count: allocs/op differ between architectures (arm64 dev vs amd64 CI)
// and Go versions, so pick a bound the path should never exceed rather than the
// exact number captured on one machine. See README.md.
func AssertMaxAllocs(t testing.TB, max float64, fn func()) {
	t.Helper()
	assertMaxAllocs(t, max, fn)
}

func assertMaxAllocs(r allocReporter, max float64, fn func()) {
	r.Helper()
	observed := testing.AllocsPerRun(allocRunsPerAssertion, fn)
	if observed > max {
		r.Errorf("allocations exceeded bound: observed %.0f allocs/op, allowed <= %.0f", observed, max)
	}
}
