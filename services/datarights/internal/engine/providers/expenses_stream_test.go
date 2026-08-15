package providers

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// streamRowFixtures builds n expense rows for the stream in chronological order.
// Every row carries a tag id and a complete version-1 identity snapshot so tag
// resolution and formatRow run on each one, exercising the same per-row work the
// real consumer does.
func streamRowFixtures(n int) []*expensepb.ExpenseData {
	rows := make([]*expensepb.ExpenseData, n)
	for i := range rows {
		amount := int64(1000 + i)
		rows[i] = &expensepb.ExpenseData{
			Id:                    fmt.Sprintf("exp-%08d", i),
			Name:                  "Expense",
			ExpenseType:           "essentials",
			TagId:                 "tag-1",
			ExpenseDate:           "2026-05-01",
			PeriodYear:            2026,
			PeriodMonth:           5,
			Status:                "active",
			CreatedAt:             fmt.Sprintf("2026-05-01T%02d:%02d:%02dZ", i/3600%24, i/60%60, i%60),
			TransactionCurrency:   "USD",
			TransactionAmount:     amount,
			ReportingAmount:       amount,
			ReportingCurrency:     "USD",
			ExchangeRate:          "1",
			ExchangeRateSource:    "identity",
			ExchangeRateTimestamp: fmt.Sprintf("2026-05-01T%02d:%02d:%02dZ", i/3600%24, i/60%60, i%60),
		}
	}
	return rows
}

// discardCSVSink returns an emit callback that writes each formatted row into a
// csv.Writer backed by io.Discard, modelling the incremental ZIP write without
// retaining any row. flush surfaces encoding errors.
func discardCSVSink() (emit func([]string) error, flush func() error) {
	w := csv.NewWriter(io.Discard)
	emit = func(row []string) error { return w.Write(row) }
	flush = func() error {
		w.Flush()
		return w.Error()
	}
	return emit, flush
}

// BenchmarkExpensesProvider_StreamIncrementalWrite streams the full history into
// a discarding CSV sink at two very different row counts. Peak *retained* memory
// stays O(pageSize) regardless of total rows (asserted by
// TestExpensesProvider_StreamedConsumptionIsMemoryBounded); total allocations
// still scale with row count because every row is formatted, which is expected.
// The committed baseline records both shapes.
func BenchmarkExpensesProvider_StreamIncrementalWrite(b *testing.B) {
	for _, n := range []int{1000, 50000} {
		rows := streamRowFixtures(n)
		p := NewExpensesProvider(&mockExpenseServiceClient{streamRows: rows}, nil, nil)
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				emit, flush := discardCSVSink()
				if err := p.streamExpenses(context.Background(), "user-1", emit); err != nil {
					b.Fatal(err)
				}
				if err := flush(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// retainedHeapBytes reports the live heap bytes that survive build(). A GC before
// and after each reading drops transient per-row garbage, and anything allocated
// before the call (the seeded stream rows) is live in both readings and cancels
// out, so the delta isolates what the consumer itself keeps alive.
func retainedHeapBytes(build func() any) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	kept := build()

	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(kept)

	if after.HeapAlloc <= before.HeapAlloc {
		return 0
	}
	return after.HeapAlloc - before.HeapAlloc
}

// TestExpensesProvider_StreamedConsumptionIsMemoryBounded is the bounded-allocation
// growth-ratio regression. It measures the streamExpenses
// primitive paired with a non-buffering sink: given a sink that writes each row
// onward (an incremental ZIP writer), the primitive retains nothing, so its peak
// memory stays O(pageSize) as the row count grows 50x. The buffered contrast
// (append every formatted row) retains O(N) and
// blows past the bound, so reverting to buffered collection fails this test.
//
// This bound is a property of the primitive + sink, NOT of production Collect:
// Collect adapts the primitive with an append sink to satisfy the DataProvider
// [][]string contract, so it (and thus the export engine, which buffers each
// provider before BuildZIP) stays O(total); making the whole export O(pageSize)
// would require streaming each provider straight into the ZIP writer.
func TestExpensesProvider_StreamedConsumptionIsMemoryBounded(t *testing.T) {
	const (
		smallRows           = 1000
		largeRows           = 50000
		allowedGrowthFactor = 8
		// noiseFloorBytes keeps the ratio meaningful when the streamed path
		// retains ~nothing: heap-measurement jitter must not manufacture a huge
		// relative growth from a near-zero baseline. The buffered contrast retains
		// at least the outer [][]string backing (largeRows * 24B ~= 1.2MB at 50k),
		// a hard structural floor well above this bound.
		noiseFloorBytes = 64 * 1024
	)

	streamed := func(n int) func() any {
		return func() any {
			// Build the fixtures inside the measured closure so the seed rows are
			// not live during retainedHeapBytes' baseline (before) read; only what
			// the consumer retains after the call should count.
			rows := streamRowFixtures(n)
			p := NewExpensesProvider(&mockExpenseServiceClient{streamRows: rows}, nil, nil)
			emit, flush := discardCSVSink()
			require.NoError(t, p.streamExpenses(context.Background(), "user-1", emit))
			require.NoError(t, flush())
			return nil // the streamed consumer retains nothing
		}
	}

	buffered := func(n int) func() any {
		return func() any {
			rows := streamRowFixtures(n)
			p := NewExpensesProvider(&mockExpenseServiceClient{streamRows: rows}, nil, nil)
			var out [][]string
			require.NoError(t, p.streamExpenses(context.Background(), "user-1", func(row []string) error {
				out = append(out, row)
				return nil
			}))
			return out // retains O(N): the buffered shape
		}
	}

	smallStreamed := retainedHeapBytes(streamed(smallRows))
	largeStreamed := retainedHeapBytes(streamed(largeRows))

	base := smallStreamed
	if base < noiseFloorBytes {
		base = noiseFloorBytes
	}
	bound := base * allowedGrowthFactor

	assert.LessOrEqualf(t, largeStreamed, bound,
		"streamed consumption must stay O(pageSize): retained %d bytes at %d rows vs %d at %d rows (bound %d)",
		largeStreamed, largeRows, smallStreamed, smallRows, bound)

	// Guard: the old buffered shape must exceed the bound, proving the
	// assertion above actually distinguishes streaming from buffering.
	largeBuffered := retainedHeapBytes(buffered(largeRows))

	t.Logf("retained heap bytes: streamed[%d]=%d streamed[%d]=%d buffered[%d]=%d (bound %d)",
		smallRows, smallStreamed, largeRows, largeStreamed, largeRows, largeBuffered, bound)

	assert.Greaterf(t, largeBuffered, bound,
		"buffered consumption should exceed the streamed bound (retained %d bytes, bound %d); "+
			"if this fails the memory-bound assertion is not meaningful", largeBuffered, bound)
}

// TestExpensesProvider_StreamCancellation_StopsPromptly verifies a mid-stream
// cancellation stops the consumer immediately rather than draining the rest of
// the stream.
func TestExpensesProvider_StreamCancellation_StopsPromptly(t *testing.T) {
	const total = 1000
	p := NewExpensesProvider(&mockExpenseServiceClient{streamRows: streamRowFixtures(total)}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumed := 0
	err := p.streamExpenses(ctx, "user-1", func(_ []string) error {
		consumed++
		if consumed == 3 {
			cancel()
		}
		return nil
	})

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 3, consumed, "consumer must stop promptly after cancellation, not drain the stream")
	assert.Less(t, consumed, total)
}
