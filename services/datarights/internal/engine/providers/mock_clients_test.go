package providers

import (
	"context"

	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// mockFinanceServiceClient implements FinanceServiceClient for tests.
type mockFinanceServiceClient struct {
	getAllUserDataResp *financepb.AllUserDataResponse
	getAllUserDataErr  error
	listTagsResp      *financepb.TagListResponse
	listTagsErr       error
}

func (m *mockFinanceServiceClient) GetAllUserData(_ context.Context, _ *financepb.GetAllUserDataRequest, _ ...grpc.CallOption) (*financepb.AllUserDataResponse, error) {
	if m.getAllUserDataErr != nil {
		return nil, m.getAllUserDataErr
	}
	return m.getAllUserDataResp, nil
}

func (m *mockFinanceServiceClient) ListTags(_ context.Context, _ *financepb.ListTagsRequest, _ ...grpc.CallOption) (*financepb.TagListResponse, error) {
	if m.listTagsErr != nil {
		return nil, m.listTagsErr
	}
	return m.listTagsResp, nil
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

// mockExpenseServiceClient implements ExpenseServiceClient for tests.
type mockExpenseServiceClient struct {
	// getAllUserExpensesResponses is a list of responses, one per page call.
	getAllUserExpensesResponses []*expensepb.ExpenseListResponse
	getAllUserExpensesErr       error
	callCount                  int
}

func (m *mockExpenseServiceClient) GetAllUserExpenses(_ context.Context, _ *expensepb.GetAllUserExpensesRequest, _ ...grpc.CallOption) (*expensepb.ExpenseListResponse, error) {
	if m.getAllUserExpensesErr != nil {
		return nil, m.getAllUserExpensesErr
	}
	if m.callCount >= len(m.getAllUserExpensesResponses) {
		return &expensepb.ExpenseListResponse{}, nil
	}
	resp := m.getAllUserExpensesResponses[m.callCount]
	m.callCount++
	return resp, nil
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
func (m *mockExpenseServiceClient) StreamAllUserExpenses(_ context.Context, _ *expensepb.StreamAllUserExpensesRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[expensepb.ExpenseData], error) {
	return nil, nil
}
