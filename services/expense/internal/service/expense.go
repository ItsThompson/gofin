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

	period, err := s.periodClient.GetPeriodContext(ctx, userID, req.PeriodYear, req.PeriodMonth)
	if err != nil {
		return nil, err
	}

	transactionCurrency, err := s.resolveCreateTransactionCurrency(period, req)
	if err != nil {
		return nil, err
	}

	now := s.clock().UTC().Format(time.RFC3339)

	// Normalize the period reporting currency once and use it for validation,
	// the identity-vs-FX decision, the snapshot, and the FX target. Finance owns
	// the stored value, but normalizing here keeps casing/whitespace drift from
	// routing a same-currency write to FX or persisting mismatched casing.
	reportingCurrency := normalizeCurrencyCode(period.ReportingCurrency)

	// Validate the period reporting currency before any FX call. The period
	// context is the source of truth for the reporting currency; an unsupported
	// reporting currency is an internal invariant violation, not a user input
	// error, so it is reported and mapped to a safe internal error.
	if err := validateReportingCurrency(reportingCurrency); err != nil {
		s.logger.Error("unsupported reporting currency from period context",
			slog.String("event", "unsupported_reporting_currency"),
			slog.String("reporting_currency", reportingCurrency),
		)
		return nil, err
	}

	// Resolve the money snapshot. Same-currency expenses bypass the FX provider
	// and write an identity snapshot (rate "1", source "identity"). Foreign-currency
	// conversion calls the FX Service ConvertAmount RPC before writing the row.
	var snapshot model.Expense
	if transactionCurrency == reportingCurrency {
		snapshot = buildIdentitySnapshot(req.Amount, transactionCurrency, reportingCurrency, now)
	} else {
		fxResp, convErr := s.fxClient.ConvertAmount(ctx, FxConvertRequest{
			Amount:         req.Amount,
			SourceCurrency: transactionCurrency,
			TargetCurrency: reportingCurrency,
			RequestedAt:    now,
		})
		if convErr != nil {
			s.logger.Info("foreign currency conversion unavailable",
				slog.String("event", "foreign_currency_conversion_unavailable"),
				slog.String("transaction_currency", transactionCurrency),
				slog.String("reporting_currency", reportingCurrency),
			)
			return nil, convErr
		}
		snapshot = buildProviderSnapshot(req.Amount, transactionCurrency, reportingCurrency, fxResp)
	}

	expense := &model.Expense{
		ID:                    uuid.New().String(),
		UserID:                userID,
		Name:                  req.Name,
		Amount:                req.Amount,
		TransactionCurrency:   transactionCurrency,
		Currency:              transactionCurrency,
		ExpenseType:           req.ExpenseType,
		TagID:                 req.TagID,
		ExpenseDate:           req.ExpenseDate,
		PeriodYear:            req.PeriodYear,
		PeriodMonth:           req.PeriodMonth,
		Status:                "active",
		CorrectsID:            "",
		IsProRata:             req.IsProRata,
		ProRataGroup:          req.ProRataGroup,
		ProRataIndex:          req.ProRataIndex,
		ProRataTotal:          req.ProRataTotal,
		CreatedAt:             now,
		TransactionAmount:     snapshot.TransactionAmount,
		ReportingAmount:       snapshot.ReportingAmount,
		ReportingCurrency:     snapshot.ReportingCurrency,
		ExchangeRate:          snapshot.ExchangeRate,
		ExchangeRateSource:    snapshot.ExchangeRateSource,
		ExchangeRateTimestamp: snapshot.ExchangeRateTimestamp,
		ExchangeRateExpiresAt: snapshot.ExchangeRateExpiresAt,
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

	s.resolveExpensesWithPeriodContext(ctx, req.UserID, req.Year, req.Month, expenses)

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

	s.resolveExpenseWithPeriodContext(ctx, expense)

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

	transactionCurrency := original.TransactionCurrency // correction currency is inherited, not changeable
	createdAt := s.clock().UTC().Format(time.RFC3339)
	// Same-currency identity is correct here: correction currency is inherited
	// and foreign-currency corrections are not yet supported.
	snapshot := buildIdentitySnapshot(req.Amount, transactionCurrency, original.ReportingCurrency, createdAt)

	correction := &model.Expense{
		ID:                    uuid.New().String(),
		UserID:                userID,
		Name:                  req.Name,
		Amount:                req.Amount,
		TransactionCurrency:   transactionCurrency,
		Currency:              transactionCurrency, // immudb currency column is NOT NULL; mirror the transaction currency
		ExpenseType:           req.ExpenseType,
		TagID:                 req.TagID,
		ExpenseDate:           req.ExpenseDate,
		PeriodYear:            original.PeriodYear,  // Period is immutable
		PeriodMonth:           original.PeriodMonth, // Period is immutable
		Status:                "active",
		CorrectsID:            original.ID,
		IsProRata:             original.IsProRata,
		ProRataGroup:          original.ProRataGroup,
		ProRataIndex:          original.ProRataIndex,
		ProRataTotal:          original.ProRataTotal,
		CreatedAt:             createdAt,
		TransactionAmount:     snapshot.TransactionAmount,
		ReportingAmount:       snapshot.ReportingAmount,
		ReportingCurrency:     snapshot.ReportingCurrency,
		ExchangeRate:          snapshot.ExchangeRate,
		ExchangeRateSource:    snapshot.ExchangeRateSource,
		ExchangeRateTimestamp: snapshot.ExchangeRateTimestamp,
		ExchangeRateExpiresAt: snapshot.ExchangeRateExpiresAt,
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

	// All entries in a correction chain share the same period (period is
	// immutable across corrections), so resolve period context once and apply it
	// to every entry instead of issuing one Finance call per row.
	if len(chain) > 0 {
		s.resolveExpensesWithPeriodContext(ctx, userID, chain[0].PeriodYear, chain[0].PeriodMonth, chain)
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

	// Pro-rata installments may target different periods, so resolve per unique
	// period using a local cache to avoid redundant Finance calls.
	s.resolveExpensesWithCachedPeriodContext(ctx, userID, expenses, make(map[string]string))

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

	// Local cache for period reporting currency by "year:month" key, so the
	// stream resolves legacy snapshots without a Finance call per row.
	periodCurrencyCache := make(map[string]string)

	for expense := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.resolveExpensesWithCachedPeriodContext(ctx, userID, []*model.Expense{expense}, periodCurrencyCache)
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

// buildIdentitySnapshot builds the money snapshot for a same-currency expense:
// transaction and reporting amounts are equal, the rate is "1", and the
// source is "identity". The exchange-rate timestamp is the ledger write time.
// Identity snapshots have no cache expiry.
func buildIdentitySnapshot(amount int64, transactionCurrency, reportingCurrency, timestamp string) model.Expense {
	return model.Expense{
		TransactionAmount:     amount,
		TransactionCurrency:   transactionCurrency,
		ReportingAmount:       amount,
		ReportingCurrency:     reportingCurrency,
		ExchangeRate:          "1",
		ExchangeRateSource:    model.ExchangeSourceIdentity,
		ExchangeRateTimestamp: timestamp,
	}
}

// buildProviderSnapshot builds the money snapshot for a foreign-currency expense
// from the FX Service ConvertAmount response. The transaction amount and
// currency are the user's original input (unchanged); the reporting amount is
// the FX-converted amount; and the exchange rate, source, timestamp, and expiry
// are the provider facts returned by FX.
func buildProviderSnapshot(transactionAmount int64, transactionCurrency, reportingCurrency string, fx *FxConvertResponse) model.Expense {
	return model.Expense{
		MoneySnapshotVersion:  1,
		TransactionAmount:     transactionAmount,
		TransactionCurrency:   transactionCurrency,
		ReportingAmount:       fx.ConvertedAmount,
		ReportingCurrency:     reportingCurrency,
		ExchangeRate:          fx.ExchangeRate,
		ExchangeRateSource:    fx.Source,
		ExchangeRateTimestamp: fx.RateTimestamp,
		ExchangeRateExpiresAt: fx.ExpiresAt,
	}
}

// resolveLegacySnapshot normalizes a legacy-synthesized expense to the period
// reporting currency and emits telemetry. It is called by every read path after
// the repository returns expenses, ensuring expense detail, expense log,
// dashboard, correction history, pro-rata group, and export streams all use the
// same snapshot resolution behavior (spec AC10).
//
// The repository's rowToExpense synthesizes a migration snapshot for legacy
// rows (null money_snapshot_version) but leaves the synthesized currencies as
// the legacy currency. This method normalizes both TransactionCurrency and
// ReportingCurrency to the period reporting currency and emits the appropriate
// telemetry event:
//
//   - legacy_snapshot_synthesized: legacy currency matches period currency
//   - legacy_currency_mismatch_normalized: legacy currency is supported but differs
//   - legacy_currency_unsupported_normalized: legacy currency is not in the catalog
//   - partial_snapshot_fields_ignored: legacy row had partial nullable columns
//
// Non-legacy rows (version-1 with complete fields) pass through unchanged.
func (s *ExpenseService) resolveLegacySnapshot(expense *model.Expense, periodReportingCurrency string) {
	if !expense.LegacySynthesized {
		return
	}

	periodCurrency := normalizeCurrencyCode(periodReportingCurrency)
	legacyCurrency := normalizeCurrencyCode(expense.Currency)

	// Defensive guard: normalize only when the period reporting currency is
	// supported by the shared catalog. The Finance migration guarantees every
	// stored period has a supported currency, but this makes the conditional
	// explicit rather than relying on that invariant (review N3).
	if !currencycatalog.IsSupported(periodCurrency) {
		s.logger.Warn("period reporting currency unsupported for legacy normalization",
			slog.String("event", "period_reporting_currency_unsupported"),
			slog.String("expense_id", expense.ID),
			slog.String("reporting_currency", periodCurrency),
		)
		return
	}

	if expense.PartialSnapshotFields {
		s.logger.Info("partial snapshot fields ignored on legacy row",
			slog.String("event", "partial_snapshot_fields_ignored"),
			slog.String("expense_id", expense.ID),
		)
	}

	// Normalize both synthesized currencies to the period reporting currency.
	// The legacy amount is treated as already denominated in the period reporting
	// currency (spec historical data policy), so TransactionAmount and
	// ReportingAmount are unchanged.
	expense.TransactionCurrency = periodCurrency
	expense.ReportingCurrency = periodCurrency

	switch {
	case legacyCurrency == "":
		// No legacy currency field to compare; nothing more to emit.
	case legacyCurrency == periodCurrency:
		s.logger.Info("legacy snapshot synthesized",
			slog.String("event", "legacy_snapshot_synthesized"),
			slog.String("expense_id", expense.ID),
			slog.String("reporting_currency", periodCurrency),
		)
	case currencycatalog.IsSupported(legacyCurrency):
		s.logger.Info("legacy currency mismatch normalized to period reporting currency",
			slog.String("event", "legacy_currency_mismatch_normalized"),
			slog.String("expense_id", expense.ID),
			slog.String("legacy_currency", legacyCurrency),
			slog.String("period_reporting_currency", periodCurrency),
		)
	default:
		s.logger.Info("unsupported legacy currency normalized to period reporting currency",
			slog.String("event", "legacy_currency_unsupported_normalized"),
			slog.String("expense_id", expense.ID),
			slog.String("legacy_currency", legacyCurrency),
			slog.String("period_reporting_currency", periodCurrency),
		)
	}
}

// resolveExpensesWithPeriodContext fetches the period reporting currency for the
// given user/year/month and resolves legacy snapshots in the expense list. If
// the period context fetch fails, legacy rows keep the repository's basic
// synthesis (legacy currency as both currencies) and a warning is logged so the
// read does not fail solely because the period context is unavailable.
func (s *ExpenseService) resolveExpensesWithPeriodContext(ctx context.Context, userID string, year, month int32, expenses []*model.Expense) {
	if !hasLegacyRows(expenses) {
		return
	}
	period, err := s.periodClient.GetPeriodContext(ctx, userID, year, month)
	if err != nil {
		s.logger.Warn("failed to fetch period context for legacy resolution",
			slog.String("event", "period_context_fetch_failed"),
			slog.String("user_id", userID),
			slog.Int("year", int(year)),
			slog.Int("month", int(month)),
			slog.String("error", err.Error()),
		)
		return
	}
	for _, exp := range expenses {
		s.resolveLegacySnapshot(exp, period.ReportingCurrency)
	}
}

// resolveExpenseWithPeriodContext resolves a single expense's legacy snapshot
// using its own period year/month. Used by GetExpense and GetCorrectionHistory.
func (s *ExpenseService) resolveExpenseWithPeriodContext(ctx context.Context, expense *model.Expense) {
	if expense == nil || !expense.LegacySynthesized {
		return
	}
	period, err := s.periodClient.GetPeriodContext(ctx, expense.UserID, expense.PeriodYear, expense.PeriodMonth)
	if err != nil {
		s.logger.Warn("failed to fetch period context for legacy resolution",
			slog.String("event", "period_context_fetch_failed"),
			slog.String("user_id", expense.UserID),
			slog.Int("year", int(expense.PeriodYear)),
			slog.Int("month", int(expense.PeriodMonth)),
			slog.String("error", err.Error()),
		)
		return
	}
	s.resolveLegacySnapshot(expense, period.ReportingCurrency)
}

// hasLegacyRows returns true if any expense in the list was legacy-synthesized.
func hasLegacyRows(expenses []*model.Expense) bool {
	for _, exp := range expenses {
		if exp.LegacySynthesized {
			return true
		}
	}
	return false
}

// resolveExpensesWithCachedPeriodContext resolves legacy snapshots for a list
// of expenses that may span multiple periods. It caches period context per
// unique (year, month) pair to avoid redundant Finance calls. Used by pro-rata
// group reads and the export stream.
func (s *ExpenseService) resolveExpensesWithCachedPeriodContext(ctx context.Context, userID string, expenses []*model.Expense, cache map[string]string) {
	for _, exp := range expenses {
		if !exp.LegacySynthesized {
			continue
		}
		key := fmt.Sprintf("%d:%d", exp.PeriodYear, exp.PeriodMonth)
		periodCurrency, ok := cache[key]
		if !ok {
			period, err := s.periodClient.GetPeriodContext(ctx, userID, exp.PeriodYear, exp.PeriodMonth)
			if err != nil {
				s.logger.Warn("failed to fetch period context for legacy resolution",
					slog.String("event", "period_context_fetch_failed"),
					slog.String("user_id", userID),
					slog.Int("year", int(exp.PeriodYear)),
					slog.Int("month", int(exp.PeriodMonth)),
					slog.String("error", err.Error()),
				)
				continue
			}
			periodCurrency = period.ReportingCurrency
			cache[key] = periodCurrency
		}
		s.resolveLegacySnapshot(exp, periodCurrency)
	}
}

// validateReportingCurrency checks that the period reporting currency is in the
// shared static catalog. The period context is trusted (Finance owns it), so an
// unsupported reporting currency is an internal invariant violation, not a
// user input error.
func validateReportingCurrency(reportingCurrency string) *apierr.Error {
	code := normalizeCurrencyCode(reportingCurrency)
	if !currencycatalog.IsSupported(code) {
		return &apierr.Error{
			Code:    model.ErrConversionUnavailable,
			Message: "reporting currency is not supported",
			Status:  http.StatusServiceUnavailable,
		}
	}
	return nil
}

func (s *ExpenseService) resolveCreateTransactionCurrency(period *PeriodContext, req *model.CreateExpenseRequest) (string, error) {
	transactionCurrency := normalizeCurrencyCode(req.TransactionCurrency)
	if transactionCurrency == "" {
		transactionCurrency = normalizeCurrencyCode(period.ReportingCurrency)
		s.logger.Info("transaction currency defaulted",
			slog.String("event", "transaction_currency_defaulted"),
			slog.String("reporting_currency", transactionCurrency),
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
