package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	"github.com/ItsThompson/gofin/services/shared/reporting"
	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
)

// ProductUsageMetrics contains one typed result for each finance metric.
type ProductUsageMetrics struct {
	BudgetPeriods    *reportingpb.CountResult
	Tags             *reportingpb.CountResult
	ProrataSchedules *reportingpb.CountResult
}

// ProductUsageService computes finance product-usage metrics for a shared set
// of reporting windows.
type ProductUsageService struct {
	repo repository.ProductUsageRepository
}

func NewProductUsageService(repo repository.ProductUsageRepository) *ProductUsageService {
	return &ProductUsageService{repo: repo}
}

// GetProductUsageMetrics runs each metric query independently. A query failure
// makes only that metric unavailable. Context cancellation and panics abort the
// whole operation because they do not describe a metric-specific query result.
func (s *ProductUsageService) GetProductUsageMetrics(ctx context.Context, windowSet *reportingpb.ReportWindowSet) (*ProductUsageMetrics, error) {
	if err := reporting.ValidateWindowSet(windowSet); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	type metricCall struct {
		name string
		call func() (repository.ProductUsageCounts, error)
	}
	type metricOutcome struct {
		name   string
		counts repository.ProductUsageCounts
		err    error
	}

	calls := []metricCall{
		{
			name: "budget_periods",
			call: func() (repository.ProductUsageCounts, error) {
				return s.repo.CountBudgetPeriods(ctx, windowSet)
			},
		},
		{
			name: "tags",
			call: func() (repository.ProductUsageCounts, error) {
				return s.repo.CountTags(ctx, windowSet)
			},
		},
		{
			name: "prorata_schedules",
			call: func() (repository.ProductUsageCounts, error) {
				return s.repo.CountProRataSchedules(ctx, windowSet)
			},
		},
	}

	outcomes := make([]metricOutcome, 0, len(calls))
	for _, metric := range calls {
		counts, err := callProductUsageMetric(metric.name, metric.call)
		outcomes = append(outcomes, metricOutcome{name: metric.name, counts: counts, err: err})
	}

	result := &ProductUsageMetrics{}
	for _, outcome := range outcomes {
		if outcome.err != nil {
			var panicErr *productUsagePanicError
			if errors.As(outcome.err, &panicErr) || errors.Is(outcome.err, context.Canceled) || errors.Is(outcome.err, context.DeadlineExceeded) {
				return nil, outcome.err
			}
			result.setUnavailable(outcome.name)
			continue
		}
		result.setAvailable(outcome.name, outcome.counts)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

type productUsagePanicError struct {
	metric string
	value  any
}

func (e *productUsagePanicError) Error() string {
	return fmt.Sprintf("product usage metric %s panicked: %v", e.metric, e.value)
}

func callProductUsageMetric(metric string, call func() (repository.ProductUsageCounts, error)) (counts repository.ProductUsageCounts, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			counts = repository.ProductUsageCounts{}
			err = &productUsagePanicError{metric: metric, value: recovered}
		}
	}()
	return call()
}

func (m *ProductUsageMetrics) setAvailable(metric string, counts repository.ProductUsageCounts) {
	result := &reportingpb.CountResult{
		State: &reportingpb.CountResult_Available{Available: &reportingpb.CountValues{
			ReportWeek:             counts.ReportWeek,
			PreviousWeek:           counts.PreviousWeek,
			TrailingFourWeeksTotal: counts.TrailingFourWeeksTotal,
		}},
	}
	m.set(metric, result)
}

func (m *ProductUsageMetrics) setUnavailable(metric string) {
	result := &reportingpb.CountResult{
		State: &reportingpb.CountResult_Unavailable{
			Unavailable: &reportingpb.MetricUnavailable{
				Code: reportingpb.MetricErrorCode_METRIC_ERROR_CODE_QUERY_FAILED,
			},
		},
	}
	m.set(metric, result)
}

func (m *ProductUsageMetrics) set(metric string, result *reportingpb.CountResult) {
	switch metric {
	case "budget_periods":
		m.BudgetPeriods = result
	case "tags":
		m.Tags = result
	case "prorata_schedules":
		m.ProrataSchedules = result
	}
}
