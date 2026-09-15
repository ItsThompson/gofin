package discord

import (
	"fmt"
	"unicode/utf8"
)

const MaxReportRunes = 2000

func RuneCount(value string) int { return utf8.RuneCountInString(value) }

func ValidateRuneLimit(value string) error {
	if count := RuneCount(value); count > MaxReportRunes {
		return fmt.Errorf("report exceeds %d runes (%d)", MaxReportRunes, count)
	}
	return nil
}
