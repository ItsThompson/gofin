package providers

import (
	"fmt"
	"strconv"

	sharedcurrency "github.com/ItsThompson/gofin/services/shared/currency"
)

// formatMinorUnits converts an integer minor-unit amount to a decimal string
// using the currency's minor-unit digit count. Zero-digit currencies such as
// JPY render as a plain integer with no forced decimal places.
func formatMinorUnits(amount int64, code string) (string, error) {
	definition, ok := sharedcurrency.Get(code)
	if !ok {
		return "", fmt.Errorf("unsupported currency %q", code)
	}
	return formatMinorUnitsWithDigits(amount, definition.MinorUnitDigits), nil
}

// formatMinorUnitsWithDigits scales an integer minor-unit amount into a decimal
// string with exactly the given number of fraction digits. Callers that already
// resolved a digit count (for example, the two-digit fallback for legacy
// default settings) use this directly.
func formatMinorUnitsWithDigits(amount int64, digits int) string {
	if digits == 0 {
		return strconv.FormatInt(amount, 10)
	}

	factor := int64(1)
	for range digits {
		factor *= 10
	}

	negative := amount < 0
	if negative {
		amount = -amount
	}

	whole := amount / factor
	fraction := amount % factor
	fractionStr := strconv.FormatInt(fraction, 10)
	for len(fractionStr) < digits {
		fractionStr = "0" + fractionStr
	}

	result := strconv.FormatInt(whole, 10) + "." + fractionStr
	if negative {
		return "-" + result
	}
	return result
}

// formatBool converts a boolean to "true" or "false" string.
func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// resolveTagName looks up a tag name by ID, returning "Unknown" for missing tags.
func resolveTagName(tagID string, tagMap map[string]string) string {
	if tagID == "" {
		return ""
	}
	if name, ok := tagMap[tagID]; ok {
		return name
	}
	return "Unknown"
}

// formatOptionalInt renders an int32 as a string only when the condition is true.
// Returns empty string when the condition is false (used for pro-rata fields on non-pro-rata expenses).
func formatOptionalInt(value int32, condition bool) string {
	if !condition {
		return ""
	}
	return strconv.FormatInt(int64(value), 10)
}
