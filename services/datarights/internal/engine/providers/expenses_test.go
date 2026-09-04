package providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
)

// buildExpenseSnapshot returns a complete identity expense snapshot,
// the shape the expense stream sends for post-cutover same-currency rows.
func buildExpenseSnapshot(id string, amount int64, currency string, overrides ...func(*expensepb.ExpenseData)) *expensepb.ExpenseData {
	exp := &expensepb.ExpenseData{
		Id:                    id,
		Name:                  "Groceries",
		ExpenseType:           "essentials",
		TagId:                 "tag-1",
		ExpenseDate:           "2026-05-01",
		PeriodYear:            2026,
		PeriodMonth:           5,
		Status:                "active",
		CreatedAt:             "2026-05-01T12:00:00Z",
		TransactionCurrency:   currency,
		TransactionAmount:     amount,
		ReportingAmount:       amount,
		ReportingCurrency:     currency,
		ExchangeRate:          "1",
		ExchangeRateSource:    exchangesource.Identity,
		ExchangeRateTimestamp: "2026-05-01T12:00:00Z",
	}
	for _, override := range overrides {
		override(exp)
	}
	return exp
}

func TestExpensesProvider_Name(t *testing.T) {
	p := NewExpensesProvider(nil, nil, nil)
	assert.Equal(t, "expenses", p.Name())
}

func TestExpensesProvider_Headers(t *testing.T) {
	p := NewExpensesProvider(nil, nil, nil)
	expected := []string{
		"id", "name", "transaction_amount", "transaction_currency",
		"reporting_amount", "reporting_currency", "exchange_rate",
		"exchange_rate_source", "exchange_rate_timestamp", "expense_type",
		"tag_name", "expense_date", "period_year", "period_month", "status",
		"corrects_id", "is_pro_rata", "pro_rata_group", "pro_rata_index",
		"pro_rata_total", "created_at",
	}
	assert.Equal(t, expected, p.Headers())
	assert.Len(t, p.Headers(), 21)
}

func TestExpensesProvider_Collect_Success(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food", "tag-2": "Transport"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			buildExpenseSnapshot("exp-1", 4599, "USD"),
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap, nil)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)

	expected := []string{
		"exp-1", "Groceries", "45.99", "USD", "45.99", "USD", "1", "identity", "2026-05-01T12:00:00Z",
		"essentials", "Food", "2026-05-01", "2026", "5", "active", "", "false", "", "", "",
		"2026-05-01T12:00:00Z",
	}
	assert.Equal(t, expected, rows[0])
}

func TestExpensesProvider_Collect_ProRataExpense(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Rent"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			buildExpenseSnapshot("exp-pr-1", 50000, "USD", func(exp *expensepb.ExpenseData) {
				exp.Name = "Rent (1/3)"
				exp.TagId = "tag-1"
				exp.IsProRata = true
				exp.ProRataGroup = "group-abc"
				exp.ProRataIndex = 1
				exp.ProRataTotal = 3
				exp.CreatedAt = "2026-05-01T10:00:00Z"
			}),
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap, nil)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)

	expected := []string{
		"exp-pr-1", "Rent (1/3)", "500.00", "USD", "500.00", "USD", "1", "identity", "2026-05-01T12:00:00Z",
		"essentials", "Rent", "2026-05-01", "2026", "5", "active", "", "true", "group-abc", "1", "3",
		"2026-05-01T10:00:00Z",
	}
	assert.Equal(t, expected, rows[0])
}

func TestExpensesProvider_Collect_MissingTagResolvesToUnknown(t *testing.T) {
	tagMap := map[string]string{} // No tags

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			buildExpenseSnapshot("exp-1", 1000, "USD", func(exp *expensepb.ExpenseData) {
				exp.Name = "Mystery"
				exp.ExpenseType = "desires"
				exp.TagId = "deleted-tag-id"
				exp.ExpenseDate = "2026-01-15"
				exp.PeriodMonth = 1
				exp.CreatedAt = "2026-01-15T08:00:00Z"
			}),
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap, nil)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Unknown", rows[0][10]) // tag_name column
}

func TestExpensesProvider_Collect_MultipleRowsInStreamOrder(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food"}

	// The server streams every expense in one ordered pass (chronological:
	// created_at ASC, id ASC); the consumer does not page.
	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			buildExpenseSnapshot("exp-1", 100, "USD", func(exp *expensepb.ExpenseData) {
				exp.Name = "First"
				exp.PeriodMonth = 1
				exp.CreatedAt = "2026-01-01T00:00:00Z"
			}),
			buildExpenseSnapshot("exp-2", 200, "USD", func(exp *expensepb.ExpenseData) {
				exp.Name = "Second"
				exp.PeriodMonth = 1
				exp.CreatedAt = "2026-01-02T00:00:00Z"
			}),
			buildExpenseSnapshot("exp-3", 300, "USD", func(exp *expensepb.ExpenseData) {
				exp.Name = "Third"
				exp.PeriodMonth = 2
				exp.CreatedAt = "2026-02-01T00:00:00Z"
			}),
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap, nil)
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

func TestExpensesProvider_Collect_ForeignCurrencyRow(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			{
				Id:                    "exp-fx",
				Name:                  "Hotel",
				ExpenseType:           "essentials",
				TagId:                 "tag-1",
				ExpenseDate:           "2026-05-01",
				PeriodYear:            2026,
				PeriodMonth:           5,
				Status:                "active",
				CreatedAt:             "2026-05-01T12:00:00Z",
				TransactionCurrency:   "EUR",
				TransactionAmount:     1250,
				ReportingAmount:       1364,
				ReportingCurrency:     "USD",
				ExchangeRate:          "1.0912",
				ExchangeRateSource:    exchangesource.OpenExchangeRates,
				ExchangeRateTimestamp: "2026-08-14T10:00:00Z",
			},
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap, nil)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "12.50", rows[0][2])  // transaction_amount in EUR
	assert.Equal(t, "EUR", rows[0][3])    // transaction_currency
	assert.Equal(t, "13.64", rows[0][4])  // reporting_amount in USD
	assert.Equal(t, "USD", rows[0][5])    // reporting_currency
	assert.Equal(t, "1.0912", rows[0][6]) // exchange_rate
	assert.Equal(t, exchangesource.OpenExchangeRates, rows[0][7])
	assert.Equal(t, "2026-08-14T10:00:00Z", rows[0][8])
}

func TestExpensesProvider_Collect_JPYHasNoForcedDecimals(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			buildExpenseSnapshot("exp-jpy", 4599, "JPY"),
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap, nil)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "4599", rows[0][2])
	assert.Equal(t, "4599", rows[0][4])
	assert.Equal(t, "JPY", rows[0][3])
	assert.Equal(t, "JPY", rows[0][5])
}

func TestExpensesProvider_Collect_LegacyRowNormalizesToPeriodCurrency(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food"}
	periodCurrencies := map[string]string{"2026:5": "USD"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			{
				Id:                    "exp-legacy",
				Name:                  "Groceries",
				ExpenseType:           "essentials",
				TagId:                 "tag-1",
				ExpenseDate:           "2026-05-01",
				PeriodYear:            2026,
				PeriodMonth:           5,
				Status:                "active",
				CreatedAt:             "2026-05-01T12:00:00Z",
				TransactionAmount:     4599,
				TransactionCurrency:   "EUR",
				ReportingAmount:       4599,
				ReportingCurrency:     "EUR",
				ExchangeRate:          "1",
				ExchangeRateSource:    exchangesource.Migration,
				ExchangeRateTimestamp: "2026-05-01T12:00:00Z",
			},
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap, periodCurrencies)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "45.99", rows[0][2])
	assert.Equal(t, "USD", rows[0][3])
	assert.Equal(t, "45.99", rows[0][4])
	assert.Equal(t, "USD", rows[0][5])
	assert.Equal(t, "1", rows[0][6])
	assert.Equal(t, exchangesource.Identity, rows[0][7])
	assert.Equal(t, "2026-05-01T12:00:00Z", rows[0][8])
}

func TestExpensesProvider_Collect_LegacyRowFallsBackToStreamCurrency(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			{
				Id:                    "exp-legacy",
				Name:                  "Groceries",
				ExpenseType:           "essentials",
				TagId:                 "tag-1",
				ExpenseDate:           "2026-05-01",
				PeriodYear:            2026,
				PeriodMonth:           5,
				Status:                "active",
				CreatedAt:             "2026-05-01T12:00:00Z",
				TransactionAmount:     4599,
				TransactionCurrency:   "USD",
				ReportingAmount:       4599,
				ReportingCurrency:     "USD",
				ExchangeRate:          "1",
				ExchangeRateSource:    exchangesource.Migration,
				ExchangeRateTimestamp: "2026-05-01T12:00:00Z",
			},
		},
	}

	// No period currency map: the provider trusts the reporting currency the
	// expense stream already resolved.
	p := NewExpensesProvider(expenseClient, tagMap, nil)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "USD", rows[0][3])
	assert.Equal(t, "USD", rows[0][5])
}

func TestExpensesProvider_Collect_IncompleteSnapshotFails(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			{
				Id:                  "exp-bad",
				Name:                "Broken",
				ExpenseType:         "essentials",
				TagId:               "tag-1",
				PeriodYear:          2026,
				PeriodMonth:         5,
				Status:              "active",
				TransactionCurrency: "USD",
				TransactionAmount:   4599,
				ReportingAmount:     4599,
				ReportingCurrency:   "USD",
				ExchangeRate:        "1",
				ExchangeRateSource:  exchangesource.Identity,
			},
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap, nil)
	rows, err := p.Collect(context.Background(), "user-123")

	assert.Nil(t, rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching expenses")
	assert.Contains(t, err.Error(), "incomplete money snapshot")
}

func TestExpensesProvider_Collect_UnknownSourceFails(t *testing.T) {
	tagMap := map[string]string{"tag-1": "Food"}

	expenseClient := &mockExpenseServiceClient{
		streamRows: []*expensepb.ExpenseData{
			buildExpenseSnapshot("exp-bad", 100, "USD", func(exp *expensepb.ExpenseData) {
				exp.ExchangeRateSource = ""
			}),
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap, nil)
	rows, err := p.Collect(context.Background(), "user-123")

	assert.Nil(t, rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid exchange_rate_source")
}

func TestExpensesProvider_Collect_EmptyData(t *testing.T) {
	expenseClient := &mockExpenseServiceClient{streamRows: nil}

	p := NewExpensesProvider(expenseClient, map[string]string{}, nil)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestExpensesProvider_Collect_StreamOpenError(t *testing.T) {
	expenseClient := &mockExpenseServiceClient{
		streamOpenErr: fmt.Errorf("connection refused"),
	}

	p := NewExpensesProvider(expenseClient, map[string]string{}, nil)
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
			buildExpenseSnapshot("exp-1", 100, "USD", func(exp *expensepb.ExpenseData) {
				exp.PeriodMonth = 1
				exp.CreatedAt = "2026-01-01T00:00:00Z"
			}),
			buildExpenseSnapshot("exp-2", 200, "USD", func(exp *expensepb.ExpenseData) {
				exp.PeriodMonth = 1
				exp.CreatedAt = "2026-01-02T00:00:00Z"
			}),
		},
		recvErr:   fmt.Errorf("stream reset"),
		recvErrAt: 3,
	}

	p := NewExpensesProvider(expenseClient, tagMap, nil)
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
			buildExpenseSnapshot("exp-correction", 5099, "USD", func(exp *expensepb.ExpenseData) {
				exp.Name = "Groceries (corrected)"
				exp.Status = "corrected"
				exp.CorrectsId = "exp-original"
				exp.CreatedAt = "2026-05-02T09:00:00Z"
			}),
		},
	}

	p := NewExpensesProvider(expenseClient, tagMap, nil)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "corrected", rows[0][14])    // status
	assert.Equal(t, "exp-original", rows[0][15]) // corrects_id
}
