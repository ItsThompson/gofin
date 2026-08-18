package model

// CurrencyData is one entry of the supported-currency catalog served by
// GET /api/finance/currencies.
type CurrencyData struct {
	Code            string `json:"code"`
	Symbol          string `json:"symbol"`
	Name            string `json:"name"`
	MinorUnitDigits int    `json:"minorUnitDigits"`
}

// CurrencyListResponse is the JSON body returned for listing supported currencies.
type CurrencyListResponse struct {
	Currencies []CurrencyData `json:"currencies"`
}
