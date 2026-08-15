package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
	currencycatalog "github.com/ItsThompson/gofin/services/shared/currency"
	"github.com/ItsThompson/gofin/services/shared/validator"
)

// CalculateInstallments divides totalAmount across months using integer division.
// The first installment absorbs the remainder.
// Example: 10000 cents / 3 = [3334, 3333, 3333].
func CalculateInstallments(totalAmount int64, months int32) []int64 {
	if months <= 0 {
		return nil
	}
	if months == 1 {
		return []int64{totalAmount}
	}

	baseAmount := totalAmount / int64(months)
	remainder := totalAmount - (baseAmount * int64(months))

	installments := make([]int64, months)
	installments[0] = baseAmount + remainder
	for i := int32(1); i < months; i++ {
		installments[i] = baseAmount
	}
	return installments
}

// AdvanceMonth returns the next (year, month) pair, rolling over December to January.
func AdvanceMonth(year int32, month int32) (int32, int32) {
	month++
	if month > 12 {
		month = 1
		year++
	}
	return year, month
}

// monthLabel formats a year/month as "January 2026".
func monthLabel(year int32, month int32) string {
	t := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return t.Format("January 2006")
}

// CreateProRataExpense creates a pro-rata schedule with explicit selected-period
// context and captured FX intent. Finance validates the creation period, captures
// one full provider snapshot, writes the first installment through the trusted
// internal Expense contract, and stores future rows with the same snapshot.
func (s *FinanceService) CreateProRataExpense(ctx context.Context, userID string, req *model.CreateProRataRequest) (*model.ProRataResponse, error) {
	v := validator.New()
	v.Check(strings.TrimSpace(req.Name) != "", "name", "required")
	v.Check(req.TotalAmount > 0, "totalAmount", "must be positive")
	v.Check(req.Months >= 2, "months", "must be at least 2")
	validTypes := map[string]bool{"essentials": true, "desires": true, "savings": true}
	v.Check(validTypes[req.ExpenseType], "expenseType", "must be essentials, desires, or savings")
	v.Check(strings.TrimSpace(req.TagID) != "", "tagId", "required")
	v.Check(strings.TrimSpace(req.ExpenseDate) != "", "expenseDate", "required")
	v.Check(req.PeriodYear >= 1, "periodYear", "required")
	v.Check(req.PeriodMonth >= 1 && req.PeriodMonth <= 12, "periodMonth", "must be between 1 and 12")
	if v.HasErrors() {
		return nil, apierr.Validation("validation failed", v.Errors())
	}

	// The creation period is the schedule's first target. Validate it exists
	// before any first-installment write or future schedule insert.
	period, err := s.GetCurrentPeriod(ctx, userID, req.PeriodYear, req.PeriodMonth)
	if err != nil {
		return nil, err
	}
	reportingCurrency := normalizeCurrencyCode(period.ReportingCurrency)
	if !currencycatalog.IsSupported(reportingCurrency) {
		return nil, apierr.Internal("creation period reporting currency is not supported")
	}

	transactionCurrency, err := s.resolveProRataTransactionCurrency(period, req)
	if err != nil {
		return nil, err
	}


	installments := CalculateInstallments(req.TotalAmount, req.Months)
	proRataGroup := uuid.New().String()
	now := s.nowFunc().UTC().Format(time.RFC3339)

	// Pro-rata always spans at least two months, so capture the full provider
	// snapshot before the first installment or any future row is written.
	if s.fxClient == nil {
		return nil, apierr.Internal("pro-rata snapshot capture is not configured")
	}
	snapshot, err := s.fxClient.CaptureRateSnapshot(ctx, FxCaptureRequest{
		RequiredCurrencies: []string{transactionCurrency, reportingCurrency},
		RequestedAt:        now,
	})
	if err != nil {
		s.logger.Info("pro-rata snapshot capture failed",
			slog.String("method", "CreateProRataExpense"),
			slog.String("user_id", userID),
			slog.String("transaction_currency", transactionCurrency),
			slog.String("reporting_currency", reportingCurrency),
			slog.String("error", err.Error()),
		)
		return nil, err
	}
	if snapshot == nil {
		return nil, apierr.Internal("FX returned an empty pro-rata snapshot")
	}

	created, err := s.expenseClient.CreateProRataInstallment(ctx, CreateProRataInstallmentInput{
		UserID: userID,
		PeriodContext: TrustedPeriodContext{
			PeriodID:          period.ID,
			UserID:            period.UserID,
			Year:              period.Year,
			Month:             period.Month,
			ReportingCurrency: reportingCurrency,
			Source:            "finance_service",
		},
		Name:                 req.Name,
		Amount:               installments[0],
		Currency:             transactionCurrency,
		ExpenseType:          req.ExpenseType,
		TagID:                req.TagID,
		ExpenseDate:          req.ExpenseDate,
		ProRataGroup:         proRataGroup,
		ProRataIndex:         1,
		ProRataTotal:         req.Months,
		CapturedRateSnapshot: snapshot,
	})
	if err != nil {
		return nil, fmt.Errorf("creating first installment via expense service: %w", err)
	}

	schedules := make([]*model.ProRataSchedule, 0, req.Months-1)
	targetYear, targetMonth := req.PeriodYear, req.PeriodMonth
	for i := int32(2); i <= req.Months; i++ {
		targetYear, targetMonth = AdvanceMonth(targetYear, targetMonth)

		schedule, err := s.repo.CreateProRataSchedule(ctx, &model.ProRataSchedule{
			UserID:                    userID,
			ProRataGroup:              proRataGroup,
			Name:                      req.Name,
			Amount:                    installments[i-1],
			Currency:                  transactionCurrency,
			ExpenseType:               req.ExpenseType,
			TagID:                     req.TagID,
			TargetYear:                targetYear,
			TargetMonth:               targetMonth,
			InstallmentIndex:          i,
			InstallmentTotal:          req.Months,
			TransactionAmount:         installments[i-1],
			TransactionCurrency:       transactionCurrency,
			CreationReportingCurrency: reportingCurrency,
			CapturedRateSnapshot:      snapshot,
		})
		if err != nil {
			// Log the inconsistency and return an error (the first installment is already written).
			s.logger.Error("pro-rata schedule creation failed after expense write",
				slog.String("method", "CreateProRataExpense"),
				slog.String("user_id", userID),
				slog.String("pro_rata_group", proRataGroup),
				slog.Int("installment_index", int(i)),
				slog.String("error", err.Error()),
			)
			return nil, apierr.Internal("First installment was created but schedule creation failed. Please contact support.")
		}
		schedules = append(schedules, schedule)
	}

	s.logger.Info("pro-rata expense created",
		slog.String("method", "CreateProRataExpense"),
		slog.String("user_id", userID),
		slog.String("pro_rata_group", proRataGroup),
		slog.Int("months", int(req.Months)),
		slog.Int64("total_amount", req.TotalAmount),
		slog.String("snapshot_rate_timestamp", snapshot.RateTimestamp),
	)

	return &model.ProRataResponse{
		Expense: &model.CreatedExpense{
			ID:                  created.ID,
			Name:                req.Name,
			Amount:              installments[0],
			TransactionCurrency: transactionCurrency,
			Currency:            transactionCurrency,
			ExpenseType:         req.ExpenseType,
			TagID:               req.TagID,
			ExpenseDate:         req.ExpenseDate,
			PeriodYear:          req.PeriodYear,
			PeriodMonth:         req.PeriodMonth,
			IsProRata:           true,
			ProRataGroup:        proRataGroup,
			ProRataIndex:        1,
			ProRataTotal:        req.Months,
			CreatedAt:           created.CreatedAt,
		},
		Schedules: schedules,
	}, nil
}

// resolveProRataTransactionCurrency resolves the transaction currency from the
// request. When absent, it defaults to the creation period reporting currency.
func (s *FinanceService) resolveProRataTransactionCurrency(period *model.BudgetPeriod, req *model.CreateProRataRequest) (string, error) {
	transactionCurrency := normalizeCurrencyCode(req.TransactionCurrency)
	if transactionCurrency != "" {
		return s.validateProRataTransactionCurrency(transactionCurrency)
	}

	defaultCurrency := normalizeCurrencyCode(period.ReportingCurrency)
	s.logger.Info("transaction currency defaulted",
		slog.String("event", "transaction_currency_defaulted"),
		slog.String("reporting_currency", defaultCurrency),
	)
	return s.validateProRataTransactionCurrency(defaultCurrency)
}

func (s *FinanceService) validateProRataTransactionCurrency(currencyCode string) (string, error) {
	if verr := validateSupportedCurrency("transactionCurrency", currencyCode); verr != nil {
		return "", verr
	}
	return currencyCode, nil
}

// GetUpcomingProRata returns all pending pro-rata schedules for the user.
func (s *FinanceService) GetUpcomingProRata(ctx context.Context, userID string) ([]*model.ProRataSchedule, error) {
	schedules, err := s.repo.GetUpcomingProRata(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting upcoming pro-rata: %w", err)
	}
	return schedules, nil
}

// applyPendingProRata applies all pending pro-rata schedules whose target
// year/month matches the just-created target period. The target period
// provides the reporting currency; Finance resolves it locally and passes
// trusted period context plus the captured snapshot to Expense so the
// installment write never re-enters Finance for period facts and never calls a
// live provider rate.
//
// Each schedule is applied independently: a deterministic failure on one row
// (missing snapshot currency, legacy differing currency) marks that row failed
// and continues to the next. A transient Expense write or conversion failure
// leaves the row pending so it can be retried; no partial expense row is
// visible because Expense only persists after a successful conversion. Finance
// marks a row applied only after the Expense ledger write succeeds.
func (s *FinanceService) applyPendingProRata(ctx context.Context, userID string, period *model.BudgetPeriod) ([]*model.ProRataSchedule, error) {
	pending, err := s.repo.GetPendingProRata(ctx, userID, period.Year, period.Month)
	if err != nil {
		return nil, fmt.Errorf("getting pending pro-rata for %d-%02d: %w", period.Year, period.Month, err)
	}

	if len(pending) == 0 {
		return nil, nil
	}

	targetReportingCurrency := normalizeCurrencyCode(period.ReportingCurrency)
	trustedCtx := TrustedPeriodContext{
		PeriodID:          period.ID,
		UserID:            period.UserID,
		Year:              period.Year,
		Month:             period.Month,
		ReportingCurrency: targetReportingCurrency,
		Source:            "finance_service",
	}

	applied := make([]*model.ProRataSchedule, 0, len(pending))
	for _, schedule := range pending {
		if appliedRow := s.applyOneProRataSchedule(ctx, userID, schedule, targetReportingCurrency, trustedCtx); appliedRow != nil {
			applied = append(applied, appliedRow)
		}
	}

	s.logger.Info("pro-rata installments applied",
		slog.String("method", "applyPendingProRata"),
		slog.String("user_id", userID),
		slog.Int("year", int(period.Year)),
		slog.Int("month", int(period.Month)),
		slog.Int("pending", len(pending)),
		slog.Int("applied", len(applied)),
	)

	return applied, nil
}

// applyOneProRataSchedule applies a single pending pro-rata schedule against the
// target period. It returns the updated schedule when the ledger write succeeds
// (status "applied"), or nil when the row remains pending or moves to failed.
func (s *FinanceService) applyOneProRataSchedule(ctx context.Context, userID string, schedule *model.ProRataSchedule, targetReportingCurrency string, trustedCtx TrustedPeriodContext) *model.ProRataSchedule {
	expenseDate := fmt.Sprintf("%04d-%02d-01", schedule.TargetYear, schedule.TargetMonth)

	hasSnapshot := schedule.CapturedRateSnapshot != nil
	transactionCurrency := normalizeCurrencyCode(schedule.TransactionCurrency)
	if transactionCurrency == "" {
		// Legacy schedules store the original currency in the Currency column.
		transactionCurrency = normalizeCurrencyCode(schedule.Currency)
	}

	if !hasSnapshot {
		return s.applyLegacyProRataSchedule(ctx, userID, schedule, targetReportingCurrency, transactionCurrency, trustedCtx, expenseDate)
	}

	// Captured-snapshot schedules: validate the snapshot can derive the target
	// reporting currency before asking Expense to write. A missing currency is
	// a deterministic failure (the captured intent cannot service this period).
	if _, ok := schedule.CapturedRateSnapshot.RatesByCurrency[targetReportingCurrency]; !ok {
		s.markProRataFailed(ctx, schedule, model.ErrSnapshotCurrencyMissing)
		return nil
	}

	_, err := s.expenseClient.CreateProRataInstallment(ctx, CreateProRataInstallmentInput{
		UserID:               userID,
		PeriodContext:        trustedCtx,
		Name:                 schedule.Name,
		Amount:               schedule.Amount,
		Currency:             transactionCurrency,
		ExpenseType:          schedule.ExpenseType,
		TagID:                schedule.TagID,
		ExpenseDate:          expenseDate,
		ProRataGroup:         schedule.ProRataGroup,
		ProRataIndex:         schedule.InstallmentIndex,
		ProRataTotal:         schedule.InstallmentTotal,
		CapturedRateSnapshot: schedule.CapturedRateSnapshot,
	})
	if err != nil {
		// Transient Expense write or snapshot-conversion failure: leave the row
		// pending so it can be retried. No partial expense row is visible because
		// Expense only persists after a successful conversion.
		s.logger.Error("pro-rata installment write failed; schedule remains pending",
			slog.String("method", "applyOneProRataSchedule"),
			slog.String("schedule_id", schedule.ID),
			slog.String("pro_rata_group", schedule.ProRataGroup),
			slog.String("failure_reason", "transient_write_failure"),
			slog.String("error", err.Error()),
		)
		return nil
	}

	if err := s.repo.MarkProRataApplied(ctx, schedule.ID); err != nil {
		s.logger.Error("failed to mark pro-rata as applied after successful ledger write",
			slog.String("method", "applyOneProRataSchedule"),
			slog.String("schedule_id", schedule.ID),
			slog.String("error", err.Error()),
		)
		return nil
	}

	schedule.Status = "applied"
	return schedule
}

// applyLegacyProRataSchedule handles a legacy pending schedule with no captured
// snapshot. When the target period reporting currency equals the stored schedule
// currency, Finance asks Expense to write a migration snapshot
// (rate = "1", source = "migration"). When the currencies differ, migration must
// not invent a rate, so the row moves to failed with missing_captured_rate_snapshot.
func (s *FinanceService) applyLegacyProRataSchedule(ctx context.Context, userID string, schedule *model.ProRataSchedule, targetReportingCurrency, transactionCurrency string, trustedCtx TrustedPeriodContext, expenseDate string) *model.ProRataSchedule {
	if transactionCurrency != targetReportingCurrency {
		s.markProRataFailed(ctx, schedule, model.ErrMissingCapturedRateSnapshot)
		return nil
	}

	_, err := s.expenseClient.CreateProRataInstallment(ctx, CreateProRataInstallmentInput{
		UserID:          userID,
		PeriodContext:   trustedCtx,
		Name:            schedule.Name,
		Amount:          schedule.Amount,
		Currency:        transactionCurrency,
		ExpenseType:     schedule.ExpenseType,
		TagID:           schedule.TagID,
		ExpenseDate:     expenseDate,
		ProRataGroup:    schedule.ProRataGroup,
		ProRataIndex:    schedule.InstallmentIndex,
		ProRataTotal:    schedule.InstallmentTotal,
		LegacyMigration: true,
	})
	if err != nil {
		s.logger.Error("legacy pro-rata installment write failed; schedule remains pending",
			slog.String("method", "applyLegacyProRataSchedule"),
			slog.String("schedule_id", schedule.ID),
			slog.String("pro_rata_group", schedule.ProRataGroup),
			slog.String("failure_reason", "transient_write_failure"),
			slog.String("error", err.Error()),
		)
		return nil
	}

	if err := s.repo.MarkProRataApplied(ctx, schedule.ID); err != nil {
		s.logger.Error("failed to mark legacy pro-rata as applied after successful ledger write",
			slog.String("method", "applyLegacyProRataSchedule"),
			slog.String("schedule_id", schedule.ID),
			slog.String("error", err.Error()),
		)
		return nil
	}

	schedule.Status = "applied"
	return schedule
}

// markProRataFailed moves a schedule to failed with the typed failure reason and
// emits a diagnostic log so operators can see failed pro-rata rows. A repo
// failure during the status update is logged but does not roll back the decision.
func (s *FinanceService) markProRataFailed(ctx context.Context, schedule *model.ProRataSchedule, failureReason string) {
	if err := s.repo.MarkProRataFailed(ctx, schedule.ID, failureReason); err != nil {
		s.logger.Error("failed to mark pro-rata schedule as failed",
			slog.String("method", "markProRataFailed"),
			slog.String("schedule_id", schedule.ID),
			slog.String("intended_failure_reason", failureReason),
			slog.String("error", err.Error()),
		)
		return
	}
	schedule.Status = "failed"
	schedule.FailureReason = failureReason
	s.logger.Warn("pro-rata schedule marked failed",
		slog.String("method", "markProRataFailed"),
		slog.String("schedule_id", schedule.ID),
		slog.String("pro_rata_group", schedule.ProRataGroup),
		slog.String("failure_reason", failureReason),
		slog.Int("installment_index", int(schedule.InstallmentIndex)),
		slog.Int("target_year", int(schedule.TargetYear)),
		slog.Int("target_month", int(schedule.TargetMonth)),
	)
}

// CreatePeriodWithProRata creates a budget period and applies any pending pro-rata
// schedules. It also handles missed months: if the user has skipped months, intermediate
// periods are auto-created with defaults and their pro-rata installments are applied.
func (s *FinanceService) CreatePeriodWithProRata(ctx context.Context, userID string, req *model.CreatePeriodRequest) (*model.CreatePeriodResponse, error) {
	if verr := ValidateEDSSplit(req.EssentialsPercent, req.DesiresPercent, req.SavingsPercent); verr != nil {
		return nil, verr
	}
	if req.Month < 1 || req.Month > 12 {
		return nil, apierr.Validation("Month must be between 1 and 12", map[string]string{"month": "must be between 1 and 12"})
	}

	// REST binding guarantees a non-nil BudgetAmount, but gRPC can only express
	// the int64 zero value, so treat nil as 0 rather than dereferencing blindly.
	budgetAmount := int64(0)
	if req.BudgetAmount != nil {
		budgetAmount = *req.BudgetAmount
	}
	if budgetAmount < 0 {
		return nil, budgetAmountError()
	}

	reportingCurrency := normalizeCurrencyCode(req.ReportingCurrency)
	if verr := validateSupportedCurrency("reportingCurrency", reportingCurrency); verr != nil {
		return nil, verr
	}

	latestPeriod, err := s.repo.GetLatestPeriod(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting latest period: %w", err)
	}

	autoCreatedMonths, allAppliedProRata, err := s.createMissedPeriods(ctx, userID, latestPeriod, req.Year, req.Month)
	if err != nil {
		return nil, err
	}

	period, err := s.repo.CreatePeriod(ctx, &model.BudgetPeriod{
		UserID:            userID,
		Year:              req.Year,
		Month:             req.Month,
		BudgetAmount:      budgetAmount,
		ReportingCurrency: reportingCurrency,
		EssentialsPercent: req.EssentialsPercent,
		DesiresPercent:    req.DesiresPercent,
		SavingsPercent:    req.SavingsPercent,
	})
	if err != nil {
		return nil, fmt.Errorf("creating period: %w", err)
	}

	// Apply pro-rata for the current month. applyPendingProRata never returns a
	// fatal error: deterministic failures mark individual rows failed and
	// transient failures leave them pending.
	applied, applyErr := s.applyPendingProRata(ctx, userID, period)
	if applyErr != nil {
		s.logger.Error("failed to load pending pro-rata for new period",
			slog.Int("year", int(req.Year)),
			slog.Int("month", int(req.Month)),
			slog.String("error", applyErr.Error()),
		)
	}
	allAppliedProRata = append(allAppliedProRata, applied...)

	s.logger.Info("budget period created with pro-rata",
		slog.String("method", "CreatePeriodWithProRata"),
		slog.String("user_id", userID),
		slog.Int("year", int(req.Year)),
		slog.Int("month", int(req.Month)),
		slog.Int("applied_pro_rata", len(allAppliedProRata)),
	)

	return &model.CreatePeriodResponse{
		Period:             period,
		AppliedProRata:     allAppliedProRata,
		AutoCreatedPeriods: len(autoCreatedMonths),
		AutoCreatedMonths:  autoCreatedMonths,
	}, nil
}

// createMissedPeriods auto-creates periods for the months skipped between the
// latest period and the requested one, then applies their pending pro-rata
// installments. It returns the labels of the auto-created months and the
// applied schedules. Nothing is created when there is no latest period or no
// missed month.
func (s *FinanceService) createMissedPeriods(
	ctx context.Context,
	userID string,
	latestPeriod *model.BudgetPeriod,
	targetYear, targetMonth int32,
) ([]string, []*model.ProRataSchedule, error) {
	if latestPeriod == nil {
		return nil, nil, nil
	}

	missedMonths := computeMissedMonths(latestPeriod.Year, latestPeriod.Month, targetYear, targetMonth)
	if len(missedMonths) == 0 {
		return nil, nil, nil
	}

	defaults, err := s.getPeriodCreationDefaults(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	var autoCreatedMonths []string
	var allApplied []*model.ProRataSchedule
	for _, missed := range missedMonths {
		autoPeriod, err := s.repo.CreatePeriod(ctx, &model.BudgetPeriod{
			UserID:            userID,
			Year:              missed.year,
			Month:             missed.month,
			BudgetAmount:      defaults.BudgetAmount,
			ReportingCurrency: defaults.Currency,
			EssentialsPercent: defaults.EssentialsPercent,
			DesiresPercent:    defaults.DesiresPercent,
			SavingsPercent:    defaults.SavingsPercent,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("auto-creating period for %d-%02d: %w", missed.year, missed.month, err)
		}

		// Apply pro-rata for the missed month. applyPendingProRata never
		// returns a fatal error: deterministic failures mark individual rows
		// failed and transient failures leave them pending.
		applied, applyErr := s.applyPendingProRata(ctx, userID, autoPeriod)
		if applyErr != nil {
			s.logger.Error("failed to load pending pro-rata for auto-created period",
				slog.Int("year", int(missed.year)),
				slog.Int("month", int(missed.month)),
				slog.String("error", applyErr.Error()),
			)
		}
		allApplied = append(allApplied, applied...)
		autoCreatedMonths = append(autoCreatedMonths, monthLabel(missed.year, missed.month))
	}

	s.logger.Info("auto-created periods for missed months",
		slog.String("method", "CreatePeriodWithProRata"),
		slog.String("user_id", userID),
		slog.Int("missed_count", len(missedMonths)),
	)

	return autoCreatedMonths, allApplied, nil
}

// yearMonth is a simple helper for month iteration.
type yearMonth struct {
	year  int32
	month int32
}

// computeMissedMonths returns the months between (lastYear, lastMonth) exclusive
// and (targetYear, targetMonth) exclusive. Only returns months in between: not
// the latest period's month and not the target month.
func computeMissedMonths(lastYear, lastMonth, targetYear, targetMonth int32) []yearMonth {
	var missed []yearMonth

	year, month := lastYear, lastMonth
	for {
		year, month = AdvanceMonth(year, month)
		if year > targetYear || (year == targetYear && month >= targetMonth) {
			break
		}
		missed = append(missed, yearMonth{year, month})
	}

	return missed
}
