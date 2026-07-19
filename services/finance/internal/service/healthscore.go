package service

import (
	"math"
	"time"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// Health-score formula constants. All are gathered here so tuning the scoring
// model is a one-line change. Weights are the full-mark points per component;
// the coefficients tune the asymmetric allocation penalty. v2 rebalances the
// weights and adds spending stability.
const (
	weightSavings   = 25.0
	weightBudget    = 25.0
	weightAlloc     = 30.0
	weightStability = 20.0

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

	// stabilityMinMonths is the closed-month desires history required before the
	// spending-stability sub-score is scored. Below it, stability is dropped and
	// the card shows "building baseline".
	stabilityMinMonths = 3
	// stabilityWindowMonths caps the recent closed months of desires used for the
	// coefficient of variation.
	stabilityWindowMonths = 6
	// stabilityCoVCap is the coefficient of variation at which stability scores 0;
	// below it the factor ramps linearly from full marks at CoV 0.
	stabilityCoVCap = 1.0
)

// defaultCurrency is used when a user's default settings are missing or carry no
// currency (VC3: currency is set at onboarding, so this is a safety net).
const defaultCurrency = "USD"

// weightResolution holds the effective (float) weights and integer maxes for the
// components present this month. The maxes always sum to 100. A component is
// "present" by rule (savings dropped when savingsTarget == 0; stability dropped
// with fewer than stabilityMinMonths of history), never because its score is
// zero, so a legitimately-zero component keeps its weight.
type weightResolution struct {
	savingsDropped   bool
	stabilityDropped bool
	savingsWeight    float64
	budgetWeight     float64
	allocWeight      float64
	stabilityWeight  float64
	maxSavings       int32
	maxBudget        int32
	maxAlloc         int32
	maxStability     int32
}

// resolveWeights renormalizes the base weights over the present component set by
// division (Formula D). Each present component's effective weight is
// 100 * base / Σ(base of present); its integer max is that value rounded. Any
// rounding remainder is assigned to the largest-base present component (always
// allocation, base 30, which is never dropped), so the integer maxes sum to
// exactly 100 for every present-set. Budget and allocation are always present;
// savings and stability are present per the drop rules keyed by the caller.
func resolveWeights(savingsPresent, stabilityPresent bool) weightResolution {
	denom := weightBudget + weightAlloc
	if savingsPresent {
		denom += weightSavings
	}
	if stabilityPresent {
		denom += weightStability
	}

	effWeight := func(base float64) float64 { return 100 * base / denom }

	res := weightResolution{
		savingsDropped:   !savingsPresent,
		stabilityDropped: !stabilityPresent,
		budgetWeight:     effWeight(weightBudget),
		allocWeight:      effWeight(weightAlloc),
		maxBudget:        int32(math.Round(effWeight(weightBudget))),
		maxAlloc:         int32(math.Round(effWeight(weightAlloc))),
	}
	if savingsPresent {
		res.savingsWeight = effWeight(weightSavings)
		res.maxSavings = int32(math.Round(effWeight(weightSavings)))
	}
	if stabilityPresent {
		res.stabilityWeight = effWeight(weightStability)
		res.maxStability = int32(math.Round(effWeight(weightStability)))
	}

	// Absorb the rounding remainder into allocation (the unique largest base,
	// always present) so the integer maxes sum to exactly 100.
	res.maxAlloc += 100 - (res.maxSavings + res.maxBudget + res.maxAlloc + res.maxStability)
	return res
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
// zero-budget case before reaching here. desiresWindow holds the recent closed
// months' discretionary (desires) totals that feed the stability sub-score;
// fewer than stabilityMinMonths entries drops stability (building baseline).
// now controls the clock so provisional stays deterministic in tests; currency
// selects the insight money symbol.
func ComputeHealthScore(period *model.BudgetPeriod, expenses []ExpenseData, desiresWindow []int64, year, month int32, now time.Time, currency string) *model.HealthScore {
	essentialsActual, desiresActual, savingsActual := sumActualsByType(expenses)
	edActual := essentialsActual + desiresActual

	// Derived targets use integer division (cents), matching allocateCategories.
	budget := period.BudgetAmount
	savingsTarget := budget * int64(period.SavingsPercent) / 100
	combinedTarget := budget * int64(period.EssentialsPercent+period.DesiresPercent) / 100

	// Drops are decided by rule: savings when its target rounds to 0 (the single
	// predicate that also keys the allocation category set), stability when there
	// is too little closed-month history.
	savingsPresent := savingsTarget != 0
	stabilityPresent := len(desiresWindow) >= stabilityMinMonths
	weights := resolveWeights(savingsPresent, stabilityPresent)
	symbol := currencySymbol(currency)

	components := make([]model.HealthComponent, 0, 4)
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

	if !weights.stabilityDropped {
		stabilityFloat, stabilityDetail, _ := stabilityComponent(desiresWindow, weights.stabilityWeight)
		components = append(components, model.HealthComponent{
			Key: model.HealthKeyStability, Score: roundClamp(stabilityFloat, weights.maxStability), Max: weights.maxStability, Detail: stabilityDetail,
		})
	}

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
		maxStability:   weights.maxStability,
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
