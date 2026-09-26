//go:build integration

package repository

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const (
	grpcDefaultMaxReceiveBytes = 4 * 1024 * 1024
	concurrentCompleteReads    = 8
)

func TestGetActiveExpensesByPeriodAfter_IntegrationTransportDecision(t *testing.T) {
	client := connectRealImmudb(t)
	repo := NewImmudbExpenseRepository(client, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx := context.Background()
	require.NoError(t, repo.InitSchema(ctx))

	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "period-itest-" + runID
	makeID := func(prefix string, i int) string { return fmt.Sprintf("%s-%s-%03d", runID, prefix, i) }

	const activeRows = 123
	for i := 0; i < activeRows; i++ {
		row := buildTestExpense(makeID("active", i), userID, fmt.Sprintf("2026-05-01T00:%02d:%02dZ", i/60, i%60))
		row.ExpenseDateIso = fmt.Sprintf("2026-05-%02d", i%3+1)
		require.NoError(t, createIntegrationExpense(ctx, repo, row))
	}
	for i := 0; i < 7; i++ {
		row := buildTestExpense(makeID("corrected", i), userID, fmt.Sprintf("2026-05-10T00:00:%02dZ", i))
		row.Status = "corrected"
		require.NoError(t, createIntegrationExpense(ctx, repo, row))
	}
	otherPeriod := buildTestExpense(makeID("other-period", 0), userID, "2026-05-20T00:00:00Z")
	otherPeriod.PeriodMonth = 6
	require.NoError(t, createIntegrationExpense(ctx, repo, otherPeriod))

	firstRead, firstPages, firstPageBytes, err := readAllActivePeriod(repo, userID, 2026, 5)
	require.NoError(t, err)
	secondRead, secondPages, _, err := readAllActivePeriod(repo, userID, 2026, 5)
	require.NoError(t, err)

	require.Len(t, firstRead, activeRows)
	assert.Equal(t, firstPages, secondPages)
	assert.Equal(t, expenseIDs(firstRead), expenseIDs(secondRead), "cursor walk must be deterministic")
	assert.Equal(t, 3, firstPages, "123 rows at a 50-row page size require three bounded pages")
	assert.Less(t, firstPageBytes, grpcDefaultMaxReceiveBytes, "one bounded response must fit grpc-go's default receive limit")

	seen := make(map[string]struct{}, len(firstRead))
	for i, row := range firstRead {
		assert.Equal(t, "active", row.Status)
		assert.Equal(t, int32(2026), row.PeriodYear)
		assert.Equal(t, int32(5), row.PeriodMonth)
		assert.Equal(t, userID, row.UserID)
		_, duplicate := seen[row.ID]
		assert.False(t, duplicate, "row %s repeated across pages", row.ID)
		seen[row.ID] = struct{}{}
		if i > 0 {
			previous := firstRead[i-1]
			assert.True(t, periodCursorLess(previous, row), "rows must follow the complete cursor order")
		}
	}

	queries := client.recordedQueries()
	sawPeriodQuery := false
	sawTargetedIndex := false
	for _, query := range queries {
		upper := strings.ToUpper(query)
		if strings.Contains(query, "idx_expenses_user_period_order") {
			sawTargetedIndex = true
		}
		if strings.Contains(upper, "ORDER BY EXPENSE_DATE ASC, CREATED_AT ASC, ID ASC") {
			sawPeriodQuery = true
			assert.NotContains(t, upper, "OFFSET")
			assert.NotContains(t, upper, "COUNT(*)")
		}
	}
	assert.True(t, sawPeriodQuery, "expected the active-period keyset query")
	assert.True(t, sawTargetedIndex, "expected the filter-plus-cursor index to be declared")

	peakHeapBytes := measureConcurrentPeriodReadHeap(t, repo, userID)
	t.Logf("transport=bounded-keyset page_size=%d pages=%d grpc_response_bytes=%d grpc_default_receive_limit_bytes=%d concurrent_reads=%d peak_heap_inuse_bytes=%d expense_memory_limit_bytes=%d", CompletePeriodPageSize, firstPages, firstPageBytes, grpcDefaultMaxReceiveBytes, concurrentCompleteReads, peakHeapBytes, 256*1024*1024)
	assert.Less(t, peakHeapBytes, uint64(256*1024*1024), "concurrent complete reads must stay below expense service memory limit")
}

func createIntegrationExpense(ctx context.Context, repo *ImmudbExpenseRepository, row *model.Expense) error {
	_, err := repo.CreateExpense(ctx, row)
	return err
}

func readAllActivePeriod(repo *ImmudbExpenseRepository, userID string, year, month int32) ([]*model.Expense, int, int, error) {
	cursor := ActivePeriodCursor{}
	all := make([]*model.Expense, 0, 123)
	pages := 0
	firstPageBytes := 0
	for {
		page, next, hasMore, err := repo.GetActiveExpensesByPeriodAfter(context.Background(), userID, year, month, cursor, CompletePeriodPageSize)
		if err != nil {
			return nil, 0, 0, err
		}
		if pages == 0 {
			firstPageBytes = proto.Size(&pb.ExpenseListResponse{Data: expensesToProto(page)})
		}
		all = append(all, page...)
		pages++
		if !hasMore {
			return all, pages, firstPageBytes, nil
		}
		cursor = next
		if pages > 100 {
			return nil, 0, 0, fmt.Errorf("active period keyset did not terminate")
		}
	}
}

func expensesToProto(rows []*model.Expense) []*pb.ExpenseData {
	result := make([]*pb.ExpenseData, 0, len(rows))
	for _, row := range rows {
		result = append(result, &pb.ExpenseData{
			Id:                                    row.ID,
			UserId:                                row.UserID,
			Name:                                  row.Name,
			ExpenseType:                           row.ExpenseType,
			TagId:                                 row.TagID,
			ExpenseDateIso:                        row.ExpenseDateIso,
			PeriodYear:                            row.PeriodYear,
			PeriodMonth:                           row.PeriodMonth,
			Status:                                row.Status,
			CorrectsId:                            row.CorrectsID,
			IsProRata:                             row.IsProRata,
			ProRataGroup:                          row.ProRataGroup,
			ProRataIndex:                          row.ProRataIndex,
			ProRataTotal:                          row.ProRataTotal,
			CreatedAt:                             row.CreatedAt,
			TransactionCurrencyCode:               row.TransactionCurrencyCode,
			OriginalTransactionAmountInMinorUnits: row.OriginalTransactionAmountInMinorUnits,
			ReportingAmountInMinorUnits:           row.ReportingAmountInMinorUnits,
			ReportingCurrencyCode:                 row.ReportingCurrencyCode,
			SourceToTargetExchangeRate:            row.SourceToTargetExchangeRate,
			ExchangeRateSource:                    row.ExchangeRateSource,
			ExchangeRateTimestamp:                 row.ExchangeRateTimestamp,
			ExchangeRateCacheExpiresAt:            row.ExchangeRateCacheExpiresAt,
		})
	}
	return result
}

func expenseIDs(rows []*model.Expense) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func periodCursorLess(previous, current *model.Expense) bool {
	if previous.ExpenseDateIso != current.ExpenseDateIso {
		return previous.ExpenseDateIso < current.ExpenseDateIso
	}
	if previous.CreatedAt != current.CreatedAt {
		return previous.CreatedAt < current.CreatedAt
	}
	return previous.ID < current.ID
}

func measureConcurrentPeriodReadHeap(t *testing.T, repo *ImmudbExpenseRepository, userID string) uint64 {
	t.Helper()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, concurrentCompleteReads)
	results := make([][]*model.Expense, concurrentCompleteReads)
	for i := 0; i < concurrentCompleteReads; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			rows, _, _, err := readAllActivePeriod(repo, userID, 2026, 5)
			if err != nil {
				errs <- err
				return
			}
			results[index] = rows
		}(i)
	}
	close(start)

	peak := before.HeapInuse
	stop := make(chan struct{})
	var samples sync.WaitGroup
	samples.Add(1)
	go func() {
		defer samples.Done()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var stats runtime.MemStats
				runtime.ReadMemStats(&stats)
				if stats.HeapInuse > peak {
					peak = stats.HeapInuse
				}
			case <-stop:
				return
			}
		}
	}()
	wg.Wait()
	close(stop)
	samples.Wait()
	select {
	case err := <-errs:
		t.Fatalf("concurrent complete read failed: %v", err)
	default:
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if after.HeapInuse > peak {
		peak = after.HeapInuse
	}
	_ = results
	return peak - before.HeapInuse
}
