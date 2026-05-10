package providers

import (
	"fmt"
	"strconv"
)

// formatCentsToDollars converts an amount in cents to a decimal string with 2 decimal places.
func formatCentsToDollars(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100.0)
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
