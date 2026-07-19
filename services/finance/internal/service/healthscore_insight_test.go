package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// --- Insight driver selection & tie-break (Formula F, E4) ---

func TestSelectDriver_LowestWithPrecedence(t *testing.T) {
	// Savings and budget tie at the min -> savings wins (precedence).
	tied := []model.HealthComponent{
		{Key: model.HealthKeySavings, Score: 15, Max: 30},
		{Key: model.HealthKeyBudget, Score: 15, Max: 30},
		{Key: model.HealthKeyAllocation, Score: 40, Max: 40},
	}
	assert.Equal(t, model.HealthKeySavings, selectDriver(tied).Key)

	// Savings dropped: budget and allocation tie -> budget wins.
	dropped := []model.HealthComponent{
		{Key: model.HealthKeyBudget, Score: 20, Max: 43},
		{Key: model.HealthKeyAllocation, Score: 20, Max: 57},
	}
	assert.Equal(t, model.HealthKeyBudget, selectDriver(dropped).Key)

	// Allocation uniquely lowest.
	allocLowest := []model.HealthComponent{
		{Key: model.HealthKeySavings, Score: 30, Max: 30},
		{Key: model.HealthKeyBudget, Score: 25, Max: 30},
		{Key: model.HealthKeyAllocation, Score: 20, Max: 40},
	}
	assert.Equal(t, model.HealthKeyAllocation, selectDriver(allocLowest).Key)
}

// --- Savings-driver insight (exact gap and points) ---

func TestComputeHealthScore_SavingsDriverNudge(t *testing.T) {
	period := healthPeriod(300000, 50, 30, 20)
	expenses := []ExpenseData{
		healthExpense("essentials", 130000),
		healthExpense("desires", 90000),
		healthExpense("savings", 30000),
	}
	score := ComputeHealthScore(period, expenses, 2026, 5, currentMonthNow, "USD")

	assert.Equal(t, int32(79), score.Total)
	assert.Equal(t, model.HealthBandAmber, score.Band)
	assert.Equal(t, model.HealthKeySavings, score.Insight.Driver)
	assert.Equal(t, "Savings is the softest score this month.", score.Insight.Summary)
	assert.Equal(t,
		"Move an extra $300 to savings to reach your target and lift your score about 15 points.",
		score.Insight.Nudge)
}

// --- Budget-driver insight (overspent: exact trim and points) ---

func TestComputeHealthScore_BudgetOverspentNudge(t *testing.T) {
	period := healthPeriod(300000, 50, 30, 20)
	expenses := []ExpenseData{
		healthExpense("essentials", 200000),
		healthExpense("desires", 100000),
		healthExpense("savings", 60000),
	}
	score := ComputeHealthScore(period, expenses, 2026, 5, currentMonthNow, "USD")

	assert.Equal(t, model.HealthKeyBudget, score.Insight.Driver)
	assert.Equal(t, "Budget adherence is the softest score this month.", score.Insight.Summary)
	assert.Equal(t,
		"Trim about $600 from essentials and desires to get back to plan and lift your score about 15 points.",
		score.Insight.Nudge)
}

// --- Allocation-driver insight (direction derived from deviation signs) ---

func TestComputeHealthScore_AllocationDriverNudge_DerivedDirection(t *testing.T) {
	// Essentials under, desires over -> suggest shifting toward Essentials.
	period := healthPeriod(300000, 50, 30, 20)
	expenses := []ExpenseData{
		healthExpense("essentials", 40000),
		healthExpense("desires", 150000),
		healthExpense("savings", 60000),
	}
	score := ComputeHealthScore(period, expenses, 2026, 5, currentMonthNow, "USD")

	assert.Equal(t, model.HealthKeyAllocation, score.Insight.Driver)
	assert.Equal(t, "Your category balance is the softest score this month.", score.Insight.Summary)
	assert.Equal(t,
		"Desires is running 30 pts over its target share. Shifting spend toward Essentials could recover up to 20 points.",
		score.Insight.Nudge)
}

func TestBuildInsight_AllocationDirectionFromDevSigns(t *testing.T) {
	components := []model.HealthComponent{
		{Key: model.HealthKeySavings, Score: 25, Max: 30},
		{Key: model.HealthKeyBudget, Score: 28, Max: 30},
		{Key: model.HealthKeyAllocation, Score: 15, Max: 40},
	}

	// Typical drift: desires over, savings under -> toward Savings.
	typical := buildInsight(components, insightInputs{
		maxAlloc: 40, symbol: "$",
		allocDevs: []categoryDev{{"Essentials", 0.0}, {"Desires", 0.15}, {"Savings", -0.15}},
	})
	assert.Equal(t,
		"Desires is running 15 pts over its target share. Shifting spend toward Savings could recover up to 25 points.",
		typical.Nudge)

	// Atypical drift: essentials over, desires under -> toward Desires.
	atypical := buildInsight(components, insightInputs{
		maxAlloc: 40, symbol: "$",
		allocDevs: []categoryDev{{"Essentials", 0.12}, {"Desires", -0.12}, {"Savings", 0.0}},
	})
	assert.Equal(t,
		"Essentials is running 12 pts over its target share. Shifting spend toward Desires could recover up to 25 points.",
		atypical.Nudge)
}

// --- E5: budget driver but not overspent -> qualitative fallback ---

func TestBuildInsight_BudgetDriverNotOverspent(t *testing.T) {
	components := []model.HealthComponent{
		{Key: model.HealthKeySavings, Score: 30, Max: 30},
		{Key: model.HealthKeyBudget, Score: 20, Max: 30},
		{Key: model.HealthKeyAllocation, Score: 40, Max: 40},
	}
	insight := buildInsight(components, insightInputs{
		maxBudget: 30, symbol: "$",
		edActual: 100000, combinedTarget: 240000, // not overspent
	})
	assert.Equal(t, model.HealthKeyBudget, insight.Driver)
	assert.Equal(t, "Keep essentials and desires within your plan to lift this score.", insight.Nudge)
}
