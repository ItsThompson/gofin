package service

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// GetAllUserData retrieves all tags, budget periods, and default settings for a user.
// Returns empty slices (not errors) for users with no data beyond defaults.
// Used by the datarights service for GDPR data export.
//
// The three repository reads are independent, so they run concurrently under a
// bounded errgroup; each goroutine writes its own distinct variable (the writes
// are disjoint by construction, not shared slice slots). Nil-slice normalization
// and result assembly run after the g.Wait() barrier, so the output is identical
// to the serial version. The bound is dashboardFanoutLimit, shared with the
// dashboard fan-out for a uniform cap; this fixed 3-read fan-out never reaches it.
func (s *FinanceService) GetAllUserData(ctx context.Context, userID string) (*model.AllUserData, error) {
	var (
		tags     []*model.Tag
		periods  []*model.BudgetPeriod
		defaults *model.DefaultSettings
	)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(dashboardFanoutLimit)
	g.Go(func() error {
		v, err := s.repo.ListTags(gctx, userID)
		if err != nil {
			return fmt.Errorf("listing tags for export: %w", err)
		}
		tags = v
		return nil
	})
	g.Go(func() error {
		v, err := s.repo.ListPeriods(gctx, userID)
		if err != nil {
			return fmt.Errorf("listing periods for export: %w", err)
		}
		periods = v
		return nil
	})
	g.Go(func() error {
		v, err := s.repo.GetDefaults(gctx, userID)
		if err != nil {
			return fmt.Errorf("getting defaults for export: %w", err)
		}
		defaults = v
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
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
