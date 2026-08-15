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
						Id:                "e1",
						Amount:            25000, // legacy amount in transaction currency
						ReportingAmount:   90000, // converted reporting amount
						ReportingCurrency: "USD",
						ExpenseType:       "essentials",
						TagId:             "t1",
						ExpenseDate:       "2025-01-05",
					},
					{
						Id:                "e2",
						Amount:            25000,
						ReportingAmount:   110000,
						ReportingCurrency: "USD",
						ExpenseType:       "desires",
						TagId:             "t2",
						ExpenseDate:       "2025-01-06",
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
	assert.Equal(t, int64(25000), expenses[0].Amount) // legacy amount preserved
	assert.Equal(t, int64(110000), expenses[1].ReportingAmount)
	assert.Equal(t, "USD", expenses[1].ReportingCurrency)

	// Dashboard aggregation over the mapped rows uses reporting amounts.
	var total int64
	for _, exp := range expenses {
		total += reportingAmount(exp)
	}
	assert.Equal(t, int64(200000), total)
}
