package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

const maxSuggestionPageSize int32 = 100

type suggestionGroup struct {
	latest          *model.ExpenseSuggestionInput
	nonProRataCount int32
	proRataGroups   map[string]bool
}

// GetExpenseSuggestions returns active-only, exact-name suggestions for a user.
func (s *ExpenseService) GetExpenseSuggestions(ctx context.Context, req *model.ExpenseSuggestionRequest) (*model.ExpenseSuggestionListResponse, error) {
	if req.UserID == "" {
		return nil, apierr.Validation("user_id is required", nil)
	}
	if req.Page < 1 {
		return nil, apierr.Validation("page must be positive", nil)
	}
	if req.PageSize < 1 || req.PageSize > maxSuggestionPageSize {
		return nil, apierr.Validation("pageSize must be between 1 and 100", nil)
	}

	inputs, err := s.repo.GetActiveExpenseSuggestionInputs(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("getting active expense suggestion inputs: %w", err)
	}

	suggestions := s.aggregateExpenseSuggestions(inputs)
	total := int64(len(suggestions))
	start := int((req.Page - 1) * req.PageSize)
	if start >= len(suggestions) {
		return &model.ExpenseSuggestionListResponse{Data: []*model.ExpenseSuggestion{}, Total: total, Page: req.Page, PageSize: req.PageSize, HasMore: false}, nil
	}

	end := start + int(req.PageSize)
	if end > len(suggestions) {
		end = len(suggestions)
	}

	return &model.ExpenseSuggestionListResponse{
		Data:     suggestions[start:end],
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		HasMore:  int64(end) < total,
	}, nil
}

func (s *ExpenseService) aggregateExpenseSuggestions(inputs []*model.ExpenseSuggestionInput) []*model.ExpenseSuggestion {
	groups := make(map[string]*suggestionGroup)
	for _, input := range inputs {
		group := groups[input.Name]
		if group == nil {
			group = &suggestionGroup{proRataGroups: make(map[string]bool)}
			groups[input.Name] = group
		}

		if group.latest == nil || isSuggestionInputNewer(input, group.latest) {
			group.latest = input
		}
		if input.IsProRata && input.ProRataGroup != "" {
			group.proRataGroups[input.ProRataGroup] = true
			continue
		}
		group.nonProRataCount++
	}

	suggestions := make([]*model.ExpenseSuggestion, 0, len(groups))
	for name, group := range groups {
		frequency := group.nonProRataCount + int32(len(group.proRataGroups))
		bucket, weight := recencyBucket(group.latest.CreatedAt, s.clock())
		suggestions = append(suggestions, &model.ExpenseSuggestion{
			Name:                name,
			OriginalTransactionAmountInMinorUnits:   group.latest.TransactionAmount,
			TransactionCurrencyCode: group.latest.TransactionCurrency,
			ExpenseType:         group.latest.ExpenseType,
			TagID:               group.latest.TagID,
			Frequency:           frequency,
			LastUsedAt:          group.latest.CreatedAt,
			RecencyBucket:       bucket,
			FrecencyScore:       float64(frequency * weight),
		})
	}

	sort.SliceStable(suggestions, func(i, j int) bool {
		left := suggestions[i]
		right := suggestions[j]
		if left.FrecencyScore != right.FrecencyScore {
			return left.FrecencyScore > right.FrecencyScore
		}
		if left.LastUsedAt != right.LastUsedAt {
			return left.LastUsedAt > right.LastUsedAt
		}
		return left.Name < right.Name
	})

	return suggestions
}

func isSuggestionInputNewer(candidate *model.ExpenseSuggestionInput, current *model.ExpenseSuggestionInput) bool {
	if candidate.CreatedAt != current.CreatedAt {
		return candidate.CreatedAt > current.CreatedAt
	}
	return candidate.ID > current.ID
}

func recencyBucket(createdAt string, now time.Time) (string, int32) {
	usedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return "older", 1
	}

	elapsed := now.UTC().Sub(usedAt.UTC())
	if usedAt.UTC().Format("2006-01-02") == now.UTC().Format("2006-01-02") {
		return "today", 8
	}
	if elapsed <= 7*24*time.Hour {
		return "last_7_days", 4
	}
	if elapsed <= 30*24*time.Hour {
		return "last_30_days", 2
	}
	return "older", 1
}
