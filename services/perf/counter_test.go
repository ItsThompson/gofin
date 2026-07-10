package perf_test

import (
	"sync"
	"testing"

	"github.com/ItsThompson/gofin/services/perf"
)

func TestCallCounter_CountsPerOperation(t *testing.T) {
	c := perf.NewCallCounter()

	c.Record("GetAllUserData")
	c.Record("ListTags")
	c.Record("ListTags")
	c.Record("ListTags")

	if got := c.Count("GetAllUserData"); got != 1 {
		t.Errorf("Count(GetAllUserData) = %d, want 1", got)
	}
	if got := c.Count("ListTags"); got != 3 {
		t.Errorf("Count(ListTags) = %d, want 3", got)
	}
}

func TestCallCounter_UnrecordedOperationIsZero(t *testing.T) {
	c := perf.NewCallCounter()

	if got := c.Count("NeverCalled"); got != 0 {
		t.Errorf("Count(NeverCalled) = %d, want 0", got)
	}
}

func TestCallCounter_TotalSumsAllOperations(t *testing.T) {
	c := perf.NewCallCounter()

	if got := c.Total(); got != 0 {
		t.Errorf("Total() on empty counter = %d, want 0", got)
	}

	c.Record("A")
	c.Record("B")
	c.Record("B")

	if got := c.Total(); got != 3 {
		t.Errorf("Total() = %d, want 3", got)
	}
}

// TestCallCounter_ConcurrentRecord exercises the mutex under the race detector:
// run with `go test -race`. Spies embed CallCounter and are invoked from
// errgroup fan-out goroutines, so concurrent Record must not race. A reader
// goroutine hits Count/Total during the writes so -race also covers reads
// concurrent with writes, backing the "all methods are safe" claim.
func TestCallCounter_ConcurrentRecord(t *testing.T) {
	c := perf.NewCallCounter()

	const goroutines = 50
	const recordsPerGoroutine = 100

	stop := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stop:
				return
			default:
				_ = c.Count("fanout")
				_ = c.Total()
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				c.Record("fanout")
			}
		}()
	}
	wg.Wait()
	close(stop)
	<-readerDone

	want := goroutines * recordsPerGoroutine
	if got := c.Count("fanout"); got != want {
		t.Errorf("Count(fanout) = %d, want %d", got, want)
	}
	if got := c.Total(); got != want {
		t.Errorf("Total() = %d, want %d", got, want)
	}
}
