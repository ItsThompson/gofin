package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/ItsThompson/gofin/services/expense/internal/cache"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/metrics"
)

// ReadCacheConfig controls the three expense-owned read caches.
type ReadCacheConfig = cache.Config

func defaultReadCacheConfig() ReadCacheConfig {
	return cache.DefaultConfig()
}

const (
	expenseReadOperationComplete    = "complete_period"
	expenseReadOperationRecent      = "recent_expenses"
	expenseReadOperationSuggestions = "suggestion_inputs"
)

func recordExpenseReadCacheEvent(operation string, status cache.LoadStatus) {
	metrics.ExpenseReadCacheEventsTotal.WithLabelValues(operation, string(status)).Inc()
}

type expenseReadCaches struct {
	completePeriod *cache.Cache[string, *model.CompleteExpensePageResponse]
	recent         *cache.Cache[string, *model.ExpenseListResponse]
	suggestions    *cache.Cache[string, []*model.ExpenseSuggestionInput]
}

func newExpenseReadCaches(config ReadCacheConfig, now func() time.Time) *expenseReadCaches {
	return &expenseReadCaches{
		completePeriod: cache.New[string, *model.CompleteExpensePageResponse](config, now, cloneCompleteExpensePage, jsonSize[*model.CompleteExpensePageResponse]),
		recent:         cache.New[string, *model.ExpenseListResponse](config, now, cloneExpenseListResponse, jsonSize[*model.ExpenseListResponse]),
		suggestions:    cache.New[string, []*model.ExpenseSuggestionInput](config, now, cloneSuggestionInputs, jsonSize[[]*model.ExpenseSuggestionInput]),
	}
}

func jsonSize[T any](value T) (int64, bool) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return 0, false
	}
	return int64(len(encoded)), true
}

func completePeriodCacheKey(req *model.GetActiveExpensesForPeriodPageRequest, pageSize int32) string {
	return "expense.complete_period|" + req.UserID + "|" + strconv.FormatInt(int64(req.Year), 10) + "|" + strconv.FormatInt(int64(req.Month), 10) + "|" + strconv.FormatInt(int64(pageSize), 10) + "|" + req.CursorExpenseDate + "|" + req.CursorCreatedAt + "|" + req.CursorID
}

func recentExpensesCacheKey(req *model.GetExpensesRequest, page, pageSize int32) string {
	return "expense.recent_expenses|" + req.UserID + "|" + strconv.FormatInt(int64(req.Year), 10) + "|" + strconv.FormatInt(int64(req.Month), 10) + "|" + strconv.FormatInt(int64(page), 10) + "|" + strconv.FormatInt(int64(pageSize), 10)
}

func suggestionInputsCacheKey(userID string) string {
	return "expense.suggestion_inputs|" + userID
}

func (c *expenseReadCaches) purgeUser(userID string) {
	c.completePeriod.Purge(func(key string) bool {
		return strings.HasPrefix(key, "expense.complete_period|"+userID+"|")
	})
	c.recent.Purge(func(key string) bool {
		return strings.HasPrefix(key, "expense.recent_expenses|"+userID+"|")
	})
	c.suggestions.Evict(suggestionInputsCacheKey(userID))
}

func cloneCompleteExpensePage(value *model.CompleteExpensePageResponse) *model.CompleteExpensePageResponse {
	if value == nil {
		return nil
	}
	return &model.CompleteExpensePageResponse{
		Data:            cloneExpenses(value.Data),
		NextExpenseDate: value.NextExpenseDate,
		NextCreatedAt:   value.NextCreatedAt,
		NextID:          value.NextID,
		HasMore:         value.HasMore,
	}
}

func cloneExpenseListResponse(value *model.ExpenseListResponse) *model.ExpenseListResponse {
	if value == nil {
		return nil
	}
	return &model.ExpenseListResponse{
		Data:     cloneExpenses(value.Data),
		Total:    value.Total,
		Page:     value.Page,
		PageSize: value.PageSize,
		HasMore:  value.HasMore,
	}
}

func cloneExpenses(expenses []*model.Expense) []*model.Expense {
	if expenses == nil {
		return nil
	}
	cloned := make([]*model.Expense, len(expenses))
	for index, expense := range expenses {
		if expense == nil {
			continue
		}
		copyExpense := *expense
		cloned[index] = &copyExpense
	}
	return cloned
}

func cloneSuggestionInputs(inputs []*model.ExpenseSuggestionInput) []*model.ExpenseSuggestionInput {
	if inputs == nil {
		return nil
	}
	cloned := make([]*model.ExpenseSuggestionInput, len(inputs))
	for index, input := range inputs {
		if input == nil {
			continue
		}
		copyInput := *input
		cloned[index] = &copyInput
	}
	return cloned
}
