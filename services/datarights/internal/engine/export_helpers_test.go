package engine_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/ItsThompson/gofin/services/auth/proto/authpb"
	"github.com/ItsThompson/gofin/services/datarights/internal/email"
	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/datarights/internal/engine/providers"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/repository"
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
	data    *financepb.AllUserDataResponse
	dataErr error
	tags    *financepb.TagListResponse
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
	if s.dataErr != nil {
		return nil, s.dataErr
	}
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

// stubExpenseClient serves canned expenses to the expenses provider. The
// provider consumes StreamAllUserExpenses, so the canned pages are flattened
// into a single ordered server stream (the byte-identical fixture relies on the
// stream preserving the pages' chronological order).
type stubExpenseClient struct {
	expensepb.ExpenseServiceClient
	pages []*expensepb.ExpenseListResponse
}

func (s *stubExpenseClient) StreamAllUserExpenses(_ context.Context, _ *expensepb.StreamAllUserExpensesRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[expensepb.ExpenseData], error) {
	var rows []*expensepb.ExpenseData
	for _, page := range s.pages {
		rows = append(rows, page.GetData()...)
	}
	return &fakeExpenseStream{rows: rows}, nil
}

// fakeExpenseStream is the client side of a StreamAllUserExpenses server stream.
// The embedded nil grpc.ClientStream supplies the methods the consumer never
// calls; only Recv is exercised.
type fakeExpenseStream struct {
	grpc.ClientStream
	rows []*expensepb.ExpenseData
	idx  int
}

func (f *fakeExpenseStream) Recv() (*expensepb.ExpenseData, error) {
	if f.idx >= len(f.rows) {
		return nil, io.EOF
	}
	row := f.rows[f.idx]
	f.idx++
	return row, nil
}

// buildRealProviders returns the export provider set in registration/ZIP order:
// profile, expenses, tags, budget_periods, default_settings. The finance-backed
// providers map the shared finance response (fetched once per export); the
// expenses provider resolves tag names from the derived tag map.
func buildRealProviders(
	auth authpb.AuthServiceClient,
	expense expensepb.ExpenseServiceClient,
	financeData *financepb.AllUserDataResponse,
) []engine.DataProvider {
	return []engine.DataProvider{
		providers.NewProfileProvider(auth),
		providers.NewExpensesProvider(expense, providers.BuildTagMap(financeData), providers.BuildPeriodCurrencyMap(financeData)),
		providers.NewTagsProvider(financeData),
		providers.NewBudgetPeriodsProvider(financeData),
		providers.NewDefaultSettingsProvider(financeData),
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

// cannedTags is the shared tag set. The byte-identical export guarantee rests on
// ListTags and GetAllUserData().GetTags() returning this same set, so both
// canned responses are built from it.
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
				ReportingCurrency: "USD", CreatedAt: "2026-05-01T00:00:00Z",
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
					Id: "exp-1", Name: "Groceries", TransactionAmount: 4599, TransactionCurrency: "USD",
					ExpenseType: "essentials", TagId: "tag-1", ExpenseDate: "2026-05-01",
					PeriodYear: 2026, PeriodMonth: 5, Status: "active",
					CreatedAt:           "2026-05-01T12:00:00Z",
					ReportingAmount: 4599, ReportingCurrency: "USD",
					ExchangeRate: "1", ExchangeRateSource: "identity", ExchangeRateTimestamp: "2026-05-01T12:00:00Z",
				},
				{
					Id: "exp-2", Name: "Bus pass", TransactionAmount: 3000, TransactionCurrency: "USD",
					ExpenseType: "essentials", TagId: "tag-2", ExpenseDate: "2026-05-02",
					PeriodYear: 2026, PeriodMonth: 5, Status: "active",
					CreatedAt:           "2026-05-02T09:00:00Z",
					ReportingAmount: 3000, ReportingCurrency: "USD",
					ExchangeRate: "1", ExchangeRateSource: "identity", ExchangeRateTimestamp: "2026-05-02T09:00:00Z",
				},
			},
			HasMore: false,
		},
	}
}

// newExportEngine wires the real export providers through the per-job factory.
// The finance spy is the engine's finance client; execute fetches GetAllUserData
// once per job and passes the resolved response to the factory. Used by the
// single-fetch regression test.
func newExportEngine(finance financepb.FinanceServiceClient, repo repository.JobRepository) *engine.Engine {
	auth := &stubAuthClient{user: cannedUser()}
	expense := &stubExpenseClient{pages: cannedExpensePages()}
	return engine.NewEngine(
		func(financeData *financepb.AllUserDataResponse) []engine.DataProvider {
			return buildRealProviders(auth, expense, financeData)
		},
		finance, repo, noopSender{}, 5, 30*time.Second, discardLogger(),
	)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// noopSender is an email.Sender that accepts every send.
type noopSender struct{}

var _ email.Sender = noopSender{}

func (noopSender) SendExportEmail(_ context.Context, _ string, _ []byte) error { return nil }

// recordingRepo is a JobRepository that records terminal transitions so tests
// can wait for job completion. Only the methods engine.execute calls do work.
type recordingRepo struct {
	mu        sync.Mutex
	completed int
	failed    []string
}

func newRecordingRepo() *recordingRepo { return &recordingRepo{} }

func (r *recordingRepo) UpdateStatus(_ context.Context, _, _ string) error { return nil }

func (r *recordingRepo) CompleteJob(_ context.Context, _ string, _ int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completed++
	return nil
}

func (r *recordingRepo) FailJob(_ context.Context, _ string, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed = append(r.failed, errMsg)
	return nil
}

func (r *recordingRepo) completedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.completed
}

func (r *recordingRepo) failures() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.failed...)
}

func (r *recordingRepo) CreateJob(_ context.Context, _ string) (*model.ExportJob, error) {
	return nil, nil
}
func (r *recordingRepo) GetJob(_ context.Context, _ string) (*model.ExportJob, error) {
	return nil, nil
}
func (r *recordingRepo) ListJobsByUser(_ context.Context, _ string, _, _ int) ([]*model.ExportJob, int64, error) {
	return nil, 0, nil
}
func (r *recordingRepo) GetInProgressJob(_ context.Context, _ string) (*model.ExportJob, error) {
	return nil, nil
}
func (r *recordingRepo) GetLatestNonFailedJob(_ context.Context, _ string) (*model.ExportJob, error) {
	return nil, nil
}
func (r *recordingRepo) GetNonTerminalJobs(_ context.Context) ([]model.RecoverableJob, error) {
	return nil, nil
}
