package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
)

// DefaultTags are seeded when a user completes onboarding.
var DefaultTags = []string{
	"Bills",
	"Food",
	"Household",
	"Investment",
	"Personal Care",
	"Recreation/Entertainment",
	"Self Investment",
	"Social",
	"Transport",
	"Travel",
}

// FinanceService contains the business logic for finance operations.
type FinanceService struct {
	repo      repository.FinanceRepository
	txBeginner repository.TxBeginner
	logger    *slog.Logger
}

// NewFinanceService creates a new FinanceService.
func NewFinanceService(
	repo repository.FinanceRepository,
	txBeginner repository.TxBeginner,
	logger *slog.Logger,
) *FinanceService {
	return &FinanceService{
		repo:      repo,
		txBeginner: txBeginner,
		logger:    logger,
	}
}

// ServiceError is a typed error that carries an HTTP status code and error code.
type ServiceError struct {
	Code    string
	Message string
	Status  int
}

func (e *ServiceError) Error() string {
	return e.Message
}

// ValidateEDSSplit checks that essentials + desires + savings == 100.
func ValidateEDSSplit(essentials, desires, savings int32) error {
	if essentials < 0 || desires < 0 || savings < 0 {
		return fmt.Errorf("E/D/S percentages must be non-negative")
	}
	if essentials > 100 || desires > 100 || savings > 100 {
		return fmt.Errorf("E/D/S percentages must not exceed 100")
	}
	total := essentials + desires + savings
	if total != 100 {
		return fmt.Errorf("E/D/S split must sum to 100%%, got %d%%", total)
	}
	return nil
}

// CompleteOnboarding saves the user's default settings and seeds default tags.
// Both operations run in a transaction: if tag seeding fails, the defaults
// upsert is rolled back.
func (s *FinanceService) CompleteOnboarding(ctx context.Context, userID string, req *model.OnboardingRequest) (*model.DefaultSettings, error) {
	if err := ValidateEDSSplit(req.EssentialsPercent, req.DesiresPercent, req.SavingsPercent); err != nil {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: err.Error(),
			Status:  400,
		}
	}

	tx, err := s.txBeginner.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txRepo := tx.Repo()

	// Upsert default settings
	defaults, err := txRepo.UpsertDefaults(ctx, &model.DefaultSettings{
		UserID:            userID,
		BudgetAmount:      req.BudgetAmount,
		EssentialsPercent: req.EssentialsPercent,
		DesiresPercent:    req.DesiresPercent,
		SavingsPercent:    req.SavingsPercent,
		Currency:          req.Currency,
	})
	if err != nil {
		return nil, fmt.Errorf("upserting defaults: %w", err)
	}

	// Seed default tags
	for _, tagName := range DefaultTags {
		_, err := txRepo.CreateTag(ctx, userID, tagName, true)
		if err != nil {
			return nil, fmt.Errorf("seeding tag %q: %w", tagName, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	s.logger.Info("onboarding completed",
		slog.String("method", "CompleteOnboarding"),
		slog.String("user_id", userID),
	)

	return defaults, nil
}

// GetDefaults retrieves the user's default budget settings.
func (s *FinanceService) GetDefaults(ctx context.Context, userID string) (*model.DefaultSettings, error) {
	defaults, err := s.repo.GetDefaults(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting defaults: %w", err)
	}
	if defaults == nil {
		return nil, &ServiceError{
			Code:    model.ErrNotFound,
			Message: "Default settings not found",
			Status:  404,
		}
	}
	return defaults, nil
}

// GetCurrentPeriod retrieves the budget period for a given user, year, and month.
// Returns a PERIOD_NOT_FOUND ServiceError (404) when no period exists.
func (s *FinanceService) GetCurrentPeriod(ctx context.Context, userID string, year, month int32) (*model.BudgetPeriod, error) {
	if month < 1 || month > 12 {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "Month must be between 1 and 12",
			Status:  400,
		}
	}

	period, err := s.repo.GetCurrentPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("getting current period: %w", err)
	}
	if period == nil {
		return nil, &ServiceError{
			Code:    model.ErrPeriodNotFound,
			Message: fmt.Sprintf("No budget period found for %d-%02d", year, month),
			Status:  404,
		}
	}
	return period, nil
}

// CreatePeriod creates a new budget period for the given user.
// Validates the E/D/S split sums to 100% before persisting.
func (s *FinanceService) CreatePeriod(ctx context.Context, userID string, req *model.CreatePeriodRequest) (*model.BudgetPeriod, error) {
	if err := ValidateEDSSplit(req.EssentialsPercent, req.DesiresPercent, req.SavingsPercent); err != nil {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: err.Error(),
			Status:  400,
		}
	}

	if req.Month < 1 || req.Month > 12 {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "Month must be between 1 and 12",
			Status:  400,
		}
	}

	if req.BudgetAmount < 0 {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "Budget amount must be non-negative",
			Status:  400,
		}
	}

	period, err := s.repo.CreatePeriod(ctx, &model.BudgetPeriod{
		UserID:            userID,
		Year:              req.Year,
		Month:             req.Month,
		BudgetAmount:      req.BudgetAmount,
		EssentialsPercent: req.EssentialsPercent,
		DesiresPercent:    req.DesiresPercent,
		SavingsPercent:    req.SavingsPercent,
	})
	if err != nil {
		return nil, fmt.Errorf("creating period: %w", err)
	}

	s.logger.Info("budget period created",
		slog.String("method", "CreatePeriod"),
		slog.String("user_id", userID),
		slog.Int("year", int(req.Year)),
		slog.Int("month", int(req.Month)),
	)

	return period, nil
}
