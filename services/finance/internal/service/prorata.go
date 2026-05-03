package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// CalculateInstallments divides totalAmount across months using integer division.
// The first installment absorbs the remainder.
// Example: 10000 cents / 3 = [3400, 3300, 3300].
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
	// Validate inputs
	if strings.TrimSpace(req.Name) == "" {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Name is required", Status: 400}
	}
	if req.TotalAmount <= 0 {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Total amount must be positive", Status: 400}
	}
	if req.Months < 2 {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Pro-rata requires at least 2 months", Status: 400}
	}
	validTypes := map[string]bool{"essentials": true, "desires": true, "savings": true}
	if !validTypes[req.ExpenseType] {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Expense type must be essentials, desires, or savings", Status: 400}
	}
	if strings.TrimSpace(req.Currency) == "" {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Currency is required", Status: 400}
	}
	if strings.TrimSpace(req.TagID) == "" {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Tag ID is required", Status: 400}
	}
	if strings.TrimSpace(req.ExpenseDate) == "" {
		return nil, &ServiceError{Code: model.ErrValidationError, Message: "Expense date is required", Status: 400}
	}

	if s.expenseClient == nil {
		return nil, fmt.Errorf("expense client not configured")
	}

	installments := CalculateInstallments(req.TotalAmount, req.Months)
	proRataGroup := uuid.New().String()

	now := s.nowFunc()
	currentYear := int32(now.Year())
	currentMonth := int32(now.Month())

	// Step 1: Create the first installment via expense service gRPC
	created, err := s.expenseClient.CreateExpense(ctx, CreateExpenseInput{
		UserID:       userID,
		Name:         req.Name,
		Amount:       installments[0],
		Currency:     req.Currency,
		ExpenseType:  req.ExpenseType,
		TagID:        req.TagID,
		ExpenseDate:  req.ExpenseDate,
		PeriodYear:   currentYear,
		PeriodMonth:  currentMonth,
		IsProRata:    true,
		ProRataGroup: proRataGroup,
		ProRataIndex: 1,
		ProRataTotal: req.Months,
	})
	if err != nil {
		return nil, fmt.Errorf("creating first installment via expense service: %w", err)
	}

	// Step 2: Create schedule records for months 2-N in PostgreSQL
	schedules := make([]*model.ProRataSchedule, 0, req.Months-1)
	targetYear, targetMonth := currentYear, currentMonth
	for i := int32(2); i <= req.Months; i++ {
		targetYear, targetMonth = AdvanceMonth(targetYear, targetMonth)

		schedule, err := s.repo.CreateProRataSchedule(ctx, &model.ProRataSchedule{
			UserID:           userID,
			ProRataGroup:     proRataGroup,
			Name:             req.Name,
			Amount:           installments[i-1],
			Currency:         req.Currency,
			ExpenseType:      req.ExpenseType,
			TagID:            req.TagID,
			TargetYear:       targetYear,
			TargetMonth:      targetMonth,
			InstallmentIndex: i,
			InstallmentTotal: req.Months,
		})
		if err != nil {
			// Expense was already written but schedule creation failed.
			// Log the inconsistency and return error per acceptance criteria.
			s.logger.Error("pro-rata schedule creation failed after expense write",
				slog.String("method", "CreateProRataExpense"),
				slog.String("user_id", userID),
				slog.String("pro_rata_group", proRataGroup),
				slog.Int("installment_index", int(i)),
				slog.String("error", err.Error()),
			)
			return nil, &ServiceError{
				Code:    model.ErrInternalServerError,
				Message: "First installment was created but schedule creation failed. Please contact support.",
				Status:  500,
			}
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
			Currency:     req.Currency,
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

	if s.expenseClient == nil {
		return nil, fmt.Errorf("expense client not configured")
	}

	applied := make([]*model.ProRataSchedule, 0, len(pending))
	for _, schedule := range pending {
		// Determine the expense date: first day of the target month
		expenseDate := fmt.Sprintf("%04d-%02d-01", year, month)

		_, err := s.expenseClient.CreateExpense(ctx, CreateExpenseInput{
			UserID:       userID,
			Name:         schedule.Name,
			Amount:       schedule.Amount,
			Currency:     schedule.Currency,
			ExpenseType:  schedule.ExpenseType,
			TagID:        schedule.TagID,
			ExpenseDate:  expenseDate,
			PeriodYear:   year,
			PeriodMonth:  month,
			IsProRata:    true,
			ProRataGroup: schedule.ProRataGroup,
			ProRataIndex: schedule.InstallmentIndex,
			ProRataTotal: schedule.InstallmentTotal,
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

	var autoCreatedMonths []string
	var allAppliedProRata []*model.ProRataSchedule

	// Detect missed months
	latestPeriod, err := s.repo.GetLatestPeriod(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting latest period: %w", err)
	}

	if latestPeriod != nil {
		missedMonths := computeMissedMonths(latestPeriod.Year, latestPeriod.Month, req.Year, req.Month)
		if len(missedMonths) > 0 {
			// Get defaults for auto-creating intermediate periods
			defaults, err := s.repo.GetDefaults(ctx, userID)
			if err != nil {
				return nil, fmt.Errorf("getting defaults for missed months: %w", err)
			}

			// Use fallback defaults if none exist
			if defaults == nil {
				defaults = &model.DefaultSettings{
					BudgetAmount:      0,
					EssentialsPercent: 50,
					DesiresPercent:    30,
					SavingsPercent:    20,
					Currency:          "USD",
				}
			}

			for _, missed := range missedMonths {
				_, err := s.repo.CreatePeriod(ctx, &model.BudgetPeriod{
					UserID:            userID,
					Year:              missed.year,
					Month:             missed.month,
					BudgetAmount:      defaults.BudgetAmount,
					EssentialsPercent: defaults.EssentialsPercent,
					DesiresPercent:    defaults.DesiresPercent,
					SavingsPercent:    defaults.SavingsPercent,
				})
				if err != nil {
					return nil, fmt.Errorf("auto-creating period for %d-%02d: %w", missed.year, missed.month, err)
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
				allAppliedProRata = append(allAppliedProRata, applied...)
				autoCreatedMonths = append(autoCreatedMonths, monthLabel(missed.year, missed.month))
			}

			s.logger.Info("auto-created periods for missed months",
				slog.String("method", "CreatePeriodWithProRata"),
				slog.String("user_id", userID),
				slog.Int("missed_count", len(missedMonths)),
			)
		}
	}

	// Create the requested period
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
