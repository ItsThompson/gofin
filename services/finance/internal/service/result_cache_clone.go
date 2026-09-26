package service

import "github.com/ItsThompson/gofin/services/finance/internal/model"

func cloneCachedResult[T any](value *cachedResult[T], cloneValue func(T) T) *cachedResult[T] {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Value = cloneValue(value.Value)
	if value.Dependency != nil {
		dependency := *value.Dependency
		cloned.Dependency = &dependency
	}
	return &cloned
}

func cloneBudgetPeriod(value *model.BudgetPeriod) *model.BudgetPeriod {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func clonePeriodSummary(value *model.PeriodSummary) *model.PeriodSummary {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneTagSpending(value []model.TagSpending) []model.TagSpending {
	if value == nil {
		return nil
	}
	cloned := make([]model.TagSpending, len(value))
	copy(cloned, value)
	return cloned
}

func cloneCumulativeSpend(value []model.CumulativeSpendPoint) []model.CumulativeSpendPoint {
	if value == nil {
		return nil
	}
	cloned := make([]model.CumulativeSpendPoint, len(value))
	copy(cloned, value)
	return cloned
}

func cloneHistoricalComparison(value *model.HistoricalComparison) *model.HistoricalComparison {
	if value == nil {
		return nil
	}
	cloned := *value
	if value.RollingAverage != nil {
		rollingAverage := *value.RollingAverage
		cloned.RollingAverage = &rollingAverage
	}
	return &cloned
}

func cloneTrendPoints(value []model.TrendPoint) []model.TrendPoint {
	if value == nil {
		return nil
	}
	cloned := make([]model.TrendPoint, len(value))
	copy(cloned, value)
	return cloned
}

func cloneProRataSchedules(value []*model.ProRataSchedule) []*model.ProRataSchedule {
	if value == nil {
		return nil
	}
	cloned := make([]*model.ProRataSchedule, len(value))
	for index, schedule := range value {
		if schedule == nil {
			continue
		}
		copySchedule := *schedule
		if schedule.AppliedAt != nil {
			appliedAt := *schedule.AppliedAt
			copySchedule.AppliedAt = &appliedAt
		}
		if schedule.CapturedRateSnapshot != nil {
			snapshot := *schedule.CapturedRateSnapshot
			snapshot.RatesByCurrency = make(map[string]string, len(schedule.CapturedRateSnapshot.RatesByCurrency))
			for currency, rate := range schedule.CapturedRateSnapshot.RatesByCurrency {
				snapshot.RatesByCurrency[currency] = rate
			}
			copySchedule.CapturedRateSnapshot = &snapshot
		}
		cloned[index] = &copySchedule
	}
	return cloned
}

func cloneHealthScore(value *model.HealthScore) *model.HealthScore {
	if value == nil {
		return nil
	}
	cloned := *value
	if value.Components != nil {
		cloned.Components = make([]model.HealthComponent, len(value.Components))
		copy(cloned.Components, value.Components)
	}
	return &cloned
}

func cloneHealthTrendPoints(value []model.HealthScoreTrendPoint) []model.HealthScoreTrendPoint {
	if value == nil {
		return nil
	}
	cloned := make([]model.HealthScoreTrendPoint, len(value))
	copy(cloned, value)
	return cloned
}
