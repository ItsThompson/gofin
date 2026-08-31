package providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

func TestBudgetPeriodsProvider_Name(t *testing.T) {
	p := NewBudgetPeriodsProvider(nil)
	assert.Equal(t, "budget_periods", p.Name())
}

func TestBudgetPeriodsProvider_Headers(t *testing.T) {
	p := NewBudgetPeriodsProvider(nil)
	expected := []string{
		"id", "year", "month", "budget_amount", "reporting_currency",
		"essentials_percent", "desires_percent", "savings_percent", "created_at",
	}
	assert.Equal(t, expected, p.Headers())
	assert.Len(t, p.Headers(), 9)
}

func TestBudgetPeriodsProvider_Collect_Success(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Periods: []*financepb.PeriodData{
			{
				Id:                "p1",
				Year:              2026,
				Month:             1,
				BudgetAmount:      99, // 0.99 dollars
				ReportingCurrency: "USD",
				CreatedAt:         "2026-01-01T00:00:00Z",
			},
			{
				Id:                "p2",
				Year:              2026,
				Month:             2,
				BudgetAmount:      100000, // 1000.00 dollars
				ReportingCurrency: "USD",
				CreatedAt:         "2026-02-01T00:00:00Z",
			},
			{
				Id:                "p3",
				Year:              2026,
				Month:             3,
				BudgetAmount:      12345, // 12345 yen, no decimals
				ReportingCurrency: "JPY",
				CreatedAt:         "2026-03-01T00:00:00Z",
			},
		},
	}

	p := NewBudgetPeriodsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 3)

	expected1 := []string{"p1", "2026", "1", "0.99", "USD", "0", "0", "0", "2026-01-01T00:00:00Z"}
	expected2 := []string{"p2", "2026", "2", "1000.00", "USD", "0", "0", "0", "2026-02-01T00:00:00Z"}
	expected3 := []string{"p3", "2026", "3", "12345", "JPY", "0", "0", "0", "2026-03-01T00:00:00Z"}
	assert.Equal(t, expected1, rows[0])
	assert.Equal(t, expected2, rows[1])
	assert.Equal(t, expected3, rows[2])
}

func TestBudgetPeriodsProvider_Collect_UnsupportedCurrencyFails(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Periods: []*financepb.PeriodData{
			{
				Id:                "p1",
				Year:              2026,
				Month:             1,
				BudgetAmount:      100,
				ReportingCurrency: "XXX",
			},
		},
	}

	p := NewBudgetPeriodsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	assert.Nil(t, rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported currency")
}

func TestBudgetPeriodsProvider_Collect_EmptyData(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Periods: []*financepb.PeriodData{},
	}

	p := NewBudgetPeriodsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Empty(t, rows)
}
