package config

import "time"

const (
	// FinanceValidationLease is the default lease for validating expense-dependent results.
	FinanceValidationLease = 2 * time.Minute

	// MaxExpenseFinanceEvictionTimeout keeps expense callbacks below every valid lease.
	MaxExpenseFinanceEvictionTimeout = time.Second
)
