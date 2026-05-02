package service

import (
	"context"
	"fmt"

	expensepb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

// GRPCExpenseClient implements ExpenseClient by calling the expense service gRPC API.
type GRPCExpenseClient struct {
	client expensepb.ExpenseServiceClient
}

// NewGRPCExpenseClient wraps a gRPC expense service client.
func NewGRPCExpenseClient(client expensepb.ExpenseServiceClient) *GRPCExpenseClient {
	return &GRPCExpenseClient{client: client}
}

// GetExpensesForPeriod calls the expense service to fetch all active expenses for a period.
// It uses a large page size to retrieve all expenses in a single call.
func (c *GRPCExpenseClient) GetExpensesForPeriod(ctx context.Context, userID string, year, month int32) ([]ExpenseData, error) {
	resp, err := c.client.GetExpensesForPeriod(ctx, &expensepb.GetExpensesForPeriodRequest{
		UserId:   userID,
		Year:     year,
		Month:    month,
		Page:     1,
		PageSize: 10000, // fetch all: the finance service needs every expense for accurate aggregation
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC GetExpensesForPeriod: %w", err)
	}

	expenses := make([]ExpenseData, len(resp.GetData()))
	for i, exp := range resp.GetData() {
		expenses[i] = ExpenseData{
			ID:          exp.GetId(),
			Amount:      exp.GetAmount(),
			ExpenseType: exp.GetExpenseType(),
			TagID:       exp.GetTagId(),
			ExpenseDate: exp.GetExpenseDate(),
		}
	}

	return expenses, nil
}
