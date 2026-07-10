package providers

import (
	"context"
	"io"

	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// mockFinanceServiceClient implements FinanceServiceClient for tests.
type mockFinanceServiceClient struct {
	getAllUserDataResp *financepb.AllUserDataResponse
	getAllUserDataErr  error
}

func (m *mockFinanceServiceClient) GetAllUserData(_ context.Context, _ *financepb.GetAllUserDataRequest, _ ...grpc.CallOption) (*financepb.AllUserDataResponse, error) {
	if m.getAllUserDataErr != nil {
		return nil, m.getAllUserDataErr
	}
	return m.getAllUserDataResp, nil
}

func (m *mockFinanceServiceClient) ListTags(_ context.Context, _ *financepb.ListTagsRequest, _ ...grpc.CallOption) (*financepb.TagListResponse, error) {
	return nil, nil
}

// Implement remaining interface methods as no-ops.
func (m *mockFinanceServiceClient) GetDefaults(_ context.Context, _ *financepb.GetDefaultsRequest, _ ...grpc.CallOption) (*financepb.DefaultsResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) UpdateDefaults(_ context.Context, _ *financepb.UpdateDefaultsRequest, _ ...grpc.CallOption) (*financepb.DefaultsResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) CompleteOnboarding(_ context.Context, _ *financepb.CompleteOnboardingRequest, _ ...grpc.CallOption) (*financepb.DefaultsResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) GetCurrentPeriod(_ context.Context, _ *financepb.GetCurrentPeriodRequest, _ ...grpc.CallOption) (*financepb.PeriodResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) CreatePeriod(_ context.Context, _ *financepb.CreatePeriodRequest, _ ...grpc.CallOption) (*financepb.PeriodResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) UpdatePeriod(_ context.Context, _ *financepb.UpdatePeriodRequest, _ ...grpc.CallOption) (*financepb.PeriodResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) ListPeriods(_ context.Context, _ *financepb.ListPeriodsRequest, _ ...grpc.CallOption) (*financepb.PeriodListResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) CreateTag(_ context.Context, _ *financepb.CreateTagRequest, _ ...grpc.CallOption) (*financepb.TagResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) UpdateTag(_ context.Context, _ *financepb.UpdateTagRequest, _ ...grpc.CallOption) (*financepb.TagResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) DeleteTag(_ context.Context, _ *financepb.DeleteTagRequest, _ ...grpc.CallOption) (*financepb.DeleteTagResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) CheckTagUsage(_ context.Context, _ *financepb.CheckTagUsageRequest, _ ...grpc.CallOption) (*financepb.TagUsageResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) CreateProRataExpense(_ context.Context, _ *financepb.CreateProRataExpenseRequest, _ ...grpc.CallOption) (*financepb.ProRataResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) GetUpcomingProRata(_ context.Context, _ *financepb.GetUpcomingProRataRequest, _ ...grpc.CallOption) (*financepb.UpcomingProRataListResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) GetPeriodSummary(_ context.Context, _ *financepb.GetPeriodSummaryRequest, _ ...grpc.CallOption) (*financepb.PeriodSummaryResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) GetSpendingByTag(_ context.Context, _ *financepb.GetSpendingByTagRequest, _ ...grpc.CallOption) (*financepb.TagSpendingListResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) GetCumulativeSpend(_ context.Context, _ *financepb.GetCumulativeSpendRequest, _ ...grpc.CallOption) (*financepb.CumulativeSpendResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) GetHistoricalComparison(_ context.Context, _ *financepb.GetHistoricalComparisonRequest, _ ...grpc.CallOption) (*financepb.HistoricalComparisonResponse, error) {
	return nil, nil
}
func (m *mockFinanceServiceClient) DeleteAllUserData(_ context.Context, _ *financepb.DeleteAllUserDataRequest, _ ...grpc.CallOption) (*financepb.DeleteAllUserDataResponse, error) {
	return nil, nil
}

// mockExpenseServiceClient implements ExpenseServiceClient for tests. The
// expenses provider consumes StreamAllUserExpenses, so streamRows seeds the
// server stream (one ExpenseData per Recv, in order). streamOpenErr fails the
// RPC open; recvErr fails a Recv (at recvErrAt, 1-based; 0 = after all rows).
type mockExpenseServiceClient struct {
	streamRows    []*expensepb.ExpenseData
	streamOpenErr error
	recvErr       error
	recvErrAt     int
	lastStreamReq *expensepb.StreamAllUserExpensesRequest
	callCount     int
}

// fakeExpenseStream is the client side of a StreamAllUserExpenses server stream.
// The embedded nil grpc.ClientStream supplies the ClientStream methods the
// consumer never calls; only Recv is exercised.
type fakeExpenseStream struct {
	grpc.ClientStream
	rows      []*expensepb.ExpenseData
	idx       int
	recvErr   error
	recvErrAt int
}

func (f *fakeExpenseStream) Recv() (*expensepb.ExpenseData, error) {
	if f.recvErr != nil && f.recvErrAt > 0 && f.idx+1 == f.recvErrAt {
		return nil, f.recvErr
	}
	if f.idx >= len(f.rows) {
		if f.recvErr != nil && f.recvErrAt == 0 {
			return nil, f.recvErr
		}
		return nil, io.EOF
	}
	row := f.rows[f.idx]
	f.idx++
	return row, nil
}

func (m *mockExpenseServiceClient) StreamAllUserExpenses(_ context.Context, req *expensepb.StreamAllUserExpensesRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[expensepb.ExpenseData], error) {
	m.callCount++
	m.lastStreamReq = req
	if m.streamOpenErr != nil {
		return nil, m.streamOpenErr
	}
	return &fakeExpenseStream{rows: m.streamRows, recvErr: m.recvErr, recvErrAt: m.recvErrAt}, nil
}

// Implement remaining interface methods as no-ops.
func (m *mockExpenseServiceClient) CreateExpense(_ context.Context, _ *expensepb.CreateExpenseRequest, _ ...grpc.CallOption) (*expensepb.ExpenseResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) GetExpensesForPeriod(_ context.Context, _ *expensepb.GetExpensesForPeriodRequest, _ ...grpc.CallOption) (*expensepb.ExpenseListResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) GetExpense(_ context.Context, _ *expensepb.GetExpenseRequest, _ ...grpc.CallOption) (*expensepb.ExpenseResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) CountExpensesByTag(_ context.Context, _ *expensepb.CountExpensesByTagRequest, _ ...grpc.CallOption) (*expensepb.CountExpensesByTagResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) CorrectExpense(_ context.Context, _ *expensepb.CorrectExpenseRequest, _ ...grpc.CallOption) (*expensepb.ExpenseResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) GetCorrectionHistory(_ context.Context, _ *expensepb.GetCorrectionHistoryRequest, _ ...grpc.CallOption) (*expensepb.CorrectionHistoryResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) GetProRataGroup(_ context.Context, _ *expensepb.GetProRataGroupRequest, _ ...grpc.CallOption) (*expensepb.ExpenseListResponse, error) {
	return nil, nil
}
func (m *mockExpenseServiceClient) AnonymizeAllUserExpenses(_ context.Context, _ *expensepb.AnonymizeRequest, _ ...grpc.CallOption) (*expensepb.AnonymizeResponse, error) {
	return nil, nil
}
