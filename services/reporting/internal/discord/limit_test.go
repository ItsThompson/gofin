package discord

import (
	"strings"
	"testing"
)

func TestValidateRuneLimitCountsUnicodeRunes(t *testing.T) {
	if RuneCount(strings.Repeat("界", MaxReportRunes)) != MaxReportRunes {
		t.Fatal("RuneCount must count runes")
	}
	if err := ValidateRuneLimit(strings.Repeat("界", MaxReportRunes+1)); err == nil {
		t.Fatal("expected oversized report to fail")
	}
}
