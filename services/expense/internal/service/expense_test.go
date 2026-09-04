package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
)

// mockExpenseRepository implements repository.ExpenseRepository for tests.
type mockExpenseRepository struct {
	mock.Mock
}

func (m *mockExpenseRepository) CreateExpense(ctx context.Context, expense *model.Expense) (*model.Expense, error) {
	args := m.Called(ctx, expense)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetExpensesForPeriod(ctx context.Context, userID string, year, month, page, pageSize int32) ([]*model.Expense, int64, error) {
	args := m.Called(ctx, userID, year, month, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*model.Expense), args.Get(1).(int64), args.Error(2)
}

func (m *mockExpenseRepository) GetExpenseByID(ctx context.Context, id string, userID string) (*model.Expense, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetExpenseByIdempotencyKey(ctx context.Context, userID string, key string) (*model.Expense, error) {
	// The key is now required, so every create performs a replay lookup. Most
	// create tests exercise the fresh-insert path and don't care about replays:
	// default to "no existing expense" unless a test registers an explicit
	// expectation for this method (the idempotent-replay tests do).
	for _, c := range m.ExpectedCalls {
		if c.Method == "GetExpenseByIdempotencyKey" {
			args := m.Called(ctx, userID, key)
			if args.Get(0) == nil {
				return nil, args.Error(1)
			}
			return args.Get(0).(*model.Expense), args.Error(1)
		}
	}
	return nil, nil
}

func (m *mockExpenseRepository) DeactivateExpense(ctx context.Context, id string, userID string) (int64, error) {
	args := m.Called(ctx, id, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockExpenseRepository) CountExpensesByTag(ctx context.Context, userID string, tagID string) (int64, error) {
	args := m.Called(ctx, userID, tagID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockExpenseRepository) CorrectExpense(ctx context.Context, original *model.Expense, correction *model.Expense) (*model.Expense, error) {
	args := m.Called(ctx, original, correction)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetCorrectionHistory(ctx context.Context, expenseID string, userID string) ([]*model.Expense, error) {
	args := m.Called(ctx, expenseID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetProRataGroup(ctx context.Context, groupID string, userID string) ([]*model.Expense, error) {
	args := m.Called(ctx, groupID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Expense), args.Error(1)
}

func (m *mockExpenseRepository) GetActiveExpenseSuggestionInputs(ctx context.Context, userID string) ([]*model.ExpenseSuggestionInput, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.ExpenseSuggestionInput), args.Error(1)
}

func (m *mockExpenseRepository) AnonymizeAllUserExpenses(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockExpenseRepository) GetExpensesByUserAfter(ctx context.Context, userID string, cursor repository.ExpenseCursor, pageSize int32) ([]*model.Expense, repository.ExpenseCursor, bool, error) {
	args := m.Called(ctx, userID, cursor, pageSize)
	var rows []*model.Expense
	if args.Get(0) != nil {
		rows = args.Get(0).([]*model.Expense)
	}
	return rows, args.Get(1).(repository.ExpenseCursor), args.Bool(2), args.Error(3)
}

type mockPeriodContextClient struct {
	mock.Mock
}

func (m *mockPeriodContextClient) GetPeriodContext(ctx context.Context, userID string, year, month int32) (*PeriodContext, error) {
	args := m.Called(ctx, userID, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PeriodContext), args.Error(1)
}

func newTestPeriodClient() *mockPeriodContextClient {
	client := new(mockPeriodContextClient)
	client.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)
	return client
}

func newTestService(repo *mockExpenseRepository) *ExpenseService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewExpenseService(repo, newTestPeriodClient(), &stubFxClient{}, time.Now, logger)
}

// requireAPIError asserts that err carries an *apierr.Error (via errors.As, so a
// %w-wrapped typed error still matches) and returns it for further assertions.
func requireAPIError(t *testing.T, err error) *apierr.Error {
	t.Helper()
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	return apiErr
}

func validCreateRequest() *model.CreateExpenseRequest {
	return &model.CreateExpenseRequest{
		Name:                          "Grocery shopping",
		AmountInTransactionCurrencyMinorUnits:                        2500,
		TransactionCurrencyCode:           "USD",
		ExpenseType:                   "essentials",
		TagID:                         "tag-food",
		ExpenseDateIso:                   "2026-05-03",
		PeriodYear:                    2026,
		PeriodMonth:                   5,
		ClientGeneratedIdempotencyKey: validTestUUID,
	}
}

// --- CreateExpense validation tests ---

func TestCreateExpense_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("CreateExpense", mock.Anything, mock.MatchedBy(func(expense *model.Expense) bool {
		// Same-currency (USD/USD) create writes an identity snapshot.
		return expense.TransactionCurrencyCode == "USD" &&
			expense.OriginalTransactionAmountInMinorUnits == 2500 &&
			expense.ReportingAmountInMinorUnits == 2500 &&
			expense.ReportingCurrencyCode == "USD" &&
			expense.SourceToTargetExchangeRate == "1" &&
			expense.ExchangeRateSource == exchangesource.Identity
	})).Return(&model.Expense{
		ID:                 "exp-123",
		UserID:             "user-1",
		Name:               "Grocery shopping",
		ExpenseType:        "essentials",
		TagID:              "tag-food",
		ExpenseDateIso:        "2026-05-03",
		PeriodYear:         2026,
		PeriodMonth:        5,
		Status:             "active",
		OriginalTransactionAmountInMinorUnits:  2500,
		ReportingAmountInMinorUnits:    2500,
		ReportingCurrencyCode:  "USD",
		SourceToTargetExchangeRate:       "1",
		ExchangeRateSource: exchangesource.Identity,
	}, nil)

	expense, err := svc.CreateExpense(context.Background(), "user-1", validCreateRequest())

	require.NoError(t, err)
	assert.Equal(t, "exp-123", expense.ID)
	assert.Equal(t, "user-1", expense.UserID)
	assert.Equal(t, int64(2500), expense.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, "essentials", expense.ExpenseType)
	assert.Equal(t, "active", expense.Status)
}

// TestNewExpenseService_NilFxClientPanics asserts that a forgotten FX wire-up
// fails loudly at construction rather than on the first foreign-currency user.
func TestNewExpenseService_NilFxClientPanics(t *testing.T) {
	repo := new(mockExpenseRepository)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	require.Panics(t, func() {
		NewExpenseService(repo, newTestPeriodClient(), nil, time.Now, logger)
	})
}

// mockFxClient implements FxClient for tests.
type mockFxClient struct {
	mock.Mock
}

func (m *mockFxClient) ConvertAmount(ctx context.Context, req FxConvertRequest) (*FxConvertResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FxConvertResponse), args.Error(1)
}

func (m *mockFxClient) ConvertWithSnapshot(ctx context.Context, req FxConvertWithSnapshotRequest) (*FxConvertResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FxConvertResponse), args.Error(1)
}

// stubFxClient is a non-nil FxClient for tests that never exercise FX. A call
// to it means a same-currency test accidentally routed a foreign-currency
// request, so it fails loudly instead of returning zero values.
type stubFxClient struct{}

func (stubFxClient) ConvertAmount(ctx context.Context, req FxConvertRequest) (*FxConvertResponse, error) {
	return nil, fmt.Errorf("stubFxClient: unexpected ConvertAmount call")
}

func (stubFxClient) ConvertWithSnapshot(ctx context.Context, req FxConvertWithSnapshotRequest) (*FxConvertResponse, error) {
	return nil, fmt.Errorf("stubFxClient: unexpected ConvertWithSnapshot call")
}

// TestCreateExpense_ForeignCurrencySuccessCallsFxAndWritesProviderSnapshot
// asserts that when transactionCurrency != reportingCurrency, the service calls
// FX ConvertAmount with the exact request, builds a provider snapshot from the
// FX response, and writes the ledger row with all snapshot metadata.
func TestCreateExpense_ForeignCurrencySuccessCallsFxAndWritesProviderSnapshot(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, now)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)

	requestedAt := now.UTC().Format(time.RFC3339)
	fxResp := &FxConvertResponse{
		ConvertedAmount: 1364,
		ExchangeRate:    "1.0912",
		RateTimestamp:   "2026-08-14T10:00:00Z",
		Source:          exchangesource.OpenExchangeRates,
		ExpiresAt:       "2026-08-14T11:00:00Z",
	}

	fxClient.On("ConvertAmount", mock.Anything, mock.MatchedBy(func(req FxConvertRequest) bool {
		return req.Amount == 1250 &&
			req.SourceCurrency == "EUR" &&
			req.TargetCurrency == "USD" &&
			req.RequestedAt == requestedAt
	})).Return(fxResp, nil)

	var captured *model.Expense
	repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(*model.Expense)
		}).Return(&model.Expense{
		ID:                    "exp-fx-1",
		UserID:                "user-1",
		Name:                  "Grocery shopping",
		TransactionCurrencyCode:   "EUR",
		ExpenseType:           "essentials",
		TagID:                 "tag-food",
		ExpenseDateIso:           "2026-05-03",
		PeriodYear:            2026,
		PeriodMonth:           5,
		Status:                "active",
		OriginalTransactionAmountInMinorUnits:     1250,
		ReportingAmountInMinorUnits:       1364,
		ReportingCurrencyCode:     "USD",
		SourceToTargetExchangeRate:          "1.0912",
		ExchangeRateSource:    exchangesource.OpenExchangeRates,
		ExchangeRateTimestamp: "2026-08-14T10:00:00Z",
		ExchangeRateCacheExpiresAt: "2026-08-14T11:00:00Z",
	}, nil)

	req := validCreateRequest()
	req.AmountInTransactionCurrencyMinorUnits = 1250
	req.TransactionCurrencyCode = "EUR"

	resp, err := svc.CreateExpense(context.Background(), "user-1", req)

	require.NoError(t, err)
	require.NotNil(t, captured)

	// The ledger row stores transaction amount/currency unchanged.
	assert.Equal(t, int64(1250), captured.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, "EUR", captured.TransactionCurrencyCode)

	// The ledger row stores the FX-converted reporting amount/currency.
	assert.Equal(t, int64(1364), captured.ReportingAmountInMinorUnits)
	assert.Equal(t, "USD", captured.ReportingCurrencyCode)

	// The ledger row stores the FX snapshot metadata.
	assert.Equal(t, "1.0912", captured.SourceToTargetExchangeRate)
	assert.Equal(t, exchangesource.OpenExchangeRates, captured.ExchangeRateSource)
	assert.Equal(t, "2026-08-14T10:00:00Z", captured.ExchangeRateTimestamp)
	assert.Equal(t, "2026-08-14T11:00:00Z", captured.ExchangeRateCacheExpiresAt)

	// The response returns both transaction and reporting money fields plus
	// snapshot metadata.
	assert.Equal(t, int64(1250), resp.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, "EUR", resp.TransactionCurrencyCode)
	assert.Equal(t, int64(1364), resp.ReportingAmountInMinorUnits)
	assert.Equal(t, "USD", resp.ReportingCurrencyCode)
	assert.Equal(t, "1.0912", resp.SourceToTargetExchangeRate)
	assert.Equal(t, exchangesource.OpenExchangeRates, resp.ExchangeRateSource)
	assert.Equal(t, "2026-08-14T10:00:00Z", resp.ExchangeRateTimestamp)
	assert.Equal(t, "2026-08-14T11:00:00Z", resp.ExchangeRateCacheExpiresAt)

	fxClient.AssertExpectations(t)
	repo.AssertExpectations(t)
}

// TestCreateExpense_ForeignCurrencyFxUnavailableDoesNotWrite asserts that when
// FX returns a conversion-unavailable error, the service maps it to
// CONVERSION_UNAVAILABLE (503) and does not write a ledger row.
func TestCreateExpense_ForeignCurrencyFxUnavailableDoesNotWrite(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewExpenseService(repo, periodClient, fxClient, time.Now, logger)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)

	fxClient.On("ConvertAmount", mock.Anything, mock.MatchedBy(func(req FxConvertRequest) bool {
		return req.SourceCurrency == "EUR" && req.TargetCurrency == "USD"
	})).Return(nil, conversionUnavailableError())

	req := validCreateRequest()
	req.TransactionCurrencyCode = "EUR"

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrConversionUnavailable, svcErr.Code)
	assert.Equal(t, http.StatusServiceUnavailable, svcErr.Status)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
	fxClient.AssertExpectations(t)
}

// TestCreateExpense_ForeignCurrencyFxClientReturnsConversionUnavailableDoesNotWrite
// asserts that when the injected FxClient returns a conversion-unavailable
// error, the service maps it to the safe CONVERSION_UNAVAILABLE REST error and
// does not write a ledger row.
func TestCreateExpense_ForeignCurrencyFxClientReturnsConversionUnavailableDoesNotWrite(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewExpenseService(repo, periodClient, fxClient, time.Now, logger)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)

	fxClient.On("ConvertAmount", mock.Anything, mock.Anything).Return(nil, conversionUnavailableError())

	req := validCreateRequest()
	req.TransactionCurrencyCode = "GBP"

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrConversionUnavailable, svcErr.Code)
	assert.Equal(t, http.StatusServiceUnavailable, svcErr.Status)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// TestCreateExpense_UnsupportedTransactionCurrencyDoesNotCallFx asserts that an
// unsupported transaction currency returns a 400 field-level validation error
// and does not call FX or write a ledger row.
func TestCreateExpense_UnsupportedTransactionCurrencyDoesNotCallFx(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewExpenseService(repo, periodClient, fxClient, time.Now, logger)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)

	req := validCreateRequest()
	req.TransactionCurrencyCode = "ZZZ"

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrUnsupportedCurrency, svcErr.Code)
	assert.Equal(t, http.StatusBadRequest, svcErr.Status)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
	fxClient.AssertNotCalled(t, "ConvertAmount", mock.Anything, mock.Anything)
}

// TestCreateExpense_UnsupportedReportingCurrencyDefaultsToInternal asserts that
// when both currency fields are omitted and the period reporting currency is
// unsupported, the reporting-currency invariant is checked before the
// transaction-currency defaulting branch, so the service returns a 500 internal
// error rather than a 400 UNSUPPORTED_CURRENCY.
func TestCreateExpense_UnsupportedReportingCurrencyDefaultsToInternal(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewExpenseService(repo, periodClient, fxClient, time.Now, logger)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "XXX",
	}, nil)

	req := validCreateRequest()
	req.TransactionCurrencyCode = ""

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeInternal, svcErr.Code)
	assert.Equal(t, http.StatusInternalServerError, svcErr.Status)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
	fxClient.AssertNotCalled(t, "ConvertAmount", mock.Anything, mock.Anything)
}

// TestCreateExpense_ForeignCurrencyFxServerFailureWrapsErrorWithCurrencyPair
// asserts an FX server failure (500) is wrapped in an error that carries the
// currency pair via ReportData, so the handler's errkit.Report merges it into
// the Sentry context and slog record. The service itself does not report.
func TestCreateExpense_ForeignCurrencyFxServerFailureWrapsErrorWithCurrencyPair(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewExpenseService(repo, periodClient, fxClient, time.Now, logger)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)

	fxClient.On("ConvertAmount", mock.Anything, mock.Anything).Return(nil, apierr.Internal("currency conversion failed internally"))

	req := validCreateRequest()
	req.TransactionCurrencyCode = "EUR"

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	require.Error(t, err)
	var carrier interface{ ReportData() map[string]any }
	require.ErrorAs(t, err, &carrier)
	assert.Equal(t, map[string]any{"transaction_currency": "EUR", "reporting_currency": "USD"}, carrier.ReportData())
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// TestCreateExpense_ForeignCurrencyFxClientRejectionReturnsUnwrapped asserts
// an FX client rejection (400) is returned as the original *apierr.Error, not
// wrapped in fxConversionError, so the handler maps it to a 400 without
// reporting it as a service fault.
func TestCreateExpense_ForeignCurrencyFxClientRejectionReturnsUnwrapped(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewExpenseService(repo, periodClient, fxClient, time.Now, logger)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)

	fxClient.On("ConvertAmount", mock.Anything, mock.Anything).Return(nil, apierr.Validation("The FX service rejected the conversion amount", nil))

	req := validCreateRequest()
	req.TransactionCurrencyCode = "EUR"

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	require.Error(t, err)
	var apiErr *apierr.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apierr.CodeValidation, apiErr.Code)

	var carrier interface{ ReportData() map[string]any }
	assert.False(t, errors.As(err, &carrier))
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// TestMapFxError asserts FX gRPC status codes preserve their category (spec 05):
// Unavailable/FailedPrecondition → 503, InvalidArgument → 400, Internal → 500,
// and a non-gRPC transport failure → 503.
func TestMapFxError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedCode   string
		expectedStatus int
	}{
		{"conversion unavailable", status.Error(codes.Unavailable, "CONVERSION_UNAVAILABLE"), model.ErrConversionUnavailable, http.StatusServiceUnavailable},
		{"provider auth failed", status.Error(codes.Unavailable, "PROVIDER_AUTH_FAILED"), model.ErrConversionUnavailable, http.StatusServiceUnavailable},
		{"provider response invalid", status.Error(codes.Unavailable, "PROVIDER_RESPONSE_INVALID"), model.ErrConversionUnavailable, http.StatusServiceUnavailable},
		{"rate missing live conversion", status.Error(codes.FailedPrecondition, "RATE_MISSING"), model.ErrConversionUnavailable, http.StatusServiceUnavailable},
		{"unsupported currency", status.Error(codes.InvalidArgument, "UNSUPPORTED_CURRENCY"), model.ErrUnsupportedCurrency, http.StatusBadRequest},
		{"invalid amount", status.Error(codes.InvalidArgument, "INVALID_AMOUNT"), apierr.CodeValidation, http.StatusBadRequest},
		{"internal", status.Error(codes.Internal, "INTERNAL"), apierr.CodeInternal, http.StatusInternalServerError},
		{"non-grpc error", fmt.Errorf("connection refused"), model.ErrConversionUnavailable, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := mapFxError(tt.err)
			assert.Equal(t, tt.expectedCode, apiErr.Code)
			assert.Equal(t, tt.expectedStatus, apiErr.Status)
		})
	}
}

// TestCreateExpense_IdentitySnapshotBypassesFX asserts a same-currency write
// populates the full identity snapshot and does not require any FX client setup.
func TestCreateExpense_IdentitySnapshotBypassesFX(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo) // period client reports USD; request uses USD.

	var captured *model.Expense
	returned := &model.Expense{
		ID:                  "exp-xyz",
		UserID:              "user-1",
		Name:                "Grocery shopping",
		TransactionCurrencyCode: "USD",
		ExpenseType:         "essentials",
		TagID:               "tag-food",
		ExpenseDateIso:         "2026-05-03",
		PeriodYear:          2026,
		PeriodMonth:         5,
		Status:              "active",
		OriginalTransactionAmountInMinorUnits:   2500,
		ReportingAmountInMinorUnits:     2500,
		ReportingCurrencyCode:   "USD",
		SourceToTargetExchangeRate:        "1",
		ExchangeRateSource:  exchangesource.Identity,
	}
	repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(*model.Expense)
		}).Return(returned, nil)

	resp, err := svc.CreateExpense(context.Background(), "user-1", validCreateRequest())

	require.NoError(t, err)
	require.NotNil(t, captured)
	// The expense passed to the repository carries the full identity snapshot.
	assert.Equal(t, int64(2500), captured.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, "USD", captured.TransactionCurrencyCode)
	assert.Equal(t, int64(2500), captured.ReportingAmountInMinorUnits)
	assert.Equal(t, "USD", captured.ReportingCurrencyCode)
	assert.Equal(t, "1", captured.SourceToTargetExchangeRate)
	assert.Equal(t, exchangesource.Identity, captured.ExchangeRateSource)
	assert.NotEmpty(t, captured.ExchangeRateTimestamp)
	assert.Empty(t, captured.ExchangeRateCacheExpiresAt)
	// Response carries the canonical transaction and reporting money fields.
	assert.Equal(t, int64(2500), resp.ReportingAmountInMinorUnits)
	assert.Equal(t, "USD", resp.ReportingCurrencyCode)
	assert.Equal(t, int64(2500), resp.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, "USD", resp.TransactionCurrencyCode)
}

func TestCreateExpense_MissingPeriodDoesNotWrite(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewExpenseService(repo, periodClient, &stubFxClient{}, time.Now, logger)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(nil, periodNotFoundError(2026, 5))

	_, err := svc.CreateExpense(context.Background(), "user-1", validCreateRequest())

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrPeriodNotFound, svcErr.Code)
	assert.Equal(t, "2026", svcErr.Fields["periodYear"])
	assert.Equal(t, "5", svcErr.Fields["periodMonth"])
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

func TestCreateExpense_CurrencyCompatibility(t *testing.T) {
	tests := []struct {
		name                 string
		transactionCurrency  string
		reportingCurrency    string
		expectedCurrency     string
		expectedErrorCode    string
		expectRepositoryCall bool
	}{
		{
			name:                 "transactionCurrency only",
			transactionCurrency:  "eur",
			reportingCurrency:    "EUR",
			expectedCurrency:     "EUR",
			expectRepositoryCall: true,
		},
		{
			name:                 "empty transactionCurrency defaults to period reporting currency",
			reportingCurrency:    "CHF",
			expectedCurrency:     "CHF",
			expectRepositoryCall: true,
		},
		{
			name:                "unsupported transaction currency",
			transactionCurrency: "ZZZ",
			reportingCurrency:   "USD",
			expectedErrorCode:   model.ErrUnsupportedCurrency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mockExpenseRepository)
			periodClient := new(mockPeriodContextClient)
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			svc := NewExpenseService(repo, periodClient, &stubFxClient{}, time.Now, logger)

			reportingCurrency := tt.reportingCurrency
			if reportingCurrency == "" {
				reportingCurrency = "USD"
			}
			periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
				PeriodID:          "period-1",
				UserID:            "user-1",
				Year:              2026,
				Month:             5,
				ReportingCurrency: reportingCurrency,
			}, nil)

			if tt.expectRepositoryCall {
				repo.On("CreateExpense", mock.Anything, mock.MatchedBy(func(expense *model.Expense) bool {
					// Identity snapshot for same-currency writes.
					return expense.TransactionCurrencyCode == tt.expectedCurrency &&
						expense.OriginalTransactionAmountInMinorUnits == int64(2500) &&
						expense.ReportingAmountInMinorUnits == int64(2500) &&
						expense.ReportingCurrencyCode == reportingCurrency &&
						expense.SourceToTargetExchangeRate == "1" &&
						expense.ExchangeRateSource == exchangesource.Identity
				})).Return(&model.Expense{ID: "exp-123", TransactionCurrencyCode: tt.expectedCurrency, Status: "active"}, nil)
			}

			req := validCreateRequest()
			req.TransactionCurrencyCode = tt.transactionCurrency

			expense, err := svc.CreateExpense(context.Background(), "user-1", req)
			if tt.expectedErrorCode != "" {
				svcErr := requireAPIError(t, err)
				assert.Equal(t, tt.expectedErrorCode, svcErr.Code)
				repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCurrency, expense.TransactionCurrencyCode)
			repo.AssertExpectations(t)
		})
	}
}

func TestCreateExpense_AmountMustBePositive(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	tests := []struct {
		name   string
		amount int64
	}{
		{"zero amount", 0},
		{"negative amount", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateRequest()
			req.AmountInTransactionCurrencyMinorUnits = tt.amount

			_, err := svc.CreateExpense(context.Background(), "user-1", req)

			require.Error(t, err)
			svcErr := requireAPIError(t, err)
			assert.Equal(t, apierr.CodeValidation, svcErr.Code)
			assert.Equal(t, 400, svcErr.Status)
		})
	}
}

func TestCreateExpense_RequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		modify func(req *model.CreateExpenseRequest)
	}{
		{"missing name", func(req *model.CreateExpenseRequest) { req.Name = "" }},
		{"missing tagId", func(req *model.CreateExpenseRequest) { req.TagID = "" }},
		{"missing expenseDate", func(req *model.CreateExpenseRequest) { req.ExpenseDateIso = "" }},
		{"zero periodYear", func(req *model.CreateExpenseRequest) { req.PeriodYear = 0 }},
		{"zero periodMonth", func(req *model.CreateExpenseRequest) { req.PeriodMonth = 0 }},
		{"periodMonth 13", func(req *model.CreateExpenseRequest) { req.PeriodMonth = 13 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mockExpenseRepository)
			svc := newTestService(repo)

			req := validCreateRequest()
			tt.modify(req)

			_, err := svc.CreateExpense(context.Background(), "user-1", req)

			require.Error(t, err)
			svcErr := requireAPIError(t, err)
			assert.Equal(t, apierr.CodeValidation, svcErr.Code)
		})
	}
}

func TestCreateExpense_InvalidExpenseType(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	req := validCreateRequest()
	req.ExpenseType = "luxury"

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
}

func TestCreateExpense_ValidExpenseTypes(t *testing.T) {
	for _, expenseType := range []string{"essentials", "desires", "savings"} {
		t.Run(expenseType, func(t *testing.T) {
			repo := new(mockExpenseRepository)
			svc := newTestService(repo)

			repo.On("CreateExpense", mock.Anything, mock.AnythingOfType("*model.Expense")).
				Return(&model.Expense{
					ID:          "exp-123",
					UserID:      "user-1",
					ExpenseType: expenseType,
					Status:      "active",
				}, nil)

			req := validCreateRequest()
			req.ExpenseType = expenseType

			expense, err := svc.CreateExpense(context.Background(), "user-1", req)

			require.NoError(t, err)
			assert.Equal(t, expenseType, expense.ExpenseType)
		})
	}
}

func TestCreateExpense_InvalidDateFormat(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	tests := []struct {
		name string
		date string
	}{
		{"wrong format", "05/03/2026"},
		{"datetime", "2026-05-03T12:00:00Z"},
		{"partial", "2026-05"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validCreateRequest()
			req.ExpenseDateIso = tt.date

			_, err := svc.CreateExpense(context.Background(), "user-1", req)

			require.Error(t, err)
			svcErr := requireAPIError(t, err)
			assert.Equal(t, apierr.CodeValidation, svcErr.Code)
		})
	}
}

// --- GetExpensesForPeriod tests ---

func TestGetExpensesForPeriod_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	expenses := []*model.Expense{
		{ID: "exp-1", Name: "Groceries", Status: "active"},
		{ID: "exp-2", Name: "Coffee", Status: "active"},
	}

	repo.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5), int32(1), int32(50)).
		Return(expenses, int64(2), nil)

	result, err := svc.GetExpensesForPeriod(context.Background(), &model.GetExpensesRequest{
		UserID:   "user-1",
		Year:     2026,
		Month:    5,
		Page:     1,
		PageSize: 50,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)
	assert.Len(t, result.Data, 2)
	assert.Equal(t, int32(1), result.Page)
	assert.Equal(t, int32(50), result.PageSize)
	assert.False(t, result.HasMore)
}

func TestGetExpensesForPeriod_HasMore(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	expenses := []*model.Expense{
		{ID: "exp-1", Name: "Groceries"},
	}

	repo.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5), int32(1), int32(1)).
		Return(expenses, int64(5), nil)

	result, err := svc.GetExpensesForPeriod(context.Background(), &model.GetExpensesRequest{
		UserID:   "user-1",
		Year:     2026,
		Month:    5,
		Page:     1,
		PageSize: 1,
	})

	require.NoError(t, err)
	assert.True(t, result.HasMore)
	assert.Equal(t, int64(5), result.Total)
}

func TestGetExpensesForPeriod_DefaultsPagination(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpensesForPeriod", mock.Anything, "user-1", int32(2026), int32(5), int32(1), int32(50)).
		Return([]*model.Expense{}, int64(0), nil)

	// page=0 and pageSize=0 should be defaulted
	result, err := svc.GetExpensesForPeriod(context.Background(), &model.GetExpensesRequest{
		UserID:   "user-1",
		Year:     2026,
		Month:    5,
		Page:     0,
		PageSize: 0,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(1), result.Page)
	assert.Equal(t, int32(50), result.PageSize)
}

func TestGetExpensesForPeriod_InvalidMonth(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.GetExpensesForPeriod(context.Background(), &model.GetExpensesRequest{
		UserID: "user-1",
		Year:   2026,
		Month:  13,
	})

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
}

// --- GetExpense tests ---

func TestGetExpense_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-123", "user-1").
		Return(&model.Expense{ID: "exp-123", UserID: "user-1", Name: "Coffee", Status: "active"}, nil)

	expense, err := svc.GetExpense(context.Background(), "user-1", "exp-123")

	require.NoError(t, err)
	assert.Equal(t, "exp-123", expense.ID)
}

func TestGetExpense_NotFound(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-999", "user-1").Return(nil, nil)

	_, err := svc.GetExpense(context.Background(), "user-1", "exp-999")

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeNotFound, svcErr.Code)
	assert.Equal(t, 404, svcErr.Status)
}

func TestGetExpense_EmptyID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.GetExpense(context.Background(), "user-1", "")

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
}

// --- CorrectExpense tests ---

func activeExpenseInCurrentPeriod(now time.Time) *model.Expense {
	return &model.Expense{
		ID:                    "exp-original",
		UserID:                "user-1",
		Name:                  "Coffee",
		TransactionCurrencyCode:   "USD",
		ExpenseType:           "desires",
		TagID:                 "tag-food",
		ExpenseDateIso:           now.Format("2006-01-02"),
		PeriodYear:            int32(now.Year()),
		PeriodMonth:           int32(now.Month()),
		Status:                "active",
		CreatedAt:             now.Format(time.RFC3339),
		OriginalTransactionAmountInMinorUnits:     500,
		ReportingAmountInMinorUnits:       500,
		ReportingCurrencyCode:     "USD",
		SourceToTargetExchangeRate:          "1",
		ExchangeRateSource:    exchangesource.Identity,
		ExchangeRateTimestamp: now.Format(time.RFC3339),
	}
}

func validCorrectRequest() *model.CorrectExpenseRequest {
	return &model.CorrectExpenseRequest{
		Name:        "Updated Coffee",
		AmountInTransactionCurrencyMinorUnits:      600,
		ExpenseType: "desires",
		TagID:       "tag-food",
		ExpenseDateIso: "2026-05-03",
	}
}

func newTestServiceWithClock(repo *mockExpenseRepository, now time.Time) *ExpenseService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewExpenseService(repo, newTestPeriodClient(), &stubFxClient{}, func() time.Time { return now }, logger)
}

// newTestServiceWithFxClock builds a service with a custom period client, FX
// client, and fixed clock so FX tests can assert the exact request (including
// RequestedAt) and snapshot timestamps.
func newTestServiceWithFxClock(repo *mockExpenseRepository, periodClient *mockPeriodContextClient, fxClient FxClient, now time.Time) *ExpenseService {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewExpenseService(repo, periodClient, fxClient, func() time.Time { return now }, logger)
}

func TestCorrectExpense_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	original := activeExpenseInCurrentPeriod(now)
	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(original, nil)

	repo.On("CorrectExpense", mock.Anything, original, mock.AnythingOfType("*model.Expense")).
		Run(func(args mock.Arguments) {
			correction := args.Get(2).(*model.Expense)
			// Verify correction fields
			assert.Equal(t, "Updated Coffee", correction.Name)
			assert.Equal(t, int64(600), correction.OriginalTransactionAmountInMinorUnits)
			assert.Equal(t, "active", correction.Status)
			assert.Equal(t, "exp-original", correction.CorrectsID)
			assert.Equal(t, "USD", correction.TransactionCurrencyCode)
			assert.Equal(t, original.PeriodYear, correction.PeriodYear)
			assert.Equal(t, original.PeriodMonth, correction.PeriodMonth)
			assert.NotEmpty(t, correction.ID)
			assert.NotEqual(t, original.ID, correction.ID)
			// The correction carries an identity snapshot in the inherited
			// transaction currency (foreign-currency corrections are not supported).
			assert.Equal(t, int64(600), correction.OriginalTransactionAmountInMinorUnits)
			assert.Equal(t, int64(600), correction.ReportingAmountInMinorUnits)
			assert.Equal(t, "USD", correction.ReportingCurrencyCode)
			assert.Equal(t, "1", correction.SourceToTargetExchangeRate)
			assert.Equal(t, exchangesource.Identity, correction.ExchangeRateSource)
			assert.Equal(t, now.Format(time.RFC3339), correction.ExchangeRateTimestamp)
			assert.Empty(t, correction.ExchangeRateCacheExpiresAt)
		}).
		Return(&model.Expense{
			ID:                  "exp-correction",
			UserID:              "user-1",
			Name:                "Updated Coffee",
			TransactionCurrencyCode: "USD",
			ExpenseType:         "desires",
			TagID:               "tag-food",
			ExpenseDateIso:         "2026-05-03",
			PeriodYear:          2026,
			PeriodMonth:         5,
			Status:              "active",
			CorrectsID:          "exp-original",
			CreatedAt:           "2026-05-03T10:00:00Z",
			OriginalTransactionAmountInMinorUnits:   600,
			ReportingAmountInMinorUnits:     600,
			ReportingCurrencyCode:   "USD",
			SourceToTargetExchangeRate:        "1",
			ExchangeRateSource:  exchangesource.Identity,
		}, nil)

	result, err := svc.CorrectExpense(context.Background(), "user-1", "exp-original", validCorrectRequest())

	require.NoError(t, err)
	assert.Equal(t, "exp-correction", result.ID)
	assert.Equal(t, "active", result.Status)
	assert.Equal(t, "exp-original", result.CorrectsID)
	assert.Equal(t, int64(600), result.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, "USD", result.TransactionCurrencyCode)
	assert.Equal(t, int64(600), result.ReportingAmountInMinorUnits)
	assert.Equal(t, "USD", result.ReportingCurrencyCode)
	repo.AssertExpectations(t)
}

func TestCorrectExpense_AlreadyCorrected(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	corrected := activeExpenseInCurrentPeriod(now)
	corrected.Status = "corrected"

	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(corrected, nil)

	_, err := svc.CorrectExpense(context.Background(), "user-1", "exp-original", validCorrectRequest())

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrAlreadyCorrected, svcErr.Code)
	assert.Equal(t, 409, svcErr.Status)
}

func TestCorrectExpense_PeriodLocked(t *testing.T) {
	repo := new(mockExpenseRepository)
	// Clock is May 2026, but expense is from April 2026
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	pastExpense := &model.Expense{
		ID:          "exp-past",
		UserID:      "user-1",
		Name:        "Old Coffee",
		ExpenseType: "desires",
		TagID:       "tag-food",
		ExpenseDateIso: "2026-04-15",
		PeriodYear:  2026,
		PeriodMonth: 4, // Past period
		Status:      "active",
		CreatedAt:   "2026-04-15T10:00:00Z",
	}

	repo.On("GetExpenseByID", mock.Anything, "exp-past", "user-1").Return(pastExpense, nil)

	_, err := svc.CorrectExpense(context.Background(), "user-1", "exp-past", validCorrectRequest())

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrPeriodLocked, svcErr.Code)
	assert.Equal(t, 403, svcErr.Status)
}

func TestCorrectExpense_NotFound(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	repo.On("GetExpenseByID", mock.Anything, "exp-missing", "user-1").Return(nil, nil)

	_, err := svc.CorrectExpense(context.Background(), "user-1", "exp-missing", validCorrectRequest())

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeNotFound, svcErr.Code)
	assert.Equal(t, 404, svcErr.Status)
}

func TestCorrectExpense_EmptyID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.CorrectExpense(context.Background(), "user-1", "", validCorrectRequest())

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
}

func TestCorrectExpense_ValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		modify func(req *model.CorrectExpenseRequest)
		field  string
	}{
		{"missing name", func(req *model.CorrectExpenseRequest) { req.Name = "" }, "name"},
		{"zero amount", func(req *model.CorrectExpenseRequest) { req.AmountInTransactionCurrencyMinorUnits = 0 }, "amountInTransactionCurrencyMinorUnits"},
		{"invalid type", func(req *model.CorrectExpenseRequest) { req.ExpenseType = "luxury" }, "expenseType"},
		{"missing tagId", func(req *model.CorrectExpenseRequest) { req.TagID = "" }, "tagId"},
		{"missing date", func(req *model.CorrectExpenseRequest) { req.ExpenseDateIso = "" }, "expenseDateIso"},
		{"invalid date format", func(req *model.CorrectExpenseRequest) { req.ExpenseDateIso = "05/03/2026" }, "expenseDateIso"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mockExpenseRepository)
			svc := newTestService(repo)

			req := validCorrectRequest()
			tt.modify(req)

			_, err := svc.CorrectExpense(context.Background(), "user-1", "exp-1", req)

			require.Error(t, err)
			svcErr := requireAPIError(t, err)
			assert.Equal(t, apierr.CodeValidation, svcErr.Code)
			assert.NotEmpty(t, svcErr.Fields[tt.field])
		})
	}
}

// --- Correction currency and snapshot tests ---

// TestCorrectExpense_ForeignCurrencySuccessCallsFxAndWritesProviderSnapshot
// asserts that changing the transaction currency to a foreign currency calls FX
// before the ledger mutation and writes a provider snapshot on the correction row.
func TestCorrectExpense_ForeignCurrencySuccessCallsFxAndWritesProviderSnapshot(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithFxClock(repo, periodClient, fxClient, now)

	original := activeExpenseInCurrentPeriod(now)
	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(original, nil)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)

	requestedAt := now.UTC().Format(time.RFC3339)
	fxResp := &FxConvertResponse{
		ConvertedAmount: 655,
		ExchangeRate:    "1.0912",
		RateTimestamp:   "2026-08-14T10:00:00Z",
		Source:          exchangesource.OpenExchangeRates,
		ExpiresAt:       "2026-08-14T11:00:00Z",
	}

	fxClient.On("ConvertAmount", mock.Anything, mock.MatchedBy(func(req FxConvertRequest) bool {
		return req.Amount == 600 &&
			req.SourceCurrency == "EUR" &&
			req.TargetCurrency == "USD" &&
			req.RequestedAt == requestedAt
	})).Return(fxResp, nil)

	var captured *model.Expense
	repo.On("CorrectExpense", mock.Anything, original, mock.AnythingOfType("*model.Expense")).
		Run(func(args mock.Arguments) {
			captured = args.Get(2).(*model.Expense)
		}).Return(&model.Expense{
		ID:                    "exp-correction",
		UserID:                "user-1",
		Name:                  "Updated Coffee",
		TransactionCurrencyCode:   "EUR",
		ExpenseType:           "desires",
		TagID:                 "tag-food",
		ExpenseDateIso:           "2026-05-03",
		PeriodYear:            2026,
		PeriodMonth:           5,
		Status:                "active",
		CorrectsID:            "exp-original",
		CreatedAt:             requestedAt,
		OriginalTransactionAmountInMinorUnits:     600,
		ReportingAmountInMinorUnits:       655,
		ReportingCurrencyCode:     "USD",
		SourceToTargetExchangeRate:          "1.0912",
		ExchangeRateSource:    exchangesource.OpenExchangeRates,
		ExchangeRateTimestamp: "2026-08-14T10:00:00Z",
		ExchangeRateCacheExpiresAt: "2026-08-14T11:00:00Z",
	}, nil)

	req := validCorrectRequest()
	req.TransactionCurrencyCode = "EUR"

	resp, err := svc.CorrectExpense(context.Background(), "user-1", "exp-original", req)

	require.NoError(t, err)
	require.NotNil(t, captured)

	assert.Equal(t, "EUR", captured.TransactionCurrencyCode)
	assert.Equal(t, "USD", captured.ReportingCurrencyCode)
	assert.Equal(t, int64(600), captured.OriginalTransactionAmountInMinorUnits)
	assert.Equal(t, int64(655), captured.ReportingAmountInMinorUnits)
	assert.Equal(t, "1.0912", captured.SourceToTargetExchangeRate)
	assert.Equal(t, exchangesource.OpenExchangeRates, captured.ExchangeRateSource)
	assert.Equal(t, "2026-08-14T10:00:00Z", captured.ExchangeRateTimestamp)
	assert.Equal(t, "2026-08-14T11:00:00Z", captured.ExchangeRateCacheExpiresAt)

	assert.Equal(t, int64(655), resp.ReportingAmountInMinorUnits)
	assert.Equal(t, "USD", resp.ReportingCurrencyCode)

	fxClient.AssertExpectations(t)
	repo.AssertExpectations(t)
}

// TestCorrectExpense_ForeignCurrencyFxUnavailableDoesNotCorrect asserts that a
// failed foreign-currency conversion does not call the repository correction
// method, so the original remains active and correction history is unchanged.
func TestCorrectExpense_ForeignCurrencyFxUnavailableDoesNotCorrect(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := NewExpenseService(repo, periodClient, fxClient, func() time.Time { return now }, logger)

	original := activeExpenseInCurrentPeriod(now)
	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(original, nil)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrency: "USD",
	}, nil)

	fxClient.On("ConvertAmount", mock.Anything, mock.MatchedBy(func(req FxConvertRequest) bool {
		return req.SourceCurrency == "EUR" && req.TargetCurrency == "USD"
	})).Return(nil, conversionUnavailableError())

	req := validCorrectRequest()
	req.TransactionCurrencyCode = "EUR"

	_, err := svc.CorrectExpense(context.Background(), "user-1", "exp-original", req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrConversionUnavailable, svcErr.Code)
	assert.Equal(t, http.StatusServiceUnavailable, svcErr.Status)

	repo.AssertNotCalled(t, "CorrectExpense", mock.Anything, mock.Anything, mock.Anything)
	assert.Equal(t, "active", original.Status)
	fxClient.AssertExpectations(t)
}

// --- GetCorrectionHistory tests ---

func TestGetCorrectionHistory_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	chain := []*model.Expense{
		{ID: "exp-1", Name: "Original", Status: "corrected", CorrectsID: ""},
		{ID: "exp-2", Name: "Correction 1", Status: "corrected", CorrectsID: "exp-1"},
		{ID: "exp-3", Name: "Correction 2", Status: "active", CorrectsID: "exp-2"},
	}

	repo.On("GetCorrectionHistory", mock.Anything, "exp-2", "user-1").Return(chain, nil)

	result, err := svc.GetCorrectionHistory(context.Background(), "user-1", "exp-2")

	require.NoError(t, err)
	require.Len(t, result, 3)
	assert.Equal(t, "exp-1", result[0].ID)
	assert.Equal(t, "exp-2", result[1].ID)
	assert.Equal(t, "exp-3", result[2].ID)
}

func TestGetCorrectionHistory_SingleEntry(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	chain := []*model.Expense{
		{ID: "exp-1", Name: "Standalone", Status: "active", CorrectsID: ""},
	}

	repo.On("GetCorrectionHistory", mock.Anything, "exp-1", "user-1").Return(chain, nil)

	result, err := svc.GetCorrectionHistory(context.Background(), "user-1", "exp-1")

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "exp-1", result[0].ID)
}

func TestGetCorrectionHistory_NotFound(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetCorrectionHistory", mock.Anything, "exp-missing", "user-1").Return(nil, nil)

	_, err := svc.GetCorrectionHistory(context.Background(), "user-1", "exp-missing")

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeNotFound, svcErr.Code)
}

func TestGetCorrectionHistory_EmptyID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.GetCorrectionHistory(context.Background(), "user-1", "")

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
}

// --- GetProRataGroup tests ---

func TestGetProRataGroup_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	expenses := []*model.Expense{
		{ID: "exp-1", ProRataIndex: 1, ProRataTotal: 3},
		{ID: "exp-2", ProRataIndex: 2, ProRataTotal: 3},
	}

	repo.On("GetProRataGroup", mock.Anything, "group-1", "user-1").Return(expenses, nil)

	result, err := svc.GetProRataGroup(context.Background(), "user-1", "group-1")

	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestGetProRataGroup_EmptyGroupID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	_, err := svc.GetProRataGroup(context.Background(), "user-1", "")

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
}

// --- AnonymizeAllUserExpenses tests ---

func TestAnonymizeAllUserExpenses_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").Return(nil)

	err := svc.AnonymizeAllUserExpenses(context.Background(), "user-1")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestAnonymizeAllUserExpenses_Idempotent(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	// Calling twice should succeed both times (repo returns nil both times)
	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").Return(nil)

	err := svc.AnonymizeAllUserExpenses(context.Background(), "user-1")
	require.NoError(t, err)

	err = svc.AnonymizeAllUserExpenses(context.Background(), "user-1")
	require.NoError(t, err)

	repo.AssertNumberOfCalls(t, "AnonymizeAllUserExpenses", 2)
}

func TestAnonymizeAllUserExpenses_EmptyUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	err := svc.AnonymizeAllUserExpenses(context.Background(), "")

	require.Error(t, err)
	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
	assert.Equal(t, 400, svcErr.Status)
}

func TestAnonymizeAllUserExpenses_NoExpenses(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	// User has no expenses: repo returns nil (0 rows updated is not an error)
	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-no-expenses").Return(nil)

	err := svc.AnonymizeAllUserExpenses(context.Background(), "user-no-expenses")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestAnonymizeAllUserExpenses_DatabaseFailure(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").
		Return(fmt.Errorf("connection refused"))

	err := svc.AnonymizeAllUserExpenses(context.Background(), "user-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "anonymizing user expenses")
}

// --- Idempotent Create tests ---

// validTestUUID is a well-formed RFC 4122 UUID used across idempotency tests.
const validTestUUID = "550e8400-e29b-41d4-a716-446655440000"

// TestCreateExpense_IdempotentReplayReturnsExisting asserts that a second call
// with the same idempotency key returns the already-created expense and does
// not insert a new row.
func TestCreateExpense_IdempotentReplayReturnsExisting(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	existing := &model.Expense{
		ID:                            "exp-existing",
		UserID:                        "user-1",
		Name:                          "Grocery shopping",
		Status:                        "active",
		ClientGeneratedIdempotencyKey: validTestUUID,
	}
	repo.On("GetExpenseByIdempotencyKey", mock.Anything, "user-1", validTestUUID).Return(existing, nil)

	req := validCreateRequest()
	req.ClientGeneratedIdempotencyKey = validTestUUID

	resp, err := svc.CreateExpense(context.Background(), "user-1", req)

	require.NoError(t, err)
	assert.Equal(t, "exp-existing", resp.ID)
	assert.Equal(t, validTestUUID, resp.ClientGeneratedIdempotencyKey)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
	repo.AssertExpectations(t)
}

// TestCreateExpense_IdempotentKeyInsertsOnce asserts that the first call with a
// key (no existing row) inserts a new expense carrying the key on the row.
func TestCreateExpense_IdempotentKeyInsertsOnce(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	repo.On("GetExpenseByIdempotencyKey", mock.Anything, "user-1", validTestUUID).Return(nil, nil)
	repo.On("CreateExpense", mock.Anything, mock.MatchedBy(func(e *model.Expense) bool {
		return e.ClientGeneratedIdempotencyKey == validTestUUID
	})).Return(&model.Expense{
		ID:                            "exp-123",
		UserID:                        "user-1",
		Status:                        "active",
		ClientGeneratedIdempotencyKey: validTestUUID,
	}, nil)

	req := validCreateRequest()
	req.ClientGeneratedIdempotencyKey = validTestUUID

	resp, err := svc.CreateExpense(context.Background(), "user-1", req)

	require.NoError(t, err)
	assert.Equal(t, "exp-123", resp.ID)
	assert.Equal(t, validTestUUID, resp.ClientGeneratedIdempotencyKey)
	repo.AssertExpectations(t)
}

// TestCreateExpense_MalformedIdempotencyKeyRejected asserts a non-UUID key is
// rejected with a 400 validation error before any lookup or insert.
func TestCreateExpense_MalformedIdempotencyKeyRejected(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	req := validCreateRequest()
	req.ClientGeneratedIdempotencyKey = "not-a-uuid"

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
	assert.Equal(t, 400, svcErr.Status)
	repo.AssertNotCalled(t, "GetExpenseByIdempotencyKey", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// TestCreateExpense_OversizedIdempotencyKeyRejected asserts a key longer than
// 36 characters is rejected with a 400 validation error.
func TestCreateExpense_OversizedIdempotencyKeyRejected(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	req := validCreateRequest()
	req.ClientGeneratedIdempotencyKey = strings.Repeat("a", 37)

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
	assert.Equal(t, 400, svcErr.Status)
	repo.AssertNotCalled(t, "GetExpenseByIdempotencyKey", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// TestCreateExpense_MissingIdempotencyKeyRejected asserts that an empty key is
// rejected with a 400 validation error before any lookup or insert. The key is
// required: every create must be idempotent.
func TestCreateExpense_MissingIdempotencyKeyRejected(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	req := validCreateRequest()
	req.ClientGeneratedIdempotencyKey = ""

	_, err := svc.CreateExpense(context.Background(), "user-1", req)

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
	assert.Equal(t, 400, svcErr.Status)
	repo.AssertNotCalled(t, "GetExpenseByIdempotencyKey", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// --- DeleteExpense tests ---

func TestDeleteExpense_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	original := activeExpenseInCurrentPeriod(now)
	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(original, nil)
	repo.On("DeactivateExpense", mock.Anything, "exp-original", "user-1").Return(int64(1), nil)

	err := svc.DeleteExpense(context.Background(), "user-1", "exp-original")

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeleteExpense_AlreadyCorrected(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	corrected := activeExpenseInCurrentPeriod(now)
	corrected.Status = "corrected"
	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(corrected, nil)

	err := svc.DeleteExpense(context.Background(), "user-1", "exp-original")

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrAlreadyCorrected, svcErr.Code)
	assert.Equal(t, 409, svcErr.Status)
	repo.AssertNotCalled(t, "DeactivateExpense", mock.Anything, mock.Anything, mock.Anything)
}

func TestDeleteExpense_PeriodLocked(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	pastExpense := &model.Expense{
		ID:          "exp-past",
		UserID:      "user-1",
		Name:        "Old Coffee",
		ExpenseType: "desires",
		TagID:       "tag-food",
		ExpenseDateIso: "2026-04-15",
		PeriodYear:  2026,
		PeriodMonth: 4,
		Status:      "active",
	}
	repo.On("GetExpenseByID", mock.Anything, "exp-past", "user-1").Return(pastExpense, nil)

	err := svc.DeleteExpense(context.Background(), "user-1", "exp-past")

	svcErr := requireAPIError(t, err)
	assert.Equal(t, model.ErrPeriodLocked, svcErr.Code)
	assert.Equal(t, 403, svcErr.Status)
	repo.AssertNotCalled(t, "DeactivateExpense", mock.Anything, mock.Anything, mock.Anything)
}

func TestDeleteExpense_NotFound(t *testing.T) {
	repo := new(mockExpenseRepository)
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	svc := newTestServiceWithClock(repo, now)

	repo.On("GetExpenseByID", mock.Anything, "exp-missing", "user-1").Return(nil, nil)

	err := svc.DeleteExpense(context.Background(), "user-1", "exp-missing")

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeNotFound, svcErr.Code)
	assert.Equal(t, 404, svcErr.Status)
	repo.AssertNotCalled(t, "DeactivateExpense", mock.Anything, mock.Anything, mock.Anything)
}

func TestDeleteExpense_EmptyID(t *testing.T) {
	repo := new(mockExpenseRepository)
	svc := newTestService(repo)

	err := svc.DeleteExpense(context.Background(), "user-1", "")

	svcErr := requireAPIError(t, err)
	assert.Equal(t, apierr.CodeValidation, svcErr.Code)
	assert.Equal(t, 400, svcErr.Status)
	repo.AssertNotCalled(t, "GetExpenseByID", mock.Anything, mock.Anything, mock.Anything)
}
