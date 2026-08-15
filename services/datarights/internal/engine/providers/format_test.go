package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatMinorUnits(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		code   string
		expect string
	}{
		{name: "typical USD amount", amount: 4599, code: "USD", expect: "45.99"},
		{name: "zero", amount: 0, code: "USD", expect: "0.00"},
		{name: "one cent", amount: 1, code: "USD", expect: "0.01"},
		{name: "exact dollar", amount: 10000, code: "USD", expect: "100.00"},
		{name: "large amount", amount: 1234567, code: "USD", expect: "12345.67"},
		{name: "JPY has no forced decimals", amount: 1250, code: "JPY", expect: "1250"},
		{name: "JPY zero", amount: 0, code: "JPY", expect: "0"},
		{name: "negative amount", amount: -500, code: "USD", expect: "-5.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatMinorUnits(tt.amount, tt.code)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestFormatMinorUnits_UnsupportedCurrency(t *testing.T) {
	_, err := formatMinorUnits(100, "XXX")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported currency")
}

func TestFormatMinorUnitsWithDigits(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		digits int
		expect string
	}{
		{name: "two digits", amount: 4599, digits: 2, expect: "45.99"},
		{name: "zero digits", amount: 4599, digits: 0, expect: "4599"},
		{name: "three digits", amount: 12345, digits: 3, expect: "12.345"},
		{name: "pads fraction", amount: 5, digits: 3, expect: "0.005"},
		{name: "negative", amount: -500, digits: 2, expect: "-5.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, formatMinorUnitsWithDigits(tt.amount, tt.digits))
		})
	}
}

func TestFormatBool(t *testing.T) {
	assert.Equal(t, "true", formatBool(true))
	assert.Equal(t, "false", formatBool(false))
}

func TestResolveTagName(t *testing.T) {
	tagMap := map[string]string{
		"tag-1": "Food",
		"tag-2": "Transport",
	}

	tests := []struct {
		name   string
		tagID  string
		expect string
	}{
		{name: "known tag", tagID: "tag-1", expect: "Food"},
		{name: "another known tag", tagID: "tag-2", expect: "Transport"},
		{name: "missing tag falls back to Unknown", tagID: "tag-999", expect: "Unknown"},
		{name: "empty tag ID returns empty string", tagID: "", expect: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, resolveTagName(tt.tagID, tagMap))
		})
	}
}

func TestFormatOptionalInt(t *testing.T) {
	tests := []struct {
		name      string
		value     int32
		condition bool
		expect    string
	}{
		{name: "active condition with value", value: 3, condition: true, expect: "3"},
		{name: "inactive condition with value", value: 3, condition: false, expect: ""},
		{name: "active condition with zero", value: 0, condition: true, expect: "0"},
		{name: "inactive condition with zero", value: 0, condition: false, expect: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, formatOptionalInt(tt.value, tt.condition))
		})
	}
}
