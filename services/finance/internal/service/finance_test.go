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

// TestValidateEDSSplit_Invalid locks in the aggregated behavior: every
// violation is surfaced in one pass, the top-level message standardizes to
// "validation failed", and the field-level messages carry the specific detail.
// The validator records the first error per field (check order: non-negative,
// not-over-100, sum-to-100), so the most specific relevant message wins per
// field.
func TestValidateEDSSplit_Invalid(t *testing.T) {
	tests := []struct {
		name                         string
		essentials, desires, savings int32
		wantFields                   map[string]string
	}{
		{
			name:       "sum 99 names all three percentages",
			essentials: 50, desires: 30, savings: 19,
			wantFields: map[string]string{
				"essentialsPercent": "must sum to 100 with desires and savings",
				"desiresPercent":    "must sum to 100 with essentials and savings",
				"savingsPercent":    "must sum to 100 with essentials and desires",
			},
		},
		{
			name:       "sum 101 names all three percentages",
			essentials: 50, desires: 30, savings: 21,
			wantFields: map[string]string{
				"essentialsPercent": "must sum to 100 with desires and savings",
				"desiresPercent":    "must sum to 100 with essentials and savings",
				"savingsPercent":    "must sum to 100 with essentials and desires",
			},
		},
		{
			name:       "sum 0 names all three percentages",
			essentials: 0, desires: 0, savings: 0,
			wantFields: map[string]string{
				"essentialsPercent": "must sum to 100 with desires and savings",
				"desiresPercent":    "must sum to 100 with essentials and savings",
				"savingsPercent":    "must sum to 100 with essentials and desires",
			},
		},
		{
			name:       "negative essentials only names essentials",
			essentials: -10, desires: 60, savings: 50,
			wantFields: map[string]string{
				"essentialsPercent": "must be non-negative",
			},
		},
		{
			name:       "negative desires only names desires",
			essentials: 50, desires: -10, savings: 60,
			wantFields: map[string]string{
				"desiresPercent": "must be non-negative",
			},
		},
		{
			name:       "negative savings also surfaces sum mismatch for the others",
			essentials: 50, desires: 30, savings: -10,
			wantFields: map[string]string{
				"essentialsPercent": "must sum to 100 with desires and savings",
				"desiresPercent":    "must sum to 100 with essentials and savings",
				"savingsPercent":    "must be non-negative",
			},
		},
		{
			name:       "over 100 essentials with negative savings surfaces both",
			essentials: 110, desires: 0, savings: -10,
			wantFields: map[string]string{
				"essentialsPercent": "must not exceed 100",
				"savingsPercent":    "must be non-negative",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verr := ValidateEDSSplit(tt.essentials, tt.desires, tt.savings)
			require.NotNil(t, verr)
			assert.Equal(t, apierr.CodeValidation, verr.Code)
			assert.Equal(t, "validation failed", verr.Message)
			assert.Equal(t, tt.wantFields, verr.Fields)
		})
	}
}

// TestValidateEDSSplit_FieldsPopulated locks in that a multi-field validation
// failure carries every offending field in Fields (aggregated in one pass),
// not just a flat message or a single tier's violations.
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

	t.Run("multiple negatives and over-100 aggregate all violations", func(t *testing.T) {
		verr := ValidateEDSSplit(-10, -20, 130)
		require.NotNil(t, verr)
		assert.Equal(t, apierr.CodeValidation, verr.Code)
		assert.Equal(t, map[string]string{
			"essentialsPercent": "must be non-negative",
			"desiresPercent":    "must be non-negative",
			"savingsPercent":    "must not exceed 100",
		}, verr.Fields)
	})

	t.Run("over-100 essentials with negative savings surfaces both", func(t *testing.T) {
		verr := ValidateEDSSplit(110, 0, -10)
		require.NotNil(t, verr)
		assert.Equal(t, apierr.CodeValidation, verr.Code)
		assert.Equal(t, map[string]string{
			"essentialsPercent": "must not exceed 100",
			"savingsPercent":    "must be non-negative",
		}, verr.Fields)
	})

	t.Run("negative and over-100 with sum mismatch surfaces all three", func(t *testing.T) {
		verr := ValidateEDSSplit(-10, 110, 50) // sum = 150
		require.NotNil(t, verr)
		assert.Equal(t, apierr.CodeValidation, verr.Code)
		assert.Equal(t, map[string]string{
			"essentialsPercent": "must be non-negative",
			"desiresPercent":    "must not exceed 100",
			"savingsPercent":    "must sum to 100 with essentials and desires",
		}, verr.Fields)
	})
}
