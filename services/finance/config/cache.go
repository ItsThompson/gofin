package config

import "time"

const (
	// FinanceValidationLease is the default lease for validating expense-dependent results.
	FinanceValidationLease = 2 * time.Minute

	// FinanceValidationTimeout bounds each expense revision RPC.
	FinanceValidationTimeout = 5 * time.Second

	// MaxExpenseFinanceEvictionTimeout keeps expense callbacks below every valid lease.
	MaxExpenseFinanceEvictionTimeout = time.Second

	// MaxFinanceResultCacheBytes leaves process headroom below the 512 MB finance limit.
	MaxFinanceResultCacheBytes int64 = 256 * 1024 * 1024
)
