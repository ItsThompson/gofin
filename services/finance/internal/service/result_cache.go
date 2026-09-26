package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	financecache "github.com/ItsThompson/gofin/services/finance/internal/cache"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/metrics"
)

const (
	defaultResultCacheLease = 2 * time.Minute
	operationPeriod         = "periods/current"
	operationSummary        = "summary"
	operationByTag          = "spending/by-tag"
	operationCumulative     = "spending/cumulative"
	operationComparison     = "spending/comparison"
	operationTrends         = "spending/trends"
	operationUpcoming       = "prorata/upcoming"
	operationHealth         = "health-score"
	operationHealthTrend    = "health-score/trend"
)

type ResultCacheConfig struct {
	Enabled         bool
	MaxEntries      int
	MaxBytes        int64
	MaxEntryBytes   int64
	MaxAge          time.Duration
	ValidationLease time.Duration
}

func DefaultResultCacheConfig() ResultCacheConfig {
	config := financecache.DefaultConfig()
	return ResultCacheConfig{
		Enabled:         config.Enabled,
		MaxEntries:      config.MaxEntries,
		MaxBytes:        config.MaxBytes,
		MaxEntryBytes:   config.MaxEntryBytes,
		MaxAge:          config.MaxAge,
		ValidationLease: defaultResultCacheLease,
	}
}

type cachedResult[T any] struct {
	Value       T
	Dependency  *ExpenseRevision
	ValidatedAt time.Time
	ExpiresAt   time.Time
}

type financeResultCaches struct {
	enabled     bool
	lease       time.Duration
	maxAge      time.Duration
	periods     *financecache.Cache[string, *cachedResult[*model.BudgetPeriod]]
	summary     *financecache.Cache[string, *cachedResult[*model.PeriodSummary]]
	byTag       *financecache.Cache[string, *cachedResult[[]model.TagSpending]]
	cumulative  *financecache.Cache[string, *cachedResult[[]model.CumulativeSpendPoint]]
	comparison  *financecache.Cache[string, *cachedResult[*model.HistoricalComparison]]
	trends      *financecache.Cache[string, *cachedResult[[]model.TrendPoint]]
	upcoming    *financecache.Cache[string, *cachedResult[[]*model.ProRataSchedule]]
	health      *financecache.Cache[string, *cachedResult[*model.HealthScore]]
	healthTrend *financecache.Cache[string, *cachedResult[[]model.HealthScoreTrendPoint]]
}

func newFinanceResultCaches(config ResultCacheConfig, now func() time.Time) *financeResultCaches {
	if now == nil {
		now = time.Now
	}
	defaults := DefaultResultCacheConfig()
	if config.MaxEntries <= 0 {
		config.MaxEntries = defaults.MaxEntries
	}
	if config.MaxBytes <= 0 {
		config.MaxBytes = defaults.MaxBytes
	}
	if config.MaxEntryBytes <= 0 {
		config.MaxEntryBytes = defaults.MaxEntryBytes
	}
	if config.MaxAge <= 0 {
		config.MaxAge = defaults.MaxAge
	}
	if config.ValidationLease <= 0 {
		config.ValidationLease = defaults.ValidationLease
	}
	cacheConfig := financecache.Config{
		Enabled:       config.Enabled,
		MaxEntries:    config.MaxEntries,
		MaxBytes:      config.MaxBytes,
		MaxEntryBytes: config.MaxEntryBytes,
		MaxAge:        config.MaxAge,
	}
	return &financeResultCaches{
		enabled: config.Enabled,
		lease:   config.ValidationLease,
		maxAge:  config.MaxAge,
		periods: financecache.New[string, *cachedResult[*model.BudgetPeriod]](cacheConfig, now, func(value *cachedResult[*model.BudgetPeriod]) *cachedResult[*model.BudgetPeriod] {
			return cloneCachedResult(value, cloneBudgetPeriod)
		}, jsonSize[*cachedResult[*model.BudgetPeriod]]),
		summary: financecache.New[string, *cachedResult[*model.PeriodSummary]](cacheConfig, now, func(value *cachedResult[*model.PeriodSummary]) *cachedResult[*model.PeriodSummary] {
			return cloneCachedResult(value, clonePeriodSummary)
		}, jsonSize[*cachedResult[*model.PeriodSummary]]),
		byTag: financecache.New[string, *cachedResult[[]model.TagSpending]](cacheConfig, now, func(value *cachedResult[[]model.TagSpending]) *cachedResult[[]model.TagSpending] {
			return cloneCachedResult(value, cloneTagSpending)
		}, jsonSize[*cachedResult[[]model.TagSpending]]),
		cumulative: financecache.New[string, *cachedResult[[]model.CumulativeSpendPoint]](cacheConfig, now, func(value *cachedResult[[]model.CumulativeSpendPoint]) *cachedResult[[]model.CumulativeSpendPoint] {
			return cloneCachedResult(value, cloneCumulativeSpend)
		}, jsonSize[*cachedResult[[]model.CumulativeSpendPoint]]),
		comparison: financecache.New[string, *cachedResult[*model.HistoricalComparison]](cacheConfig, now, func(value *cachedResult[*model.HistoricalComparison]) *cachedResult[*model.HistoricalComparison] {
			return cloneCachedResult(value, cloneHistoricalComparison)
		}, jsonSize[*cachedResult[*model.HistoricalComparison]]),
		trends: financecache.New[string, *cachedResult[[]model.TrendPoint]](cacheConfig, now, func(value *cachedResult[[]model.TrendPoint]) *cachedResult[[]model.TrendPoint] {
			return cloneCachedResult(value, cloneTrendPoints)
		}, jsonSize[*cachedResult[[]model.TrendPoint]]),
		upcoming: financecache.New[string, *cachedResult[[]*model.ProRataSchedule]](cacheConfig, now, func(value *cachedResult[[]*model.ProRataSchedule]) *cachedResult[[]*model.ProRataSchedule] {
			return cloneCachedResult(value, cloneProRataSchedules)
		}, jsonSize[*cachedResult[[]*model.ProRataSchedule]]),
		health: financecache.New[string, *cachedResult[*model.HealthScore]](cacheConfig, now, func(value *cachedResult[*model.HealthScore]) *cachedResult[*model.HealthScore] {
			return cloneCachedResult(value, cloneHealthScore)
		}, jsonSize[*cachedResult[*model.HealthScore]]),
		healthTrend: financecache.New[string, *cachedResult[[]model.HealthScoreTrendPoint]](cacheConfig, now, func(value *cachedResult[[]model.HealthScoreTrendPoint]) *cachedResult[[]model.HealthScoreTrendPoint] {
			return cloneCachedResult(value, cloneHealthTrendPoints)
		}, jsonSize[*cachedResult[[]model.HealthScoreTrendPoint]]),
	}
}

func jsonSize[T any](value T) (int64, bool) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return 0, false
	}
	return int64(len(encoded)), true
}

func periodResultKey(userID string, year, month int32) string {
	return strings.Join([]string{operationPeriod, userID, strconv.FormatInt(int64(year), 10), strconv.FormatInt(int64(month), 10)}, "|")
}

func dashboardResultKey(operation, userID string, year, month int32) string {
	return strings.Join([]string{operation, userID, strconv.FormatInt(int64(year), 10), strconv.FormatInt(int64(month), 10)}, "|")
}

func trendResultKey(operation, userID string, year, month, months int32) string {
	return strings.Join([]string{operation, userID, strconv.FormatInt(int64(year), 10), strconv.FormatInt(int64(month), 10), strconv.FormatInt(int64(months), 10)}, "|")
}

func upcomingResultKey(userID string) string {
	return operationUpcoming + "|" + userID
}

func cacheExpiry(now time.Time, maxAge time.Duration, year, month int32, operation string) time.Time {
	expiresAt := now.Add(maxAge)
	if operation != operationSummary && operation != operationHealth {
		return expiresAt
	}
	if int32(now.Year()) != year || int32(now.Month()) != month {
		return expiresAt
	}
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	if operation == operationSummary {
		nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		if nextDay.Before(expiresAt) {
			return nextDay
		}
	}
	if nextMonth.Before(expiresAt) {
		return nextMonth
	}
	return expiresAt
}

func recordFinanceCacheEvent(operation string, status financecache.LoadStatus) {
	metrics.FinanceResultCacheEventsTotal.WithLabelValues(operation, string(status)).Inc()
}

func (s *FinanceService) expenseRevision(ctx context.Context, userID string) (ExpenseRevision, bool, error) {
	client, ok := s.expenseClient.(interface {
		GetExpenseRevision(context.Context, string) (ExpenseRevision, error)
	})
	if !ok {
		return ExpenseRevision{}, false, fmt.Errorf("expense revision client unavailable")
	}
	revision, err := client.GetExpenseRevision(ctx, userID)
	if err != nil {
		return ExpenseRevision{}, true, fmt.Errorf("validating expense revision: %w", err)
	}
	return revision, true, nil
}

func sameExpenseRevision(left, right ExpenseRevision) bool {
	return left.Epoch == right.Epoch && left.Revision == right.Revision
}

func financeUserCacheKeyMatches(userID string) func(string) bool {
	return func(key string) bool {
		return strings.Contains(key, "|"+userID+"|") || strings.HasSuffix(key, "|"+userID)
	}
}

func (s *FinanceService) invalidateFinanceExpenseUser(userID string) {
	match := financeUserCacheKeyMatches(userID)
	s.resultCaches.summary.Purge(match)
	s.resultCaches.byTag.Purge(match)
	s.resultCaches.cumulative.Purge(match)
	s.resultCaches.comparison.Purge(match)
	s.resultCaches.trends.Purge(match)
	s.resultCaches.health.Purge(match)
	s.resultCaches.healthTrend.Purge(match)
}

func (s *FinanceService) invalidateFinanceUser(userID string) {
	match := financeUserCacheKeyMatches(userID)
	s.resultCaches.periods.Purge(match)
	s.invalidateFinanceExpenseUser(userID)
	s.resultCaches.upcoming.Purge(match)
}

type financeMutationFence struct {
	service     *FinanceService
	userID      string
	writeActive bool
}

func (s *FinanceService) mutationFence(userID string) *financeMutationFence {
	return &financeMutationFence{service: s, userID: userID}
}

func (f *financeMutationFence) markWrite() {
	f.writeActive = true
}

func (f *financeMutationFence) finalize() {
	if f.writeActive {
		f.service.invalidateFinanceUser(f.userID)
	}
}

func loadResult[T any](s *FinanceService, ctx context.Context, operation, key string, store *financecache.Cache[string, *cachedResult[T]], expiresAt time.Time, dependent bool, source func(context.Context) (T, error)) (T, error) {
	return loadCachedResult(s, ctx, operation, key, store, func(loadCtx context.Context) (*cachedResult[T], error) {
		var revision ExpenseRevision
		var available bool
		var err error
		if dependent && s.validateRevisions {
			revision, available, err = s.expenseRevision(loadCtx, userIDFromResultKey(key))
			if err != nil {
				return nil, err
			}
		}
		value, err := source(loadCtx)
		if err != nil {
			return nil, err
		}
		if dependent && s.validateRevisions && available {
			current, _, revisionErr := s.expenseRevision(loadCtx, userIDFromResultKey(key))
			if revisionErr != nil {
				return nil, revisionErr
			}
			if !sameExpenseRevision(revision, current) {
				s.invalidateFinanceExpenseUser(userIDFromResultKey(key))
				return nil, fmt.Errorf("expense revision changed during %s load", operation)
			}
			return &cachedResult[T]{Value: value, Dependency: &revision, ValidatedAt: s.nowFunc(), ExpiresAt: expiresAt}, nil
		}
		return &cachedResult[T]{Value: value, ExpiresAt: expiresAt}, nil
	})
}

func loadCachedResult[T any](s *FinanceService, ctx context.Context, operation, key string, store *financecache.Cache[string, *cachedResult[T]], source func(context.Context) (*cachedResult[T], error)) (T, error) {
	var zero T
	if !s.resultCaches.enabled {
		result, err := source(ctx)
		if err != nil {
			return zero, err
		}
		return result.Value, nil
	}

	if cached, generation, ok := store.Peek(key); ok {
		now := s.nowFunc()
		if !now.Before(cached.ExpiresAt) {
			store.Evict(key)
		} else if cached.Dependency == nil || now.Sub(cached.ValidatedAt) < s.resultCaches.lease {
			recordFinanceCacheEvent(operation, financecache.StatusHit)
			return cached.Value, nil
		} else {
			revision, available, err := s.expenseRevision(ctx, userIDFromResultKey(key))
			if err != nil {
				store.Evict(key)
				return zero, err
			}
			if !available || !sameExpenseRevision(*cached.Dependency, revision) {
				s.invalidateFinanceExpenseUser(userIDFromResultKey(key))
			} else {
				cached.ValidatedAt = now
				if store.Refresh(key, cached, generation) {
					recordFinanceCacheEvent(operation, financecache.StatusHit)
					return cached.Value, nil
				}
			}
		}
	}

	result, status, err := store.Load(ctx, key, financecache.LoadOptions{}, source)
	if err != nil {
		recordFinanceCacheEvent(operation, financecache.StatusError)
		return zero, err
	}
	recordFinanceCacheEvent(operation, status)
	return result.Value, nil
}

func userIDFromResultKey(key string) string {
	parts := strings.Split(key, "|")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
