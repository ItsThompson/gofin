package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

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
	repo          repository.FinanceRepository
	txBeginner    repository.TxBeginner
	expenseClient ExpenseClient
	nowFunc       func() time.Time
	logger        *slog.Logger
}

// NewFinanceService creates a new FinanceService.
func NewFinanceService(
	repo repository.FinanceRepository,
	txBeginner repository.TxBeginner,
	logger *slog.Logger,
) *FinanceService {
	return &FinanceService{
		repo:       repo,
		txBeginner: txBeginner,
		nowFunc:    time.Now,
		logger:     logger,
	}
}

// WithExpenseClient returns a copy of the service with the expense client set.
// This is used to inject the gRPC client after the service is constructed.
func (s *FinanceService) WithExpenseClient(client ExpenseClient) *FinanceService {
	s.expenseClient = client
	return s
}

// WithNowFunc overrides the clock function used for time-dependent logic.
// Used in tests to inject a fixed time.
func (s *FinanceService) WithNowFunc(f func() time.Time) *FinanceService {
	s.nowFunc = f
	return s
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

// UpdateDefaults updates the user's default budget settings.
// Does not affect current or past budget periods.
func (s *FinanceService) UpdateDefaults(ctx context.Context, userID string, req *model.UpdateDefaultsRequest) (*model.DefaultSettings, error) {
	if err := ValidateEDSSplit(req.EssentialsPercent, req.DesiresPercent, req.SavingsPercent); err != nil {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: err.Error(),
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

	defaults, err := s.repo.UpsertDefaults(ctx, &model.DefaultSettings{
		UserID:            userID,
		BudgetAmount:      req.BudgetAmount,
		EssentialsPercent: req.EssentialsPercent,
		DesiresPercent:    req.DesiresPercent,
		SavingsPercent:    req.SavingsPercent,
		Currency:          req.Currency,
	})
	if err != nil {
		return nil, fmt.Errorf("updating defaults: %w", err)
	}

	s.logger.Info("defaults updated",
		slog.String("method", "UpdateDefaults"),
		slog.String("user_id", userID),
	)

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

// ListPeriods returns all budget periods for a user, ordered by year/month descending.
func (s *FinanceService) ListPeriods(ctx context.Context, userID string) ([]*model.BudgetPeriod, error) {
	periods, err := s.repo.ListPeriods(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing periods: %w", err)
	}
	return periods, nil
}

// UpdatePeriod updates the budget and E/D/S split for a period.
// Only the current period can be updated: past periods are immutable (PERIOD_LOCKED).
func (s *FinanceService) UpdatePeriod(ctx context.Context, userID, periodID string, req *model.UpdatePeriodRequest) (*model.BudgetPeriod, error) {
	if err := ValidateEDSSplit(req.EssentialsPercent, req.DesiresPercent, req.SavingsPercent); err != nil {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: err.Error(),
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

	// Fetch the period to check ownership and whether it's current.
	existing, err := s.repo.GetPeriodByID(ctx, periodID, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching period: %w", err)
	}
	if existing == nil {
		return nil, &ServiceError{
			Code:    model.ErrNotFound,
			Message: "Budget period not found",
			Status:  404,
		}
	}

	// Enforce: only the current month's period can be edited.
	now := s.nowFunc()
	if existing.Year != int32(now.Year()) || existing.Month != int32(now.Month()) {
		return nil, &ServiceError{
			Code:    model.ErrPeriodLocked,
			Message: "Past periods are read-only and cannot be modified",
			Status:  403,
		}
	}

	updated, err := s.repo.UpdatePeriod(ctx, &model.BudgetPeriod{
		ID:                periodID,
		UserID:            userID,
		BudgetAmount:      req.BudgetAmount,
		EssentialsPercent: req.EssentialsPercent,
		DesiresPercent:    req.DesiresPercent,
		SavingsPercent:    req.SavingsPercent,
	})
	if err != nil {
		return nil, fmt.Errorf("updating period: %w", err)
	}
	if updated == nil {
		return nil, &ServiceError{
			Code:    model.ErrNotFound,
			Message: "Budget period not found",
			Status:  404,
		}
	}

	s.logger.Info("budget period updated",
		slog.String("method", "UpdatePeriod"),
		slog.String("user_id", userID),
		slog.String("period_id", periodID),
	)

	return updated, nil
}

// ListTags returns all tags for a user, ordered alphabetically.
// If the user has no tags, default tags are lazy-seeded before returning.
func (s *FinanceService) ListTags(ctx context.Context, userID string) ([]*model.Tag, error) {
	count, err := s.repo.CountUserTags(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("counting user tags: %w", err)
	}

	if count == 0 {
		tx, err := s.txBeginner.BeginTx(ctx)
		if err != nil {
			return nil, fmt.Errorf("beginning transaction for tag seeding: %w", err)
		}
		defer tx.Rollback(ctx)

		txRepo := tx.Repo()
		for _, tagName := range DefaultTags {
			_, err := txRepo.CreateTag(ctx, userID, tagName, true)
			if err != nil {
				return nil, fmt.Errorf("lazy-seeding tag %q: %w", tagName, err)
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("committing tag seeding: %w", err)
		}

		s.logger.Info("lazy-seeded default tags",
			slog.String("method", "ListTags"),
			slog.String("user_id", userID),
		)
	}

	tags, err := s.repo.ListTags(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	return tags, nil
}

func (s *FinanceService) CreateTag(ctx context.Context, userID string, req *model.CreateTagRequest) (*model.Tag, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Tag name is required", Status: 400}
	}
	if utf8.RuneCountInString(name) > 50 {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Tag name must be 50 characters or fewer", Status: 400}
	}

	tag, err := s.repo.CreateTag(ctx, userID, name, false)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, &ServiceError{Code: model.ErrDuplicateTag, Message: fmt.Sprintf("A tag named %q already exists", name), Status: 409}
		}
		return nil, fmt.Errorf("creating tag: %w", err)
	}

	s.logger.Info("tag created", slog.String("method", "CreateTag"), slog.String("user_id", userID), slog.String("tag_name", name))
	return tag, nil
}

func (s *FinanceService) UpdateTag(ctx context.Context, userID, tagID string, req *model.UpdateTagRequest) (*model.Tag, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Tag name is required", Status: 400}
	}
	if utf8.RuneCountInString(name) > 50 {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Tag name must be 50 characters or fewer", Status: 400}
	}

	tag, err := s.repo.UpdateTag(ctx, tagID, userID, name)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, &ServiceError{Code: model.ErrDuplicateTag, Message: fmt.Sprintf("A tag named %q already exists", name), Status: 409}
		}
		return nil, fmt.Errorf("updating tag: %w", err)
	}
	if tag == nil {
		return nil, &ServiceError{Code: model.ErrNotFound, Message: "Tag not found", Status: 404}
	}

	s.logger.Info("tag updated", slog.String("method", "UpdateTag"), slog.String("user_id", userID), slog.String("tag_id", tagID))
	return tag, nil
}

func (s *FinanceService) DeleteTag(ctx context.Context, userID, tagID string) error {
	tag, err := s.repo.GetTag(ctx, tagID, userID)
	if err != nil {
		return fmt.Errorf("getting tag: %w", err)
	}
	if tag == nil {
		return &ServiceError{Code: model.ErrNotFound, Message: "Tag not found", Status: 404}
	}
	if tag.IsDefault {
		return &ServiceError{Code: model.ErrDefaultTag, Message: "Default tags cannot be deleted, only renamed", Status: 403}
	}

	var expenseCount int64
	if s.expenseClient != nil {
		expenseCount, err = s.expenseClient.CountExpensesByTag(ctx, userID, tagID)
		if err != nil {
			return fmt.Errorf("checking tag usage in expenses: %w", err)
		}
	}

	proRataCount, err := s.repo.CountTagInProRata(ctx, tagID, userID)
	if err != nil {
		return fmt.Errorf("checking tag usage in pro-rata schedules: %w", err)
	}

	if expenseCount > 0 || proRataCount > 0 {
		parts := make([]string, 0, 2)
		if expenseCount > 0 {
			parts = append(parts, fmt.Sprintf("%d expense(s)", expenseCount))
		}
		if proRataCount > 0 {
			parts = append(parts, fmt.Sprintf("%d pending schedule(s)", proRataCount))
		}
		return &ServiceError{Code: model.ErrTagInUse, Message: fmt.Sprintf("Tag is referenced by %s", strings.Join(parts, " and ")), Status: 409}
	}

	if err := s.repo.DeleteTag(ctx, tagID, userID); err != nil {
		return fmt.Errorf("deleting tag: %w", err)
	}

	s.logger.Info("tag deleted", slog.String("method", "DeleteTag"), slog.String("user_id", userID), slog.String("tag_id", tagID))
	return nil
}

func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
