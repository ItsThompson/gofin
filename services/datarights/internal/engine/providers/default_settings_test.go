package providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

func TestDefaultSettingsProvider_Name(t *testing.T) {
	p := NewDefaultSettingsProvider(nil)
	assert.Equal(t, "default_settings", p.Name())
}

func TestDefaultSettingsProvider_Headers(t *testing.T) {
	p := NewDefaultSettingsProvider(nil)
	expected := []string{
		"budget_amount", "essentials_percent", "desires_percent",
		"savings_percent", "currency",
	}
	assert.Equal(t, expected, p.Headers())
	assert.Len(t, p.Headers(), 5)
}

func TestDefaultSettingsProvider_Collect_Success(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Defaults: &financepb.DefaultsData{
			BudgetAmount:      500000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
			Currency:          "USD",
		},
	}

	p := NewDefaultSettingsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)

	expected := []string{"5000.00", "50", "30", "20", "USD"}
	assert.Equal(t, expected, rows[0])
}

func TestDefaultSettingsProvider_Collect_AmountFormatting(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Defaults: &financepb.DefaultsData{
			BudgetAmount:      4599,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
			Currency:          "EUR",
		},
	}

	p := NewDefaultSettingsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "45.99", rows[0][0])
}

func TestDefaultSettingsProvider_Collect_JPYHasNoDecimals(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Defaults: &financepb.DefaultsData{
			BudgetAmount:      1250,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
			Currency:          "JPY",
		},
	}

	p := NewDefaultSettingsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "1250", rows[0][0])
}

func TestDefaultSettingsProvider_Collect_UnsupportedCurrencyFallsBackToTwoDecimals(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Defaults: &financepb.DefaultsData{
			BudgetAmount: 100,
			Currency:     "XXX",
		},
	}

	p := NewDefaultSettingsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "1.00", rows[0][0])
	assert.Equal(t, "XXX", rows[0][4])
}

func TestDefaultSettingsProvider_Collect_NilDefaults(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Defaults: nil,
	}

	p := NewDefaultSettingsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Empty(t, rows)
}
