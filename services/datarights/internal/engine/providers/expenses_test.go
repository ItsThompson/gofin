package providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
)

func TestExpensesProvider_Name(t *testing.T) {
	p := NewExpensesProvider(nil, nil)
	assert.Equal(t, "expenses", p.Name())
}

func TestExpensesProvider_Headers(t *testing.T) {
	p := NewExpensesProvider(nil, nil)
	expected := []string{
		"id", "name", "transaction_amount", "transaction_currency", "expense_type", "tag_name",
		"expense_date", "period_year", "period_month", "status",
		"corrects_id", "is_pro_rata", "pro_rata_group", "pro_rata_index",
		"pro_rata_total", "created_at",
	}
	assert.Equal(t, expected, p.Headers())
	assert.Len(t, p.Headers(), 16)
}

func TestExpensesProvider_Collect_Success(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food", "tag-2": "Transport"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			{
				Id:                  "exp-1",
				Name:                "Groceries",
				TransactionAmount:   4599,
				TransactionCurrency: "USD",
				ExpenseType:         "essentials",
				TagId:               "tag-1",
				ExpenseDate:         "2026-05-01",
				PeriodYear:          2026,
				PeriodMonth:         5,
				Status:              "active",
				IsProRata:           false,
				CreatedAt:           "2026-05-01T12:00:00Z",
			},
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)

	expected := []string{
		"exp-1", "Groceries", "45.99", "USD", "essentials", "Food",
		"2026-05-01", "2026", "5", "active", "", "false", "", "", "",
		"2026-05-01T12:00:00Z",
	}
	assert.Equal(t, expected, rows[0])
}

func TestExpensesProvider_Collect_ProRataExpense(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Rent"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			{
				Id:                  "exp-pr-1",
				Name:                "Rent (1/3)",
				TransactionAmount:   50000,
				TransactionCurrency: "USD",
				ExpenseType:         "essentials",
				TagId:               "tag-1",
				ExpenseDate:         "2026-05-01",
				PeriodYear:          2026,
				PeriodMonth:         5,
				Status:              "active",
				IsProRata:           true,
				ProRataGroup:        "group-abc",
				ProRataIndex:        1,
				ProRataTotal:        3,
				CreatedAt:           "2026-05-01T10:00:00Z",
			},
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)

	expected := []string{
		"exp-pr-1", "Rent (1/3)", "500.00", "USD", "essentials", "Rent",
		"2026-05-01", "2026", "5", "active", "", "true", "group-abc", "1", "3",
		"2026-05-01T10:00:00Z",
	}
	assert.Equal(t, expected, rows[0])
}

func TestExpensesProvider_Collect_MissingTagResolvesToUnknown(t *testing.T) {
	tagMap := map[string]string{} // No tags

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			{
				Id:                  "exp-1",
				Name:                "Mystery",
				TransactionAmount:   1000,
				TransactionCurrency: "USD",
				ExpenseType:         "desires",
				TagId:               "deleted-tag-id",
				ExpenseDate:         "2026-01-15",
				PeriodYear:          2026,
				PeriodMonth:         1,
				Status:              "active",
				CreatedAt:           "2026-01-15T08:00:00Z",
			},
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Unknown", rows[0][5]) // tag_name column
}

func TestExpensesProvider_Collect_MultipleRowsInStreamOrder(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food"}

	// The server streams every expense in one ordered pass (chronological:
	// created_at ASC, id ASC); the consumer does not page.
	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			{Id: "exp-1", Name: "First", TransactionAmount: 100, TagId: "tag-1", PeriodYear: 2026, PeriodMonth: 1, CreatedAt: "2026-01-01T00:00:00Z"},
			{Id: "exp-2", Name: "Second", TransactionAmount: 200, TagId: "tag-1", PeriodYear: 2026, PeriodMonth: 1, CreatedAt: "2026-01-02T00:00:00Z"},
			{Id: "exp-3", Name: "Third", TransactionAmount: 300, TagId: "tag-1", PeriodYear: 2026, PeriodMonth: 2, CreatedAt: "2026-02-01T00:00:00Z"},
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Len(t, rows, 3)
	assert.Equal(t, "exp-1", rows[0][0])
	assert.Equal(t, "exp-2", rows[1][0])
	assert.Equal(t, "exp-3", rows[2][0])
	// One StreamAllUserExpenses call fetches the whole history (no per-page loop).
	assert.Equal(t, 1, expenseClient.callCount)
	assert.Equal(t, int32(expensesPageSize), expenseClient.lastStreamReq.GetPageSize())
}

func TestExpensesProvider_Collect_EmptyData(t *testing.T) {
	expenseClient := &mockExpenseServiceClient{streamRows: nil}

	p := NewExpensesProvider(expenseClient, map[string]string{})
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestExpensesProvider_Collect_StreamOpenError(t *testing.T) {
	expenseClient := &mockExpenseServiceClient{
		streamOpenErr: fmt.Errorf("connection refused"),
	}

	p := NewExpensesProvider(expenseClient, map[string]string{})
	rows, err := p.Collect(context.Background(), "user-123")

	assert.Nil(t, rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching expenses")
}

func TestExpensesProvider_Collect_MidStreamRecvError(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food"}

	// Two rows arrive, then the third Recv fails: the error must propagate
	// (not be swallowed as a clean EOF).
	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			{Id: "exp-1", TagId: "tag-1", PeriodYear: 2026, PeriodMonth: 1, CreatedAt: "2026-01-01T00:00:00Z"},
			{Id: "exp-2", TagId: "tag-1", PeriodYear: 2026, PeriodMonth: 1, CreatedAt: "2026-01-02T00:00:00Z"},
		},
		recvErr:   fmt.Errorf("stream reset"),
		recvErrAt: 3,
	}

	p := NewExpensesProvider(expenseClient, tagMap)
	rows, err := p.Collect(context.Background(), "user-123")

	assert.Nil(t, rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching expenses")
	assert.Contains(t, err.Error(), "stream reset")
}

func TestExpensesProvider_Collect_CorrectedExpense(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			{
				Id:                  "exp-correction",
				Name:                "Groceries (corrected)",
				TransactionAmount:   5099,
				TransactionCurrency: "USD",
				ExpenseType:         "essentials",
				TagId:               "tag-1",
				ExpenseDate:         "2026-05-01",
				PeriodYear:          2026,
				PeriodMonth:         5,
				Status:              "corrected",
				CorrectsId:          "exp-original",
				CreatedAt:           "2026-05-02T09:00:00Z",
			},
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "corrected", rows[0][9])     // status
	assert.Equal(t, "exp-original", rows[0][10]) // corrects_id
}
