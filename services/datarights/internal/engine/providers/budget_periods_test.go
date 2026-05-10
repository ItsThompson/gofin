package providers

import (
	"context"
	"fmt"
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
		"id", "year", "month", "budget_amount",
		"essentials_percent", "desires_percent", "savings_percent", "created_at",
	}
	assert.Equal(t, expected, p.Headers())
	assert.Len(t, p.Headers(), 8)
}

func TestBudgetPeriodsProvider_Collect_Success(t *testing.T) {
	mockClient := &mockFinanceServiceClient{
		getAllUserDataResp: &financepb.AllUserDataResponse{
			Periods: []*financepb.PeriodData{
				{
					Id:                "period-1",
					Year:              2026,
					Month:             5,
					BudgetAmount:      500000,
					EssentialsPercent: 50,
					DesiresPercent:    30,
					SavingsPercent:    20,
					CreatedAt:         "2026-05-01T00:00:00Z",
				},
				{
					Id:                "period-2",
					Year:              2026,
					Month:             4,
					BudgetAmount:      450000,
					EssentialsPercent: 60,
					DesiresPercent:    25,
					SavingsPercent:    15,
					CreatedAt:         "2026-04-01T00:00:00Z",
				},
			},
		},
	}

	p := NewBudgetPeriodsProvider(mockClient)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 2)

	expected1 := []string{"period-1", "2026", "5", "5000.00", "50", "30", "20", "2026-05-01T00:00:00Z"}
	expected2 := []string{"period-2", "2026", "4", "4500.00", "60", "25", "15", "2026-04-01T00:00:00Z"}
	assert.Equal(t, expected1, rows[0])
	assert.Equal(t, expected2, rows[1])
}

func TestBudgetPeriodsProvider_Collect_AmountFormatting(t *testing.T) {
	mockClient := &mockFinanceServiceClient{
		getAllUserDataResp: &financepb.AllUserDataResponse{
			Periods: []*financepb.PeriodData{
				{
					Id:           "p1",
					Year:         2026,
					Month:        1,
					BudgetAmount: 99, // 0.99 dollars
					CreatedAt:    "2026-01-01T00:00:00Z",
				},
				{
					Id:           "p2",
					Year:         2026,
					Month:        2,
					BudgetAmount: 100000, // 1000.00 dollars
					CreatedAt:    "2026-02-01T00:00:00Z",
				},
			},
		},
	}

	p := NewBudgetPeriodsProvider(mockClient)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "0.99", rows[0][3])
	assert.Equal(t, "1000.00", rows[1][3])
}

func TestBudgetPeriodsProvider_Collect_EmptyData(t *testing.T) {
	mockClient := &mockFinanceServiceClient{
		getAllUserDataResp: &financepb.AllUserDataResponse{
			Periods: []*financepb.PeriodData{},
		},
	}

	p := NewBudgetPeriodsProvider(mockClient)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestBudgetPeriodsProvider_Collect_Error(t *testing.T) {
	mockClient := &mockFinanceServiceClient{
		getAllUserDataErr: fmt.Errorf("connection refused"),
	}

	p := NewBudgetPeriodsProvider(mockClient)
	rows, err := p.Collect(context.Background(), "user-123")

	assert.Nil(t, rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching user data for budget periods")
}
