package perf

import "testing"

// allocRunsPerAssertion is the number of measured runs passed to
// testing.AllocsPerRun; its integer-floored average smooths sporadic background
// allocations.
const allocRunsPerAssertion = 100

// allocReporter is the subset of testing.TB that AssertMaxAllocs needs to report
// a failure. testing.TB satisfies it; tests supply a fake to exercise the
// failure path without failing the enclosing test (testing.TB is sealed and
// cannot be implemented outside the testing package).
type allocReporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// AssertMaxAllocs fails t if fn allocates more than maxAllocs times per run,
// reporting observed vs allowed allocations.
//
// maxAllocs is an arch-stable upper bound with headroom, NOT a committed absolute
// alloc count: allocs/op differ between architectures (arm64 dev vs amd64 CI)
// and Go versions, so pick a bound the path should never exceed rather than the
// exact number captured on one machine. See README.md.
//
// This helper is a shipped part of the measurement foundation but currently has
// no caller; it is reserved for a non-scaling allocation-bound path that
// warrants a fixed bound (growth-ratio tests cover the scaling paths instead).
func AssertMaxAllocs(t testing.TB, maxAllocs float64, fn func()) {
	t.Helper()
	assertMaxAllocs(t, maxAllocs, fn)
}

func assertMaxAllocs(r allocReporter, maxAllocs float64, fn func()) {
	r.Helper()
	observed := testing.AllocsPerRun(allocRunsPerAssertion, fn)
	if observed > maxAllocs {
		r.Errorf("allocations exceeded bound: observed %.0f allocs/op, allowed <= %.0f", observed, maxAllocs)
	}
}
