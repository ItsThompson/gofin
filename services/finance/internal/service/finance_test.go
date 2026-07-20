package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/apierr"
)

func TestValidateEDSSplit_Valid(t *testing.T) {
	tests := []struct {
		name                         string
		essentials, desires, savings int32
	}{
		{"default 50/30/20", 50, 30, 20},
		{"equal 34/33/33", 34, 33, 33},
		{"extreme 100/0/0", 100, 0, 0},
		{"extreme 0/100/0", 0, 100, 0},
		{"extreme 0/0/100", 0, 0, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Nil(t, ValidateEDSSplit(tt.essentials, tt.desires, tt.savings))
		})
	}
}

func TestValidateEDSSplit_Invalid(t *testing.T) {
	tests := []struct {
		name                         string
		essentials, desires, savings int32
		errContains                  string
	}{
		{"sum 99", 50, 30, 19, "sum to 100%"},
		{"sum 101", 50, 30, 21, "sum to 100%"},
		{"sum 0", 0, 0, 0, "sum to 100%"},
		{"negative essentials", -10, 60, 50, "non-negative"},
		{"negative desires", 50, -10, 60, "non-negative"},
		{"negative savings", 50, 30, -10, "non-negative"},
		{"over 100 essentials", 110, 0, -10, "non-negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verr := ValidateEDSSplit(tt.essentials, tt.desires, tt.savings)
			require.NotNil(t, verr)
			assert.Equal(t, apierr.CodeValidation, verr.Code)
			assert.Contains(t, verr.Message, tt.errContains)
		})
	}
}

// TestValidateEDSSplit_FieldsPopulated locks in that a multi-field validation
// failure carries every offending field in Fields, not just a flat message.
func TestValidateEDSSplit_FieldsPopulated(t *testing.T) {
	t.Run("sum mismatch names all three percentages", func(t *testing.T) {
		verr := ValidateEDSSplit(50, 50, 50) // sums to 150
		require.NotNil(t, verr)
		assert.Equal(t, apierr.CodeValidation, verr.Code)
		assert.Equal(t, map[string]string{
			"essentialsPercent": "must sum to 100 with desires and savings",
			"desiresPercent":    "must sum to 100 with essentials and savings",
			"savingsPercent":    "must sum to 100 with essentials and desires",
		}, verr.Fields)
	})

	t.Run("multiple negatives name only the offending fields", func(t *testing.T) {
		verr := ValidateEDSSplit(-10, -20, 130)
		require.NotNil(t, verr)
		assert.Equal(t, apierr.CodeValidation, verr.Code)
		assert.Equal(t, map[string]string{
			"essentialsPercent": "must be non-negative",
			"desiresPercent":    "must be non-negative",
		}, verr.Fields)
	})
}
