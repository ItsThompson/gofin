package service

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// healthPeriod builds a budget period for health-score compute tests.
func healthPeriod(budget int64, essentials, desires, savings int32) *model.BudgetPeriod {
	return &model.BudgetPeriod{
		ID: "p-health", UserID: "user-1", Year: 2026, Month: 5,
		BudgetAmount: budget, EssentialsPercent: essentials, DesiresPercent: desires, SavingsPercent: savings,
	}
}

func healthExpense(expenseType string, amount int64) ExpenseData {
	return ExpenseData{ExpenseType: expenseType, Amount: amount}
}

// currentMonthNow is inside the compute period's month (May 2026) so provisional
// is true unless a test overrides it.
var currentMonthNow = time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)

// steadyWindow is a >= stabilityMinMonths desires window with zero variation, so
// stability is present and earns full marks.
var steadyWindow = []int64{80000, 80000, 80000}

// --- Money formatter ---

func TestFormatMoney(t *testing.T) {
	tests := []struct {
		name   string
		cents  int64
		symbol string
		want   string
	}{
		{"whole dollars", 42000, "$", "$420"},
		{"thousands separator", 248000, "$", "$2,480"},
		{"millions", 100000000, "$", "$1,000,000"},
		{"two decimals when not whole", 1234, "$", "$12.34"},
		{"decimal padding", 42050, "$", "$420.50"},
		{"euro symbol", 5000, "€", "€50"},
		{"gbp symbol", 5000, "£", "£50"},
		{"jpy symbol", 5000, "¥", "¥50"},
		{"fallback code", 1200, "CAD ", "CAD 12"},
		{"negative", -500, "$", "-$5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatMoney(tt.cents, tt.symbol))
		})
	}
}

func TestCurrencySymbol(t *testing.T) {
	assert.Equal(t, "$", currencySymbol("USD"))
	assert.Equal(t, "€", currencySymbol("EUR"))
	assert.Equal(t, "£", currencySymbol("GBP"))
	assert.Equal(t, "¥", currencySymbol("JPY"))
	assert.Equal(t, "CAD ", currencySymbol("CAD"))
}

// --- Savings achievement ---

func TestSavingsComponent(t *testing.T) {
	tests := []struct {
		name      string
		actual    int64
		target    int64
		wantScore float64
	}{
		{"at target", 60000, 60000, 25},
		{"above target capped", 90000, 60000, 25},
		{"half", 30000, 60000, 12.5},
		{"zero actual", 0, 60000, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _ := savingsComponent(tt.actual, tt.target, weightSavings, "$")
			assert.InDelta(t, tt.wantScore, score, 0.0001)
		})
	}

	_, detail := savingsComponent(42000, 60000, weightSavings, "$")
	assert.Equal(t, "Saved $420 of $600 target", detail)
}

// --- Budget adherence ---

func TestBudgetComponent(t *testing.T) {
	tests := []struct {
		name           string
		edActual       int64
		combinedTarget int64
		wantScore      float64
	}{
		{"under", 200000, 240000, 25},
		{"exactly 100pct", 240000, 240000, 25},
		{"125pct linear", 300000, 240000, 12.5},
		{"150pct floor", 360000, 240000, 0},
		{"200pct clamped", 480000, 240000, 0},
		{"zero target no spend (E1)", 0, 0, 25},
		{"zero target with spend (E1)", 50000, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _ := budgetComponent(tt.edActual, tt.combinedTarget, weightBudget, "$")
			assert.InDelta(t, tt.wantScore, score, 0.0001)
		})
	}

	_, detail := budgetComponent(248000, 240000, weightBudget, "$")
	assert.Equal(t, "Spent $2,480 of $2,400 plan", detail)
}

// --- Allocation balance ---

func TestAllocationComponent_WorkedChecks(t *testing.T) {
	// Perfect match -> full marks, balanced detail.
	score, detail, _ := allocationComponent(50000, 30000, 20000, 50, 30, 20, false, weightAlloc)
	assert.InDelta(t, 30, score, 0.0001)
	assert.Equal(t, allocBalancedDetail, detail)

	// Target 50/30/20, actual proportions 50/40/10 -> wdev 0.20 -> 30*0.8 = 24.
	score, detail, _ = allocationComponent(50000, 40000, 10000, 50, 30, 20, false, weightAlloc)
	assert.InDelta(t, 24, score, 0.0001)
	assert.Equal(t, "Desires 10 pts over target share", detail)

	// Target 50/30/20, actual 50/50/00 -> wdev 0.40 -> 30*0.6 = 18.
	score, _, _ = allocationComponent(50000, 50000, 0, 50, 30, 20, false, weightAlloc)
	assert.InDelta(t, 18, score, 0.0001)
}

func TestAllocationComponent_ZeroSpendFullMarks(t *testing.T) {
	// no spend over the surviving categories -> full marks.
	score, detail, devs := allocationComponent(0, 0, 0, 50, 30, 20, false, weightAlloc)
	assert.InDelta(t, 30, score, 0.0001)
	assert.Equal(t, allocBalancedDetail, detail)
	assert.Nil(t, devs)
}

func TestAllocationComponent_SavingsDroppedExcludesSavings(t *testing.T) {
	// Perfect E/D-only match -> full marks.
	score, _, devs := allocationComponent(50000, 50000, 0, 50, 50, 0, true, weightAlloc)
	assert.InDelta(t, 30, score, 0.0001)
	assert.Len(t, devs, 2, "savings must not be a surviving category when dropped")

	// with savings dropped, the savings actual must not enter the denominator.
	// Two calls that differ only in savingsActual must produce identical scores.
	scoreNoSavings, _, _ := allocationComponent(50000, 50000, 0, 50, 49, 1, true, weightAlloc)
	scoreHugeSavings, _, _ := allocationComponent(50000, 50000, 999999, 50, 49, 1, true, weightAlloc)
	assert.Equal(t, scoreNoSavings, scoreHugeSavings,
		"savings actual must be excluded from the allocation denominator when dropped")
}

// --- Weight resolution: division renorm over the present set ---

func TestResolveWeights_AllPresent(t *testing.T) {
	w := resolveWeights(true, true)
	assert.False(t, w.savingsDropped)
	assert.False(t, w.stabilityDropped)
	assert.Equal(t, int32(25), w.maxSavings)
	assert.Equal(t, int32(25), w.maxBudget)
	assert.Equal(t, int32(30), w.maxAlloc)
	assert.Equal(t, int32(20), w.maxStability)
	assert.Equal(t, int32(100), w.maxSavings+w.maxBudget+w.maxAlloc+w.maxStability)
}

func TestResolveWeights_SavingsDropped(t *testing.T) {
	// Renorm over {budget 25, alloc 30, stability 20}, denom 75 -> 33/40/27.
	w := resolveWeights(false, true)
	assert.True(t, w.savingsDropped)
	assert.Equal(t, int32(0), w.maxSavings)
	assert.Equal(t, int32(33), w.maxBudget)
	assert.Equal(t, int32(40), w.maxAlloc)
	assert.Equal(t, int32(27), w.maxStability)
	assert.Equal(t, int32(100), w.maxBudget+w.maxAlloc+w.maxStability)
}

func TestResolveWeights_StabilityDropped(t *testing.T) {
	// Building baseline: renorm over {savings 25, budget 25, alloc 30}, denom 80
	// -> 31/31/38.
	w := resolveWeights(true, false)
	assert.False(t, w.savingsDropped)
	assert.True(t, w.stabilityDropped)
	assert.Equal(t, int32(31), w.maxSavings)
	assert.Equal(t, int32(31), w.maxBudget)
	assert.Equal(t, int32(38), w.maxAlloc)
	assert.Equal(t, int32(0), w.maxStability)
	assert.Equal(t, int32(100), w.maxSavings+w.maxBudget+w.maxAlloc)
}

func TestResolveWeights_BothDropped(t *testing.T) {
	// Renorm over {budget 25, alloc 30}, denom 55 -> 45/55.
	w := resolveWeights(false, false)
	assert.True(t, w.savingsDropped)
	assert.True(t, w.stabilityDropped)
	assert.Equal(t, int32(0), w.maxSavings)
	assert.Equal(t, int32(45), w.maxBudget)
	assert.Equal(t, int32(55), w.maxAlloc)
	assert.Equal(t, int32(0), w.maxStability)
	assert.Equal(t, int32(100), w.maxBudget+w.maxAlloc)
}

func TestRoundClamp(t *testing.T) {
	assert.Equal(t, int32(30), roundClamp(29.6, 30))
	assert.Equal(t, int32(30), roundClamp(30.4, 30), "clamps to max")
	assert.Equal(t, int32(0), roundClamp(-1.0, 30), "clamps to zero")
	assert.Equal(t, int32(16), roundClamp(15.5, 30), "rounds half away from zero")
	assert.Equal(t, int32(15), roundClamp(15.4, 30))
}

// --- Aggregate via ComputeHealthScore ---

func TestComputeHealthScore_BuildingBaseline(t *testing.T) {
	// Fewer than stabilityMinMonths of history -> stability dropped, three
	// components renormalized to 31/31/38 (building baseline).
	period := healthPeriod(300000, 50, 30, 20)
	expenses := []ExpenseData{
		healthExpense("essentials", 140000),
		healthExpense("desires", 80000),
		healthExpense("savings", 40000),
	}

	score := ComputeHealthScore(period, expenses, nil, 2026, 5, currentMonthNow, "USD")

	assert.Len(t, score.Components, 3, "stability is dropped without enough history")
	// savings 21, budget 31, allocation 35 -> total 87 (green).
	var sum int32
	for _, component := range score.Components {
		sum += component.Score
	}
	assert.Equal(t, sum, score.Total, "total must equal the sum of components")
	assert.Equal(t, int32(87), score.Total)
	assert.Equal(t, model.HealthBandGreen, score.Band)
	assert.Equal(t, model.FormulaVersion, score.FormulaVersion)
	assert.True(t, score.Provisional, "current month is provisional")
	assert.Equal(t, model.HealthKeySavings, score.Components[0].Key)
	assert.Equal(t, model.HealthKeyBudget, score.Components[1].Key)
	assert.Equal(t, model.HealthKeyAllocation, score.Components[2].Key)
}

func TestComputeHealthScore_FourComponentsWithHistory(t *testing.T) {
	// With >= stabilityMinMonths of steady desires history, stability is present
	// and the four weights are 25/25/30/20.
	period := healthPeriod(300000, 50, 30, 20)
	expenses := []ExpenseData{
		healthExpense("essentials", 140000),
		healthExpense("desires", 80000),
		healthExpense("savings", 40000),
	}

	score := ComputeHealthScore(period, expenses, steadyWindow, 2026, 5, currentMonthNow, "USD")

	assert.Len(t, score.Components, 4)
	assert.Equal(t, model.HealthKeySavings, score.Components[0].Key)
	assert.Equal(t, model.HealthKeyBudget, score.Components[1].Key)
	assert.Equal(t, model.HealthKeyAllocation, score.Components[2].Key)
	assert.Equal(t, model.HealthKeyStability, score.Components[3].Key)
	assert.Equal(t, int32(25), score.Components[0].Max)
	assert.Equal(t, int32(25), score.Components[1].Max)
	assert.Equal(t, int32(30), score.Components[2].Max)
	assert.Equal(t, int32(20), score.Components[3].Max)

	// savings 17, budget 25, allocation 28, stability 20 (steady) -> total 90.
	var sum int32
	for _, component := range score.Components {
		sum += component.Score
	}
	assert.Equal(t, sum, score.Total, "total must equal the sum of components")
	assert.Equal(t, int32(90), score.Total)
	assert.Equal(t, int32(20), score.Components[3].Score, "steady desires earn full stability marks")
	assert.Equal(t, model.HealthBandGreen, score.Band)
}

func TestComputeHealthScore_TotalWithinBounds(t *testing.T) {
	period := healthPeriod(300000, 50, 30, 20)
	// Extreme overspend everywhere still stays in [0,100].
	expenses := []ExpenseData{
		healthExpense("essentials", 900000),
		healthExpense("desires", 900000),
	}
	score := ComputeHealthScore(period, expenses, steadyWindow, 2026, 5, currentMonthNow, "USD")
	assert.GreaterOrEqual(t, score.Total, int32(0))
	assert.LessOrEqual(t, score.Total, int32(100))
}

func TestComputeHealthScore_Deterministic(t *testing.T) {
	period := healthPeriod(300000, 50, 30, 20)
	expenses := []ExpenseData{
		healthExpense("essentials", 140000),
		healthExpense("desires", 80000),
		healthExpense("savings", 40000),
	}
	window := []int64{70000, 90000, 80000}
	first := ComputeHealthScore(period, expenses, window, 2026, 5, currentMonthNow, "USD")
	second := ComputeHealthScore(period, expenses, window, 2026, 5, currentMonthNow, "USD")
	assert.True(t, reflect.DeepEqual(first, second), "same inputs must yield identical scores")
}

func TestComputeHealthScore_ProvisionalFlipsWithClock(t *testing.T) {
	period := healthPeriod(300000, 50, 30, 20)
	expenses := []ExpenseData{healthExpense("savings", 60000)}

	pastNow := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	closed := ComputeHealthScore(period, expenses, steadyWindow, 2026, 5, pastNow, "USD")
	assert.False(t, closed.Provisional, "a past month is not provisional")

	current := ComputeHealthScore(period, expenses, steadyWindow, 2026, 5, currentMonthNow, "USD")
	assert.True(t, current.Provisional)

	// Determinism contract: only provisional differs when the month closes.
	assert.Equal(t, current.Total, closed.Total)
	assert.Equal(t, current.Band, closed.Band)
}

func TestComputeHealthScore_NoExpenses(t *testing.T) {
	// configured budget, no expenses, building baseline -> savings 0,
	// budget full (31), allocation full (38) -> total 69, driver savings.
	period := healthPeriod(300000, 50, 30, 20)
	score := ComputeHealthScore(period, nil, nil, 2026, 5, currentMonthNow, "USD")

	assert.Equal(t, int32(69), score.Total)
	assert.Equal(t, model.HealthBandAmber, score.Band)
	assert.Equal(t, model.HealthKeySavings, score.Insight.Driver)
}

func TestComputeHealthScore_SavingsDroppedStructure(t *testing.T) {
	// savings_percent = 0, building baseline -> two components with maxes
	// 45 and 55 (renorm over budget 25 + allocation 30).
	period := healthPeriod(300000, 60, 40, 0)
	expenses := []ExpenseData{
		healthExpense("essentials", 240000),
		healthExpense("desires", 120000),
	}
	score := ComputeHealthScore(period, expenses, nil, 2026, 5, currentMonthNow, "USD")

	assert.Len(t, score.Components, 2)
	assert.Equal(t, model.HealthKeyBudget, score.Components[0].Key)
	assert.Equal(t, model.HealthKeyAllocation, score.Components[1].Key)
	assert.Equal(t, int32(45), score.Components[0].Max)
	assert.Equal(t, int32(55), score.Components[1].Max)

	var sum int32
	for _, component := range score.Components {
		sum += component.Score
	}
	assert.Equal(t, sum, score.Total)
}

func TestComputeHealthScore_SavingsDroppedWithStability(t *testing.T) {
	// savings_percent = 0 but enough history -> three components renormalized over
	// {budget 25, alloc 30, stability 20} -> maxes 33/40/27.
	period := healthPeriod(300000, 60, 40, 0)
	expenses := []ExpenseData{
		healthExpense("essentials", 180000),
		healthExpense("desires", 120000),
	}
	score := ComputeHealthScore(period, expenses, steadyWindow, 2026, 5, currentMonthNow, "USD")

	assert.Len(t, score.Components, 3)
	assert.Equal(t, model.HealthKeyBudget, score.Components[0].Key)
	assert.Equal(t, model.HealthKeyAllocation, score.Components[1].Key)
	assert.Equal(t, model.HealthKeyStability, score.Components[2].Key)
	assert.Equal(t, int32(33), score.Components[0].Max)
	assert.Equal(t, int32(40), score.Components[1].Max)
	assert.Equal(t, int32(27), score.Components[2].Max)

	var sum int32
	for _, component := range score.Components {
		sum += component.Score
	}
	assert.Equal(t, sum, score.Total)
}

func TestComputeHealthScore_TinyBudgetDropsSavings(t *testing.T) {
	// savings_percent > 0 but savingsTarget integer-divides to 0 -> savings
	// dropped, allocation over E/D only. budget 99, 50/49/1 -> savingsTarget = 0,
	// combinedTarget = 98 (budget still scored).
	period := healthPeriod(99, 50, 49, 1)
	expenses := []ExpenseData{
		healthExpense("essentials", 50),
		healthExpense("desires", 49),
	}
	score := ComputeHealthScore(period, expenses, nil, 2026, 5, currentMonthNow, "USD")

	assert.Len(t, score.Components, 2, "tiny-budget savingsTarget=0 drops the savings component")
	assert.Equal(t, model.HealthKeyBudget, score.Components[0].Key)
	assert.Equal(t, model.HealthKeyAllocation, score.Components[1].Key)
}

func TestComputeHealthScore_AllAtMax(t *testing.T) {
	// Savings dropped, building baseline, spending exactly on plan and perfectly
	// allocated -> every present component at max -> all-on-plan insight.
	period := healthPeriod(300000, 60, 40, 0)
	expenses := []ExpenseData{
		healthExpense("essentials", 180000),
		healthExpense("desires", 120000),
	}
	score := ComputeHealthScore(period, expenses, nil, 2026, 5, currentMonthNow, "USD")

	assert.Equal(t, int32(100), score.Total)
	assert.Equal(t, "You're on plan across the board.", score.Insight.Summary)
	assert.Equal(t, "", score.Insight.Driver)
}
