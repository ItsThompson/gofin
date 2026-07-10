package providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

func TestExpensesProvider_Name(t *testing.T) {
	p := NewExpensesProvider(nil, nil)
	assert.Equal(t, "expenses", p.Name())
}

func TestExpensesProvider_Headers(t *testing.T) {
	p := NewExpensesProvider(nil, nil)
	expected := []string{
		"id", "name", "amount", "currency", "expense_type", "tag_name",
		"expense_date", "period_year", "period_month", "status",
		"corrects_id", "is_pro_rata", "pro_rata_group", "pro_rata_index",
		"pro_rata_total", "created_at",
	}
	assert.Equal(t, expected, p.Headers())
	assert.Len(t, p.Headers(), 16)
}

func TestExpensesProvider_Collect_Success(t *testing.T) {
	financeClient := &mockFinanceServiceClient{
		getAllUserDataResp: &financepb.AllUserDataResponse{
			Tags: []*financepb.TagData{
				{Id: "tag-1", Name: "Food"},
				{Id: "tag-2", Name: "Transport"},
			},
		},
	}

	expenseClient := &mockExpenseServiceClient{
		getAllUserExpensesResponses: []*expensepb.ExpenseListResponse{
			{
				Data: []*expensepb.ExpenseData{
					{
						Id:          "exp-1",
						Name:        "Groceries",
						Amount:      4599,
						Currency:    "USD",
						ExpenseType: "essentials",
						TagId:       "tag-1",
						ExpenseDate: "2026-05-01",
						PeriodYear:  2026,
						PeriodMonth: 5,
						Status:      "active",
						IsProRata:   false,
						CreatedAt:   "2026-05-01T12:00:00Z",
					},
				},
				HasMore: false,
			},
		},
	}

	p := NewExpensesProvider(expenseClient, financeClient)
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
	financeClient := &mockFinanceServiceClient{
		getAllUserDataResp: &financepb.AllUserDataResponse{
			Tags: []*financepb.TagData{
				{Id: "tag-1", Name: "Rent"},
			},
		},
	}

	expenseClient := &mockExpenseServiceClient{
		getAllUserExpensesResponses: []*expensepb.ExpenseListResponse{
			{
				Data: []*expensepb.ExpenseData{
					{
						Id:            "exp-pr-1",
						Name:          "Rent (1/3)",
						Amount:        50000,
						Currency:      "USD",
						ExpenseType:   "essentials",
						TagId:         "tag-1",
						ExpenseDate:   "2026-05-01",
						PeriodYear:    2026,
						PeriodMonth:   5,
						Status:        "active",
						IsProRata:     true,
						ProRataGroup:  "group-abc",
						ProRataIndex:  1,
						ProRataTotal:  3,
						CreatedAt:     "2026-05-01T10:00:00Z",
					},
				},
				HasMore: false,
			},
		},
	}

	p := NewExpensesProvider(expenseClient, financeClient)
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
	financeClient := &mockFinanceServiceClient{
		getAllUserDataResp: &financepb.AllUserDataResponse{
			Tags: []*financepb.TagData{}, // No tags
		},
	}

	expenseClient := &mockExpenseServiceClient{
		getAllUserExpensesResponses: []*expensepb.ExpenseListResponse{
			{
				Data: []*expensepb.ExpenseData{
					{
						Id:          "exp-1",
						Name:        "Mystery",
						Amount:      1000,
						Currency:    "USD",
						ExpenseType: "desires",
						TagId:       "deleted-tag-id",
						ExpenseDate: "2026-01-15",
						PeriodYear:  2026,
						PeriodMonth: 1,
						Status:      "active",
						CreatedAt:   "2026-01-15T08:00:00Z",
					},
				},
				HasMore: false,
			},
		},
	}

	p := NewExpensesProvider(expenseClient, financeClient)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Unknown", rows[0][5]) // tag_name column
}

func TestExpensesProvider_Collect_Pagination(t *testing.T) {
	financeClient := &mockFinanceServiceClient{
		getAllUserDataResp: &financepb.AllUserDataResponse{
			Tags: []*financepb.TagData{
				{Id: "tag-1", Name: "Food"},
			},
		},
	}

	expenseClient := &mockExpenseServiceClient{
		getAllUserExpensesResponses: []*expensepb.ExpenseListResponse{
			{
				Data: []*expensepb.ExpenseData{
					{Id: "exp-1", Name: "Page1", Amount: 100, TagId: "tag-1", PeriodYear: 2026, PeriodMonth: 1, CreatedAt: "2026-01-01T00:00:00Z"},
					{Id: "exp-2", Name: "Page1b", Amount: 200, TagId: "tag-1", PeriodYear: 2026, PeriodMonth: 1, CreatedAt: "2026-01-02T00:00:00Z"},
				},
				HasMore: true,
			},
			{
				Data: []*expensepb.ExpenseData{
					{Id: "exp-3", Name: "Page2", Amount: 300, TagId: "tag-1", PeriodYear: 2026, PeriodMonth: 2, CreatedAt: "2026-02-01T00:00:00Z"},
				},
				HasMore: false,
			},
		},
	}

	p := NewExpensesProvider(expenseClient, financeClient)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Len(t, rows, 3)
	assert.Equal(t, "exp-1", rows[0][0])
	assert.Equal(t, "exp-2", rows[1][0])
	assert.Equal(t, "exp-3", rows[2][0])
	assert.Equal(t, 2, expenseClient.callCount)
}

func TestExpensesProvider_Collect_EmptyData(t *testing.T) {
	financeClient := &mockFinanceServiceClient{
		getAllUserDataResp: &financepb.AllUserDataResponse{Tags: []*financepb.TagData{}},
	}

	expenseClient := &mockExpenseServiceClient{
		getAllUserExpensesResponses: []*expensepb.ExpenseListResponse{
			{Data: []*expensepb.ExpenseData{}, HasMore: false},
		},
	}

	p := NewExpensesProvider(expenseClient, financeClient)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestExpensesProvider_Collect_ExpenseServiceError(t *testing.T) {
	financeClient := &mockFinanceServiceClient{
		getAllUserDataResp: &financepb.AllUserDataResponse{Tags: []*financepb.TagData{}},
	}

	expenseClient := &mockExpenseServiceClient{
		getAllUserExpensesErr: fmt.Errorf("connection refused"),
	}

	p := NewExpensesProvider(expenseClient, financeClient)
	rows, err := p.Collect(context.Background(), "user-123")

	assert.Nil(t, rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching expenses")
}

func TestExpensesProvider_Collect_TagServiceError(t *testing.T) {
	financeClient := &mockFinanceServiceClient{
		getAllUserDataErr: fmt.Errorf("service unavailable"),
	}

	expenseClient := &mockExpenseServiceClient{}

	p := NewExpensesProvider(expenseClient, financeClient)
	rows, err := p.Collect(context.Background(), "user-123")

	assert.Nil(t, rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching tags")
}

func TestExpensesProvider_Collect_CorrectedExpense(t *testing.T) {
	financeClient := &mockFinanceServiceClient{
		getAllUserDataResp: &financepb.AllUserDataResponse{
			Tags: []*financepb.TagData{{Id: "tag-1", Name: "Food"}},
		},
	}

	expenseClient := &mockExpenseServiceClient{
		getAllUserExpensesResponses: []*expensepb.ExpenseListResponse{
			{
				Data: []*expensepb.ExpenseData{
					{
						Id:          "exp-correction",
						Name:        "Groceries (corrected)",
						Amount:      5099,
						Currency:    "USD",
						ExpenseType: "essentials",
						TagId:       "tag-1",
						ExpenseDate: "2026-05-01",
						PeriodYear:  2026,
						PeriodMonth: 5,
						Status:      "corrected",
						CorrectsId:  "exp-original",
						CreatedAt:   "2026-05-02T09:00:00Z",
					},
				},
				HasMore: false,
			},
		},
	}

	p := NewExpensesProvider(expenseClient, financeClient)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "corrected", rows[0][9])    // status
	assert.Equal(t, "exp-original", rows[0][10]) // corrects_id
}
