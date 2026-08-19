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

// CreateProRataExpense creates a pro-rata expense: writes the first installment via
// gRPC to the expense service, then creates PostgreSQL schedules for months 2-N.
func (s *FinanceService) CreateProRataExpense(ctx context.Context, userID string, req *model.CreateProRataRequest) (*model.ProRataResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apierr.Validation("Name is required", map[string]string{"name": "required"})
	}
	if req.TotalAmount <= 0 {
		return nil, apierr.Validation("Total amount must be positive", map[string]string{"totalAmount": "must be positive"})
	}
	if req.Months < 2 {
		return nil, apierr.Validation("Pro-rata requires at least 2 months", map[string]string{"months": "must be at least 2"})
	}
	validTypes := map[string]bool{"essentials": true, "desires": true, "savings": true}
	if !validTypes[req.ExpenseType] {
		return nil, apierr.Validation("Expense type must be essentials, desires, or savings", map[string]string{"expenseType": "must be essentials, desires, or savings"})
	}
	if strings.TrimSpace(req.TagID) == "" {
		return nil, apierr.Validation("Tag ID is required", map[string]string{"tagId": "required"})
	}
	if strings.TrimSpace(req.ExpenseDate) == "" {
		return nil, apierr.Validation("Expense date is required", map[string]string{"expenseDate": "required"})
	}

	resolvedCurrency := normalizeCurrencyCode(req.TransactionCurrency)
	if resolvedCurrency == "" {
		return nil, apierr.Validation("Transaction currency is required", map[string]string{"transactionCurrency": "required"})
	}
	if verr := validateSupportedCurrency("transactionCurrency", resolvedCurrency); verr != nil {
		return nil, verr
	}

	installments := CalculateInstallments(req.TotalAmount, req.Months)
	proRataGroup := uuid.New().String()

	now := s.nowFunc()
	currentYear := int32(now.Year())
	currentMonth := int32(now.Month())

	created, err := s.expenseClient.CreateExpense(ctx, CreateExpenseInput{
		UserID:              userID,
		Name:                req.Name,
		Amount:              installments[0],
		TransactionCurrency: resolvedCurrency,
		ExpenseType:         req.ExpenseType,
		TagID:               req.TagID,
		ExpenseDate:         req.ExpenseDate,
		PeriodYear:          currentYear,
		PeriodMonth:         currentMonth,
		IsProRata:           true,
		ProRataGroup:        proRataGroup,
		ProRataIndex:        1,
		ProRataTotal:        req.Months,
	})
	if err != nil {
		return nil, fmt.Errorf("creating first installment via expense service: %w", err)
	}

	schedules := make([]*model.ProRataSchedule, 0, req.Months-1)
	targetYear, targetMonth := currentYear, currentMonth
	for i := int32(2); i <= req.Months; i++ {
		targetYear, targetMonth = AdvanceMonth(targetYear, targetMonth)

		schedule, err := s.repo.CreateProRataSchedule(ctx, &model.ProRataSchedule{
			UserID:           userID,
			ProRataGroup:     proRataGroup,
			Name:             req.Name,
			Amount:           installments[i-1],
			Currency:         resolvedCurrency,
			ExpenseType:      req.ExpenseType,
			TagID:            req.TagID,
			TargetYear:       targetYear,
			TargetMonth:      targetMonth,
			InstallmentIndex: i,
			InstallmentTotal: req.Months,
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
	)

	return &model.ProRataResponse{
		Expense: &model.CreatedExpense{
			ID:           created.ID,
			Name:         req.Name,
			Amount:       installments[0],
			Currency:     resolvedCurrency,
			ExpenseType:  req.ExpenseType,
			TagID:        req.TagID,
			ExpenseDate:  req.ExpenseDate,
			PeriodYear:   currentYear,
			PeriodMonth:  currentMonth,
			IsProRata:    true,
			ProRataGroup: proRataGroup,
			ProRataIndex: 1,
			ProRataTotal: req.Months,
			CreatedAt:    created.CreatedAt,
		},
		Schedules: schedules,
	}, nil
}

// GetUpcomingProRata returns all pending pro-rata schedules for the user.
func (s *FinanceService) GetUpcomingProRata(ctx context.Context, userID string) ([]*model.ProRataSchedule, error) {
	schedules, err := s.repo.GetUpcomingProRata(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting upcoming pro-rata: %w", err)
	}
	return schedules, nil
}

// applyPendingProRata applies all pending pro-rata schedules for a given month by
// creating expense entries via gRPC and marking schedules as applied.
func (s *FinanceService) applyPendingProRata(ctx context.Context, userID string, year, month int32) ([]*model.ProRataSchedule, error) {
	pending, err := s.repo.GetPendingProRata(ctx, userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("getting pending pro-rata for %d-%02d: %w", year, month, err)
	}

	if len(pending) == 0 {
		return nil, nil
	}

	applied := make([]*model.ProRataSchedule, 0, len(pending))
	for _, schedule := range pending {
		// Determine the expense date: first day of the target month
		expenseDate := fmt.Sprintf("%04d-%02d-01", year, month)

		_, err := s.expenseClient.CreateExpense(ctx, CreateExpenseInput{
			UserID:              userID,
			Name:                schedule.Name,
			Amount:              schedule.Amount,
			TransactionCurrency: schedule.Currency,
			ExpenseType:         schedule.ExpenseType,
			TagID:               schedule.TagID,
			ExpenseDate:         expenseDate,
			PeriodYear:          year,
			PeriodMonth:         month,
			IsProRata:           true,
			ProRataGroup:        schedule.ProRataGroup,
			ProRataIndex:        schedule.InstallmentIndex,
			ProRataTotal:        schedule.InstallmentTotal,
		})
		if err != nil {
			s.logger.Error("failed to apply pro-rata installment",
				slog.String("method", "applyPendingProRata"),
				slog.String("schedule_id", schedule.ID),
				slog.String("pro_rata_group", schedule.ProRataGroup),
				slog.String("error", err.Error()),
			)
			return applied, fmt.Errorf("applying pro-rata schedule %s: %w", schedule.ID, err)
		}

		if err := s.repo.MarkProRataApplied(ctx, schedule.ID); err != nil {
			s.logger.Error("failed to mark pro-rata as applied",
				slog.String("method", "applyPendingProRata"),
				slog.String("schedule_id", schedule.ID),
				slog.String("error", err.Error()),
			)
			return applied, fmt.Errorf("marking schedule %s as applied: %w", schedule.ID, err)
		}

		schedule.Status = "applied"
		applied = append(applied, schedule)
	}

	s.logger.Info("pro-rata installments applied",
		slog.String("method", "applyPendingProRata"),
		slog.String("user_id", userID),
		slog.Int("year", int(year)),
		slog.Int("month", int(month)),
		slog.Int("count", len(applied)),
	)

	return applied, nil
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

	// Apply pro-rata for the current month
	applied, err := s.applyPendingProRata(ctx, userID, req.Year, req.Month)
	if err != nil {
		s.logger.Error("failed to apply pro-rata for new period",
			slog.Int("year", int(req.Year)),
			slog.Int("month", int(req.Month)),
			slog.String("error", err.Error()),
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
		_, err := s.repo.CreatePeriod(ctx, &model.BudgetPeriod{
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

		// Apply pro-rata for the missed month
		applied, err := s.applyPendingProRata(ctx, userID, missed.year, missed.month)
		if err != nil {
			s.logger.Error("failed to apply pro-rata for auto-created period",
				slog.Int("year", int(missed.year)),
				slog.Int("month", int(missed.month)),
				slog.String("error", err.Error()),
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
