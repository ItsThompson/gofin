package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	"github.com/ItsThompson/gofin/services/finance/internal/repository"
	"github.com/ItsThompson/gofin/services/pgutil"
	currencycatalog "github.com/ItsThompson/gofin/services/shared/currency"
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

// NewFinanceService creates a new FinanceService with all dependencies injected.
// expenseClient is always supplied, so the dashboard and pro-rata paths
// dereference it without a nil guard. nowFunc is the clock seam (pass time.Now
// in production); a nil nowFunc defaults to time.Now.
func NewFinanceService(
	repo repository.FinanceRepository,
	txBeginner repository.TxBeginner,
	expenseClient ExpenseClient,
	nowFunc func() time.Time,
	logger *slog.Logger,
) *FinanceService {
	if nowFunc == nil {
		nowFunc = time.Now
	}
	return &FinanceService{
		repo:          repo,
		txBeginner:    txBeginner,
		expenseClient: expenseClient,
		nowFunc:       nowFunc,
		logger:        logger,
	}
}

// ValidateEDSSplit checks that essentials + desires + savings == 100. On
// failure it returns an *apierr.Error whose Fields map names every offending
// percentage, so the response carries field-level detail.
func ValidateEDSSplit(essentials, desires, savings int32) *apierr.Error {
	if negFields := negativeEDSFields(essentials, desires, savings); len(negFields) > 0 {
		return apierr.Validation("E/D/S percentages must be non-negative", negFields)
	}
	if overFields := overHundredEDSFields(essentials, desires, savings); len(overFields) > 0 {
		return apierr.Validation("E/D/S percentages must not exceed 100", overFields)
	}
	if total := essentials + desires + savings; total != 100 {
		return apierr.Validation(
			fmt.Sprintf("E/D/S split must sum to 100%%, got %d%%", total),
			map[string]string{
				"essentialsPercent": "must sum to 100 with desires and savings",
				"desiresPercent":    "must sum to 100 with essentials and savings",
				"savingsPercent":    "must sum to 100 with essentials and desires",
			},
		)
	}
	return nil
}

func negativeEDSFields(essentials, desires, savings int32) map[string]string {
	fields := map[string]string{}
	if essentials < 0 {
		fields["essentialsPercent"] = "must be non-negative"
	}
	if desires < 0 {
		fields["desiresPercent"] = "must be non-negative"
	}
	if savings < 0 {
		fields["savingsPercent"] = "must be non-negative"
	}
	return fields
}

func overHundredEDSFields(essentials, desires, savings int32) map[string]string {
	fields := map[string]string{}
	if essentials > 100 {
		fields["essentialsPercent"] = "must not exceed 100"
	}
	if desires > 100 {
		fields["desiresPercent"] = "must not exceed 100"
	}
	if savings > 100 {
		fields["savingsPercent"] = "must not exceed 100"
	}
	return fields
}

// budgetAmountError returns a VALIDATION_ERROR for a negative budget amount.
func budgetAmountError() *apierr.Error {
	return apierr.Validation("Budget amount must be non-negative", map[string]string{
		"budgetAmount": "must be non-negative",
	})
}

func normalizeCurrencyCode(currencyCode string) string {
	return strings.ToUpper(strings.TrimSpace(currencyCode))
}

func unsupportedCurrencyError(fieldName string, currencyCode string) *apierr.Error {
	return &apierr.Error{
		Code:    model.ErrUnsupportedCurrency,
		Message: fmt.Sprintf("Unsupported currency %q", currencyCode),
		Status:  http.StatusBadRequest,
		Fields: map[string]string{
			fieldName: "unsupported currency",
		},
	}
}

func validateSupportedCurrency(fieldName string, currencyCode string) *apierr.Error {
	if currencycatalog.IsSupported(currencyCode) {
		return nil
	}
	return unsupportedCurrencyError(fieldName, currencyCode)
}

func fallbackDefaultSettings() *model.DefaultSettings {
	return &model.DefaultSettings{
		BudgetAmount:      0,
		EssentialsPercent: 50,
		DesiresPercent:    30,
		SavingsPercent:    20,
		Currency:          "USD",
	}
}

func (s *FinanceService) getPeriodCreationDefaults(ctx context.Context, userID string) (*model.DefaultSettings, error) {
	defaults, err := s.repo.GetDefaults(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting defaults for period creation: %w", err)
	}
	if defaults == nil {
		return fallbackDefaultSettings(), nil
	}

	defaults.Currency = normalizeCurrencyCode(defaults.Currency)
	if verr := validateSupportedCurrency("reportingCurrency", defaults.Currency); verr != nil {
		return nil, verr
	}
	return defaults, nil
}

// CompleteOnboarding saves the user's default settings and seeds default tags.
// Both operations run in a transaction: if tag seeding fails, the defaults
// upsert is rolled back.
func (s *FinanceService) CompleteOnboarding(ctx context.Context, userID string, req *model.OnboardingRequest) (*model.DefaultSettings, error) {
	if verr := ValidateEDSSplit(req.EssentialsPercent, req.DesiresPercent, req.SavingsPercent); verr != nil {
		return nil, verr
	}

	currencyCode := normalizeCurrencyCode(req.Currency)
	if verr := validateSupportedCurrency("currency", currencyCode); verr != nil {
		return nil, verr
	}

	tx, err := s.txBeginner.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := tx.Repo()

	defaults, err := txRepo.UpsertDefaults(ctx, &model.DefaultSettings{
		UserID:            userID,
		BudgetAmount:      req.BudgetAmount,
		EssentialsPercent: req.EssentialsPercent,
		DesiresPercent:    req.DesiresPercent,
		SavingsPercent:    req.SavingsPercent,
		Currency:          currencyCode,
	})
	if err != nil {
		return nil, fmt.Errorf("upserting defaults: %w", err)
	}

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
		return nil, apierr.NotFound("Default settings not found")
	}
	return defaults, nil
}

// UpdateDefaults updates the user's default budget settings.
// Does not affect current or past budget periods.
func (s *FinanceService) UpdateDefaults(ctx context.Context, userID string, req *model.UpdateDefaultsRequest) (*model.DefaultSettings, error) {
	if verr := ValidateEDSSplit(req.EssentialsPercent, req.DesiresPercent, req.SavingsPercent); verr != nil {
		return nil, verr
	}

	if req.BudgetAmount < 0 {
		return nil, budgetAmountError()
	}

	currencyCode := normalizeCurrencyCode(req.Currency)
	if verr := validateSupportedCurrency("currency", currencyCode); verr != nil {
		return nil, verr
	}

	defaults, err := s.repo.UpsertDefaults(ctx, &model.DefaultSettings{
		UserID:            userID,
		BudgetAmount:      req.BudgetAmount,
		EssentialsPercent: req.EssentialsPercent,
		DesiresPercent:    req.DesiresPercent,
		SavingsPercent:    req.SavingsPercent,
		Currency:          currencyCode,
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
// Returns a PERIOD_NOT_FOUND apierr.Error (404) when no period exists.
func (s *FinanceService) GetCurrentPeriod(ctx context.Context, userID string, year, month int32) (*model.BudgetPeriod, error) {
	if month < 1 || month > 12 {
		return nil, apierr.Validation("Month must be between 1 and 12", map[string]string{
			"month": "must be between 1 and 12",
		})
	}

	period, err := s.repo.GetCurrentPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("getting current period: %w", err)
	}
	if period == nil {
		return nil, &apierr.Error{
			Code:    model.ErrPeriodNotFound,
			Message: fmt.Sprintf("No budget period found for %d-%02d", year, month),
			Status:  http.StatusNotFound,
		}
	}
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
	if verr := ValidateEDSSplit(req.EssentialsPercent, req.DesiresPercent, req.SavingsPercent); verr != nil {
		return nil, verr
	}

	if req.BudgetAmount < 0 {
		return nil, budgetAmountError()
	}

	// Fetch the period to check ownership and whether it's current.
	existing, err := s.repo.GetPeriodByID(ctx, periodID, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching period: %w", err)
	}
	if existing == nil {
		return nil, apierr.NotFound("Budget period not found")
	}

	// Enforce: only the current month's period can be edited.
	now := s.nowFunc()
	if existing.Year != int32(now.Year()) || existing.Month != int32(now.Month()) {
		return nil, &apierr.Error{
			Code:    model.ErrPeriodLocked,
			Message: "Past periods are read-only and cannot be modified",
			Status:  http.StatusForbidden,
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
		return nil, apierr.NotFound("Budget period not found")
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
		defer func() { _ = tx.Rollback(ctx) }()

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
	if verr := validateTagName(name); verr != nil {
		return nil, verr
	}

	tag, err := s.repo.CreateTag(ctx, userID, name, false)
	if err != nil {
		if _, ok := pgutil.IsUniqueViolation(err); ok {
			return nil, duplicateTagError(name)
		}
		return nil, fmt.Errorf("creating tag: %w", err)
	}

	s.logger.Info("tag created", slog.String("method", "CreateTag"), slog.String("user_id", userID), slog.String("tag_name", name))
	return tag, nil
}

func (s *FinanceService) UpdateTag(ctx context.Context, userID, tagID string, req *model.UpdateTagRequest) (*model.Tag, error) {
	name := strings.TrimSpace(req.Name)
	if verr := validateTagName(name); verr != nil {
		return nil, verr
	}

	tag, err := s.repo.UpdateTag(ctx, tagID, userID, name)
	if err != nil {
		if _, ok := pgutil.IsUniqueViolation(err); ok {
			return nil, duplicateTagError(name)
		}
		return nil, fmt.Errorf("updating tag: %w", err)
	}
	if tag == nil {
		return nil, apierr.NotFound("Tag not found")
	}

	s.logger.Info("tag updated", slog.String("method", "UpdateTag"), slog.String("user_id", userID), slog.String("tag_id", tagID))
	return tag, nil
}

// validateTagName enforces the non-empty and length constraints shared by tag
// creation and rename, returning a VALIDATION_ERROR with a name field on failure.
func validateTagName(name string) *apierr.Error {
	if name == "" {
		return apierr.Validation("Tag name is required", map[string]string{"name": "required"})
	}
	if utf8.RuneCountInString(name) > 50 {
		return apierr.Validation("Tag name must be 50 characters or fewer", map[string]string{"name": "must be 50 characters or fewer"})
	}
	return nil
}

// duplicateTagError is the 409 returned when a tag name collides.
func duplicateTagError(name string) *apierr.Error {
	return apierr.Conflict(model.ErrDuplicateTag, fmt.Sprintf("A tag named %q already exists", name))
}

func (s *FinanceService) DeleteTag(ctx context.Context, userID, tagID string) error {
	tag, err := s.repo.GetTag(ctx, tagID, userID)
	if err != nil {
		return fmt.Errorf("getting tag: %w", err)
	}
	if tag == nil {
		return apierr.NotFound("Tag not found")
	}
	if tag.IsDefault {
		return &apierr.Error{
			Code:    model.ErrDefaultTag,
			Message: "Default tags cannot be deleted, only renamed",
			Status:  http.StatusForbidden,
		}
	}

	expenseCount, err := s.expenseClient.CountExpensesByTag(ctx, userID, tagID)
	if err != nil {
		return fmt.Errorf("checking tag usage in expenses: %w", err)
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
		return apierr.Conflict(model.ErrTagInUse, fmt.Sprintf("Tag is referenced by %s", strings.Join(parts, " and ")))
	}

	if err := s.repo.DeleteTag(ctx, tagID, userID); err != nil {
		return fmt.Errorf("deleting tag: %w", err)
	}

	s.logger.Info("tag deleted", slog.String("method", "DeleteTag"), slog.String("user_id", userID), slog.String("tag_id", tagID))
	return nil
}
