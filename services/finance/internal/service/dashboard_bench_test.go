package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// benchReadLatency is a synthetic per-read upstream latency baked into the
// dashboard fan-out benchmarks so wall-clock reflects the fan-out shape
// (total ≈ max·ceil(n/limit)) rather than the serial sum. It is a benchmark knob
// only; correctness tests run the fake with zero delay.
const benchReadLatency = 200 * time.Microsecond

// seedYearPeriods seeds twelve 2026 budget periods (DESC order, mirroring the
// repository's ListPeriods contract) and per-period expenses on exp, returning
// the periods for the fake repo.
func seedYearPeriods(exp *countingExpenseClient) []*model.BudgetPeriod {
	periods := make([]*model.BudgetPeriod, 0, 12)
	for m := int32(12); m >= 1; m-- {
		periods = append(periods, &model.BudgetPeriod{
			ID:                fmt.Sprintf("p-2026-%02d", m),
			UserID:            "user-1",
			Year:              2026,
			Month:             m,
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		})
		exp.set(2026, m, []ExpenseData{
			{ReportingAmount: int64(m) * 1000, ExpenseType: "essentials"},
			{ReportingAmount: int64(m) * 500, ExpenseType: "desires"},
		})
	}
	return periods
}

// BenchmarkGetSpendingTrends measures the up-to-12-read trends path at window
// sizes 1, 6, and 12. Primary signal: expense call count (one read per non-nil
// period) and wall-clock (fan-out max vs serial sum).
func BenchmarkGetSpendingTrends(b *testing.B) {
	for _, months := range []int32{1, 6, 12} {
		b.Run(fmt.Sprintf("months=%d", months), func(b *testing.B) {
			exp := newCountingExpenseClient()
			exp.delay = benchReadLatency
			repo := &fakeFanoutRepo{periods: seedYearPeriods(exp)}
			svc := newFanoutService(repo, exp)

			b.ReportAllocs()
			for b.Loop() {
				if _, err := svc.GetSpendingTrends(context.Background(), "user-1", 2026, 12, months); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkGetHistoricalComparison measures the current + up-to-3-prior read
// path (≤4 reads). Primary signal: expense call count and wall-clock.
func BenchmarkGetHistoricalComparison(b *testing.B) {
	exp := newCountingExpenseClient()
	exp.delay = benchReadLatency
	periods := seedYearPeriods(exp)
	repo := &fakeFanoutRepo{
		periods: periods,
		current: map[[2]int32]*model.BudgetPeriod{periodKey(2026, 12): periods[0]},
	}
	svc := newFanoutService(repo, exp)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := svc.GetHistoricalComparison(context.Background(), "user-1", 2026, 12); err != nil {
			b.Fatal(err)
		}
	}
}
