// Package perf provides shared test helpers for the gofin latency & efficiency
// epic: a concurrency-safe call counter that spy client/repo implementations
// embed, and an allocation-bound assertion over testing.AllocsPerRun. It carries
// no runtime dependencies and is imported only from test files. See README.md
// for the benchmark / pprof / benchstat workflow.
package perf

import "sync"

// CallCounter records how many times named operations were invoked. Spy
// implementations of gRPC clients and repository interfaces embed a
// *CallCounter so efficiency tests can assert bounds such as "GetAllUserData
// called at most once". All methods are safe for concurrent use, so spies may
// be called from errgroup fan-out goroutines (P2/P4).
type CallCounter struct {
	mu     sync.Mutex
	counts map[string]int
}

// NewCallCounter returns an empty CallCounter ready for concurrent use.
func NewCallCounter() *CallCounter {
	return &CallCounter{counts: make(map[string]int)}
}

// Record increments the invocation count for op. Call it at the top of each
// spy method.
func (c *CallCounter) Record(op string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[op]++
}

// Count returns how many times op was recorded.
func (c *CallCounter) Count(op string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[op]
}

// Total returns the total number of recorded invocations across all ops.
func (c *CallCounter) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, n := range c.counts {
		total += n
	}
	return total
}
