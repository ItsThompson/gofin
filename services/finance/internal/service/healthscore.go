package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// Health-score formula constants. All are gathered here so tuning the scoring
// model is a one-line change. Weights are the full-mark points per component;
// the coefficients tune the asymmetric allocation penalty.
const (
	weightSavings = 30.0
	weightBudget  = 30.0
	weightAlloc   = 40.0

	// budgetFloorRatio is the spend-to-target ratio at which budget adherence
	// scores 0 (150% of plan). Below it the factor ramps linearly from 1.0.
	budgetFloorRatio = 1.5

	// Allocation asymmetric coefficients: penalize lifestyle creep (desires over,
	// savings under) at full weight and the mild directions at half.
	coefEssentials   = 0.5
	coefDesiresOver  = 1.0
	coefDesiresUnder = 0.5
	coefSavingsUnder = 1.0
	coefSavingsOver  = 0.5

	// allocDevCap is the weighted deviation at which allocation scores 0.
	allocDevCap = 1.0
)

// defaultCurrency is used when a user's default settings are missing or carry no
// currency (VC3: currency is set at onboarding, so this is a safety net).
const defaultCurrency = "USD"

// weightResolution holds the effective (float) weights and the integer maxes for
// the present components. The maxes always sum to 100 in both branches.
type weightResolution struct {
	savingsDropped bool
	savingsWeight  float64
	budgetWeight   float64
	allocWeight    float64
	maxSavings     int32
	maxBudget      int32
	maxAlloc       int32
}

// resolveWeights returns the effective weights and integer maxes (Formula D).
// When savingsTarget is 0 the savings component is dropped and its weight is
// redistributed proportionally across budget and allocation. The redistribution
// remainder is assigned to allocation so the maxes always sum to 100 (no
// independent-rounding drift). savingsTarget == 0 covers both savings_percent = 0
// and the tiny-budget integer-division-to-zero corner (E2).
func resolveWeights(savingsTarget int64) weightResolution {
	if savingsTarget == 0 {
		budgetWeight := weightBudget + weightSavings*(weightBudget/(weightBudget+weightAlloc))
		allocWeight := weightAlloc + weightSavings*(weightAlloc/(weightBudget+weightAlloc))
		maxBudget := int32(math.Round(budgetWeight))
		return weightResolution{
			savingsDropped: true,
			budgetWeight:   budgetWeight,
			allocWeight:    allocWeight,
			maxBudget:      maxBudget,
			maxAlloc:       100 - maxBudget,
		}
	}
	return weightResolution{
		savingsWeight: weightSavings,
		budgetWeight:  weightBudget,
		allocWeight:   weightAlloc,
		maxSavings:    int32(weightSavings),
		maxBudget:     int32(weightBudget),
		maxAlloc:      int32(weightAlloc),
	}
}

// roundClamp rounds a component's float score and clamps it to [0, max].
func roundClamp(scoreFloat float64, max int32) int32 {
	score := int32(math.Round(scoreFloat))
	if score < 0 {
		return 0
	}
	if score > max {
		return max
	}
	return score
}

// sumActualsByType totals active expenses by E/D/S type, mirroring the switch in
// ComputePeriodSummary.
func sumActualsByType(expenses []ExpenseData) (essentials, desires, savings int64) {
	for _, expense := range expenses {
		switch expense.ExpenseType {
		case "essentials":
			essentials += expense.Amount
		case "desires":
			desires += expense.Amount
		case "savings":
			savings += expense.Amount
		}
	}
	return essentials, desires, savings
}

// isProvisional reports whether the period is the current (open) month, mirroring
// the current-month check in ComputePeriodSummary.
func isProvisional(year, month int32, now time.Time) bool {
	return int32(now.Year()) == year && int32(now.Month()) == month
}

// ComputeHealthScore is the pure computation for the monthly health score.
// Exported for direct testing without service/repo dependencies. It assumes a
// configured budget (period.BudgetAmount > 0); the service short-circuits the
// zero-budget case before reaching here. now controls the clock so provisional
// stays deterministic in tests; currency selects the insight money symbol.
func ComputeHealthScore(period *model.BudgetPeriod, expenses []ExpenseData, year, month int32, now time.Time, currency string) *model.HealthScore {
	essentialsActual, desiresActual, savingsActual := sumActualsByType(expenses)
	edActual := essentialsActual + desiresActual

	// Derived targets use integer division (cents), matching allocateCategories.
	budget := period.BudgetAmount
	savingsTarget := budget * int64(period.SavingsPercent) / 100
	combinedTarget := budget * int64(period.EssentialsPercent+period.DesiresPercent) / 100

	weights := resolveWeights(savingsTarget)
	symbol := currencySymbol(currency)

	components := make([]model.HealthComponent, 0, 3)
	if !weights.savingsDropped {
		scoreFloat, detail := savingsComponent(savingsActual, savingsTarget, weights.savingsWeight, symbol)
		components = append(components, model.HealthComponent{
			Key: model.HealthKeySavings, Score: roundClamp(scoreFloat, weights.maxSavings), Max: weights.maxSavings, Detail: detail,
		})
	}

	budgetFloat, budgetDetail := budgetComponent(edActual, combinedTarget, weights.budgetWeight, symbol)
	components = append(components, model.HealthComponent{
		Key: model.HealthKeyBudget, Score: roundClamp(budgetFloat, weights.maxBudget), Max: weights.maxBudget, Detail: budgetDetail,
	})

	allocFloat, allocDetail, allocDevs := allocationComponent(
		essentialsActual, desiresActual, savingsActual,
		period.EssentialsPercent, period.DesiresPercent, period.SavingsPercent,
		weights.savingsDropped, weights.allocWeight,
	)
	components = append(components, model.HealthComponent{
		Key: model.HealthKeyAllocation, Score: roundClamp(allocFloat, weights.maxAlloc), Max: weights.maxAlloc, Detail: allocDetail,
	})

	var total int32
	for _, component := range components {
		total += component.Score
	}
	// Each component clamps to its own max and the maxes sum to 100, so total is
	// structurally in [0,100]; this clamp is a redundant safety net.
	if total < 0 {
		total = 0
	}
	if total > 100 {
		total = 100
	}

	insight := buildInsight(components, insightInputs{
		savingsActual:  savingsActual,
		savingsTarget:  savingsTarget,
		edActual:       edActual,
		combinedTarget: combinedTarget,
		maxSavings:     weights.maxSavings,
		maxBudget:      weights.maxBudget,
		maxAlloc:       weights.maxAlloc,
		allocDevs:      allocDevs,
		symbol:         symbol,
	})

	return &model.HealthScore{
		Year:           year,
		Month:          month,
		Total:          total,
		Band:           model.Band(total),
		Provisional:    isProvisional(year, month, now),
		FormulaVersion: model.FormulaVersion,
		Components:     components,
		Insight:        insight,
	}
}

// GetHealthScore computes the monthly health score for a budget period. It
// fetches the period (404 PERIOD_NOT_FOUND when none exists), short-circuits the
// zero-budget case with a configure-budget response, reads the user currency
// from defaults, then delegates to the pure ComputeHealthScore.
func (s *FinanceService) GetHealthScore(ctx context.Context, userID string, year, month int32) (*model.HealthScore, error) {
	period, err := s.GetCurrentPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, err
	}

	if period.BudgetAmount == 0 {
		return &model.HealthScore{Year: year, Month: month, ConfigureBudget: true}, nil
	}

	currency := defaultCurrency
	defaults, err := s.repo.GetDefaults(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting defaults: %w", err)
	}
	if defaults != nil && defaults.Currency != "" {
		currency = defaults.Currency
	}

	expenses, err := s.expenseClient.GetExpensesForPeriod(ctx, userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("fetching expenses: %w", err)
	}

	return ComputeHealthScore(period, expenses, year, month, s.nowFunc(), currency), nil
}
