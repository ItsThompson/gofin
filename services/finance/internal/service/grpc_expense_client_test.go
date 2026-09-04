package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	expensepb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// stubExpenseClient embeds the generated ExpenseServiceClient interface so only
// the method under test needs to be implemented; calling any other method panics
// on the nil embedded interface, which is fine for these focused mapping tests.
type stubExpenseClient struct {
	expensepb.ExpenseServiceClient
	getExpensesForPeriod func(ctx context.Context, in *expensepb.GetExpensesForPeriodRequest, opts ...grpc.CallOption) (*expensepb.ExpenseListResponse, error)
}

func (s *stubExpenseClient) GetExpensesForPeriod(ctx context.Context, in *expensepb.GetExpensesForPeriodRequest, opts ...grpc.CallOption) (*expensepb.ExpenseListResponse, error) {
	return s.getExpensesForPeriod(ctx, in, opts...)
}

// TestGRPCExpenseClient_ReadsReportingAmountAndCurrency asserts the Finance gRPC
// client maps the canonical reporting money fields off each ExpenseData response
// row, so dashboard totals aggregate in the period reporting currency.
func TestGRPCExpenseClient_ReadsReportingAmountAndCurrency(t *testing.T) {
	stub := &stubExpenseClient{
		getExpensesForPeriod: func(_ context.Context, _ *expensepb.GetExpensesForPeriodRequest, _ ...grpc.CallOption) (*expensepb.ExpenseListResponse, error) {
			return &expensepb.ExpenseListResponse{
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

	expenses, err := client.GetExpensesForPeriod(context.Background(), "user-1", 2025, 1)
	require.NoError(t, err)
	require.Len(t, expenses, 2)

	assert.Equal(t, int64(90000), expenses[0].ReportingAmount)
	assert.Equal(t, "USD", expenses[0].ReportingCurrency)
	assert.Equal(t, int64(110000), expenses[1].ReportingAmount)
	assert.Equal(t, "USD", expenses[1].ReportingCurrency)

	// Dashboard aggregation over the mapped rows uses reporting amounts.
	var total int64
	for _, exp := range expenses {
		total += exp.ReportingAmount
	}
	assert.Equal(t, int64(200000), total)
}
