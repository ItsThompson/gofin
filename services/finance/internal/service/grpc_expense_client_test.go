package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	expensepb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// stubExpenseClient embeds the generated ExpenseServiceClient interface so only
// the method under test needs to be implemented; calling any other method panics
// on the nil embedded interface, which is fine for these focused mapping tests.
type stubExpenseClient struct {
	expensepb.ExpenseServiceClient
	getActiveExpensesForPeriodPage func(ctx context.Context, in *expensepb.GetActiveExpensesForPeriodPageRequest, opts ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error)
}

func (s *stubExpenseClient) GetActiveExpensesForPeriodPage(ctx context.Context, in *expensepb.GetActiveExpensesForPeriodPageRequest, opts ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error) {
	return s.getActiveExpensesForPeriodPage(ctx, in, opts...)
}

// TestGRPCExpenseClient_ReadsReportingAmountAndCurrency asserts the Finance gRPC
// client maps the canonical reporting money fields off each ExpenseData response
// row, so dashboard totals aggregate in the period reporting currency.
func TestGRPCExpenseClient_ReadsReportingAmountAndCurrency(t *testing.T) {
	stub := &stubExpenseClient{
		getActiveExpensesForPeriodPage: func(_ context.Context, _ *expensepb.GetActiveExpensesForPeriodPageRequest, _ ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error) {
			return &expensepb.CompleteExpensePageResponse{
				Data: []*expensepb.ExpenseData{
					{
						Id:                          "e1",
						ReportingAmountInMinorUnits: 90000, // converted reporting amount
						ReportingCurrencyCode:       "USD",
						ExpenseType:                 "essentials",
						TagId:                       "t1",
						ExpenseDateIso:              "2025-01-05",
					},
					{
						Id:                          "e2",
						ReportingAmountInMinorUnits: 110000,
						ReportingCurrencyCode:       "USD",
						ExpenseType:                 "desires",
						TagId:                       "t2",
						ExpenseDateIso:              "2025-01-06",
					},
				},
			}, nil
		},
	}

	client := NewGRPCExpenseClient(stub)

	expenses, err := client.GetActiveExpensesForPeriod(context.Background(), "user-1", 2025, 1)
	require.NoError(t, err)
	require.Len(t, expenses, 2)

	assert.Equal(t, int64(90000), expenses[0].ReportingAmount)
	assert.Equal(t, "USD", expenses[0].ReportingCurrencyCode)
	assert.Equal(t, int64(110000), expenses[1].ReportingAmount)
	assert.Equal(t, "USD", expenses[1].ReportingCurrencyCode)

	// Dashboard aggregation over the mapped rows uses reporting amounts.
	var total int64
	for _, exp := range expenses {
		total += exp.ReportingAmount
	}
	assert.Equal(t, int64(200000), total)
}

func TestGRPCExpenseClient_ReadsAllCompletePeriodPages(t *testing.T) {
	calls := 0
	stub := &stubExpenseClient{
		getActiveExpensesForPeriodPage: func(_ context.Context, request *expensepb.GetActiveExpensesForPeriodPageRequest, _ ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error) {
			calls++
			assert.Equal(t, int32(50), request.GetPageSize())
			if calls == 1 {
				assert.Empty(t, request.GetCursorId())
				return &expensepb.CompleteExpensePageResponse{
					Data:            []*expensepb.ExpenseData{{Id: "e-1", ExpenseDateIso: "2026-05-01", CreatedAt: "2026-05-01T00:00:00Z", ReportingAmountInMinorUnits: 100}},
					NextExpenseDate: "2026-05-01",
					NextCreatedAt:   "2026-05-01T00:00:00Z",
					NextId:          "e-1",
					HasMore:         true,
				}, nil
			}
			assert.Equal(t, "2026-05-01", request.GetCursorExpenseDate())
			assert.Equal(t, "e-1", request.GetCursorId())
			return &expensepb.CompleteExpensePageResponse{
				Data: []*expensepb.ExpenseData{{Id: "e-2", ReportingAmountInMinorUnits: 200}},
			}, nil
		},
	}

	expenses, err := NewGRPCExpenseClient(stub).GetActiveExpensesForPeriod(context.Background(), "user-1", 2026, 5)

	require.NoError(t, err)
	assert.Equal(t, []ExpenseData{{ID: "e-1", ReportingAmount: 100, ExpenseDate: "2026-05-01"}, {ID: "e-2", ReportingAmount: 200}}, expenses)
	assert.Equal(t, 2, calls)
}

func TestGRPCExpenseClient_ReturnsSourceErrors(t *testing.T) {
	sourceErr := errors.New("expense unavailable")
	stub := &stubExpenseClient{
		getActiveExpensesForPeriodPage: func(context.Context, *expensepb.GetActiveExpensesForPeriodPageRequest, ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error) {
			return nil, sourceErr
		},
	}

	expenses, err := NewGRPCExpenseClient(stub).GetActiveExpensesForPeriod(context.Background(), "user-1", 2026, 5)

	require.Error(t, err)
	assert.ErrorIs(t, err, sourceErr)
	assert.Nil(t, expenses)
}

func TestGRPCExpenseClient_RejectsDuplicateCursorWithoutRetryLoop(t *testing.T) {
	calls := 0
	stub := &stubExpenseClient{
		getActiveExpensesForPeriodPage: func(_ context.Context, request *expensepb.GetActiveExpensesForPeriodPageRequest, _ ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error) {
			calls++
			if calls == 1 {
				return &expensepb.CompleteExpensePageResponse{
					Data:            []*expensepb.ExpenseData{{Id: "e-1", ExpenseDateIso: "2026-05-01", CreatedAt: "2026-05-01T00:00:00Z"}},
					NextExpenseDate: "2026-05-01",
					NextCreatedAt:   "2026-05-01T00:00:00Z",
					NextId:          "e-1",
					HasMore:         true,
				}, nil
			}
			assert.Equal(t, "e-1", request.GetCursorId())
			return &expensepb.CompleteExpensePageResponse{
				Data:            []*expensepb.ExpenseData{{Id: "e-1", ExpenseDateIso: "2026-05-01", CreatedAt: "2026-05-01T00:00:00Z"}},
				NextExpenseDate: "2026-05-01",
				NextCreatedAt:   "2026-05-01T00:00:00Z",
				NextId:          "e-1",
				HasMore:         true,
			}, nil
		},
	}

	expenses, err := NewGRPCExpenseClient(stub).GetActiveExpensesForPeriod(context.Background(), "user-1", 2026, 5)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-advancing or regressing next cursor")
	assert.Nil(t, expenses)
	assert.Equal(t, 2, calls)
}

func TestGRPCExpenseClient_RejectsRegressingCursor(t *testing.T) {
	calls := 0
	stub := &stubExpenseClient{
		getActiveExpensesForPeriodPage: func(_ context.Context, request *expensepb.GetActiveExpensesForPeriodPageRequest, _ ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error) {
			calls++
			if calls == 1 {
				return &expensepb.CompleteExpensePageResponse{
					Data:            []*expensepb.ExpenseData{{Id: "e-2", ExpenseDateIso: "2026-05-02", CreatedAt: "2026-05-02T00:00:00Z"}},
					NextExpenseDate: "2026-05-02",
					NextCreatedAt:   "2026-05-02T00:00:00Z",
					NextId:          "e-2",
					HasMore:         true,
				}, nil
			}
			assert.Equal(t, "e-2", request.GetCursorId())
			return &expensepb.CompleteExpensePageResponse{
				Data:            []*expensepb.ExpenseData{{Id: "e-1", ExpenseDateIso: "2026-05-01", CreatedAt: "2026-05-01T00:00:00Z"}},
				NextExpenseDate: "2026-05-01",
				NextCreatedAt:   "2026-05-01T00:00:00Z",
				NextId:          "e-1",
				HasMore:         true,
			}, nil
		},
	}

	expenses, err := NewGRPCExpenseClient(stub).GetActiveExpensesForPeriod(context.Background(), "user-1", 2026, 5)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-advancing or regressing next cursor")
	assert.Nil(t, expenses)
	assert.Equal(t, 2, calls)
}

func TestGRPCExpenseClient_PreservesResourceExhausted(t *testing.T) {
	stub := &stubExpenseClient{
		getActiveExpensesForPeriodPage: func(context.Context, *expensepb.GetActiveExpensesForPeriodPageRequest, ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error) {
			return nil, status.Error(codes.ResourceExhausted, "response exceeds message limit")
		},
	}

	expenses, err := NewGRPCExpenseClient(stub).GetActiveExpensesForPeriod(context.Background(), "user-1", 2026, 5)

	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Nil(t, expenses)
}

func TestGRPCExpenseClient_RejectsIncompletePageCursor(t *testing.T) {
	stub := &stubExpenseClient{
		getActiveExpensesForPeriodPage: func(context.Context, *expensepb.GetActiveExpensesForPeriodPageRequest, ...grpc.CallOption) (*expensepb.CompleteExpensePageResponse, error) {
			return &expensepb.CompleteExpensePageResponse{
				Data:    []*expensepb.ExpenseData{{Id: "e-1", ExpenseDateIso: "2026-05-01", CreatedAt: "2026-05-01T00:00:00Z"}},
				HasMore: true,
			}, nil
		},
	}

	expenses, err := NewGRPCExpenseClient(stub).GetActiveExpensesForPeriod(context.Background(), "user-1", 2026, 5)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "incomplete next cursor")
	assert.Nil(t, expenses)
}
