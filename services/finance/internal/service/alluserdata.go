package service

import (
	"context"
	"fmt"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// GetAllUserData retrieves all tags, budget periods, and default settings for a user.
// Returns empty slices (not errors) for users with no data beyond defaults.
// Used by the datarights service for GDPR data export.
func (s *FinanceService) GetAllUserData(ctx context.Context, userID string) (*model.AllUserData, error) {
	tags, err := s.repo.ListTags(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing tags for export: %w", err)
	}

	periods, err := s.repo.ListPeriods(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing periods for export: %w", err)
	}

	defaults, err := s.repo.GetDefaults(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting defaults for export: %w", err)
	}

	// Normalize nil slices to empty slices
	if tags == nil {
		tags = []*model.Tag{}
	}
	if periods == nil {
		periods = []*model.BudgetPeriod{}
	}

	return &model.AllUserData{
		Tags:     tags,
		Periods:  periods,
		Defaults: defaults,
	}, nil
}
