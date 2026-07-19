package service

import (
	"strconv"
	"strings"
)

// currencySymbols maps the currency codes gofin displays with a glyph. Any code
// outside this set renders as the code plus a trailing space (e.g. "CAD 12").
// This mirrors getCurrencySymbol on the frontend; the small cross-language
// duplication is unavoidable because the server formats the insight strings.
var currencySymbols = map[string]string{
	"USD": "$",
	"EUR": "€",
	"GBP": "£",
	"JPY": "¥",
}

// currencySymbol resolves a display symbol for a currency code.
func currencySymbol(code string) string {
	if symbol, ok := currencySymbols[code]; ok {
		return symbol
	}
	return code + " "
}

// formatMoney renders a minor-unit (cents) amount for an insight string. Whole
// dollars carry thousands separators and no decimals ($2,480); a non-whole
// amount shows two decimals ($12.34). Matches the ticket examples ($420, $2,480).
func formatMoney(cents int64, symbol string) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}

	dollars := cents / 100
	remainder := cents % 100

	formatted := symbol + withThousandsSeparators(dollars)
	if remainder != 0 {
		formatted += "." + twoDigits(remainder)
	}
	if negative {
		return "-" + formatted
	}
	return formatted
}

// twoDigits renders a cents remainder in [0,99] as a zero-padded 2-digit string.
func twoDigits(remainder int64) string {
	if remainder < 10 {
		return "0" + strconv.FormatInt(remainder, 10)
	}
	return strconv.FormatInt(remainder, 10)
}

// withThousandsSeparators inserts commas into a non-negative integer's digits.
func withThousandsSeparators(n int64) string {
	digits := strconv.FormatInt(n, 10)
	if len(digits) <= 3 {
		return digits
	}

	var builder strings.Builder
	lead := len(digits) % 3
	if lead > 0 {
		builder.WriteString(digits[:lead])
		builder.WriteByte(',')
	}
	for i := lead; i < len(digits); i += 3 {
		builder.WriteString(digits[i : i+3])
		if i+3 < len(digits) {
			builder.WriteByte(',')
		}
	}
	return builder.String()
}
