package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEDSSplit_Valid(t *testing.T) {
	tests := []struct {
		name                              string
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
			err := ValidateEDSSplit(tt.essentials, tt.desires, tt.savings)
			assert.NoError(t, err)
		})
	}
}

func TestValidateEDSSplit_Invalid(t *testing.T) {
	tests := []struct {
		name                              string
		essentials, desires, savings int32
		errContains                        string
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
			err := ValidateEDSSplit(tt.essentials, tt.desires, tt.savings)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}
