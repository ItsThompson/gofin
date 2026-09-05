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
	"github.com/ItsThompson/gofin/services/shared/exchangesource"
)

var isoDateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ExpenseService contains the business logic for expense operations.
type ExpenseService struct {
	repo         repository.ExpenseRepository
	periodClient PeriodContextClient
	fxClient     FxClient
	logger       *slog.Logger
	clock        func() time.Time
}

// NewExpenseService creates a new ExpenseService. The clock seam supplies the
// current time for CreatedAt stamping and the period-lock check; production
// passes time.Now and tests inject a fixed clock. fxClient is required: a nil
// client is a programming error and panics immediately, so a forgotten wire-up
// fails at construction rather than on the first foreign-currency user.
func NewExpenseService(
	repo repository.ExpenseRepository,
	periodClient PeriodContextClient,
	fxClient FxClient,
	clock func() time.Time,
	logger *slog.Logger,
) *ExpenseService {
	if fxClient == nil {
		panic("NewExpenseService: fxClient must not be nil")
	}
	return &ExpenseService{
		repo:         repo,
		periodClient: periodClient,
		fxClient:     fxClient,
		logger:       logger,
		clock:        clock,
	}
}

// CreateExpense validates and creates a new expense entry in the ledger.
func (s *ExpenseService) CreateExpense(ctx context.Context, userID string, req *model.CreateExpenseRequest) (*model.Expense, error) {
	if err := validateCreateExpenseRequest(req); err != nil {
		return nil, err
	}

	if err := validateIdempotencyKey(req.ClientGeneratedIdempotencyKey); err != nil {
		return nil, err
	}

	replayed, err := s.lookupIdempotentReplay(ctx, userID, req.ClientGeneratedIdempotencyKey)
	if err != nil {
		return nil, err
	}
	if replayed != nil {
		return replayed, nil
	}

	period, err := s.periodClient.GetPeriodContext(ctx, userID, req.PeriodYear, req.PeriodMonth)
	if err != nil {
		return nil, err
	}

	// Normalize the period reporting currency once and validate it before any
	// currency resolution or FX work. Finance owns the stored value, but an
	// unsupported reporting currency is an internal invariant violation that
	// must surface as a 500, not a 400 transaction-currency error from the
	// defaulting branch. Normalization keeps casing/whitespace drift from
	// routing a same-currency write to FX or persisting mismatched casing.
	reportingCurrency := normalizeCurrencyCode(period.ReportingCurrencyCode)
	if err := validateReportingCurrency(reportingCurrency); err != nil {
		s.logger.Error("unsupported reporting currency from period context",
			slog.String("event", "unsupported_reporting_currency"),
			slog.String("reporting_currency", reportingCurrency),
		)
		return nil, err
	}

	transactionCurrency, err := s.resolveCreateTransactionCurrency(period, req)
	if err != nil {
		return nil, err
	}

	now := s.clock().UTC().Format(time.RFC3339)

	// Resolve the money snapshot. Same-currency expenses bypass the FX provider
	// and write an identity snapshot (rate "1", source "identity"). Foreign-currency
	// conversion calls the FX Service ConvertAmount RPC before writing the row.
	var snapshot model.Expense
	if transactionCurrency == reportingCurrency {
		snapshot = buildIdentitySnapshot(req.AmountInTransactionCurrencyMinorUnits, transactionCurrency, reportingCurrency, now)
	} else {
		fxResp, convErr := s.fxClient.ConvertAmount(ctx, FxConvertRequest{
			Amount:         req.AmountInTransactionCurrencyMinorUnits,
			SourceCurrency: transactionCurrency,
			TargetCurrency: reportingCurrency,
			RequestedAt:    now,
		})
		if convErr != nil {
			return nil, s.handleFxConversionFailure(convErr, transactionCurrency, reportingCurrency)
		}
		snap, invErr := buildProviderSnapshot(req.AmountInTransactionCurrencyMinorUnits, transactionCurrency, reportingCurrency, fxResp)
		if invErr != nil {
			return nil, invErr
		}
		snapshot = snap
	}

	expense := &model.Expense{
		ID:                                    uuid.New().String(),
		UserID:                                userID,
		Name:                                  req.Name,
		TransactionCurrencyCode:               transactionCurrency,
		ExpenseType:                           req.ExpenseType,
		TagID:                                 req.TagID,
		ExpenseDateIso:                        req.ExpenseDateIso,
		PeriodYear:                            req.PeriodYear,
		PeriodMonth:                           req.PeriodMonth,
		Status:                                "active",
		CorrectsID:                            "",
		IsProRata:                             req.IsProRata,
		ProRataGroup:                          req.ProRataGroup,
		ProRataIndex:                          req.ProRataIndex,
		ProRataTotal:                          req.ProRataTotal,
		CreatedAt:                             now,
		OriginalTransactionAmountInMinorUnits: snapshot.OriginalTransactionAmountInMinorUnits,
		ReportingAmountInMinorUnits:           snapshot.ReportingAmountInMinorUnits,
		ReportingCurrencyCode:                 snapshot.ReportingCurrencyCode,
		SourceToTargetExchangeRate:            snapshot.SourceToTargetExchangeRate,
		ExchangeRateSource:                    snapshot.ExchangeRateSource,
		ExchangeRateTimestamp:                 snapshot.ExchangeRateTimestamp,
		ExchangeRateCacheExpiresAt:            snapshot.ExchangeRateCacheExpiresAt,
		ClientGeneratedIdempotencyKey:         req.ClientGeneratedIdempotencyKey,
	}

	created, err := s.repo.CreateExpense(ctx, expense)
	if err != nil {
		return nil, fmt.Errorf("creating expense: %w", err)
	}

	s.logger.Info("expense created",
		slog.String("method", "CreateExpense"),
		slog.String("user_id", userID),
		slog.String("expense_id", created.ID),
		slog.Int64("amount", created.OriginalTransactionAmountInMinorUnits),
		slog.String("expense_type", created.ExpenseType),
	)

	metrics.ExpenseEntriesTotal.Inc()

	return created, nil
}

// GetActiveExpensesForPeriod returns materialized expenses for a period with pagination.
func (s *ExpenseService) GetActiveExpensesForPeriod(ctx context.Context, req *model.GetExpensesRequest) (*model.ExpenseListResponse, error) {
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

	expenses, total, err := s.repo.GetActiveExpensesForPeriod(ctx, req.UserID, req.Year, req.Month, page, pageSize)
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

	// Resolve the original expense's period reporting currency before any
	// conversion. Corrections convert against the original period currency, not
	// the current user currency or default settings (US-CORRECTION-02).
	period, err := s.periodClient.GetPeriodContext(ctx, userID, original.PeriodYear, original.PeriodMonth)
	if err != nil {
		return nil, err
	}

	reportingCurrency := normalizeCurrencyCode(period.ReportingCurrencyCode)
	if err := validateReportingCurrency(reportingCurrency); err != nil {
		s.logger.Error("unsupported reporting currency from period context",
			slog.String("event", "unsupported_reporting_currency"),
			slog.String("reporting_currency", reportingCurrency),
		)
		return nil, err
	}

	transactionCurrency, err := s.resolveCorrectionTransactionCurrency(original, req)
	if err != nil {
		return nil, err
	}

	nowTS := now.UTC().Format(time.RFC3339)

	// Resolve the correction snapshot before any ledger mutation. A foreign-
	// currency correction calls FX first; on failure the original remains active
	// and no correction row is appended.
	var snapshot model.Expense
	if transactionCurrency == reportingCurrency {
		snapshot = buildIdentitySnapshot(req.AmountInTransactionCurrencyMinorUnits, transactionCurrency, reportingCurrency, nowTS)
	} else {
		fxResp, convErr := s.fxClient.ConvertAmount(ctx, FxConvertRequest{
			Amount:         req.AmountInTransactionCurrencyMinorUnits,
			SourceCurrency: transactionCurrency,
			TargetCurrency: reportingCurrency,
			RequestedAt:    nowTS,
		})
		if convErr != nil {
			return nil, s.handleFxConversionFailure(convErr, transactionCurrency, reportingCurrency)
		}
		snap, invErr := buildProviderSnapshot(req.AmountInTransactionCurrencyMinorUnits, transactionCurrency, reportingCurrency, fxResp)
		if invErr != nil {
			return nil, invErr
		}
		snapshot = snap
	}

	correction := &model.Expense{
		ID:                                    uuid.New().String(),
		UserID:                                userID,
		Name:                                  req.Name,
		TransactionCurrencyCode:               transactionCurrency,
		ExpenseType:                           req.ExpenseType,
		TagID:                                 req.TagID,
		ExpenseDateIso:                        req.ExpenseDateIso,
		PeriodYear:                            original.PeriodYear,
		PeriodMonth:                           original.PeriodMonth,
		Status:                                "active",
		CorrectsID:                            original.ID,
		IsProRata:                             original.IsProRata,
		ProRataGroup:                          original.ProRataGroup,
		ProRataIndex:                          original.ProRataIndex,
		ProRataTotal:                          original.ProRataTotal,
		CreatedAt:                             nowTS,
		OriginalTransactionAmountInMinorUnits: snapshot.OriginalTransactionAmountInMinorUnits,
		ReportingAmountInMinorUnits:           snapshot.ReportingAmountInMinorUnits,
		ReportingCurrencyCode:                 snapshot.ReportingCurrencyCode,
		SourceToTargetExchangeRate:            snapshot.SourceToTargetExchangeRate,
		ExchangeRateSource:                    snapshot.ExchangeRateSource,
		ExchangeRateTimestamp:                 snapshot.ExchangeRateTimestamp,
		ExchangeRateCacheExpiresAt:            snapshot.ExchangeRateCacheExpiresAt,
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

// DeleteExpense soft-deletes an active expense by flipping it to "corrected"
// with no replacement row. Mirrors CorrectExpense's period-lock and error
// patterns but creates no correction entry. Past-period expenses cannot be
// deleted, preventing alteration of closed financial periods.
func (s *ExpenseService) DeleteExpense(ctx context.Context, userID string, expenseID string) error {
	if expenseID == "" {
		return apierr.Validation("expense ID is required", nil)
	}

	expense, err := s.repo.GetExpenseByID(ctx, expenseID, userID)
	if err != nil {
		return fmt.Errorf("fetching expense for deletion: %w", err)
	}
	if expense == nil {
		return apierr.NotFound(fmt.Sprintf("expense %s not found", expenseID))
	}

	if expense.Status != "active" {
		return apierr.Conflict(model.ErrAlreadyCorrected, "this expense has already been corrected or deleted")
	}

	// Period-lock: same rule as CorrectExpense.
	now := s.clock()
	currentYear := int32(now.Year())
	currentMonth := int32(now.Month())
	if expense.PeriodYear != currentYear || expense.PeriodMonth != currentMonth {
		return &apierr.Error{
			Code:    model.ErrPeriodLocked,
			Message: "cannot delete expenses from a past period",
			Status:  http.StatusForbidden,
		}
	}

	if err := s.repo.DeactivateExpense(ctx, expenseID, userID); err != nil {
		return fmt.Errorf("deleting expense: %w", err)
	}

	s.logger.Info("expense deleted",
		slog.String("method", "DeleteExpense"),
		slog.String("user_id", userID),
		slog.String("expense_id", expenseID),
	)

	metrics.ExpenseDeletesTotal.Inc()

	return nil
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

// GetExpensesInProRataGroup returns all expenses belonging to a pro-rata group.
func (s *ExpenseService) GetExpensesInProRataGroup(ctx context.Context, userID string, groupID string) ([]*model.Expense, error) {
	if groupID == "" {
		return nil, apierr.Validation("group ID is required", nil)
	}

	expenses, err := s.repo.GetExpensesInProRataGroup(ctx, groupID, userID)
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

// conversionUnavailableError is returned when a foreign-currency expense cannot
// be converted because the FX provider is unavailable. No ledger row is written.
func conversionUnavailableError() *apierr.Error {
	return &apierr.Error{
		Code:    model.ErrConversionUnavailable,
		Message: "Conversion unavailable. Try again later, or enter the manually converted amount in the period currency.",
		Status:  http.StatusServiceUnavailable,
	}
}

// fxConversionError wraps an FX conversion failure that is the service's fault
// (a 500) and carries the currency pair so the handler's errkit.Report merges it
// into the Sentry context block and slog record automatically.
type fxConversionError struct {
	err                 error
	transactionCurrency string
	reportingCurrency   string
}

func (e *fxConversionError) Error() string { return e.err.Error() }
func (e *fxConversionError) Unwrap() error { return e.err }
func (e *fxConversionError) ReportData() map[string]any {
	return map[string]any{"transaction_currency": e.transactionCurrency, "reporting_currency": e.reportingCurrency}
}

// handleFxConversionFailure classifies an FX conversion failure and returns the
// error the caller should surface. The handler reports 500s via errkit.Report,
// so the service only logs here and never reports: a 503 outage and a 400 client
// error are expected outcomes, while a 500 is wrapped in fxConversionError so the
// handler's report carries the currency pair.
func (s *ExpenseService) handleFxConversionFailure(err error, transactionCurrency, reportingCurrency string) error {
	var apiErr *apierr.Error
	if errors.As(err, &apiErr) && apiErr.Code == model.ErrConversionUnavailable {
		s.logger.Info("foreign currency conversion unavailable",
			slog.String("event", "foreign_currency_conversion_unavailable"),
			slog.String("transaction_currency", transactionCurrency),
			slog.String("reporting_currency", reportingCurrency),
		)
		return err
	}
	if errors.As(err, &apiErr) && !apierr.IsServerError(apiErr) {
		s.logger.Info("foreign currency conversion rejected",
			slog.String("event", "foreign_currency_conversion_rejected"),
			slog.String("transaction_currency", transactionCurrency),
			slog.String("reporting_currency", reportingCurrency),
			slog.String("error", err.Error()),
		)
		return err
	}
	return &fxConversionError{
		err:                 err,
		transactionCurrency: transactionCurrency,
		reportingCurrency:   reportingCurrency,
	}
}

// lookupIdempotentReplay returns the previously-stored expense when the client-
// generated idempotency key matches an existing row, signalling an idempotent
// replay. It returns (nil, nil) when no match exists, so the caller proceeds
// with a fresh create. Validation guarantees the key is non-empty before this
// is called.
func (s *ExpenseService) lookupIdempotentReplay(ctx context.Context, userID, key string) (*model.Expense, error) {
	existing, err := s.repo.GetExpenseByIdempotencyKey(ctx, userID, key)
	if err != nil {
		return nil, fmt.Errorf("checking idempotency key: %w", err)
	}
	if existing == nil {
		return nil, nil
	}
	s.logger.Info("expense created (idempotent replay)",
		slog.String("method", "CreateExpense"),
		slog.String("user_id", userID),
		slog.String("expense_id", existing.ID),
		slog.Bool("replayed", true),
	)
	return existing, nil
}

// buildIdentitySnapshot builds the money snapshot for a same-currency expense:
// transaction and reporting amounts are equal, the rate is "1", and the
// source is "identity". The exchange-rate timestamp is the ledger write time.
// Identity snapshots have no cache expiry.
func buildIdentitySnapshot(amount int64, transactionCurrency, reportingCurrency, timestamp string) model.Expense {
	return model.Expense{
		OriginalTransactionAmountInMinorUnits: amount,
		TransactionCurrencyCode:               transactionCurrency,
		ReportingAmountInMinorUnits:           amount,
		ReportingCurrencyCode:                 reportingCurrency,
		SourceToTargetExchangeRate:            "1",
		ExchangeRateSource:                    exchangesource.Identity,
		ExchangeRateTimestamp:                 timestamp,
	}
}

// buildProviderSnapshot builds the money snapshot for a foreign-currency expense
// from the FX Service ConvertAmount response. The transaction amount and
// currency are the user's original input (unchanged); the reporting amount is
// the FX-converted amount; and the exchange rate, source, timestamp, and expiry
// are the provider facts returned by FX.
//
// The source is validated as a write-path invariant: a source outside the
// valid set is an internal error and nothing is written to the ledger.
func buildProviderSnapshot(transactionAmount int64, transactionCurrency, reportingCurrency string, fx *FxConvertResponse) (model.Expense, *apierr.Error) {
	if !exchangesource.IsValid(fx.Source) {
		return model.Expense{}, apierr.Internal(fmt.Sprintf("invalid exchange rate source: %q", fx.Source))
	}
	return model.Expense{
		OriginalTransactionAmountInMinorUnits: transactionAmount,
		TransactionCurrencyCode:               transactionCurrency,
		ReportingAmountInMinorUnits:           fx.ConvertedAmount,
		ReportingCurrencyCode:                 reportingCurrency,
		SourceToTargetExchangeRate:            fx.ExchangeRate,
		ExchangeRateSource:                    fx.Source,
		ExchangeRateTimestamp:                 fx.RateTimestamp,
		ExchangeRateCacheExpiresAt:            fx.ExpiresAt,
	}, nil
}

// validateReportingCurrency checks that the period reporting currency is in the
// shared static catalog. The period context is trusted (Finance owns it), so an
// unsupported reporting currency is an internal invariant violation, not a user
// input error or a conversion outage. It surfaces as a 500 internal error.
func validateReportingCurrency(reportingCurrency string) *apierr.Error {
	code := normalizeCurrencyCode(reportingCurrency)
	if !currencycatalog.IsSupported(code) {
		return apierr.Internal("reporting currency is not supported")
	}
	return nil
}

func (s *ExpenseService) resolveCreateTransactionCurrency(period *PeriodContext, req *model.CreateExpenseRequest) (string, error) {
	transactionCurrency := normalizeCurrencyCode(req.TransactionCurrencyCode)
	if transactionCurrency == "" {
		transactionCurrency = normalizeCurrencyCode(period.ReportingCurrencyCode)
		s.logger.Info("transaction currency defaulted",
			slog.String("event", "transaction_currency_defaulted"),
			slog.String("reporting_currency", transactionCurrency),
		)
	}
	return s.validateTransactionCurrency(transactionCurrency)
}

func (s *ExpenseService) resolveCorrectionTransactionCurrency(original *model.Expense, req *model.CorrectExpenseRequest) (string, error) {
	transactionCurrency := normalizeCurrencyCode(req.TransactionCurrencyCode)
	if transactionCurrency == "" {
		transactionCurrency = normalizeCurrencyCode(original.TransactionCurrencyCode)
		s.logger.Info("correction currency preserved",
			slog.String("event", "correction_currency_preserved"),
			slog.String("transaction_currency", transactionCurrency),
		)
	}
	return s.validateTransactionCurrency(transactionCurrency)
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
	if req.AmountInTransactionCurrencyMinorUnits <= 0 {
		fields["amountInTransactionCurrencyMinorUnits"] = "amount must be positive"
	}
	if !model.ValidExpenseTypes[req.ExpenseType] {
		fields["expenseType"] = "expense_type must be one of: essentials, desires, savings"
	}
	if req.TagID == "" {
		fields["tagId"] = "tag_id is required"
	}
	if req.ExpenseDateIso == "" {
		fields["expenseDateIso"] = "expense_date is required"
	} else if !isoDateRegex.MatchString(req.ExpenseDateIso) {
		fields["expenseDateIso"] = "expense_date must be in ISO format (YYYY-MM-DD)"
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
	if req.AmountInTransactionCurrencyMinorUnits <= 0 {
		fields["amountInTransactionCurrencyMinorUnits"] = "amount must be positive"
	}
	if !model.ValidExpenseTypes[req.ExpenseType] {
		fields["expenseType"] = "expense_type must be one of: essentials, desires, savings"
	}
	if req.TagID == "" {
		fields["tagId"] = "tag_id is required"
	}
	if req.ExpenseDateIso == "" {
		fields["expenseDateIso"] = "expense_date is required"
	} else if !isoDateRegex.MatchString(req.ExpenseDateIso) {
		fields["expenseDateIso"] = "expense_date must be in ISO format (YYYY-MM-DD)"
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

// validateIdempotencyKey returns an *apierr.Validation if the key is empty,
// not a well-formed RFC 4122 UUID, or exceeds the 36-character column width.
// Called before any lookup or insert so a malformed or missing key is rejected
// without touching the ledger. The key is required: every create must be
// idempotent so a client retry returns the already-created expense.
func validateIdempotencyKey(key string) *apierr.Error {
	if key == "" {
		return apierr.Validation("clientGeneratedIdempotencyKey is required", nil)
	}
	if len(key) > 36 {
		return apierr.Validation("clientGeneratedIdempotencyKey must be at most 36 characters", nil)
	}
	if _, err := uuid.Parse(key); err != nil {
		return apierr.Validation("clientGeneratedIdempotencyKey must be a valid UUID", nil)
	}
	return nil
}
