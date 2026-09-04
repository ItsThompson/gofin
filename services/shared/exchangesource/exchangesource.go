// Package exchangesource holds the exchange-rate snapshot source values: the
// single source of truth for the exchange_rate_source string used across the
// expense, fx, and datarights services. The expense service writes these into
// the immutable ledger; the fx service produces provider snapshots; the
// datarights export reads and normalizes them.
package exchangesource

const (
	Identity          = "identity"
	OpenExchangeRates = "open_exchange_rates"
	Migration         = "migration"
)
