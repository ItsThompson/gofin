package exchangesource

const (
	Identity          = "identity"
	OpenExchangeRates = "open_exchange_rates"
	Migration         = "migration"
)

// Valid is the set of allowed exchange_rate_source values.
var Valid = map[string]bool{
	Identity:          true,
	OpenExchangeRates: true,
	Migration:         true,
}

// IsValid reports whether source is one of the allowed exchange_rate_source
// values. It is used as a write-path invariant guard: the expense service
// rejects a money snapshot whose source is outside the valid set before
// writing to the ledger.
func IsValid(source string) bool {
	return Valid[source]
}