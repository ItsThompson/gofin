package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
)

func prorataSnapshot() *CapturedRateSnapshot {
	return &CapturedRateSnapshot{
		SnapshotVersion: 1,
		Source:          exchangesource.OpenExchangeRates,
		BaseCurrency:    "USD",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		CapturedAt:      "2026-05-15T12:00:00Z",
		ExpiresAt:       "2026-05-15T13:00:00Z",
		RatesByCurrency: map[string]string{"USD": "1", "EUR": "0.92", "GBP": "0.79"},
	}
}

func validProRataInstallmentRequest() *CreateProRataInstallmentRequest {
	return &CreateProRataInstallmentRequest{
		UserID: "user-1",
		PeriodContext: TrustedPeriodContext{
			PeriodID:          "period-1",
			UserID:            "user-1",
			Year:              2026,
			Month:             5,
			ReportingCurrency: "USD",
			Source:            "finance_service",
		},
		Name:                 "Annual subscription",
		Amount:               3334,
		TransactionCurrency:  "USD",
		ExpenseType:          "essentials",
		TagID:                "tag-1",
		ExpenseDate:          "2026-05-15",
		ProRataGroup:         "group-1",
		ProRataIndex:         1,
		ProRataTotal:         3,
		CapturedRateSnapshot: prorataSnapshot(),
	}
}

func TestCreateProRataInstallment_ForeignCurrencyUsesCapturedSnapshot(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, now)

	req := validProRataInstallmentRequest()
	req.TransactionCurrency = "EUR"

	fxClient.On("ConvertWithSnapshot", mock.Anything, mock.MatchedBy(func(r FxConvertWithSnapshotRequest) bool {
		return r.Amount == 3334 &&
			r.SourceCurrency == "EUR" &&
			r.TargetCurrency == "USD" &&
			r.Snapshot != nil &&
			r.Snapshot.RateTimestamp == "2026-05-15T10:00:00Z"
	})).Return(&FxConvertResponse{
		ConvertedAmount: 3624,
		ExchangeRate:    "1.0872",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		Source:          exchangesource.OpenExchangeRates,
		ExpiresAt:       "2026-05-15T13:00:00Z",
	}, nil)

	var captured *model.Expense
	repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(*model.Expense)
		}).Return(&model.Expense{ID: "exp-1", UserID: "user-1"}, nil)

	created, err := svc.CreateProRataInstallment(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotNil(t, captured)
	assert.Equal(t, "user-1", captured.UserID)
	assert.Equal(t, int32(2026), captured.PeriodYear)
	assert.Equal(t, int32(5), captured.PeriodMonth)
	assert.True(t, captured.IsProRata)
	assert.Equal(t, "group-1", captured.ProRataGroup)
	assert.Equal(t, int32(1), captured.ProRataIndex)
	assert.Equal(t, int32(3), captured.ProRataTotal)
	assert.Equal(t, "EUR", captured.TransactionCurrencyCode)
	assert.Equal(t, int64(3334), captured.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, "USD", captured.ReportingCurrencyCode)
	assert.Equal(t, int64(3624), captured.ReportingAmountInMinorUnits)
	assert.Equal(t, exchangesource.OpenExchangeRates, captured.ExchangeRateSource)
	assert.Equal(t, "2026-05-15T10:00:00Z", captured.ExchangeRateTimestamp)

	// Finance-originated writes must not call back into Finance for period context.
	periodClient.AssertNotCalled(t, "GetPeriodContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateProRataInstallment_SameCurrencyStillUsesSnapshotSource(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, now)

	fxClient.On("ConvertWithSnapshot", mock.Anything, mock.Anything).Return(&FxConvertResponse{
		ConvertedAmount: 3334,
		ExchangeRate:    "1",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		Source:          exchangesource.OpenExchangeRates,
		ExpiresAt:       "2026-05-15T13:00:00Z",
	}, nil)

	var captured *model.Expense
	repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(*model.Expense)
		}).Return(&model.Expense{ID: "exp-1", UserID: "user-1"}, nil)

	_, err := svc.CreateProRataInstallment(context.Background(), validProRataInstallmentRequest())

	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, exchangesource.OpenExchangeRates, captured.ExchangeRateSource)
	assert.Equal(t, "2026-05-15T10:00:00Z", captured.ExchangeRateTimestamp)
	assert.Equal(t, int64(3334), captured.ReportingAmountInMinorUnits)
}

func TestCreateProRataInstallment_MismatchedContextUser(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC))

	req := validProRataInstallmentRequest()
	req.PeriodContext.UserID = "user-2"

	_, err := svc.CreateProRataInstallment(context.Background(), req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeForbidden, svcErr.Code)
	fxClient.AssertNotCalled(t, "ConvertWithSnapshot", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

func TestCreateProRataInstallment_NonFinanceSource(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC))

	req := validProRataInstallmentRequest()
	req.PeriodContext.Source = "browser"

	_, err := svc.CreateProRataInstallment(context.Background(), req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeInternal, svcErr.Code)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

func TestCreateProRataInstallment_StructuralContextViolationsAreInternal(t *testing.T) {
	cases := []struct {
		name string
		mut  func(req *CreateProRataInstallmentRequest)
	}{
		{"missing period id", func(req *CreateProRataInstallmentRequest) { req.PeriodContext.PeriodID = "" }},
		{"zero year", func(req *CreateProRataInstallmentRequest) { req.PeriodContext.Year = 0 }},
		{"month below range", func(req *CreateProRataInstallmentRequest) { req.PeriodContext.Month = 0 }},
		{"month above range", func(req *CreateProRataInstallmentRequest) { req.PeriodContext.Month = 13 }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(mockExpenseRepository)
			periodClient := new(mockPeriodContextClient)
			fxClient := new(mockFxClient)
			svc := newTestServiceWithFxClock(repo, periodClient, fxClient, time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC))

			req := validProRataInstallmentRequest()
			tc.mut(req)

			_, err := svc.CreateProRataInstallment(context.Background(), req)

			svcErr := requireAPIError(t, err)
			assert.Equal(t, apierr.CodeInternal, svcErr.Code)
			fxClient.AssertNotCalled(t, "ConvertWithSnapshot", mock.Anything, mock.Anything)
			repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
		})
	}
}

func TestCreateProRataInstallment_UnsupportedTransactionCurrency(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC))

	req := validProRataInstallmentRequest()
	req.TransactionCurrency = "XYZ"

	_, err := svc.CreateProRataInstallment(context.Background(), req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrUnsupportedCurrency, svcErr.Code)
	fxClient.AssertNotCalled(t, "ConvertWithSnapshot", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

func TestCreateProRataInstallment_MissingSnapshotCoverage(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC))

	req := validProRataInstallmentRequest()
	req.TransactionCurrency = "EUR"
	delete(req.CapturedRateSnapshot.RatesByCurrency, "EUR")

	_, err := svc.CreateProRataInstallment(context.Background(), req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrSnapshotCurrencyMissing, svcErr.Code)
	fxClient.AssertNotCalled(t, "ConvertWithSnapshot", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

func TestCreateProRataInstallment_NilSnapshot(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC))

	req := validProRataInstallmentRequest()
	req.CapturedRateSnapshot = nil

	_, err := svc.CreateProRataInstallment(context.Background(), req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrSnapshotCurrencyMissing, svcErr.Code)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

func TestCreateProRataInstallment_FxFailureDoesNotWrite(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC))

	req := validProRataInstallmentRequest()
	req.TransactionCurrency = "EUR"

	fxClient.On("ConvertWithSnapshot", mock.Anything, mock.Anything).Return(nil, conversionUnavailableError())

	_, err := svc.CreateProRataInstallment(context.Background(), req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrConversionUnavailable, svcErr.Code)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// TestCreateProRataInstallment_DifferentTargetCurrencyUsesCapturedSnapshot
// asserts that a future installment whose target period reporting currency
// differs from the creation reporting currency is converted through the
// captured snapshot (no live provider call semantics: FX is fed the snapshot).
func TestCreateProRataInstallment_DifferentTargetCurrencyUsesCapturedSnapshot(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, now)

	req := validProRataInstallmentRequest()
	req.PeriodContext = TrustedPeriodContext{
		PeriodID:          "period-jul",
		UserID:            "user-1",
		Year:              2026,
		Month:             7,
		ReportingCurrency: "EUR",
		Source:            "finance_service",
	}
	req.TransactionCurrency = "USD"
	// Snapshot covers both USD (transaction) and EUR (target reporting).
	req.CapturedRateSnapshot = &CapturedRateSnapshot{
		SnapshotVersion: 1,
		Source:          exchangesource.OpenExchangeRates,
		BaseCurrency:    "USD",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		CapturedAt:      "2026-05-15T12:00:00Z",
		ExpiresAt:       "2026-05-15T13:00:00Z",
		RatesByCurrency: map[string]string{"USD": "1", "EUR": "0.92", "GBP": "0.79"},
	}

	fxClient.On("ConvertWithSnapshot", mock.Anything, mock.MatchedBy(func(r FxConvertWithSnapshotRequest) bool {
		return r.SourceCurrency == "USD" && r.TargetCurrency == "EUR" && r.Snapshot != nil
	})).Return(&FxConvertResponse{
		ConvertedAmount: 3067,
		ExchangeRate:    "0.92",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		Source:          exchangesource.OpenExchangeRates,
		ExpiresAt:       "2026-05-15T13:00:00Z",
	}, nil)

	var captured *model.Expense
	repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(*model.Expense)
		}).Return(&model.Expense{ID: "exp-future", UserID: "user-1"}, nil)

	_, err := svc.CreateProRataInstallment(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, captured)
	// The applied expense reports in the target period currency while the
	// transaction currency (captured schedule context) is preserved.
	assert.Equal(t, "USD", captured.TransactionCurrencyCode)
	assert.Equal(t, "EUR", captured.ReportingCurrencyCode)
	assert.Equal(t, int64(3334), captured.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, int64(3067), captured.ReportingAmountInMinorUnits)
	assert.Equal(t, "0.92", captured.SourceToTargetExchangeRate)
}

// TestCreateProRataInstallment_CapturedSnapshotMissingTargetCurrencyRejects
// asserts that when the captured snapshot cannot derive the target reporting
// currency, Expense rejects the write with SNAPSHOT_CURRENCY_MISSING and no
// ledger row is written.
func TestCreateProRataInstallment_CapturedSnapshotMissingTargetCurrencyRejects(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))

	req := validProRataInstallmentRequest()
	req.PeriodContext = TrustedPeriodContext{
		PeriodID:          "period-jul",
		UserID:            "user-1",
		Year:              2026,
		Month:             7,
		ReportingCurrency: "JPY",
		Source:            "finance_service",
	}
	req.TransactionCurrency = "USD"
	// Snapshot lacks JPY.
	req.CapturedRateSnapshot = &CapturedRateSnapshot{
		SnapshotVersion: 1,
		Source:          exchangesource.OpenExchangeRates,
		BaseCurrency:    "USD",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		CapturedAt:      "2026-05-15T12:00:00Z",
		ExpiresAt:       "2026-05-15T13:00:00Z",
		RatesByCurrency: map[string]string{"USD": "1", "EUR": "0.92"},
	}

	_, err := svc.CreateProRataInstallment(context.Background(), req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrSnapshotCurrencyMissing, svcErr.Code)
	fxClient.AssertNotCalled(t, "ConvertWithSnapshot", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// TestCreateProRataInstallment_ExpenseWriteFailureDoesNotLeavePartialRow
// asserts that a repository write failure after a successful snapshot
// conversion surfaces an error and does not return a created expense. Finance
// keeps the schedule pending because no ledger row was confirmed.
func TestCreateProRataInstallment_ExpenseWriteFailureDoesNotLeavePartialRow(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, now)

	req := validProRataInstallmentRequest()
	req.TransactionCurrency = "EUR"

	fxClient.On("ConvertWithSnapshot", mock.Anything, mock.Anything).Return(&FxConvertResponse{
		ConvertedAmount: 3624,
		ExchangeRate:    "1.0872",
		RateTimestamp:   "2026-05-15T10:00:00Z",
		Source:          exchangesource.OpenExchangeRates,
		ExpiresAt:       "2026-05-15T13:00:00Z",
	}, nil)

	repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
		Return(nil, fmt.Errorf("immudb write failed"))

	_, err := svc.CreateProRataInstallment(context.Background(), req)

	require.Error(t, err)
	repo.AssertNumberOfCalls(t, "CreateExpense", 1)
}
