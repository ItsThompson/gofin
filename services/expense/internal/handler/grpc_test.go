package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/service"
	pb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
)

func newTestGRPCHandler(repo *mockExpenseRepository) *GRPCHandler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, newTestPeriodClient(), &stubFxClient{}, time.Now, logger)
	return NewGRPCHandler(expenseSvc)
}

// mockFxClient implements service.FxClient for handler tests.
type mockFxClient struct {
	mock.Mock
}

func (m *mockFxClient) ConvertAmount(ctx context.Context, req service.FxConvertRequest) (*service.FxConvertResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.FxConvertResponse), args.Error(1)
}

func (m *mockFxClient) ConvertWithSnapshot(ctx context.Context, req service.FxConvertWithSnapshotRequest) (*service.FxConvertResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.FxConvertResponse), args.Error(1)
}

// stubFxClient is a non-nil FxClient for handler tests that never exercise FX.
// A call to it means a test accidentally routed a foreign-currency request.
type stubFxClient struct{}

func (stubFxClient) ConvertAmount(ctx context.Context, req service.FxConvertRequest) (*service.FxConvertResponse, error) {
	return nil, fmt.Errorf("stubFxClient: unexpected ConvertAmount call")
}

func (stubFxClient) ConvertWithSnapshot(ctx context.Context, req service.FxConvertWithSnapshotRequest) (*service.FxConvertResponse, error) {
	return nil, fmt.Errorf("stubFxClient: unexpected ConvertWithSnapshot call")
}

// TestGRPC_RemovedReadRPCsAreNotRegistered asserts GetCorrectionHistory and
// GetExpensesInProRataGroup are served over REST, not gRPC. This guards against
// re-introducing an unscoped read RPC on the gRPC surface.
func TestGRPC_RemovedReadRPCsAreNotRegistered(t *testing.T) {
	registered := make(map[string]bool)
	for _, m := range pb.ExpenseService_ServiceDesc.Methods {
		registered[m.MethodName] = true
	}
	for _, s := range pb.ExpenseService_ServiceDesc.Streams {
		registered[s.StreamName] = true
	}

	assert.NotContains(t, registered, "GetCorrectionHistory")
	assert.NotContains(t, registered, "GetExpensesInProRataGroup")

	// The rest of the gRPC surface is unchanged.
	assert.Contains(t, registered, "CreateExpense")
	assert.Contains(t, registered, "GetExpensesForPeriod")
	assert.Contains(t, registered, "GetExpense")
	assert.Contains(t, registered, "CorrectExpense")
	assert.Contains(t, registered, "CountExpensesByTag")
	assert.Contains(t, registered, "AnonymizeAllUserExpenses")
	assert.Contains(t, registered, "StreamAllUserExpenses")
}

func TestGRPC_CreateExpense_UsesTransactionCurrency(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&service.PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrencyCode: "EUR",
	}, nil)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, periodClient, &stubFxClient{}, time.Now, logger)
	handler := NewGRPCHandler(expenseSvc)

	repo.On("CreateExpense", mock.Anything, mock.MatchedBy(func(expense *model.Expense) bool {
		return expense.TransactionCurrencyCode == "EUR"
	})).Return(&model.Expense{
		ID:                                    "exp-1",
		UserID:                                "user-1",
		TransactionCurrencyCode:               "EUR",
		Status:                                "active",
		OriginalTransactionAmountInMinorUnits: 1200,
		ReportingAmountInMinorUnits:           1200,
		ReportingCurrencyCode:                 "EUR",
		SourceToTargetExchangeRate:            "1",
		ExchangeRateSource:                    exchangesource.Identity,
	}, nil)

	resp, err := handler.CreateExpense(context.Background(), &pb.CreateExpenseRequest{
		UserId:                                "user-1",
		Name:                                  "Coffee",
		AmountInTransactionCurrencyMinorUnits: 1200,
		TransactionCurrencyCode:               "EUR",
		ExpenseType:                           "desires",
		TagId:                                 "tag-food",
		ExpenseDateIso:                        "2026-05-03",
		PeriodYear:                            2026,
		PeriodMonth:                           5,
		ClientGeneratedIdempotencyKey:         "550e8400-e29b-41d4-a716-446655440000",
	})

	require.NoError(t, err)
	require.NotNil(t, resp.GetExpense())
	assert.Equal(t, "EUR", resp.GetExpense().GetTransactionCurrencyCode())
	// Canonical transaction and reporting money fields are present in the response.
	assert.Equal(t, int64(1200), resp.GetExpense().GetOriginalTransactionAmountInMinorUnits())
	assert.Equal(t, int64(1200), resp.GetExpense().GetReportingAmountInMinorUnits())
	assert.Equal(t, "EUR", resp.GetExpense().GetReportingCurrencyCode())
	assert.Equal(t, "1", resp.GetExpense().GetSourceToTargetExchangeRate())
	assert.Equal(t, exchangesource.Identity, resp.GetExpense().GetExchangeRateSource())
	repo.AssertExpectations(t)
}

// TestGRPC_CorrectExpense_MapsTransactionCurrency asserts the gRPC correction
// handler maps transaction_currency into the service request. The deprecated
// currency alias is no longer mapped.
func TestGRPC_CorrectExpense_MapsTransactionCurrency(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	expenseSvc := service.NewExpenseService(repo, periodClient, &stubFxClient{}, func() time.Time { return now }, logger)
	handler := NewGRPCHandler(expenseSvc)

	original := &model.Expense{
		ID:                                    "exp-original",
		UserID:                                "user-1",
		Name:                                  "Coffee",
		TransactionCurrencyCode:               "USD",
		ExpenseType:                           "desires",
		TagID:                                 "tag-food",
		ExpenseDateIso:                        "2026-05-01",
		PeriodYear:                            2026,
		PeriodMonth:                           5,
		Status:                                "active",
		CreatedAt:                             "2026-05-01T10:00:00Z",
		OriginalTransactionAmountInMinorUnits: 500,
		ReportingAmountInMinorUnits:           500,
		ReportingCurrencyCode:                 "USD",
		SourceToTargetExchangeRate:            "1",
		ExchangeRateSource:                    exchangesource.Identity,
		ExchangeRateTimestamp:                 "2026-05-01T10:00:00Z",
	}
	repo.On("GetExpenseByID", mock.Anything, "exp-original", "user-1").Return(original, nil)
	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&service.PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrencyCode: "USD",
	}, nil)

	repo.On("CorrectExpense", mock.Anything, original, mock.MatchedBy(func(correction *model.Expense) bool {
		return correction.TransactionCurrencyCode == "USD"
	})).Return(&model.Expense{
		ID:                      "exp-correction",
		UserID:                  "user-1",
		Name:                    "Updated Coffee",
		TransactionCurrencyCode: "USD",
		ExpenseType:             "desires",
		TagID:                   "tag-food",
		ExpenseDateIso:          "2026-05-01",
		PeriodYear:              2026,
		PeriodMonth:             5,
		Status:                  "active",
		CorrectsID:              "exp-original",
		CreatedAt:               "2026-05-03T10:00:00Z",
	}, nil)

	resp, err := handler.CorrectExpense(context.Background(), &pb.CorrectExpenseRequest{
		ExpenseId:                             "exp-original",
		UserId:                                "user-1",
		Name:                                  "Updated Coffee",
		AmountInTransactionCurrencyMinorUnits: 600,
		TransactionCurrencyCode:               "USD",
		ExpenseType:                           "desires",
		TagId:                                 "tag-food",
		ExpenseDateIso:                        "2026-05-01",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "USD", resp.GetExpense().GetTransactionCurrencyCode())
	repo.AssertExpectations(t)
}

func TestGRPC_CreateExpense_MissingPeriodReturnsNotFoundWithYearMonth(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, periodClient, &stubFxClient{}, time.Now, logger)
	handler := NewGRPCHandler(expenseSvc)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(6)).
		Return(nil, &apierr.Error{
			Code:    model.ErrPeriodNotFound,
			Message: "No budget period found for 2026-06",
			Status:  404,
			Fields:  map[string]string{"periodYear": "2026", "periodMonth": "6"},
		})

	resp, err := handler.CreateExpense(context.Background(), &pb.CreateExpenseRequest{
		UserId:                                "user-1",
		Name:                                  "Coffee",
		AmountInTransactionCurrencyMinorUnits: 450,
		ExpenseType:                           "desires",
		TagId:                                 "tag-food",
		ExpenseDateIso:                        "2026-06-03",
		PeriodYear:                            2026,
		PeriodMonth:                           6,
		ClientGeneratedIdempotencyKey:         "550e8400-e29b-41d4-a716-446655440000",
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "2026-06")
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// TestGRPC_CreateExpense_ForeignCurrencyFxSuccess asserts a foreign-currency
// expense with a wired FX client calls FX, writes the provider snapshot, and
// returns both transaction and reporting money fields in the gRPC response.
func TestGRPC_CreateExpense_ForeignCurrencyFxSuccess(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)

	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	requestedAt := now.UTC().Format(time.RFC3339)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&service.PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrencyCode: "USD",
	}, nil)

	fxClient.On("ConvertAmount", mock.Anything, mock.MatchedBy(func(req service.FxConvertRequest) bool {
		return req.Amount == 1250 &&
			req.SourceCurrency == "EUR" &&
			req.TargetCurrency == "USD" &&
			req.RequestedAt == requestedAt
	})).Return(&service.FxConvertResponse{
		ConvertedAmount: 1364,
		ExchangeRate:    "1.0912",
		RateTimestamp:   "2026-08-14T10:00:00Z",
		Source:          exchangesource.OpenExchangeRates,
		ExpiresAt:       "2026-08-14T11:00:00Z",
	}, nil)

	repo.On("CreateExpense", mock.Anything, mock.Anything).Return(&model.Expense{
		ID:                                    "exp-fx-1",
		UserID:                                "user-1",
		Name:                                  "Cafe",
		TransactionCurrencyCode:               "EUR",
		ExpenseType:                           "desires",
		TagID:                                 "tag-food",
		ExpenseDateIso:                        "2026-05-03",
		PeriodYear:                            2026,
		PeriodMonth:                           5,
		Status:                                "active",
		OriginalTransactionAmountInMinorUnits: 1250,
		ReportingAmountInMinorUnits:           1364,
		ReportingCurrencyCode:                 "USD",
		SourceToTargetExchangeRate:            "1.0912",
		ExchangeRateSource:                    exchangesource.OpenExchangeRates,
		ExchangeRateTimestamp:                 "2026-08-14T10:00:00Z",
		ExchangeRateCacheExpiresAt:            "2026-08-14T11:00:00Z",
	}, nil)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, periodClient, fxClient, func() time.Time { return now }, logger)
	handler := NewGRPCHandler(expenseSvc)

	resp, err := handler.CreateExpense(context.Background(), &pb.CreateExpenseRequest{
		UserId:                                "user-1",
		Name:                                  "Cafe",
		AmountInTransactionCurrencyMinorUnits: 1250,
		TransactionCurrencyCode:               "EUR",
		ExpenseType:                           "desires",
		TagId:                                 "tag-food",
		ExpenseDateIso:                        "2026-05-03",
		PeriodYear:                            2026,
		PeriodMonth:                           5,
		ClientGeneratedIdempotencyKey:         "550e8400-e29b-41d4-a716-446655440000",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	exp := resp.GetExpense()
	assert.Equal(t, int64(1250), exp.GetOriginalTransactionAmountInMinorUnits())
	assert.Equal(t, "EUR", exp.GetTransactionCurrencyCode())
	assert.Equal(t, int64(1364), exp.GetReportingAmountInMinorUnits())
	assert.Equal(t, "USD", exp.GetReportingCurrencyCode())
	assert.Equal(t, "1.0912", exp.GetSourceToTargetExchangeRate())
	assert.Equal(t, exchangesource.OpenExchangeRates, exp.GetExchangeRateSource())
	assert.Equal(t, "2026-08-14T10:00:00Z", exp.GetExchangeRateTimestamp())
	assert.Equal(t, "2026-08-14T11:00:00Z", exp.GetExchangeRateCacheExpiresAt())
}

// TestGRPC_CreateExpense_ForeignCurrencyFxUnavailable asserts a foreign-currency
// expense with an FX failure maps to codes.Unavailable and does not write a ledger row.
func TestGRPC_CreateExpense_ForeignCurrencyFxUnavailable(t *testing.T) {
	repo := new(mockExpenseRepository)
	periodClient := new(mockPeriodContextClient)
	fxClient := new(mockFxClient)

	periodClient.On("GetPeriodContext", mock.Anything, "user-1", int32(2026), int32(5)).Return(&service.PeriodContext{
		PeriodID:          "period-1",
		UserID:            "user-1",
		Year:              2026,
		Month:             5,
		ReportingCurrencyCode: "USD",
	}, nil)

	fxClient.On("ConvertAmount", mock.Anything, mock.Anything).Return(nil, &apierr.Error{
		Code:    model.ErrConversionUnavailable,
		Message: "currency conversion is unavailable",
		Status:  http.StatusServiceUnavailable,
	})

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	expenseSvc := service.NewExpenseService(repo, periodClient, fxClient, time.Now, logger)
	handler := NewGRPCHandler(expenseSvc)

	resp, err := handler.CreateExpense(context.Background(), &pb.CreateExpenseRequest{
		UserId:                                "user-1",
		Name:                                  "Cafe",
		AmountInTransactionCurrencyMinorUnits: 1250,
		TransactionCurrencyCode:               "EUR",
		ExpenseType:                           "desires",
		TagId:                                 "tag-food",
		ExpenseDateIso:                        "2026-05-03",
		PeriodYear:                            2026,
		PeriodMonth:                           5,
		ClientGeneratedIdempotencyKey:         "550e8400-e29b-41d4-a716-446655440000",
	})

	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
	repo.AssertNotCalled(t, "CreateExpense", mock.Anything, mock.Anything)
}

// TestGRPC_GetExpense_WrappedTypedErrorClassifies asserts a typed *apierr.Error
// that the service %w-wraps before it reaches the gRPC handler must still
// classify via errors.As (not collapse to codes.Internal).
func TestGRPC_GetExpense_WrappedTypedErrorClassifies(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	repo.On("GetExpenseByID", mock.Anything, "exp-1", "user-1").
		Return(nil, apierr.NotFound("expense exp-1 not found"))

	resp, err := handler.GetExpense(context.Background(), &pb.GetExpenseRequest{
		UserId: "user-1",
		Id:     "exp-1",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// --- AnonymizeAllUserExpenses gRPC handler tests ---

func TestGRPC_AnonymizeAllUserExpenses_Success(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").Return(nil)

	resp, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{
		UserId: "user-1",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	repo.AssertExpectations(t)
}

func TestGRPC_AnonymizeAllUserExpenses_EmptyUserID(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	resp, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{
		UserId: "",
	})

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "user_id is required")
}

func TestGRPC_AnonymizeAllUserExpenses_Idempotent(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	// Calling for already-redacted data: repo returns nil (success)
	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-already-redacted").Return(nil)

	resp, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{
		UserId: "user-already-redacted",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGRPC_AnonymizeAllUserExpenses_NoExpenses(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	// User has no expenses: repo returns nil (0 rows updated)
	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-no-data").Return(nil)

	resp, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{
		UserId: "user-no-data",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGRPC_AnonymizeAllUserExpenses_DatabaseFailure(t *testing.T) {
	repo := new(mockExpenseRepository)
	handler := newTestGRPCHandler(repo)

	repo.On("AnonymizeAllUserExpenses", mock.Anything, "user-1").
		Return(fmt.Errorf("database connection timeout"))

	resp, err := handler.AnonymizeAllUserExpenses(context.Background(), &pb.AnonymizeRequest{
		UserId: "user-1",
	})

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "failed to anonymize expenses")
}
