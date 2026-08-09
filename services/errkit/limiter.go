package errkit

import (
	"fmt"
	"sync"
	"time"
)

// Limiter bounds one reporting site to a single report per window, for the error
// paths that fire per request or per heartbeat rather than per incident. Those
// paths can emit thousands of events from one outage, and the monthly event
// allowance is shared across every project in the organization, so an unbounded
// site spends everyone's budget on one already-alerting incident.
//
// It is in-memory and per-process, deliberately: there is one container per
// service, so a shared limiter would add a dependency and a failure mode without
// bounding anything a local one does not.
//
// One Limiter per site. The window belongs to the site it guards, so two sites
// sharing an instance would suppress each other's first report.
type Limiter struct {
	window time.Duration
	// now is a field so a test can drive the window without sleeping.
	now func() time.Time

	mu sync.Mutex
	// reported is the time Allow last returned true. The zero value means
	// nothing has been reported yet, which is what makes the first call allow.
	reported time.Time
}

// NewLimiter returns a Limiter that allows one report per window.
//
// It panics on a non-positive window. Every call site passes a constant, so a
// non-positive value can only arrive by an edit, and what it would produce is
// exactly the unbounded emitter this type exists to prevent.
func NewLimiter(window time.Duration) *Limiter {
	if window <= 0 {
		panic(fmt.Sprintf("errkit: NewLimiter needs a positive window, got %s", window))
	}
	return &Limiter{window: window, now: time.Now}
}

// Allow reports whether the site may report now, and consumes the window when it
// does. A suppressed call does not extend the window, so a site under continuous
// load still reports once per window rather than falling silent.
//
// The caller keeps its own log record outside the gate: the record is the durable
// artifact and stays per occurrence, while the report is what costs quota.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if !l.reported.IsZero() && now.Sub(l.reported) < l.window {
		return false
	}

	l.reported = now
	return true
}
