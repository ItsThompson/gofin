package repository

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
)

const benchUser = "bench-user"

func newBenchRepo(client ImmudbClient) *ImmudbExpenseRepository {
	return NewImmudbExpenseRepository(client, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

// exportViaKeyset walks the full export using the NEW keyset cursor path
// (GetExpensesByUserAfter): no OFFSET, no per-page COUNT.
func exportViaKeyset(b *testing.B, repo *ImmudbExpenseRepository, pageSize int32) {
	b.Helper()
	cursor := ExpenseCursor{}
	for {
		_, next, hasMore, err := repo.GetExpensesByUserAfter(context.Background(), benchUser, cursor, pageSize)
		if err != nil {
			b.Fatalf("keyset export failed: %v", err)
		}
		if !hasMore {
			return
		}
		cursor = next
	}
}

// BenchmarkExportExpenseRead characterizes the keyset export path
// (GetExpensesByUserAfter) at increasing page counts P. Alongside wall-clock
// ns/op (a same-machine reference only), it reports the portable structural
// signals:
//   - queries/export: total queries issued for a full export
//   - counts/export:  COUNT(*) queries issued for a full export
//
// Keyset issues P queries (one data query per page, zero COUNT) and never
// rescans (O(P) rows scanned), versus the removed OFFSET path's 2*P queries and
// O(P^2) rows scanned (recorded in perf/baseline/read.txt). The rows-scanned
// shape is an execution property of the real database, verified against real
// immudb in the integration test; the recording mock reproduces query
// shape/count, not scan cost.
func BenchmarkExportExpenseRead(b *testing.B) {
	const pageSize = int32(50)
	for _, pages := range []int{1, 10, 50, 100} {
		rows := seedExportRows(benchUser, pages*int(pageSize))

		b.Run(fmt.Sprintf("Keyset/P=%d", pages), func(b *testing.B) {
			probe := newRecordingImmudbClient(rows...)
			exportViaKeyset(b, newBenchRepo(probe), pageSize)
			queries := float64(len(probe.Queries()))
			counts := float64(probe.countQueriesContaining("COUNT(*)"))

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				exportViaKeyset(b, newBenchRepo(newRecordingImmudbClient(rows...)), pageSize)
			}
			// Report after the timed loop: ResetTimer clears custom metrics (b.extra).
			b.ReportMetric(queries, "queries/export")
			b.ReportMetric(counts, "counts/export")
		})
	}
}
