package errkit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clock is a manually advanced time source, so the window is asserted exactly
// rather than by sleeping past a real one.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// newTestLimiter returns a limiter reading the returned clock, which starts at a
// non-zero instant so the "nothing reported yet" zero value stays distinguishable
// from a real report.
func newTestLimiter(window time.Duration) (*Limiter, *clock) {
	c := &clock{now: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)}
	limiter := NewLimiter(window)
	limiter.now = c.Now
	return limiter, c
}

func TestLimiter_AllowsTheFirstReport(t *testing.T) {
	limiter, _ := newTestLimiter(time.Hour)

	assert.True(t, limiter.Allow())
}

func TestLimiter_SuppressesEveryFurtherReportInsideTheWindow(t *testing.T) {
	limiter, c := newTestLimiter(time.Hour)

	require.True(t, limiter.Allow())

	for _, elapsed := range []time.Duration{0, time.Second, 30 * time.Minute} {
		c.advance(elapsed)
		assert.False(t, limiter.Allow(), "a report %s into the window", elapsed)
	}
}

func TestLimiter_AllowsAgainOnceTheWindowHasPassed(t *testing.T) {
	limiter, c := newTestLimiter(time.Hour)

	require.True(t, limiter.Allow())
	c.advance(time.Hour)

	assert.True(t, limiter.Allow())
}

// This is the quota arithmetic the sinks depend on. A site under continuous load
// calls Allow far more often than the window, and a suppressed call must not push
// the next allowed report further out, or a long outage reports once and then goes
// quiet.
func TestLimiter_BoundsAContinuousOutageToOneReportPerWindow(t *testing.T) {
	limiter, c := newTestLimiter(time.Hour)

	// Three hours of a downstream polled every 30 seconds.
	allowed := 0
	for range 3 * 120 {
		if limiter.Allow() {
			allowed++
		}
		c.advance(30 * time.Second)
	}

	assert.Equal(t, 3, allowed)
}

// The gateway's proxy error handler runs on one goroutine per in-flight request,
// so concurrent callers must not each get the first slot.
func TestLimiter_ConcurrentCallersGetOneSlot(t *testing.T) {
	const callers = 64

	limiter, _ := newTestLimiter(time.Hour)

	var allowed atomic.Int64
	var waitGroup sync.WaitGroup
	waitGroup.Add(callers)
	for range callers {
		go func() {
			defer waitGroup.Done()
			if limiter.Allow() {
				allowed.Add(1)
			}
		}()
	}
	waitGroup.Wait()

	assert.Equal(t, int64(1), allowed.Load())
}

func TestNewLimiter_PanicsOnANonPositiveWindow(t *testing.T) {
	for _, window := range []time.Duration{0, -time.Second} {
		assert.Panics(t, func() { NewLimiter(window) },
			"a %s window would leave the site unbounded", window)
	}
}
