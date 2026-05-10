package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	"github.com/ItsThompson/gofin/services/metrics"
)

var isoDateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ExpenseService contains the business logic for expense operations.
type ExpenseService struct {
	repo   repository.ExpenseRepository
	logger *slog.Logger
	clock  func() time.Time
}

// NewExpenseService creates a new ExpenseService.
func NewExpenseService(
	repo repository.ExpenseRepository,
	logger *slog.Logger,
) *ExpenseService {
	return &ExpenseService{
		repo:   repo,
		logger: logger,
		clock:  time.Now,
	}
}

// WithClock returns a copy of the service with a custom clock function.
// Used in tests to inject a fixed time.
func (s *ExpenseService) WithClock(clock func() time.Time) *ExpenseService {
	s.clock = clock
	return s
}

// ServiceError is a typed error that carries an HTTP status code, error code,
// and optional field-level validation details.
type ServiceError struct {
	Code    string
	Message string
	Status  int
	Fields  map[string]string
}

func (e *ServiceError) Error() string {
	return e.Message
}

// CreateExpense validates and creates a new expense entry in the ledger.
func (s *ExpenseService) CreateExpense(ctx context.Context, userID string, req *model.CreateExpenseRequest) (*model.Expense, error) {
	if err := validateCreateExpenseRequest(req); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	expense := &model.Expense{
		ID:           uuid.New().String(),
		UserID:       userID,
		Name:         req.Name,
		Amount:       req.Amount,
		Currency:     req.Currency,
		ExpenseType:  req.ExpenseType,
		TagID:        req.TagID,
		ExpenseDate:  req.ExpenseDate,
		PeriodYear:   req.PeriodYear,
		PeriodMonth:  req.PeriodMonth,
		Status:       "active",
		CorrectsID:   "",
		IsProRata:    req.IsProRata,
		ProRataGroup: req.ProRataGroup,
		ProRataIndex: req.ProRataIndex,
		ProRataTotal: req.ProRataTotal,
		CreatedAt:    now,
	}

	created, err := s.repo.CreateExpense(ctx, expense)
	if err != nil {
		return nil, fmt.Errorf("creating expense: %w", err)
	}

	s.logger.Info("expense created",
		slog.String("method", "CreateExpense"),
		slog.String("user_id", userID),
		slog.String("expense_id", created.ID),
		slog.Int64("amount", created.Amount),
		slog.String("expense_type", created.ExpenseType),
	)

	metrics.ExpenseEntriesTotal.Inc()

	return created, nil
}

// GetExpensesForPeriod returns materialized expenses for a period with pagination.
func (s *ExpenseService) GetExpensesForPeriod(ctx context.Context, req *model.GetExpensesRequest) (*model.ExpenseListResponse, error) {
	if req.Year < 1 {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "year must be positive",
			Status:  400,
		}
	}
	if req.Month < 1 || req.Month > 12 {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "month must be between 1 and 12",
			Status:  400,
		}
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	expenses, total, err := s.repo.GetExpensesForPeriod(ctx, req.UserID, req.Year, req.Month, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("getting expenses for period: %w", err)
	}

	hasMore := int64(page)*int64(pageSize) < total

	return &model.ExpenseListResponse{
		Data:     expenses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  hasMore,
	}, nil
}

// GetExpense returns a single expense by ID, scoped to the requesting user.
func (s *ExpenseService) GetExpense(ctx context.Context, userID string, id string) (*model.Expense, error) {
	if id == "" {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "expense ID is required",
			Status:  400,
		}
	}

	expense, err := s.repo.GetExpenseByID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("getting expense: %w", err)
	}
	if expense == nil {
		return nil, &ServiceError{
			Code:    model.ErrNotFound,
			Message: fmt.Sprintf("expense %s not found", id),
			Status:  404,
		}
	}

	return expense, nil
}

// CountExpensesByTag returns the count of active expenses for a user that reference a given tag.
func (s *ExpenseService) CountExpensesByTag(ctx context.Context, userID string, tagID string) (int64, error) {
	if tagID == "" {
		return 0, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "tag_id is required",
			Status:  400,
		}
	}

	count, err := s.repo.CountExpensesByTag(ctx, userID, tagID)
	if err != nil {
		return 0, fmt.Errorf("counting expenses by tag: %w", err)
	}

	return count, nil
}

// CorrectExpense creates a correction entry that supersedes the original.
// The original is marked as "corrected" and a new entry with status "active"
// is created atomically. Returns the new correction entry.
func (s *ExpenseService) CorrectExpense(ctx context.Context, userID string, expenseID string, req *model.CorrectExpenseRequest) (*model.Expense, error) {
	if expenseID == "" {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "expense ID is required",
			Status:  400,
		}
	}

	if err := validateCorrectExpenseRequest(req); err != nil {
		return nil, err
	}

	// Fetch the original expense
	original, err := s.repo.GetExpenseByID(ctx, expenseID, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching expense for correction: %w", err)
	}
	if original == nil {
		return nil, &ServiceError{
			Code:    model.ErrNotFound,
			Message: fmt.Sprintf("expense %s not found", expenseID),
			Status:  404,
		}
	}

	// Check if already corrected
	if original.Status != "active" {
		return nil, &ServiceError{
			Code:    model.ErrAlreadyCorrected,
			Message: "this expense has already been corrected",
			Status:  409,
		}
	}

	// Check if the expense is in the current budget period
	now := s.clock()
	currentYear := int32(now.Year())
	currentMonth := int32(now.Month())
	if original.PeriodYear != currentYear || original.PeriodMonth != currentMonth {
		return nil, &ServiceError{
			Code:    model.ErrPeriodLocked,
			Message: "cannot correct expenses from a past period",
			Status:  403,
		}
	}

	// Build the correction entry
	correction := &model.Expense{
		ID:           uuid.New().String(),
		UserID:       userID,
		Name:         req.Name,
		Amount:       req.Amount,
		Currency:     original.Currency, // Currency is inherited, not changeable
		ExpenseType:  req.ExpenseType,
		TagID:        req.TagID,
		ExpenseDate:  req.ExpenseDate,
		PeriodYear:   original.PeriodYear,  // Period is immutable
		PeriodMonth:  original.PeriodMonth, // Period is immutable
		Status:       "active",
		CorrectsID:   original.ID,
		IsProRata:    original.IsProRata,
		ProRataGroup: original.ProRataGroup,
		ProRataIndex: original.ProRataIndex,
		ProRataTotal: original.ProRataTotal,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	created, err := s.repo.CorrectExpense(ctx, original, correction)
	if err != nil {
		return nil, fmt.Errorf("correcting expense: %w", err)
	}

	s.logger.Info("expense corrected",
		slog.String("method", "CorrectExpense"),
		slog.String("user_id", userID),
		slog.String("original_id", original.ID),
		slog.String("correction_id", created.ID),
	)

	metrics.CorrectionsTotal.Inc()

	return created, nil
}

// GetCorrectionHistory returns the full correction chain for an expense,
// ordered chronologically (original first, latest correction last).
func (s *ExpenseService) GetCorrectionHistory(ctx context.Context, userID string, expenseID string) ([]*model.Expense, error) {
	if expenseID == "" {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "expense ID is required",
			Status:  400,
		}
	}

	chain, err := s.repo.GetCorrectionHistory(ctx, expenseID, userID)
	if err != nil {
		return nil, fmt.Errorf("getting correction history: %w", err)
	}
	if chain == nil {
		return nil, &ServiceError{
			Code:    model.ErrNotFound,
			Message: fmt.Sprintf("expense %s not found", expenseID),
			Status:  404,
		}
	}

	return chain, nil
}

// GetProRataGroup returns all expenses belonging to a pro-rata group.
func (s *ExpenseService) GetProRataGroup(ctx context.Context, userID string, groupID string) ([]*model.Expense, error) {
	if groupID == "" {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "group ID is required",
			Status:  400,
		}
	}

	expenses, err := s.repo.GetProRataGroup(ctx, groupID, userID)
	if err != nil {
		return nil, fmt.Errorf("getting pro-rata group: %w", err)
	}

	return expenses, nil
}

// GetAllUserExpenses returns all expenses (active + corrected) for a user,
// with pagination. Used by the datarights service for GDPR data export.
func (s *ExpenseService) GetAllUserExpenses(ctx context.Context, userID string, page, pageSize int32) (*model.ExpenseListResponse, error) {
	if userID == "" {
		return nil, &ServiceError{
			Code:    model.ErrValidationError,
			Message: "user_id is required",
			Status:  400,
		}
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	expenses, total, err := s.repo.GetAllExpensesByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("getting all user expenses: %w", err)
	}

	hasMore := int64(page)*int64(pageSize) < total

	return &model.ExpenseListResponse{
		Data:     expenses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  hasMore,
	}, nil
}

// AnonymizeAllUserExpenses redacts PII fields on all expense rows for a user.
// Satisfies GDPR right-to-erasure by overwriting the current accessible state.
// Idempotent: calling for already-redacted data returns success.
func (s *ExpenseService) AnonymizeAllUserExpenses(ctx context.Context, userID string) error {
	if userID == "" {
		return &ServiceError{
			Code:    model.ErrValidationError,
			Message: "user_id is required",
			Status:  400,
		}
	}

	if err := s.repo.AnonymizeAllUserExpenses(ctx, userID); err != nil {
		return fmt.Errorf("anonymizing user expenses: %w", err)
	}

	s.logger.Info("user expenses anonymized",
		slog.String("method", "AnonymizeAllUserExpenses"),
		slog.String("user_id", userID),
	)

	return nil
}

// validateCorrectExpenseRequest checks all required fields for a correction.
func validateCorrectExpenseRequest(req *model.CorrectExpenseRequest) *ServiceError {
	fields := make(map[string]string)

	if req.Name == "" {
		fields["name"] = "name is required"
	}
	if req.Amount <= 0 {
		fields["amount"] = "amount must be positive"
	}
	if !model.ValidExpenseTypes[req.ExpenseType] {
		fields["expenseType"] = "expense_type must be one of: essentials, desires, savings"
	}
	if req.TagID == "" {
		fields["tagId"] = "tag_id is required"
	}
	if req.ExpenseDate == "" {
		fields["expenseDate"] = "expense_date is required"
	} else if !isoDateRegex.MatchString(req.ExpenseDate) {
		fields["expenseDate"] = "expense_date must be in ISO format (YYYY-MM-DD)"
	}

	if len(fields) > 0 {
		return &ServiceError{
			Code:    model.ErrValidationError,
			Message: "validation failed",
			Status:  400,
			Fields:  fields,
		}
	}
	return nil
}

// validateCreateExpenseRequest checks all required fields and business rules.
func validateCreateExpenseRequest(req *model.CreateExpenseRequest) *ServiceError {
	fields := make(map[string]string)

	if req.Name == "" {
		fields["name"] = "name is required"
	}
	if req.Amount <= 0 {
		fields["amount"] = "amount must be positive"
	}
	if req.Currency == "" {
		fields["currency"] = "currency is required"
	}
	if !model.ValidExpenseTypes[req.ExpenseType] {
		fields["expenseType"] = "expense_type must be one of: essentials, desires, savings"
	}
	if req.TagID == "" {
		fields["tagId"] = "tag_id is required"
	}
	if req.ExpenseDate == "" {
		fields["expenseDate"] = "expense_date is required"
	} else if !isoDateRegex.MatchString(req.ExpenseDate) {
		fields["expenseDate"] = "expense_date must be in ISO format (YYYY-MM-DD)"
	}
	if req.PeriodYear < 1 {
		fields["periodYear"] = "period_year must be positive"
	}
	if req.PeriodMonth < 1 || req.PeriodMonth > 12 {
		fields["periodMonth"] = "period_month must be between 1 and 12"
	}

	if len(fields) > 0 {
		return &ServiceError{
			Code:    model.ErrValidationError,
			Message: "validation failed",
			Status:  400,
			Fields:  fields,
		}
	}
	return nil
}
