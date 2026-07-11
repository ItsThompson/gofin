package providers

import (
	"context"

	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// ---------------------------------------------------------------------------
// Mock finance client
// ---------------------------------------------------------------------------

type mockFinanceClient struct {
	deleteAllUserDataErr error
	deleteCalledWith     string
}

func (m *mockFinanceClient) DeleteAllUserData(_ context.Context, req *financepb.DeleteAllUserDataRequest, _ ...grpc.CallOption) (*financepb.DeleteAllUserDataResponse, error) {
	m.deleteCalledWith = req.GetUserId()
	if m.deleteAllUserDataErr != nil {
		return nil, m.deleteAllUserDataErr
	}
	return &financepb.DeleteAllUserDataResponse{}, nil
}

// No-op stubs for remaining FinanceServiceClient interface methods.
func (m *mockFinanceClient) GetDefaults(_ context.Context, _ *financepb.GetDefaultsRequest, _ ...grpc.CallOption) (*financepb.DefaultsResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) UpdateDefaults(_ context.Context, _ *financepb.UpdateDefaultsRequest, _ ...grpc.CallOption) (*financepb.DefaultsResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) CompleteOnboarding(_ context.Context, _ *financepb.CompleteOnboardingRequest, _ ...grpc.CallOption) (*financepb.DefaultsResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) GetAllUserData(_ context.Context, _ *financepb.GetAllUserDataRequest, _ ...grpc.CallOption) (*financepb.AllUserDataResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) GetCurrentPeriod(_ context.Context, _ *financepb.GetCurrentPeriodRequest, _ ...grpc.CallOption) (*financepb.PeriodResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) CreatePeriod(_ context.Context, _ *financepb.CreatePeriodRequest, _ ...grpc.CallOption) (*financepb.PeriodResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) UpdatePeriod(_ context.Context, _ *financepb.UpdatePeriodRequest, _ ...grpc.CallOption) (*financepb.PeriodResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) ListPeriods(_ context.Context, _ *financepb.ListPeriodsRequest, _ ...grpc.CallOption) (*financepb.PeriodListResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) ListTags(_ context.Context, _ *financepb.ListTagsRequest, _ ...grpc.CallOption) (*financepb.TagListResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) CreateTag(_ context.Context, _ *financepb.CreateTagRequest, _ ...grpc.CallOption) (*financepb.TagResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) UpdateTag(_ context.Context, _ *financepb.UpdateTagRequest, _ ...grpc.CallOption) (*financepb.TagResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) DeleteTag(_ context.Context, _ *financepb.DeleteTagRequest, _ ...grpc.CallOption) (*financepb.DeleteTagResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) CheckTagUsage(_ context.Context, _ *financepb.CheckTagUsageRequest, _ ...grpc.CallOption) (*financepb.TagUsageResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) CreateProRataExpense(_ context.Context, _ *financepb.CreateProRataExpenseRequest, _ ...grpc.CallOption) (*financepb.ProRataResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) GetUpcomingProRata(_ context.Context, _ *financepb.GetUpcomingProRataRequest, _ ...grpc.CallOption) (*financepb.UpcomingProRataListResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) GetPeriodSummary(_ context.Context, _ *financepb.GetPeriodSummaryRequest, _ ...grpc.CallOption) (*financepb.PeriodSummaryResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) GetSpendingByTag(_ context.Context, _ *financepb.GetSpendingByTagRequest, _ ...grpc.CallOption) (*financepb.TagSpendingListResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) GetCumulativeSpend(_ context.Context, _ *financepb.GetCumulativeSpendRequest, _ ...grpc.CallOption) (*financepb.CumulativeSpendResponse, error) {
	return nil, nil
}
func (m *mockFinanceClient) GetHistoricalComparison(_ context.Context, _ *financepb.GetHistoricalComparisonRequest, _ ...grpc.CallOption) (*financepb.HistoricalComparisonResponse, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock expense client
// ---------------------------------------------------------------------------

type mockExpenseClient struct {
	anonymizeErr        error
	anonymizeCalledWith string
}

func (m *mockExpenseClient) AnonymizeAllUserExpenses(_ context.Context, req *expensepb.AnonymizeRequest, _ ...grpc.CallOption) (*expensepb.AnonymizeResponse, error) {
	m.anonymizeCalledWith = req.GetUserId()
	if m.anonymizeErr != nil {
		return nil, m.anonymizeErr
	}
	return &expensepb.AnonymizeResponse{}, nil
}

// No-op stubs for remaining ExpenseServiceClient interface methods.
func (m *mockExpenseClient) CreateExpense(_ context.Context, _ *expensepb.CreateExpenseRequest, _ ...grpc.CallOption) (*expensepb.ExpenseResponse, error) {
	return nil, nil
}
func (m *mockExpenseClient) GetExpensesForPeriod(_ context.Context, _ *expensepb.GetExpensesForPeriodRequest, _ ...grpc.CallOption) (*expensepb.ExpenseListResponse, error) {
	return nil, nil
}
func (m *mockExpenseClient) GetExpense(_ context.Context, _ *expensepb.GetExpenseRequest, _ ...grpc.CallOption) (*expensepb.ExpenseResponse, error) {
	return nil, nil
}
func (m *mockExpenseClient) CountExpensesByTag(_ context.Context, _ *expensepb.CountExpensesByTagRequest, _ ...grpc.CallOption) (*expensepb.CountExpensesByTagResponse, error) {
	return nil, nil
}
func (m *mockExpenseClient) CorrectExpense(_ context.Context, _ *expensepb.CorrectExpenseRequest, _ ...grpc.CallOption) (*expensepb.ExpenseResponse, error) {
	return nil, nil
}
func (m *mockExpenseClient) GetCorrectionHistory(_ context.Context, _ *expensepb.GetCorrectionHistoryRequest, _ ...grpc.CallOption) (*expensepb.CorrectionHistoryResponse, error) {
	return nil, nil
}
func (m *mockExpenseClient) GetProRataGroup(_ context.Context, _ *expensepb.GetProRataGroupRequest, _ ...grpc.CallOption) (*expensepb.ExpenseListResponse, error) {
	return nil, nil
}
func (m *mockExpenseClient) StreamAllUserExpenses(_ context.Context, _ *expensepb.StreamAllUserExpensesRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[expensepb.ExpenseData], error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock auth client
// ---------------------------------------------------------------------------

type mockAuthClient struct {
	deleteUserDataErr        error
	deleteUserDataCalledWith string
}

func (m *mockAuthClient) DeleteUserData(_ context.Context, req *authpb.DeleteUserDataRequest, _ ...grpc.CallOption) (*authpb.DeleteUserDataResponse, error) {
	m.deleteUserDataCalledWith = req.GetUserId()
	if m.deleteUserDataErr != nil {
		return nil, m.deleteUserDataErr
	}
	return &authpb.DeleteUserDataResponse{}, nil
}

// No-op stubs for remaining AuthServiceClient interface methods.
func (m *mockAuthClient) Register(_ context.Context, _ *authpb.RegisterRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) Login(_ context.Context, _ *authpb.LoginRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) ValidateToken(_ context.Context, _ *authpb.ValidateTokenRequest, _ ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) VerifyPassword(_ context.Context, _ *authpb.VerifyPasswordRequest, _ ...grpc.CallOption) (*authpb.VerifyPasswordResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) RefreshToken(_ context.Context, _ *authpb.RefreshTokenRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) Logout(_ context.Context, _ *authpb.LogoutRequest, _ ...grpc.CallOption) (*authpb.LogoutResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) AssumeIdentity(_ context.Context, _ *authpb.AssumeIdentityRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) RestoreIdentity(_ context.Context, _ *authpb.RestoreIdentityRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) ListUsers(_ context.Context, _ *authpb.ListUsersRequest, _ ...grpc.CallOption) (*authpb.ListUsersResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) GetUser(_ context.Context, _ *authpb.GetUserRequest, _ ...grpc.CallOption) (*authpb.UserResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) UpdateUser(_ context.Context, _ *authpb.UpdateUserRequest, _ ...grpc.CallOption) (*authpb.UserResponse, error) {
	return nil, nil
}
func (m *mockAuthClient) ChangePassword(_ context.Context, _ *authpb.ChangePasswordRequest, _ ...grpc.CallOption) (*authpb.ChangePasswordResponse, error) {
	return nil, nil
}
