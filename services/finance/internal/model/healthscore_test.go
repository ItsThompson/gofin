package model

import (
	"encoding/json"
	"testing"
)

func TestBand_Boundaries(t *testing.T) {
	tests := []struct {
		total int32
		want  string
	}{
		{100, HealthBandGreen},
		{80, HealthBandGreen},
		{79, HealthBandAmber},
		{55, HealthBandAmber},
		{54, HealthBandRed},
		{0, HealthBandRed},
	}
	for _, tt := range tests {
		if got := Band(tt.total); got != tt.want {
			t.Errorf("Band(%d) = %q, want %q", tt.total, got, tt.want)
		}
	}
}

func TestHealthScore_MarshalsToTicketShape(t *testing.T) {
	score := &HealthScore{
		Year:           2026,
		Month:          7,
		Total:          72,
		Band:           HealthBandAmber,
		Provisional:    true,
		FormulaVersion: FormulaVersion,
		Components: []HealthComponent{
			{Key: HealthKeySavings, Score: 21, Max: 30, Detail: "Saved $420 of $600 target"},
			{Key: HealthKeyBudget, Score: 24, Max: 30, Detail: "Spent $2,480 of $2,400 plan"},
			{Key: HealthKeyAllocation, Score: 27, Max: 40, Detail: "Desires 8 pts over target share"},
		},
		Insight: HealthInsight{
			Summary: "Savings is the softest score this month.",
			Driver:  HealthKeySavings,
			Nudge:   "Move an extra $180 to savings to reach your target and lift your score about 9 points.",
		},
	}

	raw, err := json.Marshal(HealthScoreResponse{HealthScore: score})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hs, ok := decoded["healthScore"].(map[string]any)
	if !ok {
		t.Fatalf("response missing healthScore object: %s", raw)
	}

	for _, key := range []string{"year", "month", "total", "band", "provisional", "formulaVersion", "components", "insight"} {
		if _, ok := hs[key]; !ok {
			t.Errorf("healthScore JSON missing key %q: %s", key, raw)
		}
	}

	// configureBudget must be omitted for a scored response (omitempty).
	if _, present := hs["configureBudget"]; present {
		t.Errorf("configureBudget should be omitted for a scored response: %s", raw)
	}

	insight, ok := hs["insight"].(map[string]any)
	if !ok {
		t.Fatalf("insight is not an object: %s", raw)
	}
	for _, key := range []string{"summary", "driver", "nudge"} {
		if _, ok := insight[key]; !ok {
			t.Errorf("insight JSON missing key %q: %s", key, raw)
		}
	}
}

func TestHealthScore_ConfigureBudgetShape(t *testing.T) {
	raw, err := json.Marshal(HealthScoreResponse{
		HealthScore: &HealthScore{Year: 2026, Month: 7, ConfigureBudget: true},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	hs := decoded["healthScore"].(map[string]any)

	if hs["configureBudget"] != true {
		t.Errorf("expected configureBudget true, got %v: %s", hs["configureBudget"], raw)
	}
}
