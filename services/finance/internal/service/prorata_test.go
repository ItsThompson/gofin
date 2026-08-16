package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

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

func TestCreateProRataExpense_Success(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, fixedNow(2026, 5, 15))

	expClient.On("CreateExpense", mock.Anything, mock.MatchedBy(func(req CreateExpenseInput) bool {
		return req.Name == "Annual subscription" &&
			req.Amount == int64(3334) && // 10000/3 = 3333, first gets +1
			req.IsProRata &&
			req.ProRataIndex == int32(1) &&
			req.ProRataTotal == int32(3) &&
			req.PeriodYear == int32(2026) &&
			req.PeriodMonth == int32(5)
	})).Return(&CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-05-15T12:00:00Z"}, nil)

	// Schedules for months 2 and 3
	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.InstallmentIndex == 2 && s.TargetYear == 2026 && s.TargetMonth == 6
	})).Return(&model.ProRataSchedule{
		ID: "sched-1", InstallmentIndex: 2, TargetYear: 2026, TargetMonth: 6, Amount: 3333, Status: "pending",
	}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.InstallmentIndex == 3 && s.TargetYear == 2026 && s.TargetMonth == 7
	})).Return(&model.ProRataSchedule{
		ID: "sched-2", InstallmentIndex: 3, TargetYear: 2026, TargetMonth: 7, Amount: 3333, Status: "pending",
	}, nil)

	result, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name:        "Annual subscription",
		TotalAmount: 10000,
		Currency:    "USD",
		ExpenseType: "essentials",
		TagID:       "tag-1",
		ExpenseDate: "2026-05-15",
		Months:      3,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "exp-1", result.Expense.ID)
	assert.Equal(t, int64(3334), result.Expense.Amount)
	assert.Len(t, result.Schedules, 2)
	assert.Equal(t, int32(6), result.Schedules[0].TargetMonth)
	assert.Equal(t, int32(7), result.Schedules[1].TargetMonth)
}

func TestCreateProRataExpense_YearRollover(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, fixedNow(2026, 11, 1))

	expClient.On("CreateExpense", mock.Anything, mock.Anything).
		Return(&CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-11-01T00:00:00Z"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.InstallmentIndex == 2 && s.TargetYear == 2026 && s.TargetMonth == 12
	})).Return(&model.ProRataSchedule{ID: "s-1", TargetYear: 2026, TargetMonth: 12, Status: "pending"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.MatchedBy(func(s *model.ProRataSchedule) bool {
		return s.InstallmentIndex == 3 && s.TargetYear == 2027 && s.TargetMonth == 1
	})).Return(&model.ProRataSchedule{ID: "s-2", TargetYear: 2027, TargetMonth: 1, Status: "pending"}, nil)

	result, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name: "Insurance", TotalAmount: 6000, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", ExpenseDate: "2026-11-01", Months: 3,
	})

	require.NoError(t, err)
	assert.Len(t, result.Schedules, 2)
	assert.Equal(t, int32(2026), result.Schedules[0].TargetYear)
	assert.Equal(t, int32(12), result.Schedules[0].TargetMonth)
	assert.Equal(t, int32(2027), result.Schedules[1].TargetYear)
	assert.Equal(t, int32(1), result.Schedules[1].TargetMonth)
}

func TestCreateProRataExpense_Validation(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	svc := newTagTestService(repo, txBeg, nil)

	tests := []struct {
		name string
		req  *model.CreateProRataRequest
		msg  string
	}{
		{"empty name", &model.CreateProRataRequest{TotalAmount: 100, Months: 2, Currency: "USD", ExpenseType: "essentials", TagID: "t", ExpenseDate: "2026-05-01"}, "Name is required"},
		{"zero amount", &model.CreateProRataRequest{Name: "X", TotalAmount: 0, Months: 2, Currency: "USD", ExpenseType: "essentials", TagID: "t", ExpenseDate: "2026-05-01"}, "positive"},
		{"one month", &model.CreateProRataRequest{Name: "X", TotalAmount: 100, Months: 1, Currency: "USD", ExpenseType: "essentials", TagID: "t", ExpenseDate: "2026-05-01"}, "at least 2"},
		{"bad type", &model.CreateProRataRequest{Name: "X", TotalAmount: 100, Months: 2, Currency: "USD", ExpenseType: "invalid", TagID: "t", ExpenseDate: "2026-05-01"}, "essentials, desires, or savings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateProRataExpense(context.Background(), "user-1", tt.req)
			require.Error(t, err)
			svcErr := requireAPIError(t, err)
			assert.Equal(t, apierr.CodeValidation, svcErr.Code)
			assert.Contains(t, svcErr.Message, tt.msg)
		})
	}
}

func TestCreateProRataExpense_ScheduleFailure(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestServiceNow(repo, txBeg, expClient, fixedNow(2026, 5, 15))

	expClient.On("CreateExpense", mock.Anything, mock.Anything).
		Return(&CreatedExpenseData{ID: "exp-1", CreatedAt: "2026-05-15T12:00:00Z"}, nil)

	repo.On("CreateProRataSchedule", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("db error"))

	_, err := svc.CreateProRataExpense(context.Background(), "user-1", &model.CreateProRataRequest{
		Name: "Test", TotalAmount: 6000, Currency: "USD", ExpenseType: "essentials",
		TagID: "tag-1", ExpenseDate: "2026-05-15", Months: 2,
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
		Year: 2026, Month: 5, BudgetAmount: 300000,
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: "USD",
	})

	require.NoError(t, err)
	assert.Equal(t, "p-1", result.Period.ID)
	assert.Empty(t, result.AppliedProRata)
	assert.Equal(t, 0, result.AutoCreatedPeriods)
}

func TestCreatePeriodWithProRata_DefaultsReportingCurrency(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	repo.On("GetDefaults", mock.Anything, "user-1").Return(&model.DefaultSettings{
		BudgetAmount:      200000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
		Currency:          "jpy",
	}, nil)
	repo.On("GetLatestPeriod", mock.Anything, "user-1").Return(nil, nil)
	repo.On("CreatePeriod", mock.Anything, mock.MatchedBy(func(p *model.BudgetPeriod) bool {
		return p.ReportingCurrency == "JPY"
	})).Return(makePeriod("p-1", 2026, 5), nil)
	repo.On("GetPendingProRata", mock.Anything, "user-1", int32(2026), int32(5)).
		Return([]*model.ProRataSchedule{}, nil)

	_, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: 300000,
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
	})

	require.NoError(t, err)
}

func TestCreatePeriodWithProRata_RejectsUnsupportedReportingCurrency(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	_, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: 300000,
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

	expClient.On("CreateExpense", mock.Anything, mock.MatchedBy(func(req CreateExpenseInput) bool {
		return req.Name == "Insurance" && req.Amount == 3333 && req.ProRataIndex == 2
	})).Return(&CreatedExpenseData{ID: "exp-applied", CreatedAt: "2026-05-01T00:00:00Z"}, nil)

	repo.On("MarkProRataApplied", mock.Anything, "sched-1").Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: 300000,
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
		Year: 2026, Month: 5, BudgetAmount: 300000,
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

	expClient.On("CreateExpense", mock.Anything, mock.MatchedBy(func(req CreateExpenseInput) bool {
		return req.PeriodMonth == 4 && req.ProRataIndex == 2
	})).Return(&CreatedExpenseData{ID: "exp-apr", CreatedAt: "2026-04-01T00:00:00Z"}, nil)

	repo.On("MarkProRataApplied", mock.Anything, "s-apr").Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: 300000,
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

	expClient.On("CreateExpense", mock.Anything, mock.Anything).
		Return(&CreatedExpenseData{ID: "e-1", CreatedAt: "2026-05-01"}, nil)

	repo.On("MarkProRataApplied", mock.Anything, "s-1").Return(nil)

	result, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: 300000,
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		ReportingCurrency: "USD",
	})

	require.NoError(t, err)
	assert.Len(t, result.AppliedProRata, 1)
	assert.Equal(t, "applied", result.AppliedProRata[0].Status)

	// Verify MarkProRataApplied was called
	repo.AssertCalled(t, "MarkProRataApplied", mock.Anything, "s-1")
	// Verify CreateExpense was called with correct data
	expClient.AssertCalled(t, "CreateExpense", mock.Anything, mock.MatchedBy(func(req CreateExpenseInput) bool {
		return req.ProRataGroup == "g-1" && req.ProRataIndex == 2
	}))
}

func TestCreatePeriodWithProRata_NilDefaultsReturnsError(t *testing.T) {
	repo := new(mockRepo)
	txBeg := new(mockTxBeg)
	expClient := new(mockExpClient)
	svc := newTagTestService(repo, txBeg, expClient)

	// No defaults row - user has not completed onboarding.
	repo.On("GetDefaults", mock.Anything, "user-1").Return(nil, nil)

	_, err := svc.CreatePeriodWithProRata(context.Background(), "user-1", &model.CreatePeriodRequest{
		Year: 2026, Month: 5, BudgetAmount: 300000,
		EssentialsPercent: 50, DesiresPercent: 30, SavingsPercent: 20,
		// No reportingCurrency so the service falls back to GetDefaults.
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "defaults row missing")

	// CreatePeriod must never have been called.
	repo.AssertNotCalled(t, "CreatePeriod", mock.Anything, mock.Anything)
}
