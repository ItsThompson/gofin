package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/expense/internal/model"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
	"github.com/ItsThompson/gofin/services/metrics"
	"github.com/ItsThompson/gofin/services/serverkit"
	currencycatalog "github.com/ItsThompson/gofin/services/shared/currency"
)

var isoDateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ExpenseService contains the business logic for expense operations.
type ExpenseService struct {
	repo         repository.ExpenseRepository
	periodClient PeriodContextClient
	logger       *slog.Logger
	clock        func() time.Time
}

// NewExpenseService creates a new ExpenseService. The clock seam supplies the
// current time for CreatedAt stamping and the period-lock check; production
// passes time.Now and tests inject a fixed clock.
func NewExpenseService(
	repo repository.ExpenseRepository,
	periodClient PeriodContextClient,
	clock func() time.Time,
	logger *slog.Logger,
) *ExpenseService {
	return &ExpenseService{
		repo:         repo,
		periodClient: periodClient,
		logger:       logger,
		clock:        clock,
	}
}

// CreateExpense validates and creates a new expense entry in the ledger.
func (s *ExpenseService) CreateExpense(ctx context.Context, userID string, req *model.CreateExpenseRequest) (*model.Expense, error) {
	if err := validateCreateExpenseRequest(req); err != nil {
		return nil, err
	}

	period, err := s.periodClient.GetPeriodContext(ctx, userID, req.PeriodYear, req.PeriodMonth)
	if err != nil {
		return nil, err
	}

	transactionCurrency, err := s.resolveCreateTransactionCurrency(period, req)
	if err != nil {
		return nil, err
	}

	now := s.clock().UTC().Format(time.RFC3339)
	expense := &model.Expense{
		ID:                  uuid.New().String(),
		UserID:              userID,
		Name:                req.Name,
		Amount:              req.Amount,
		TransactionCurrency: transactionCurrency,
		Currency:            transactionCurrency,
		ExpenseType:         req.ExpenseType,
		TagID:               req.TagID,
		ExpenseDate:         req.ExpenseDate,
		PeriodYear:          req.PeriodYear,
		PeriodMonth:         req.PeriodMonth,
		Status:              "active",
		CorrectsID:          "",
		IsProRata:           req.IsProRata,
		ProRataGroup:        req.ProRataGroup,
		ProRataIndex:        req.ProRataIndex,
		ProRataTotal:        req.ProRataTotal,
		CreatedAt:           now,
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
		return nil, apierr.Validation("year must be positive", nil)
	}
	if req.Month < 1 || req.Month > 12 {
		return nil, apierr.Validation("month must be between 1 and 12", nil)
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
		return nil, apierr.Validation("expense ID is required", nil)
	}

	expense, err := s.repo.GetExpenseByID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("getting expense: %w", err)
	}
	if expense == nil {
		return nil, apierr.NotFound(fmt.Sprintf("expense %s not found", id))
	}

	return expense, nil
}

// CountExpensesByTag returns the count of active expenses for a user that reference a given tag.
func (s *ExpenseService) CountExpensesByTag(ctx context.Context, userID string, tagID string) (int64, error) {
	if tagID == "" {
		return 0, apierr.Validation("tag_id is required", nil)
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
		return nil, apierr.Validation("expense ID is required", nil)
	}

	if err := validateCorrectExpenseRequest(req); err != nil {
		return nil, err
	}

	original, err := s.repo.GetExpenseByID(ctx, expenseID, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching expense for correction: %w", err)
	}
	if original == nil {
		return nil, apierr.NotFound(fmt.Sprintf("expense %s not found", expenseID))
	}

	if original.Status != "active" {
		return nil, apierr.Conflict(model.ErrAlreadyCorrected, "this expense has already been corrected")
	}

	// Check if the expense is in the current budget period
	now := s.clock()
	currentYear := int32(now.Year())
	currentMonth := int32(now.Month())
	if original.PeriodYear != currentYear || original.PeriodMonth != currentMonth {
		return nil, &apierr.Error{
			Code:    model.ErrPeriodLocked,
			Message: "cannot correct expenses from a past period",
			Status:  http.StatusForbidden,
		}
	}

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
		CreatedAt:    s.clock().UTC().Format(time.RFC3339),
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
		return nil, apierr.Validation("expense ID is required", nil)
	}

	chain, err := s.repo.GetCorrectionHistory(ctx, expenseID, userID)
	if err != nil {
		return nil, fmt.Errorf("getting correction history: %w", err)
	}
	if chain == nil {
		return nil, apierr.NotFound(fmt.Sprintf("expense %s not found", expenseID))
	}

	return chain, nil
}

// GetProRataGroup returns all expenses belonging to a pro-rata group.
func (s *ExpenseService) GetProRataGroup(ctx context.Context, userID string, groupID string) ([]*model.Expense, error) {
	if groupID == "" {
		return nil, apierr.Validation("group ID is required", nil)
	}

	expenses, err := s.repo.GetProRataGroup(ctx, groupID, userID)
	if err != nil {
		return nil, fmt.Errorf("getting pro-rata group: %w", err)
	}

	return expenses, nil
}

// StreamAllUserExpenses walks the full expense history (active + corrected) for
// a user with keyset pagination and invokes send for each row in chronological
// order (created_at ASC, id ASC), bounding memory to O(pageSize). It backs the
// StreamAllUserExpenses server-streaming RPC.
//
// A single producer goroutine pages the keyset cursor and feeds a rows channel
// buffered to pageSize; this method (the sole stream owner) consumes it and
// calls send. Buffering to one page lets the producer fetch page N+1 while the
// consumer is still sending page N, while keeping retained memory O(pageSize).
// The producer reports its terminal status on a buffered (cap 1) error channel
// and selects on context cancellation for every hand-off, so a send error, a
// client disconnect, or a job timeout stops the walk promptly without leaking
// the goroutine.
func (s *ExpenseService) StreamAllUserExpenses(ctx context.Context, userID string, pageSize int32, send func(*model.Expense) error) error {
	if userID == "" {
		return apierr.Validation("user_id is required", nil)
	}
	if pageSize < 1 {
		pageSize = repository.DefaultStreamPageSize
	}

	// Derive a cancellable context so returning for any reason (send error,
	// cancellation, clean EOF) unblocks the producer goroutine.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	rows := make(chan *model.Expense, pageSize) // buffered to one page: bounds retained memory to O(pageSize)
	errc := make(chan error, 1)                 // cap 1: producer reports terminal status without blocking

	go s.produceExpensePages(ctx, userID, pageSize, rows, errc)

	for expense := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := send(expense); err != nil {
			return err
		}
	}
	return <-errc
}

// produceExpensePages is the single owner and closer of rows. It pages the
// keyset cursor, feeds each row to rows (selecting on ctx.Done() so
// cancellation stops it promptly), and reports its terminal status (nil on a
// clean end-of-history) on errc before returning.
//
// It holds at most one page at a time (the current page slice plus the
// pageSize-buffered rows channel), so producer-side memory stays O(pageSize)
// regardless of total history size.
func (s *ExpenseService) produceExpensePages(ctx context.Context, userID string, pageSize int32, rows chan<- *model.Expense, errc chan<- error) {
	defer close(rows)

	// The producer runs on its own goroutine, and recover() does not cross
	// goroutines, so the gRPC stream interceptor cannot see a panic raised here.
	// Recovering alone would hang the RPC: the consumer's range over rows ends
	// when close(rows) runs and it then blocks on <-errc, so the guard has to
	// send a terminal error too. errc is buffered (cap 1) and provably empty on
	// this path, because every other send is immediately followed by a return, so
	// the send cannot block whichever order the two defers run in. An unbuffered
	// errc would deadlock here instead.
	defer func() {
		if recovered := recover(); recovered != nil {
			serverkit.LogRecoveredPanic(ctx, s.logger, "goroutine.expense_page_producer",
				"recovered panic in expense page producer", recovered,
				slog.String("user_id", userID),
			)
			errc <- errors.New("streaming user expenses failed unexpectedly")
		}
	}()

	cursor := repository.ExpenseCursor{}
	for {
		page, next, hasMore, err := s.repo.GetExpensesByUserAfter(ctx, userID, cursor, pageSize)
		if err != nil {
			errc <- fmt.Errorf("streaming user expenses: %w", err)
			return
		}
		for _, expense := range page {
			select {
			case rows <- expense:
			case <-ctx.Done():
				errc <- ctx.Err()
				return
			}
		}
		if !hasMore {
			errc <- nil
			return
		}
		cursor = next
	}
}

// AnonymizeAllUserExpenses redacts PII fields on all expense rows for a user.
// Satisfies GDPR right-to-erasure by overwriting the current accessible state.
// Idempotent: calling for already-redacted data returns success.
func (s *ExpenseService) AnonymizeAllUserExpenses(ctx context.Context, userID string) error {
	if userID == "" {
		return apierr.Validation("user_id is required", nil)
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

func normalizeCurrencyCode(currencyCode string) string {
	return strings.ToUpper(strings.TrimSpace(currencyCode))
}

func unsupportedCurrencyError(currencyCode string) *apierr.Error {
	return &apierr.Error{
		Code:    model.ErrUnsupportedCurrency,
		Message: fmt.Sprintf("Unsupported currency %q", currencyCode),
		Status:  http.StatusBadRequest,
		Fields: map[string]string{
			"transactionCurrency": "unsupported currency",
		},
	}
}

func currencyConflictError() *apierr.Error {
	return &apierr.Error{
		Code:    model.ErrCurrencyConflict,
		Message: "transactionCurrency and currency must match when both are provided",
		Status:  http.StatusBadRequest,
		Fields: map[string]string{
			"transactionCurrency": "must match currency",
			"currency":            "must match transactionCurrency",
		},
	}
}

func (s *ExpenseService) resolveCreateTransactionCurrency(period *PeriodContext, req *model.CreateExpenseRequest) (string, error) {
	transactionCurrency := normalizeCurrencyCode(req.TransactionCurrency)
	legacyCurrency := normalizeCurrencyCode(req.Currency)

	if transactionCurrency != "" && legacyCurrency != "" {
		if transactionCurrency != legacyCurrency {
			s.logger.Info("legacy currency conflict",
				slog.String("event", "legacy_currency_conflict"),
				slog.String("transaction_currency", transactionCurrency),
				slog.String("currency", legacyCurrency),
			)
			return "", currencyConflictError()
		}
		s.logger.Info("legacy currency duplicate same value",
			slog.String("event", "legacy_currency_duplicate_same_value"),
			slog.String("transaction_currency", transactionCurrency),
		)
		return s.validateTransactionCurrency(transactionCurrency)
	}

	if transactionCurrency != "" {
		return s.validateTransactionCurrency(transactionCurrency)
	}

	if legacyCurrency != "" {
		s.logger.Info("legacy currency alias used",
			slog.String("event", "legacy_currency_alias_used"),
			slog.String("currency", legacyCurrency),
		)
		return s.validateTransactionCurrency(legacyCurrency)
	}

	defaultCurrency := normalizeCurrencyCode(period.ReportingCurrency)
	s.logger.Info("transaction currency defaulted",
		slog.String("event", "transaction_currency_defaulted"),
		slog.String("reporting_currency", defaultCurrency),
	)
	return s.validateTransactionCurrency(defaultCurrency)
}

func (s *ExpenseService) validateTransactionCurrency(currencyCode string) (string, error) {
	if currencycatalog.IsSupported(currencyCode) {
		return currencyCode, nil
	}
	return "", unsupportedCurrencyError(currencyCode)
}

// validateCorrectExpenseRequest checks all required fields for a correction.
func validateCorrectExpenseRequest(req *model.CorrectExpenseRequest) *apierr.Error {
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
		return apierr.Validation("validation failed", fields)
	}
	return nil
}

// validateCreateExpenseRequest checks all required fields and business rules.
func validateCreateExpenseRequest(req *model.CreateExpenseRequest) *apierr.Error {
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
	if req.PeriodYear < 1 {
		fields["periodYear"] = "period_year must be positive"
	}
	if req.PeriodMonth < 1 || req.PeriodMonth > 12 {
		fields["periodMonth"] = "period_month must be between 1 and 12"
	}

	if len(fields) > 0 {
		return apierr.Validation("validation failed", fields)
	}
	return nil
}
