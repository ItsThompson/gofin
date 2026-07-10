package engine_test

import (
	"context"

	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/datarights/internal/engine/providers"
	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
	"github.com/ItsThompson/gofin/services/perf"
)

// financeSpy is a finance client that records how many times each RPC is
// invoked, so efficiency tests can assert the export deduplicates finance
// calls. It embeds the full FinanceServiceClient interface (left nil): any RPC
// other than GetAllUserData / ListTags panics, surfacing unintended finance
// traffic. It embeds *perf.CallCounter for the concurrency-safe Count/Record.
type financeSpy struct {
	financepb.FinanceServiceClient
	*perf.CallCounter
	data *financepb.AllUserDataResponse
	tags *financepb.TagListResponse
}

func newFinanceSpy(data *financepb.AllUserDataResponse, tags *financepb.TagListResponse) *financeSpy {
	return &financeSpy{
		CallCounter: perf.NewCallCounter(),
		data:        data,
		tags:        tags,
	}
}

func (s *financeSpy) GetAllUserData(_ context.Context, _ *financepb.GetAllUserDataRequest, _ ...grpc.CallOption) (*financepb.AllUserDataResponse, error) {
	s.Record("GetAllUserData")
	return s.data, nil
}

func (s *financeSpy) ListTags(_ context.Context, _ *financepb.ListTagsRequest, _ ...grpc.CallOption) (*financepb.TagListResponse, error) {
	s.Record("ListTags")
	return s.tags, nil
}

// stubAuthClient serves a single canned profile for the profile provider.
type stubAuthClient struct {
	authpb.AuthServiceClient
	user *authpb.UserResponse
}

func (s *stubAuthClient) GetUser(_ context.Context, _ *authpb.GetUserRequest, _ ...grpc.CallOption) (*authpb.UserResponse, error) {
	return s.user, nil
}

// stubExpenseClient serves canned expense pages for the expenses provider.
type stubExpenseClient struct {
	expensepb.ExpenseServiceClient
	pages []*expensepb.ExpenseListResponse
	calls int
}

func (s *stubExpenseClient) GetAllUserExpenses(_ context.Context, _ *expensepb.GetAllUserExpensesRequest, _ ...grpc.CallOption) (*expensepb.ExpenseListResponse, error) {
	if s.calls >= len(s.pages) {
		return &expensepb.ExpenseListResponse{}, nil
	}
	resp := s.pages[s.calls]
	s.calls++
	return resp, nil
}

// buildRealProviders returns the export provider set in registration/ZIP order:
// profile, expenses, tags, budget_periods, default_settings. The finance-backed
// providers share the injected finance client.
func buildRealProviders(
	auth authpb.AuthServiceClient,
	expense expensepb.ExpenseServiceClient,
	finance financepb.FinanceServiceClient,
) []engine.DataProvider {
	return []engine.DataProvider{
		providers.NewProfileProvider(auth),
		providers.NewExpensesProvider(expense, finance),
		providers.NewTagsProvider(finance),
		providers.NewBudgetPeriodsProvider(finance),
		providers.NewDefaultSettingsProvider(finance),
	}
}

// cannedUser is the fixed profile served by stubAuthClient.
func cannedUser() *authpb.UserResponse {
	return &authpb.UserResponse{
		Username:  "alex",
		Email:     "alex@example.com",
		Currency:  "USD",
		Role:      "member",
		CreatedAt: "2025-01-01T00:00:00Z",
	}
}

// cannedTags is the shared tag set. The byte-identical guarantee (US-EXP-01)
// rests on ListTags and GetAllUserData().GetTags() returning this same set, so
// both canned responses are built from it.
func cannedTags() []*financepb.TagData {
	return []*financepb.TagData{
		{Id: "tag-1", Name: "Food", IsDefault: true, CreatedAt: "2025-06-01T10:00:00Z"},
		{Id: "tag-2", Name: "Transport", IsDefault: false, CreatedAt: "2025-07-15T14:30:00Z"},
	}
}

func cannedAllUserData() *financepb.AllUserDataResponse {
	return &financepb.AllUserDataResponse{
		Tags: cannedTags(),
		Periods: []*financepb.PeriodData{
			{
				Id: "period-1", Year: 2026, Month: 5, BudgetAmount: 250000,
				EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
				CreatedAt: "2026-05-01T00:00:00Z",
			},
		},
		Defaults: &financepb.DefaultsData{
			BudgetAmount: 300000, EssentialsPercent: 50, DesiresPercent: 30,
			SavingsPercent: 20, Currency: "USD",
		},
	}
}

func cannedTagList() *financepb.TagListResponse {
	return &financepb.TagListResponse{Tags: cannedTags()}
}

func cannedExpensePages() []*expensepb.ExpenseListResponse {
	return []*expensepb.ExpenseListResponse{
		{
			Data: []*expensepb.ExpenseData{
				{
					Id: "exp-1", Name: "Groceries", Amount: 4599, Currency: "USD",
					ExpenseType: "essentials", TagId: "tag-1", ExpenseDate: "2026-05-01",
					PeriodYear: 2026, PeriodMonth: 5, Status: "active",
					CreatedAt: "2026-05-01T12:00:00Z",
				},
				{
					Id: "exp-2", Name: "Bus pass", Amount: 3000, Currency: "USD",
					ExpenseType: "essentials", TagId: "tag-2", ExpenseDate: "2026-05-02",
					PeriodYear: 2026, PeriodMonth: 5, Status: "active",
					CreatedAt: "2026-05-02T09:00:00Z",
				},
			},
			HasMore: false,
		},
	}
}
