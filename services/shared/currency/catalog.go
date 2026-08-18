// Package currency holds the supported-currency catalog: the single source of
// truth for which currencies GoFin accepts. The finance service owns this
// catalog and serves it to the frontend through GET /api/finance/currencies.
package currency

// Definition describes one supported currency.
type Definition struct {
	Code            string
	Symbol          string
	Name            string
	MinorUnitDigits int
}

// supportedCurrencies is the complete catalog, in display order.
var supportedCurrencies = []Definition{
	{Code: "USD", Symbol: "$", Name: "US Dollar", MinorUnitDigits: 2},
	{Code: "EUR", Symbol: "€", Name: "Euro", MinorUnitDigits: 2},
	{Code: "GBP", Symbol: "£", Name: "British Pound", MinorUnitDigits: 2},
	{Code: "JPY", Symbol: "¥", Name: "Japanese Yen", MinorUnitDigits: 0},
	{Code: "CAD", Symbol: "C$", Name: "Canadian Dollar", MinorUnitDigits: 2},
	{Code: "AUD", Symbol: "A$", Name: "Australian Dollar", MinorUnitDigits: 2},
	{Code: "CHF", Symbol: "CHF", Name: "Swiss Franc", MinorUnitDigits: 2},
	{Code: "CNY", Symbol: "¥", Name: "Chinese Yuan", MinorUnitDigits: 2},
	{Code: "SGD", Symbol: "S$", Name: "Singapore Dollar", MinorUnitDigits: 2},
	{Code: "HKD", Symbol: "HK$", Name: "Hong Kong Dollar", MinorUnitDigits: 2},
}

// All returns a copy of the catalog in display order.
func All() []Definition {
	definitions := make([]Definition, len(supportedCurrencies))
	copy(definitions, supportedCurrencies)
	return definitions
}

// Get returns the definition for code. Codes are uppercase, and callers
// normalize input before calling (the finance service uppercases user input).
func Get(code string) (Definition, bool) {
	for _, definition := range supportedCurrencies {
		if definition.Code == code {
			return definition, true
		}
	}
	return Definition{}, false
}

// IsSupported reports whether code is a supported currency.
func IsSupported(code string) bool {
	_, ok := Get(code)
	return ok
}
