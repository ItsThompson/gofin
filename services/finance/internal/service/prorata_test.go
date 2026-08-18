package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// int64Ptr returns a pointer to v for building *int64 request fields.
func int64Ptr(v int64) *int64 { return &v }

// mockFxClient implements FxClient for service tests.
type mockFxClient struct {
	mock.Mock
}

func (m *mockFxClient) CaptureRateSnapshot(ctx context.Context, req FxCaptureRequest) (*model.CapturedRateSnapshot, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.CapturedRateSnapshot), args.Error(1)
}

// --- CalculateInstallments Tests ---

func TestCalculateInstallments_EvenSplit(t *testing.T) {
	result := CalculateInstallments(9000, 3)
	assert.Equal(t, []int64{3000, 3000, 3000}, result)
}

func TestCalculateInstallments_RemainderAbsorbedByFirst(t *testing.T) {
	result := CalculateInstallments(10000, 3)
	// 10000 / 3 = 3333 base, remainder = 1
	assert.Equal(t, []int64{3334, 3333, 3333}, result)

	total := int64(0)
	for _, v := range result {
		total += v
	}
	assert.Equal(t, int64(10000), total)
}

func TestCalculateInstallments_TwoMonths(t *testing.T) {
	result := CalculateInstallments(10001, 2)
	assert.Equal(t, []int64{5001, 5000}, result)
}

func TestCalculateInstallments_SingleMonth(t *testing.T) {
	result := CalculateInstallments(5000, 1)
	assert.Equal(t, []int64{5000}, result)
}

func TestCalculateInstallments_ZeroMonths(t *testing.T) {
	result := CalculateInstallments(5000, 0)
	assert.Nil(t, result)
}

func TestCalculateInstallments_LargeRemainder(t *testing.T) {
	// 7 cents / 3 months = 2 base, 1 remainder
	result := CalculateInstallments(7, 3)
	assert.Equal(t, []int64{3, 2, 2}, result)
	assert.Equal(t, int64(7), result[0]+result[1]+result[2])
}

func TestCalculateInstallments_SpecExample(t *testing.T) {
	// $100 over 3 months: base 3333, first installment absorbs the +1 remainder -> 3334 + 3333 + 3333
	result := CalculateInstallments(10000, 3)
	assert.Equal(t, []int64{3334, 3333, 3333}, result)
	assert.Equal(t, int64(10000), result[0]+result[1]+result[2])
}

// --- AdvanceMonth Tests ---

func TestAdvanceMonth_Normal(t *testing.T) {
	y, m := AdvanceMonth(2026, 5)
	assert.Equal(t, int32(2026), y)
	assert.Equal(t, int32(6), m)
}

func TestAdvanceMonth_YearRollover(t *testing.T) {
	y, m := AdvanceMonth(2026, 12)
	assert.Equal(t, int32(2027), y)
	assert.Equal(t, int32(1), m)
}

func TestAdvanceMonth_NovemberPlusThree(t *testing.T) {
	// Nov + 1 = Dec, Dec + 1 = Jan next year, Jan + 1 = Feb next year
	y, m := int32(2026), int32(11)
	y, m = AdvanceMonth(y, m) // Dec 2026
	assert.Equal(t, int32(2026), y)
	assert.Equal(t, int32(12), m)

	y, m = AdvanceMonth(y, m) // Jan 2027
	assert.Equal(t, int32(2027), y)
	assert.Equal(t, int32(1), m)

	y, m = AdvanceMonth(y, m) // Feb 2027
	assert.Equal(t, int32(2027), y)
	assert.Equal(t, int32(2), m)
}

// --- computeMissedMonths Tests ---

func TestComputeMissedMonths_NoGap(t *testing.T) {
	// Last period April, creating May: no missed months
	missed := computeMissedMonths(2026, 4, 2026, 5)
	assert.Empty(t, missed)
}

func TestComputeMissedMonths_OneMonthGap(t *testing.T) {
	// Last period March, creating May: April is missed
	missed := computeMissedMonths(2026, 3, 2026, 5)
	assert.Equal(t, []yearMonth{{2026, 4}}, missed)
}

func TestComputeMissedMonths_MultiMonthGap(t *testing.T) {
	// Last period Jan, creating May: Feb, Mar, Apr missed
	missed := computeMissedMonths(2026, 1, 2026, 5)
	assert.Equal(t, []yearMonth{
		{2026, 2},
		{2026, 3},
		{2026, 4},
	}, missed)
}

func TestComputeMissedMonths_YearBoundary(t *testing.T) {
	// Last period Nov 2025, creating Feb 2026: Dec 2025, Jan 2026 missed
	missed := computeMissedMonths(2025, 11, 2026, 2)
	assert.Equal(t, []yearMonth{
		{2025, 12},
		{2026, 1},
	}, missed)
}

func TestComputeMissedMonths_SameMonth(t *testing.T) {
	// Edge case: same month, no gap
	missed := computeMissedMonths(2026, 5, 2026, 5)
	assert.Empty(t, missed)
}

// --- CreateProRataExpense Tests ---

func snapshotFixture() *model.CapturedRateSnapshot {
	return &model.CapturedRateSnapshot{
		SnapshotVersion: 1,
		Source:          "open_exchange_rates",
		BaseCurrency:    "USD",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		CapturedAt:      "2026-05-15T12:00:00Z",
		ExpiresAt:       "2026-05-15T13:00:00Z",
		RatesByCurrency: map[string]string{
			"USD": "1",
			"EUR": "0.92",
			"GBP": "0.79",
			"JPY": "150.00",
		},
	}
}

func newProRataTestService(repo *mockRepo, txBeg *mockTxBeg, expClient *mockExpClient, fxClient *mockFxClient, nowFunc func() time.Time) *FinanceService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	var ec ExpenseClient
	if expClient != nil {
		ec = expClient
	}
	var fc FxClient
	if fxClient != nil {
		fc = fxClient
	}
	return NewFinanceServiceWithFx(repo, txBeg, ec, fc, nowFunc, logger)
}

// stubProRataRepo wires GetCurrentPeriod for the creation period so tests that
// reach the period check do not need to repeat the mock setup.
func stubCreationPeriod(repo *mockRepo, year, month int32) {
	repo.On("GetCurrentPeriod", mock.Anything, "user-1", year, month).
		Return(makePeriod("period-"+fmt.Sprintf("%d-%02d", year, month), year, month), nil)
}

func TestCreateProRataExpense_Success(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	fxClient := new(mockFxClient)
	svc := newProRataTestService(repo, txBeg, expClient, fxClient, fixedNow(2026, 5, 15))

	stubCreationPeriod(repo, 2026, 5)
	snapshot := snapshotFixture()
	fxClient.On("CaptureRateSnapshot", mock.Anything, mock.MatchedBy(func(req FxCaptureRequest) bool {
		return len(req.RequiredCurrencies) == 2 && req.RequiredCurrencies[0] == "USD" && req.RequiredCurrencies[1] == "USD"
	})).Return(snapshot, nil)

	expClient.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req CreateProRataInstallmentInput) bool {
		return req.Name == "Annual subscription" &&
			req.Amount == int64(3334) &&
			req.Currency == "USD" &&
			req.ProRataIndex == int32(1) &&
			req.ProRataTotal == int32(3) &&
			req.PeriodContext.PeriodID == "period-2026-05" &&
			req.PeriodContext.UserID == "user-1" &&
			req.PeriodContext.Year == 2026 &&
			req.PeriodContext.Month == 5 &&
			req.PeriodContext.ReportingCurrency == "USD" &&
			req.PeriodContext.Source == "finance_service" &&
			req.CapturedRateSnapshot == snapshot
	})).Return(&CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-05-15T12:00:00Z"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.InstallmentIndex == 2 && s.TargetYear == 2026 && s.TargetMonth == 6 &&
			s.TransactionAmount == 3333 && s.TransactionCurrency == "USD" &&
			s.CreationReportingCurrency == "USD" &&
			s.CapturedRateSnapshot.RateTimestamp == snapshot.RateTimestamp &&
			s.CapturedRateSnapshot.Source == snapshot.Source
	})).Return(&model.ProRataSchedule{
		ID: "sched-1", InstallmentIndex: 2, TargetYear: 2026, TargetMonth: 6, Amount: 3333, Status: "pending",
	}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.InstallmentIndex == 3 && s.TargetYear == 2026 && s.TargetMonth == 7 &&
			s.TransactionAmount == 3333 && s.TransactionCurrency == "USD" &&
			s.CreationReportingCurrency == "USD" &&
			s.CapturedRateSnapshot.RateTimestamp == snapshot.RateTimestamp &&
			s.CapturedRateSnapshot.Source == snapshot.Source
	})).Return(&model.ProRataSchedule{
		ID: "sched-2", InstallmentIndex: 3, TargetYear: 2026, TargetMonth: 7, Amount: 3333, Status: "pending",
	}, nil)

	result, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name:        "Annual subscription",
		TotalAmount: 10000,
		TransactionCurrency: "USD",
		ExpenseType: "essentials",
		TagID:       "tag-1",
		ExpenseDate: "2026-05-15",
		Months:      3,
		PeriodYear:  2026,
		PeriodMonth: 5,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "exp-1", result.Expense.ID)
	assert.Equal(t, int64(3334), result.Expense.Amount)
	assert.Len(t, result.Schedules, 2)
	assert.Equal(t, int32(6), result.Schedules[0].TargetMonth)
	assert.Equal(t, int32(7), result.Schedules[1].TargetMonth)
	fxClient.AssertExpectations(t)
	expClient.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestCreateProRataExpense_YearRollover(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	fxClient := new(mockFxClient)
	svc := newProRataTestService(repo, txBeg, expClient, fxClient, fixedNow(2026, 11, 1))

	stubCreationPeriod(repo, 2026, 11)
	fxClient.On("CaptureRateSnapshot", mock.Anything, mock.Anything).Return(snapshotFixture(), nil)
	expClient.On("CreateProRataInstallment", mock.Anything, mock.Anything).
		Return(&CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-11-01T00:00:00Z"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.InstallmentIndex == 2 && s.TargetYear == 2026 && s.TargetMonth == 12
	})).Return(&model.ProRataSchedule{ID: "s-1", TargetYear: 2026, TargetMonth: 12, Status: "pending"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.InstallmentIndex == 3 && s.TargetYear == 2027 && s.TargetMonth == 1
	})).Return(&model.ProRataSchedule{ID: "s-2", TargetYear: 2027, TargetMonth: 1, Status: "pending"}, nil)

	result, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name: "Insurance", TotalAmount: 6000, TransactionCurrency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", ExpenseDate: "2026-11-01", Months: 3, PeriodYear: 2026, PeriodMonth: 11,
	})

	require.NoError(t, err)
	assert.Len(t, result.Schedules, 2)
	assert.Equal(t, int32(2026), result.Schedules[0].TargetYear)
	assert.Equal(t, int32(12), result.Schedules[0].TargetMonth)
	assert.Equal(t, int32(2027), result.Schedules[1].TargetYear)
	assert.Equal(t, int32(1), result.Schedules[1].TargetMonth)
}

func TestCreateProRataExpense_TransactionCurrencyOnly(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	fxClient := new(mockFxClient)
	svc := newProRataTestService(repo, txBeg, expClient, fxClient, fixedNow(2026, 5, 15))

	stubCreationPeriod(repo, 2026, 5)
	fxClient.On("CaptureRateSnapshot", mock.Anything, mock.Anything).Return(snapshotFixture(), nil)
	expClient.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req CreateProRataInstallmentInput) bool {
		return req.Currency == "EUR"
	})).Return(&CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-05-15T12:00:00Z"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.TransactionCurrency == "EUR"
	})).Return(&model.ProRataSchedule{
		ID: "sched-1", Status: "pending",
	}, nil)

	result, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name:                "Annual subscription",
		TotalAmount:         6000,
		TransactionCurrency: "EUR",
		ExpenseType:         "essentials",
		TagID:               "tag-1",
		ExpenseDate:         "2026-05-15",
		Months:              2,
		PeriodYear:          2026,
		PeriodMonth:         5,
	})

	require.NoError(t, err)
	assert.Equal(t, "EUR", result.Expense.TransactionCurrency)
	expClient.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestCreateProRataExpense_MissingCurrencyDefaultsToPeriodReportingCurrency(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	fxClient := new(mockFxClient)
	svc := newProRataTestService(repo, txBeg, expClient, fxClient, fixedNow(2026, 5, 15))

	stubCreationPeriod(repo, 2026, 5)
	fxClient.On("CaptureRateSnapshot", mock.Anything, mock.Anything).Return(snapshotFixture(), nil)
	expClient.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req CreateProRataInstallmentInput) bool {
		return req.Currency == "USD"
	})).Return(&CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-05-15T12:00:00Z"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.Anything).
		Return(&model.ProRataSchedule{ID: "sched-1", Status: "pending"}, nil)

	result, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name:        "Insurance",
		TotalAmount: 6000,
		ExpenseType: "essentials",
		TagID:       "tag-1",
		ExpenseDate: "2026-05-15",
		Months:      2,
		PeriodYear:  2026,
		PeriodMonth: 5,
	})

	require.NoError(t, err)
	assert.Equal(t, "USD", result.Expense.TransactionCurrency)
}

func TestCreateProRataExpense_MissingPeriodFields(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	fxClient := new(mockFxClient)
	svc := newProRataTestService(repo, txBeg, expClient, fxClient, fixedNow(2026, 5, 15))

	_, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name: "Insurance", TotalAmount: 6000, TransactionCurrency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", ExpenseDate: "2026-05-15", Months: 2,
	})

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
	assert.Contains(t, svcErr.Fields, "periodYear")
	repo.AssertNotCalled(t, "GetCurrentPeriod", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	fxClient.AssertNotCalled(t, "CaptureRateSnapshot", mock.Anything, mock.Anything)
	expClient.AssertNotCalled(t, "CreateProRataInstallment", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateProRataSchedule", mock.Anything, mock.Anything)
}

func TestCreateProRataExpense_MissingCreationPeriod(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	fxClient := new(mockFxClient)
	svc := newProRataTestService(repo, txBeg, expClient, fxClient, fixedNow(2026, 5, 15))

	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(6)).Return(nil, nil)

	_, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name: "Insurance", TotalAmount: 6000, TransactionCurrency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", ExpenseDate: "2026-05-15", Months: 2, PeriodYear: 2026, PeriodMonth: 6,
	})

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrPeriodNotFound, svcErr.Code)
	fxClient.AssertNotCalled(t, "CaptureRateSnapshot", mock.Anything, mock.Anything)
	expClient.AssertNotCalled(t, "CreateProRataInstallment", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateProRataSchedule", mock.Anything, mock.Anything)
}

func TestCreateProRataExpense_FxCaptureFailure(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	fxClient := new(mockFxClient)
	svc := newProRataTestService(repo, txBeg, expClient, fxClient, fixedNow(2026, 5, 15))

	stubCreationPeriod(repo, 2026, 5)
	fxClient.On("CaptureRateSnapshot", mock.Anything, mock.Anything).
		Return(nil, &apierr.Error{Code: model.ErrConversionUnavailable, Message: "conversion unavailable", Status: 503})

	_, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name: "Insurance", TotalAmount: 6000, TransactionCurrency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", ExpenseDate: "2026-05-15", Months: 2, PeriodYear: 2026, PeriodMonth: 5,
	})

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrConversionUnavailable, svcErr.Code)
	expClient.AssertNotCalled(t, "CreateProRataInstallment", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateProRataSchedule", mock.Anything, mock.Anything)
}

func TestCreateProRataExpense_Validation(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	fxClient := new(mockFxClient)
	svc := newProRataTestService(repo, txBeg, nil, fxClient, fixedNow(2026, 5, 15))

	tests := []struct {
		name  string
		req   *model.CreateProRataRequest
		field string
		msg   string
	}{
		{"empty name", &model.CreateProRataRequest{TotalAmount: 100, Months: 2, TransactionCurrency: "USD", ExpenseType: "essentials", TagID: "t", ExpenseDate: "2026-05-01", PeriodYear: 2026, PeriodMonth: 5}, "name", "required"},
		{"zero amount", &model.CreateProRataRequest{Name: "X", TotalAmount: 0, Months: 2, TransactionCurrency: "USD", ExpenseType: "essentials", TagID: "t", ExpenseDate: "2026-05-01", PeriodYear: 2026, PeriodMonth: 5}, "totalAmount", "must be positive"},
		{"one month", &model.CreateProRataRequest{Name: "X", TotalAmount: 100, Months: 1, TransactionCurrency: "USD", ExpenseType: "essentials", TagID: "t", ExpenseDate: "2026-05-01", PeriodYear: 2026, PeriodMonth: 5}, "months", "must be at least 2"},
		{"bad type", &model.CreateProRataRequest{Name: "X", TotalAmount: 100, Months: 2, TransactionCurrency: "USD", ExpenseType: "invalid", TagID: "t", ExpenseDate: "2026-05-01", PeriodYear: 2026, PeriodMonth: 5}, "expenseType", "must be essentials, desires, or savings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateProRataExpense(context.Background(), "user-1", tt.req)
			require.Error(t, err)
			svcErr := requireAPIError(t, err)
			assert.Equal(t, apierr.CodeValidation, svcErr.Code)
			assert.Equal(t, "validation failed", svcErr.Message)
			assert.Equal(t, tt.msg, svcErr.Fields[tt.field])
		})
	}
}

func TestCreateProRataExpense_ValidationAggregatesAllErrors(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	fxClient := new(mockFxClient)
	svc := newProRataTestService(repo, txBeg, nil, fxClient, fixedNow(2026, 5, 15))

	_, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name: "  ", TotalAmount: 0, Months: 1, TransactionCurrency: "USD",
		ExpenseType: "invalid", TagID: " ", ExpenseDate: " ", PeriodYear: 0, PeriodMonth: 0,
	})

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
	assert.Equal(t, "validation failed", svcErr.Message)
	assert.Equal(t, map[string]string{
		"name":        "required",
		"totalAmount": "must be positive",
		"months":      "must be at least 2",
		"expenseType": "must be essentials, desires, or savings",
		"tagId":       "required",
		"expenseDate": "required",
		"periodYear":  "required",
		"periodMonth": "must be between 1 and 12",
	}, svcErr.Fields)
}

func TestCreateProRataExpense_ScheduleFailure(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	fxClient := new(mockFxClient)
	svc := newProRataTestService(repo, txBeg, expClient, fxClient, fixedNow(2026, 5, 15))

	stubCreationPeriod(repo, 2026, 5)
	fxClient.On("CaptureRateSnapshot", mock.Anything, mock.Anything).Return(snapshotFixture(), nil)
	expClient.On("CreateProRataInstallment", mock.Anything, mock.Anything).
		Return(&CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-05-15T12:00:00Z"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("db error"))

	_, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name: "Test", TotalAmount: 6000, TransactionCurrency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", ExpenseDate: "2026-05-15", Months: 2, PeriodYear: 2026, PeriodMonth: 5,
	})

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeInternal, svcErr.Code)
	assert.Contains(t, svcErr.Message, "schedule creation failed")
}

// --- CreatePeriodWithProRata Tests ---

func TestCreatePeriodWithProRata_NoPriorPeriod(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetLatestPeriod", mock.Anything, "user-1").Return(nil, nil)

	createdPeriod := makePeriod("p-1", 2026, 5)
	repo.On("CreatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).Return(createdPeriod, nil)
	repo.On("GetPendingProRata", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{}, nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: "USD",
	})

	require.NoError(t, err)
	assert.Equal(t, "p-1", result.Period.ID)
	assert.Empty(t, result.AppliedProRata)
	assert.Equal(t, 0, result.AutoCreatedPeriods)
}

func TestCreatePeriodWithProRata_AutoCreatedPeriodsUseDefaultCurrency(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetLatestPeriod", mock.Anything, "user-1").
		Return(makePeriod("p-mar", 2026, 3), nil)
	repo.On("GetDefaults", mock.Anything, "user-1").Return(&model.DefaultSettings{
		BudgetAmount:      200000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
		Currency:          "EUR",
	}, nil)
	repo.On("CreatePeriod", mock.Anything, mock.MatchedBy(func(p *model.BudgetPeriod) bool {
		return p.Year == 2026 && p.Month == 4 && p.ReportingCurrency == "EUR"
	})).Return(makePeriod("p-apr", 2026, 4), nil)
	repo.On("CreatePeriod", mock.Anything, mock.MatchedBy(func(p *model.BudgetPeriod) bool {
		return p.Year == 2026 && p.Month == 5 && p.ReportingCurrency == "JPY"
	})).Return(makePeriod("p-may", 2026, 5), nil)
	repo.On("GetPendingProRata", mock.Anything, "user-1", mock.Anything, mock.Anything).
		Return([]*model.ProRataSchedule{}, nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: "JPY",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.AutoCreatedPeriods)
	repo.AssertExpectations(t)
}
func TestCreatePeriodWithProRata_RejectsUnsupportedReportingCurrency(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	_, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: "XYZ",
	})

	require.Error(t, err)
	var svcErr *apierr.Error
	require.ErrorAs(t, err, &svcErr)
	assert.Equal(t, model.ErrUnsupportedCurrency, svcErr.Code)
	assert.Equal(t, "unsupported currency", svcErr.Fields["reportingCurrency"])
	repo.AssertNotCalled(t, "CreatePeriod", mock.Anything, mock.Anything)
}

func TestCreatePeriodWithProRata_AppliesSchedules(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetLatestPeriod", mock.Anything, "user-1").
		Return(makePeriod("p-prev", 2026, 4), nil)

	createdPeriod := makePeriod("p-1", 2026, 5)
	repo.On("CreatePeriod", mock.Anything, mock.AnythingOfType("*model.BudgetPeriod")).Return(createdPeriod, nil)

	pendingSchedule := &model.ProRataSchedule{
		ID: "sched-1", UserID: "user-1", ProRataGroup: "group-1",
		Name: "Insurance", Amount: 3333, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 5, InstallmentIndex: 2,
		InstallmentTotal: 3, Status: "pending",
	}
	repo.On("GetPendingProRata", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{pendingSchedule}, nil)

	expClient.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req CreateProRataInstallmentInput) bool {
		return req.Name == "Insurance" && req.Amount == 3333 && req.ProRataIndex == 2 &&
			req.LegacyMigration && req.PeriodContext.ReportingCurrency == "USD" &&
			req.Currency == "USD"
	})).Return(&CreatedExpenseData{ID: "exp-applied", CreatedAt: "2026-05-01T00:00:00Z"}, nil)

	repo.On("MarkProRataApplied", mock.Anything, "sched-1").Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: "USD",
	})

	require.NoError(t, err)
	assert.Len(t, result.AppliedProRata, 1)
	assert.Equal(t, "sched-1", result.AppliedProRata[0].ID)
	assert.Equal(t, "applied", result.AppliedProRata[0].Status)
	repo.AssertCalled(t, "MarkProRataApplied", mock.Anything, "sched-1")
}

func TestCreatePeriodWithProRata_MissedMonths(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	// Last period was February, creating May: March and April are missed
	repo.On("GetLatestPeriod", mock.Anything, "user-1").
		Return(makePeriod("p-feb", 2026, 2), nil)

	defaults := &model.DefaultSettings{
		BudgetAmount: 200000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20, Currency: "USD",
	}
	repo.On("GetDefaults", mock.Anything, "user-1").Return(defaults, nil)

	// Auto-create March and April
	repo.On("CreatePeriod", mock.Anything, mock.MatchedBy(func(p *model.BudgetPeriod) bool {
		return p.Year == 2026 && p.Month == 3
	})).Return(makePeriod("p-mar", 2026, 3), nil)

	repo.On("CreatePeriod", mock.Anything, mock.MatchedBy(func(p *model.BudgetPeriod) bool {
		return p.Year == 2026 && p.Month == 4
	})).Return(makePeriod("p-apr", 2026, 4), nil)

	// Create May period
	repo.On("CreatePeriod", mock.Anything, mock.MatchedBy(func(p *model.BudgetPeriod) bool {
		return p.Year == 2026 && p.Month == 5
	})).Return(makePeriod("p-may", 2026, 5), nil)

	// No pending pro-rata for any month
	repo.On("GetPendingProRata", mock.Anything, "user-1", mock.Anything, mock.Anything).
		Return([]*model.ProRataSchedule{}, nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: "USD",
	})

	require.NoError(t, err)
	assert.Equal(t, 2, result.AutoCreatedPeriods)
	assert.Len(t, result.AutoCreatedMonths, 2)
	assert.Contains(t, result.AutoCreatedMonths[0], "March")
	assert.Contains(t, result.AutoCreatedMonths[1], "April")
}

func TestCreatePeriodWithProRata_MissedMonthsWithProRata(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetLatestPeriod", mock.Anything, "user-1").
		Return(makePeriod("p-mar", 2026, 3), nil)

	defaults := &model.DefaultSettings{
		BudgetAmount: 200000, EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20, Currency: "USD",
	}
	repo.On("GetDefaults", mock.Anything, "user-1").Return(defaults, nil)

	// Auto-create April
	repo.On("CreatePeriod", mock.Anything, mock.MatchedBy(func(p *model.BudgetPeriod) bool {
		return p.Month == 4
	})).Return(makePeriod("p-apr", 2026, 4), nil)

	// Create May
	repo.On("CreatePeriod", mock.Anything, mock.MatchedBy(func(p *model.BudgetPeriod) bool {
		return p.Month == 5
	})).Return(makePeriod("p-may", 2026, 5), nil)

	// April has a pending pro-rata
	aprSchedule := &model.ProRataSchedule{
		ID: "s-apr", UserID: "user-1", ProRataGroup: "g1",
		Name: "Subscription", Amount: 5000, Currency: "USD", ExpenseType: "desires",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 4, InstallmentIndex: 2,
		InstallmentTotal: 3, Status: "pending",
	}
	repo.On("GetPendingProRata", mock.Anything, "user-1", int32(2026), int32(4)).
		Return([]*model.ProRataSchedule{aprSchedule}, nil)

	// May has no pending pro-rata
	repo.On("GetPendingProRata", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{}, nil)

	expClient.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req CreateProRataInstallmentInput) bool {
		return req.PeriodContext.Month == 4 && req.ProRataIndex == 2 && req.LegacyMigration
	})).Return(&CreatedExpenseData{ID: "exp-apr", CreatedAt: "2026-04-01T00:00:00Z"}, nil)

	repo.On("MarkProRataApplied", mock.Anything, "s-apr").Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: "USD",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, result.AutoCreatedPeriods)
	assert.Len(t, result.AppliedProRata, 1)
	assert.Equal(t, "s-apr", result.AppliedProRata[0].ID)
}

func TestGetUpcomingProRata_Success(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	schedules := []*model.ProRataSchedule{
		{ID: "s-1", Name: "Insurance", Amount: 5000, InstallmentIndex: 2, InstallmentTotal: 4, TargetYear: 2026, TargetMonth: 6},
		{ID: "s-2", Name: "Gym", Amount: 2500, InstallmentIndex: 3, InstallmentTotal: 6, TargetYear: 2026, TargetMonth: 6},
	}
	repo.On("GetUpcomingProRata", mock.Anything, "user-1").Return(schedules, nil)

	result, err := svc.GetUpcomingProRata(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Insurance", result[0].Name)
}

// --- StatusTransition Tests ---

func TestProRataScheduleStatusTransitions(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	// Set up: period exists, one pending schedule
	repo.On("GetLatestPeriod", mock.Anything, "user-1").
		Return(makePeriod("p-prev", 2026, 4), nil)

	repo.On("CreatePeriod", mock.Anything, mock.Anything).
		Return(makePeriod("p-may", 2026, 5), nil)

	schedule := &model.ProRataSchedule{
		ID: "s-1", UserID: "user-1", Status: "pending",
		Name: "Test", Amount: 1000, Currency: "USD", ExpenseType: "essentials",
		TagID: "t-1", TargetYear: 2026, TargetMonth: 5,
		InstallmentIndex: 2, InstallmentTotal: 3, ProRataGroup: "g-1",
	}

	repo.On("GetPendingProRata", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{schedule}, nil)

	expClient.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req CreateProRataInstallmentInput) bool {
		return req.LegacyMigration && req.ProRataGroup == "g-1" && req.ProRataIndex == 2
	})).Return(&CreatedExpenseData{ID: "e-1", CreatedAt: "2026-05-01"}, nil)

	repo.On("MarkProRataApplied", mock.Anything, "s-1").Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: "USD",
	})

	require.NoError(t, err)
	assert.Len(t, result.AppliedProRata, 1)
	assert.Equal(t, "applied", result.AppliedProRata[0].Status)

	// Verify MarkProRataApplied was called only after the ledger write succeeded.
	repo.AssertCalled(t, "MarkProRataApplied", mock.Anything, "s-1")
	// Verify CreateProRataInstallment was called with trusted context and legacy
	// migration flag.
	expClient.AssertCalled(t, "CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req CreateProRataInstallmentInput) bool {
		return req.ProRataGroup == "g-1" && req.ProRataIndex == 2 && req.LegacyMigration
	}))
}

func TestCreatePeriodWithProRata_NilDefaultsReturnsError(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	// No defaults row - user has not completed onboarding.
	repo.On("GetDefaults", mock.Anything, "user-1").Return(nil, nil)

	// A prior period exists, so the missed month (April) needs auto-creation,
	// which requires the defaults row.
	repo.On("GetLatestPeriod", mock.Anything, "user-1").
		Return(makePeriod("p-mar", 2026, 3), nil)

	_, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: "USD",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "defaults row missing")

	// CreatePeriod must never have been called.
	repo.AssertNotCalled(t, "CreatePeriod", mock.Anything, mock.Anything)
}

// --- Pro-rata future application tests (ticket 14) ---
//
// These tests exercise applyPendingProRata through CreatePeriodWithProRata,
// which is the public trigger that creates a target budget period and then
// applies pending installments for that exact year and month.

// setupPeriodCreationMocks wires the common CreatePeriodWithProRata mocks: a
// latest prior period (so no missed months are auto-created), the target
// period creation, and the pending-pro-rata loader. The caller adds
// schedule-specific mocks (CreateProRataInstallment / MarkProRata*).
func setupPeriodCreationMocks(repo *mockRepo, pending []*model.ProRataSchedule, targetYear, targetMonth int32, reportingCurrency string) {
	repo.On("GetLatestPeriod", mock.Anything, "user-1").
		Return(makePeriod("p-prev", targetYear, targetMonth-1), nil)
	targetPeriod := makePeriod("p-target", targetYear, targetMonth)
	targetPeriod.ReportingCurrency = reportingCurrency
	repo.On("CreatePeriod", mock.Anything, mock.MatchedBy(func(p *model.BudgetPeriod) bool {
		return p.Year == targetYear && p.Month == targetMonth
	})).Return(targetPeriod, nil)
	repo.On("GetPendingProRata", mock.Anything, "user-1", targetYear, targetMonth).
		Return(pending, nil)
}

func createPeriodRequest(year, month int32, reportingCurrency string) *model.CreatePeriodRequest {
	return &model.CreatePeriodRequest{
		Year: year, Month: month, BudgetAmount: int64Ptr(300000),
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: reportingCurrency,
	}
}

// TestApplyProRata_CapturedSnapshotSameTargetCurrencyAppliesWithSnapshot asserts
// that a captured-snapshot schedule whose target period reporting currency
// equals the creation currency is applied through CreateProRataInstallment with
// the captured snapshot and marked applied only after the ledger write succeeds.
func TestApplyProRata_CapturedSnapshotSameTargetCurrencyAppliesWithSnapshot(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	snap := snapshotFixture()
	pending := []*model.ProRataSchedule{{
		ID: "s-1", UserID: "user-1", ProRataGroup: "g-1", Status: "pending",
		Name: "Subscription", Amount: 3333, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 6,
		InstallmentIndex: 2, InstallmentTotal: 3,
		TransactionAmount: 3333, TransactionCurrency: "USD",
		CreationReportingCurrency: "USD", CapturedRateSnapshot: snap,
	}}
	setupPeriodCreationMocks(repo, pending, 2026, 6, "USD")

	expClient.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req CreateProRataInstallmentInput) bool {
		return req.Currency == "USD" && !req.LegacyMigration &&
			req.PeriodContext.ReportingCurrency == "USD" &&
			req.CapturedRateSnapshot == snap && req.Amount == 3333
	})).Return(&CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-06-01T00:00:00Z"}, nil)
	repo.On("MarkProRataApplied", mock.Anything, "s-1").Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", createPeriodRequest(2026, 6, "USD"))
	require.NoError(t, err)
	require.Len(t, result.AppliedProRata, 1)
	assert.Equal(t, "applied", result.AppliedProRata[0].Status)
	assert.Equal(t, "s-1", result.AppliedProRata[0].ID)
	repo.AssertCalled(t, "MarkProRataApplied", mock.Anything, "s-1")
	repo.AssertNotCalled(t, "MarkProRataFailed", mock.Anything, mock.Anything, mock.Anything)
}

// TestApplyProRata_CapturedSnapshotDifferentTargetCurrencyAppliesInTargetCurrency
// asserts that when the target period reporting currency differs from the
// creation currency, the installment is applied with the target period currency
// while preserving the captured schedule snapshot context.
func TestApplyProRata_CapturedSnapshotDifferentTargetCurrencyAppliesInTargetCurrency(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	snap := snapshotFixture() // covers USD, EUR, GBP, JPY
	pending := []*model.ProRataSchedule{{
		ID: "s-2", UserID: "user-1", ProRataGroup: "g-1", Status: "pending",
		Name: "Subscription", Amount: 3333, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 7,
		InstallmentIndex: 3, InstallmentTotal: 3,
		TransactionAmount: 3333, TransactionCurrency: "USD",
		CreationReportingCurrency: "USD", CapturedRateSnapshot: snap,
	}}
	setupPeriodCreationMocks(repo, pending, 2026, 7, "EUR")

	expClient.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req CreateProRataInstallmentInput) bool {
		// Target period reports in EUR; the transaction currency (captured
		// schedule context) stays USD and the snapshot is forwarded.
		return req.Currency == "USD" && !req.LegacyMigration &&
			req.PeriodContext.ReportingCurrency == "EUR" &&
			req.CapturedRateSnapshot == snap
	})).Return(&CreatedExpenseData{ID: "exp-2", CreatedAt: "2026-07-01T00:00:00Z"}, nil)
	repo.On("MarkProRataApplied", mock.Anything, "s-2").Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", createPeriodRequest(2026, 7, "EUR"))
	require.NoError(t, err)
	require.Len(t, result.AppliedProRata, 1)
	assert.Equal(t, "applied", result.AppliedProRata[0].Status)
}

// TestApplyProRata_CapturedSnapshotMissingTargetCurrencyMarksFailed asserts
// that a schedule whose captured snapshot lacks the target reporting currency
// is marked failed with snapshot_currency_missing and writes no ledger row.
func TestApplyProRata_CapturedSnapshotMissingTargetCurrencyMarksFailed(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	snap := &model.CapturedRateSnapshot{
		SnapshotVersion: 1, Source: "open_exchange_rates", BaseCurrency: "USD",
		RateTimestamp: "2026-05-15T10:00:00Z", CapturedAt: "2026-05-15T12:00:00Z",
		ExpiresAt:       "2026-05-15T13:00:00Z",
		RatesByCurrency: map[string]string{"USD": "1", "EUR": "0.92"},
	}
	pending := []*model.ProRataSchedule{{
		ID: "s-3", UserID: "user-1", ProRataGroup: "g-1", Status: "pending",
		Name: "Subscription", Amount: 3333, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 8,
		InstallmentIndex: 2, InstallmentTotal: 3,
		TransactionAmount: 3333, TransactionCurrency: "USD",
		CreationReportingCurrency: "USD", CapturedRateSnapshot: snap,
	}}
	setupPeriodCreationMocks(repo, pending, 2026, 8, "JPY")

	repo.On("MarkProRataFailed", mock.Anything, "s-3", model.ErrSnapshotCurrencyMissing).Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", createPeriodRequest(2026, 8, "JPY"))
	require.NoError(t, err)
	assert.Empty(t, result.AppliedProRata)
	repo.AssertCalled(t, "MarkProRataFailed", mock.Anything, "s-3", model.ErrSnapshotCurrencyMissing)
	repo.AssertNotCalled(t, "MarkProRataApplied", mock.Anything, mock.Anything)
	expClient.AssertNotCalled(t, "CreateProRataInstallment", mock.Anything, mock.Anything)
}

// TestApplyProRata_ExpenseWriteFailureLeavesSchedulePending asserts that a
// transient Expense write failure leaves the schedule pending (no partial
// expense row) and does not mark it applied or failed.
func TestApplyProRata_ExpenseWriteFailureLeavesSchedulePending(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	snap := snapshotFixture()
	pending := []*model.ProRataSchedule{{
		ID: "s-4", UserID: "user-1", ProRataGroup: "g-1", Status: "pending",
		Name: "Subscription", Amount: 3333, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 9,
		InstallmentIndex: 2, InstallmentTotal: 3,
		TransactionAmount: 3333, TransactionCurrency: "USD",
		CreationReportingCurrency: "USD", CapturedRateSnapshot: snap,
	}}
	setupPeriodCreationMocks(repo, pending, 2026, 9, "USD")

	expClient.On("CreateProRataInstallment", mock.Anything, mock.Anything).
		Return(nil, &apierr.Error{Code: model.ErrConversionUnavailable, Message: "conversion unavailable", Status: 503})

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", createPeriodRequest(2026, 9, "USD"))
	require.NoError(t, err)
	assert.Empty(t, result.AppliedProRata)
	// Schedule remains pending: neither applied nor failed is recorded.
	repo.AssertNotCalled(t, "MarkProRataApplied", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "MarkProRataFailed", mock.Anything, mock.Anything, mock.Anything)
}

// TestApplyProRata_LegacySameCurrencyAppliesWithMigration asserts that a legacy
// pending schedule (no captured snapshot) whose target period reporting
// currency equals the stored schedule currency is applied with a migration
// snapshot (exchangeRateSource = migration, exchangeRate = 1) via the legacy
// migration path.
func TestApplyProRata_LegacySameCurrencyAppliesWithMigration(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	pending := []*model.ProRataSchedule{{
		ID: "s-5", UserID: "user-1", ProRataGroup: "g-legacy", Status: "pending",
		Name: "Insurance", Amount: 5000, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 10,
		InstallmentIndex: 2, InstallmentTotal: 3,
		// Legacy: no TransactionCurrency / CapturedRateSnapshot.
	}}
	setupPeriodCreationMocks(repo, pending, 2026, 10, "USD")

	expClient.On("CreateProRataInstallment", mock.Anything, mock.MatchedBy(func(req CreateProRataInstallmentInput) bool {
		return req.LegacyMigration && req.Currency == "USD" &&
			req.PeriodContext.ReportingCurrency == "USD" &&
			req.CapturedRateSnapshot == nil
	})).Return(&CreatedExpenseData{ID: "exp-leg", CreatedAt: "2026-10-01T00:00:00Z"}, nil)
	repo.On("MarkProRataApplied", mock.Anything, "s-5").Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", createPeriodRequest(2026, 10, "USD"))
	require.NoError(t, err)
	require.Len(t, result.AppliedProRata, 1)
	assert.Equal(t, "applied", result.AppliedProRata[0].Status)
	repo.AssertCalled(t, "MarkProRataApplied", mock.Anything, "s-5")
}

// TestApplyProRata_LegacyDifferentCurrencyMarksFailed asserts that a legacy
// pending schedule whose target period reporting currency differs from the
// stored schedule currency is marked failed with missing_captured_rate_snapshot
// and writes no ledger row.
func TestApplyProRata_LegacyDifferentCurrencyMarksFailed(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	pending := []*model.ProRataSchedule{{
		ID: "s-6", UserID: "user-1", ProRataGroup: "g-legacy", Status: "pending",
		Name: "Insurance", Amount: 5000, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 11,
		InstallmentIndex: 2, InstallmentTotal: 3,
	}}
	setupPeriodCreationMocks(repo, pending, 2026, 11, "EUR")

	repo.On("MarkProRataFailed", mock.Anything, "s-6", model.ErrMissingCapturedRateSnapshot).Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", createPeriodRequest(2026, 11, "EUR"))
	require.NoError(t, err)
	assert.Empty(t, result.AppliedProRata)
	repo.AssertCalled(t, "MarkProRataFailed", mock.Anything, "s-6", model.ErrMissingCapturedRateSnapshot)
	repo.AssertNotCalled(t, "MarkProRataApplied", mock.Anything, mock.Anything)
	expClient.AssertNotCalled(t, "CreateProRataInstallment", mock.Anything, mock.Anything)
}

// TestApplyProRata_LegacyNoTargetPeriodRemainsPending asserts that a legacy
// pending schedule with no matching target period is never loaded or applied:
// applyPendingProRata is only triggered after a period exists, and a month with
// no period creation produces no ledger write.
func TestApplyProRata_LegacyNoTargetPeriodRemainsPending(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	// Creating a period for a *different* month (Dec) must not apply a pending
	// schedule targeted at November.
	repo.On("GetLatestPeriod", mock.Anything, "user-1").
		Return(makePeriod("p-prev", 2026, 11), nil)
	repo.On("CreatePeriod", mock.Anything, mock.MatchedBy(func(p *model.BudgetPeriod) bool {
		return p.Year == 2026 && p.Month == 12
	})).Return(makePeriod("p-dec", 2026, 12), nil)
	// December has no pending schedules; November's pending schedule is never
	// loaded because no period was created for November.
	repo.On("GetPendingProRata", mock.Anything, "user-1", int32(2026), int32(12)).
		Return([]*model.ProRataSchedule{}, nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", createPeriodRequest(2026, 12, "USD"))
	require.NoError(t, err)
	assert.Empty(t, result.AppliedProRata)
	repo.AssertNotCalled(t, "MarkProRataApplied", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "MarkProRataFailed", mock.Anything, mock.Anything, mock.Anything)
	expClient.AssertNotCalled(t, "CreateProRataInstallment", mock.Anything, mock.Anything)
}

// TestApplyProRata_FailedScheduleNotReapplied asserts that a schedule already
// marked failed is not re-applied: GetPendingProRata only returns pending rows.
func TestApplyProRata_FailedScheduleNotReapplied(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	// The repository returns only pending rows; a failed row is excluded.
	setupPeriodCreationMocks(repo, []*model.ProRataSchedule{}, 2026, 12, "USD")

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", createPeriodRequest(2026, 12, "USD"))
	require.NoError(t, err)
	assert.Empty(t, result.AppliedProRata)
	expClient.AssertNotCalled(t, "CreateProRataInstallment", mock.Anything, mock.Anything)
}

// TestApplyProRata_DashboardExcludesPendingAndFailed asserts that dashboard
// totals come from applied ledger rows only. Pending and failed schedules never
// write a ledger row, so the dashboard's expense read is the only source. This
// test documents the invariant: a pending schedule produces no ExpenseData.
func TestApplyProRata_DashboardExcludesPendingAndFailed(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	// A pending schedule that stays pending (write fails) contributes no
	// expense to the period. The dashboard reads GetExpensesForPeriod, which
	// returns only written ledger rows.
	snap := snapshotFixture()
	pending := []*model.ProRataSchedule{{
		ID: "s-pending", UserID: "user-1", ProRataGroup: "g-1", Status: "pending",
		Name: "Subscription", Amount: 3333, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 12,
		InstallmentIndex: 2, InstallmentTotal: 3,
		TransactionAmount: 3333, TransactionCurrency: "USD",
		CreationReportingCurrency: "USD", CapturedRateSnapshot: snap,
	}}
	setupPeriodCreationMocks(repo, pending, 2026, 12, "USD")

	expClient.On("CreateProRataInstallment", mock.Anything, mock.Anything).
		Return(nil, &apierr.Error{Code: model.ErrConversionUnavailable, Message: "conversion unavailable", Status: 503})

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", createPeriodRequest(2026, 12, "USD"))
	require.NoError(t, err)
	assert.Empty(t, result.AppliedProRata)

	// The dashboard reads only applied ledger rows. A pending/failed schedule
	// produced no row, so the expense client returns an empty set.
	expClient.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(12)).
		Return([]ExpenseData{}, nil)
	repo.On("GetCurrentPeriod", mock.Anything, "user-1", int32(2026), int32(12)).
		Return(makePeriod("p-target", 2026, 12), nil)
	summary, err := svc.GetPeriodSummary(context.Background(), "user-1", 2026, 12)
	require.NoError(t, err)
	assert.NotNil(t, summary)
}

// TestApplyProRata_ExpenseSnapshotCurrencyMissingMarksFailed asserts that when
// Expense surfaces a snapshot_currency_missing failure (e.g. the transaction
// currency is missing from the captured snapshot, which Finance's target-only
// pre-check does not cover), Finance classifies it as deterministic and moves the
// row to failed rather than stranding it in pending for a retry that would fail
// identically.
func TestApplyProRata_ExpenseSnapshotCurrencyMissingMarksFailed(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	// Snapshot covers the target reporting currency (USD) so Finance's pre-check
	// passes, but Expense's defense-in-depth coverage check fails for the
	// transaction currency (EUR is missing).
	snap := &model.CapturedRateSnapshot{
		SnapshotVersion: 1, Source: "open_exchange_rates", BaseCurrency: "USD",
		RateTimestamp: "2026-05-15T10:00:00Z", CapturedAt: "2026-05-15T12:00:00Z",
		ExpiresAt:       "2026-05-15T13:00:00Z",
		RatesByCurrency: map[string]string{"USD": "1", "GBP": "0.79"},
	}
	pending := []*model.ProRataSchedule{{
		ID: "s-snap", UserID: "user-1", ProRataGroup: "g-1", Status: "pending",
		Name: "Subscription", Amount: 3333, Currency: "EUR", ExpenseType: "essentials",
		TagID: "tag-1", TargetYear: 2026, TargetMonth: 12,
		InstallmentIndex: 2, InstallmentTotal: 3,
		TransactionAmount: 3333, TransactionCurrency: "EUR",
		CreationReportingCurrency: "USD", CapturedRateSnapshot: snap,
	}}
	setupPeriodCreationMocks(repo, pending, 2026, 12, "USD")

	// Finance's target-currency pre-check passes (USD is in the snapshot), so it
	// calls Expense, which surfaces the snapshot_currency_missing failure.
	expClient.On("CreateProRataInstallment", mock.Anything, mock.Anything).
		Return(nil, &apierr.Error{Code: model.ErrSnapshotCurrencyMissing, Message: "snapshot currency missing", Status: 409})
	repo.On("MarkProRataFailed", mock.Anything, "s-snap", model.ErrSnapshotCurrencyMissing).Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", createPeriodRequest(2026, 12, "USD"))
	require.NoError(t, err)
	assert.Empty(t, result.AppliedProRata)
	repo.AssertCalled(t, "MarkProRataFailed", mock.Anything, "s-snap", model.ErrSnapshotCurrencyMissing)
	repo.AssertNotCalled(t, "MarkProRataApplied", mock.Anything, mock.Anything)
}

// TestClassifyProRataExpenseError_TableTest asserts the deterministic-vs-transient
// classification of Expense-returned errors, covering both the in-process
// *apierr.Error path (mocks) and the wrapped gRPC status path (production).
func TestClassifyProRataExpenseError_TableTest(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "in-process snapshot currency missing",
			err:  &apierr.Error{Code: model.ErrSnapshotCurrencyMissing, Message: "missing", Status: 409},
			want: model.ErrSnapshotCurrencyMissing,
		},
		{
			name: "in-process conversion unavailable is transient",
			err:  &apierr.Error{Code: model.ErrConversionUnavailable, Message: "unavailable", Status: 503},
			want: "",
		},
		{
			name: "gRPC FailedPrecondition maps to snapshot currency missing",
			err:  status.Error(codes.FailedPrecondition, "The captured rate snapshot does not contain a required currency"),
			want: model.ErrSnapshotCurrencyMissing,
		},
		{
			name: "gRPC Unavailable is transient",
			err:  status.Error(codes.Unavailable, "conversion unavailable"),
			want: "",
		},
		{
			name: "gRPC Internal is transient",
			err:  status.Error(codes.Internal, "internal"),
			want: "",
		},
		{
			name: "wrapped gRPC FailedPrecondition is still classified",
			err:  fmt.Errorf("gRPC CreateProRataInstallment: %w", status.Error(codes.FailedPrecondition, "snapshot currency missing")),
			want: model.ErrSnapshotCurrencyMissing,
		},
		{
			name: "plain non-gRPC error is transient",
			err:  fmt.Errorf("some transport failure"),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, classifyProRataExpenseError(tt.err))
		})
	}
}
