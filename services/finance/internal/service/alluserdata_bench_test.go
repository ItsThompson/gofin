package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// allUserDataReadLatency is a synthetic per-read upstream latency baked into the
// GetAllUserData benchmark so wall-clock reflects the fan-out shape (max of the
// three reads) rather than the serial sum. It is a benchmark knob only;
// correctness tests run the fake with zero delay.
const allUserDataReadLatency = 200 * time.Microsecond

// seedAllUserData returns a fake repo populated with a representative export
// payload: ten tags, a full year of budget periods, and default settings.
func seedAllUserData(delay time.Duration) *countingAllUserDataRepo {
	repo := newCountingAllUserDataRepo()
	repo.delay = delay
	for i := 0; i < 10; i++ {
		repo.tags = append(repo.tags, &model.Tag{
			ID:        fmt.Sprintf("tag-%02d", i),
			UserID:    "user-1",
			Name:      fmt.Sprintf("Tag %02d", i),
			IsDefault: i < 8,
		})
	}
	for m := int32(12); m >= 1; m-- {
		repo.periods = append(repo.periods, &model.BudgetPeriod{
			ID:                fmt.Sprintf("p-2026-%02d", m),
			UserID:            "user-1",
			Year:              2026,
			Month:             m,
			BudgetAmount:      300000,
			EssentialsPercent: 50,
			DesiresPercent:    30,
			SavingsPercent:    20,
		})
	}
	repo.defaults = &model.DefaultSettings{
		UserID:            "user-1",
		BudgetAmount:      300000,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
		Currency:          "GBP",
	}
	return repo
}

// BenchmarkGetAllUserData measures the 3-read export path (tags, periods,
// defaults). Primary signal: repo call count (exactly one each) and wall-clock
// (fan-out max vs serial sum). GetAllUserData never touches the expense client,
// so none is injected.
func BenchmarkGetAllUserData(b *testing.B) {
	repo := seedAllUserData(allUserDataReadLatency)
	svc := newFanoutService(repo, nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.GetAllUserData(context.Background(), "user-1"); err != nil {
			b.Fatal(err)
		}
	}
}
