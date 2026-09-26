//go:build integration

package repository

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
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
	sawRequiredIndexDDL := false
	for _, query := range queries {
		upper := strings.ToUpper(query)
		if strings.Contains(query, "CREATE INDEX IF NOT EXISTS ON expenses (user_id, period_year, period_month, status, expense_date, created_at, id)") {
			sawRequiredIndexDDL = true
		}
		if strings.Contains(upper, "ORDER BY EXPENSE_DATE ASC, CREATED_AT ASC, ID ASC") {
			sawPeriodQuery = true
			assert.NotContains(t, upper, "OFFSET")
			assert.NotContains(t, upper, "COUNT(*)")
		}
	}
	assert.True(t, sawPeriodQuery, "expected the active-period keyset query")
	assert.True(t, sawRequiredIndexDDL, "expected the required filter-plus-cursor index DDL")

	peakMemory := measureConcurrentPeriodReadMemory(t, repo, userID)
	memoryLimitBytes := uint64(256 * 1024 * 1024)
	t.Logf("transport=bounded-keyset page_size=%d pages=%d grpc_response_bytes=%d grpc_default_receive_limit_bytes=%d concurrent_reads=%d peak_heap_inuse_bytes=%d peak_runtime_sys_bytes=%d expense_memory_limit_bytes=%d", CompletePeriodPageSize, firstPages, firstPageBytes, grpcDefaultMaxReceiveBytes, concurrentCompleteReads, peakMemory.heapInuseBytes, peakMemory.runtimeSysBytes, memoryLimitBytes)
	assert.Less(t, peakMemory.heapInuseBytes, memoryLimitBytes, "absolute peak heap in use must stay below expense service memory limit")
	assert.Less(t, peakMemory.runtimeSysBytes, memoryLimitBytes, "absolute Go runtime memory must stay below expense service memory limit")
}

func TestCompletePeriodReadGRPCMessageSize_Integration(t *testing.T) {
	client := connectRealImmudb(t)
	repo := NewImmudbExpenseRepository(client, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx := context.Background()
	require.NoError(t, repo.InitSchema(ctx))

	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	userID := "gpit-" + runID
	for i := 0; i < 51; i++ {
		row := buildTestExpense(fmt.Sprintf("%s-%03d", runID, i), userID, fmt.Sprintf("2026-05-01T00:00:%02dZ", i))
		require.NoError(t, createIntegrationExpense(ctx, repo, row))
	}

	probeClient := newCompletePeriodGRPCClient(t, &completePeriodGRPCProbe{repo: repo})
	response, err := probeClient.GetActiveExpensesForPeriodPage(ctx, &pb.GetActiveExpensesForPeriodPageRequest{
		UserId:   userID,
		Year:     2026,
		Month:    5,
		PageSize: CompletePeriodPageSize,
	})
	require.NoError(t, err)
	require.Len(t, response.GetData(), int(CompletePeriodPageSize))
	assert.True(t, response.GetHasMore())
	wireMessageBytes := proto.Size(response)
	assert.Less(t, wireMessageBytes, grpcDefaultMaxReceiveBytes)
	t.Logf("grpc_transport=bufconn rpc=GetActiveExpensesForPeriodPage response_bytes=%d default_client_receive_limit_bytes=%d page_size=%d", wireMessageBytes, grpcDefaultMaxReceiveBytes, CompletePeriodPageSize)

	overLimitResponse := oversizedCompletePeriodResponse()
	overLimitClient := newCompletePeriodGRPCClient(t, &completePeriodGRPCProbe{response: overLimitResponse})
	assert.Greater(t, proto.Size(overLimitResponse), grpcDefaultMaxReceiveBytes)
	_, err = overLimitClient.GetActiveExpensesForPeriodPage(ctx, &pb.GetActiveExpensesForPeriodPageRequest{UserId: userID, Year: 2026, Month: 5, PageSize: CompletePeriodPageSize})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

type completePeriodGRPCProbe struct {
	pb.UnimplementedExpenseServiceServer
	repo     *ImmudbExpenseRepository
	response *pb.CompleteExpensePageResponse
}

func (p *completePeriodGRPCProbe) GetActiveExpensesForPeriodPage(ctx context.Context, req *pb.GetActiveExpensesForPeriodPageRequest) (*pb.CompleteExpensePageResponse, error) {
	if p.response != nil {
		return p.response, nil
	}

	rows, next, hasMore, err := p.repo.GetActiveExpensesByPeriodAfter(ctx, req.GetUserId(), req.GetYear(), req.GetMonth(), ActivePeriodCursor{
		ExpenseDate: req.GetCursorExpenseDate(),
		CreatedAt:   req.GetCursorCreatedAt(),
		ID:          req.GetCursorId(),
	}, req.GetPageSize())
	if err != nil {
		return nil, err
	}
	return &pb.CompleteExpensePageResponse{
		Data:            expensesToProto(rows),
		NextExpenseDate: next.ExpenseDate,
		NextCreatedAt:   next.CreatedAt,
		NextId:          next.ID,
		HasMore:         hasMore,
	}, nil
}

func newCompletePeriodGRPCClient(t *testing.T, probe *completePeriodGRPCProbe) pb.ExpenseServiceClient {
	t.Helper()
	listener := bufconn.Listen(8 * 1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterExpenseServiceServer(server, probe)
	go func() { _ = server.Serve(listener) }()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		server.Stop()
		_ = conn.Close()
		_ = listener.Close()
	})
	return pb.NewExpenseServiceClient(conn)
}

func oversizedCompletePeriodResponse() *pb.CompleteExpensePageResponse {
	data := make([]*pb.ExpenseData, 30_000)
	for i := range data {
		data[i] = &pb.ExpenseData{
			Id:                      fmt.Sprintf("oversized-%d", i),
			UserId:                  "user-oversized",
			Name:                    strings.Repeat("x", 255),
			ExpenseType:             "essentials",
			TagId:                   "tag-oversized",
			ExpenseDateIso:          "2026-05-01",
			CreatedAt:               "2026-05-01T00:00:00Z",
			TransactionCurrencyCode: "USD",
			ReportingCurrencyCode:   "USD",
			ExchangeRateSource:      "identity",
			ExchangeRateTimestamp:   "2026-05-01T00:00:00Z",
		}
	}
	return &pb.CompleteExpensePageResponse{Data: data}
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

type periodReadMemory struct {
	heapInuseBytes  uint64
	runtimeSysBytes uint64
}

func measureConcurrentPeriodReadMemory(t *testing.T, repo *ImmudbExpenseRepository, userID string) periodReadMemory {
	t.Helper()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, concurrentCompleteReads)
	results := make([][]*model.Expense, concurrentCompleteReads)
	readerHeapPeaks := make([]uint64, concurrentCompleteReads)
	readerRuntimePeaks := make([]uint64, concurrentCompleteReads)
	for i := 0; i < concurrentCompleteReads; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			var readerBefore runtime.MemStats
			runtime.ReadMemStats(&readerBefore)
			rows, _, _, err := readAllActivePeriod(repo, userID, 2026, 5)
			if err != nil {
				errs <- err
				return
			}
			var readerAfter runtime.MemStats
			runtime.ReadMemStats(&readerAfter)
			readerHeapPeaks[index] = maxUint64(readerBefore.HeapInuse, readerAfter.HeapInuse)
			readerRuntimePeaks[index] = maxUint64(readerBefore.Sys, readerAfter.Sys)
			results[index] = rows
		}(i)
	}

	peakHeapInuse := before.HeapInuse
	peakRuntimeSys := before.Sys
	stop := make(chan struct{})
	var samples sync.WaitGroup
	samplerReady := make(chan struct{})
	samples.Add(1)
	go func() {
		close(samplerReady)
		defer samples.Done()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var stats runtime.MemStats
				runtime.ReadMemStats(&stats)
				if stats.HeapInuse > peakHeapInuse {
					peakHeapInuse = stats.HeapInuse
				}
				if stats.Sys > peakRuntimeSys {
					peakRuntimeSys = stats.Sys
				}
			case <-stop:
				return
			}
		}
	}()
	<-samplerReady
	close(start)
	wg.Wait()
	close(stop)
	samples.Wait()
	select {
	case err := <-errs:
		t.Fatalf("concurrent complete read failed: %v", err)
	default:
	}

	runtime.KeepAlive(results)
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if after.HeapInuse > peakHeapInuse {
		peakHeapInuse = after.HeapInuse
	}
	if after.Sys > peakRuntimeSys {
		peakRuntimeSys = after.Sys
	}
	for i := range readerHeapPeaks {
		if readerHeapPeaks[i] > peakHeapInuse {
			peakHeapInuse = readerHeapPeaks[i]
		}
		if readerRuntimePeaks[i] > peakRuntimeSys {
			peakRuntimeSys = readerRuntimePeaks[i]
		}
	}
	return periodReadMemory{heapInuseBytes: peakHeapInuse, runtimeSysBytes: peakRuntimeSys}
}

func maxUint64(first, second uint64) uint64 {
	if first > second {
		return first
	}
	return second
}
