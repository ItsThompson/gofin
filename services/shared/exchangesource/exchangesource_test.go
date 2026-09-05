package exchangesource

import "testing"

func TestIsValid(t *testing.T) {
	tests := []struct {
		source string
		valid  bool
	}{
		{Identity, true},
		{OpenExchangeRates, true},
		{Migration, true},
		{"unknown", false},
		{"", false},
		{"openexchangerates", false},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			if got := IsValid(tt.source); got != tt.valid {
				t.Errorf("IsValid(%q) = %v, want %v", tt.source, got, tt.valid)
			}
		})
	}
}

func TestValidSetContainsAllThreeSources(t *testing.T) {
	if len(Valid) != 3 {
		t.Fatalf("expected 3 valid sources, got %d", len(Valid))
	}
	for _, s := range []string{Identity, OpenExchangeRates, Migration} {
		if !Valid[s] {
			t.Errorf("expected %q in Valid set", s)
		}
	}
}