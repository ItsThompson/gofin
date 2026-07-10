package perf

import (
	"fmt"
	"strings"
	"testing"
)

// allocSink gives allocations in the test closures a heap escape so
// testing.AllocsPerRun counts them; without an escape the compiler may
// stack-allocate and report zero. It is read back to confirm the allocation
// actually happened (and to keep it from being a write-only variable).
var allocSink []byte

// fakeReporter stands in for testing.TB so the failure path can be exercised
// without failing the enclosing test. testing.TB is sealed and cannot be
// implemented outside the testing package, so AssertMaxAllocs delegates to
// assertMaxAllocs, which accepts the narrower allocReporter.
type fakeReporter struct {
	helperCalls int
	messages    []string
}

func (f *fakeReporter) Helper() { f.helperCalls++ }

func (f *fakeReporter) Errorf(format string, args ...any) {
	f.messages = append(f.messages, fmt.Sprintf(format, args...))
}

func TestAssertMaxAllocs_FailsWhenOverBound(t *testing.T) {
	r := &fakeReporter{}

	assertMaxAllocs(r, 0, func() { allocSink = make([]byte, 64) })

	if len(allocSink) != 64 {
		t.Fatalf("sink not populated; allocation may have been optimized away (len=%d)", len(allocSink))
	}
	if r.helperCalls == 0 {
		t.Error("expected Helper() to be called")
	}
	if len(r.messages) != 1 {
		t.Fatalf("expected exactly 1 failure message, got %d: %v", len(r.messages), r.messages)
	}
	msg := r.messages[0]
	if !strings.Contains(msg, "observed") || !strings.Contains(msg, "allowed") {
		t.Errorf("failure message should report observed vs allowed, got: %q", msg)
	}
	if !strings.Contains(msg, "<= 0") {
		t.Errorf("failure message should report the allowed bound (0), got: %q", msg)
	}
}

func TestAssertMaxAllocs_PassesWhenWithinBound(t *testing.T) {
	r := &fakeReporter{}

	assertMaxAllocs(r, 2, func() { allocSink = make([]byte, 32) })

	if len(r.messages) != 0 {
		t.Errorf("expected no failure within bound, got: %v", r.messages)
	}
}

func TestAssertMaxAllocs_PassesForZeroAllocFunc(t *testing.T) {
	r := &fakeReporter{}

	assertMaxAllocs(r, 0, func() {})

	if len(r.messages) != 0 {
		t.Errorf("expected no failure for zero-allocation func, got: %v", r.messages)
	}
}

// TestAssertMaxAllocs_PublicPassPath drives the exported wrapper with a real
// *testing.T to confirm it does not fail the test when fn stays within bound.
func TestAssertMaxAllocs_PublicPassPath(t *testing.T) {
	AssertMaxAllocs(t, 5, func() { allocSink = make([]byte, 16) })
}
