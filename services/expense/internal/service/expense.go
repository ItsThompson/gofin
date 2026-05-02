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
)

var isoDateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ExpenseService contains the business logic for expense operations.
type ExpenseService struct {
	repo   repository.ExpenseRepository
	logger *slog.Logger
}

// NewExpenseService creates a new ExpenseService.
func NewExpenseService(
	repo repository.ExpenseRepository,
	logger *slog.Logger,
) *ExpenseService {
	return &ExpenseService{
		repo:   repo,
		logger: logger,
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
		}
	}
	return nil
}
