package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatCentsToDollars(t *testing.T) {
	tests := []struct {
		name   string
		cents  int64
		expect string
	}{
		{name: "typical amount", cents: 4599, expect: "45.99"},
		{name: "zero", cents: 0, expect: "0.00"},
		{name: "one cent", cents: 1, expect: "0.01"},
		{name: "exact dollar", cents: 10000, expect: "100.00"},
		{name: "large amount", cents: 1234567, expect: "12345.67"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, formatCentsToDollars(tt.cents))
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
