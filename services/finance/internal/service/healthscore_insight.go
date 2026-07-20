package service

import (
	"fmt"
	"math"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// insightInputs carries the raw numbers buildInsight needs beyond the rounded
// components: the cleanly invertible dollar figures (savings gap, overspend) and
// the allocation deviations used to derive the rebalance direction.
type insightInputs struct {
	savingsActual  int64
	savingsTarget  int64
	edActual       int64
	combinedTarget int64
	maxSavings     int32
	maxBudget      int32
	maxAlloc       int32
	maxStability   int32
	allocDevs      []categoryDev
	symbol         string
}

// buildInsight selects the driver (lowest present component) and templates the
// summary and nudge.
//
// The recoverable "points" figure is each driver component's own max-minus-score
// and is exact for that component. It is deliberately per-component, not a total
// delta: acting on a nudge also shifts a shared actual (adding to savings also
// moves the allocation proportions), so the realized total change differs
// slightly. The "about" hedge in every nudge keeps that honest. The dollar
// figures (gap, overspend) are exact.
func buildInsight(components []model.HealthComponent, in insightInputs) model.HealthInsight {
	if allAtMax(components) {
		return model.HealthInsight{
			Summary: "You're on plan across the board.",
			Nudge:   "Keep your spending and savings on their current track to hold this score.",
		}
	}

	driver := selectDriver(components)
	switch driver.Key {
	case model.HealthKeySavings:
		gap := in.savingsTarget - in.savingsActual
		points := in.maxSavings - driver.Score
		return model.HealthInsight{
			Summary: "Savings is the softest score this month.",
			Driver:  driver.Key,
			Nudge: fmt.Sprintf(
				"Move an extra %s to savings to reach your target and lift your score about %d points.",
				formatMoney(gap, in.symbol), points),
		}

	case model.HealthKeyBudget:
		points := in.maxBudget - driver.Score
		overspend := in.edActual - in.combinedTarget
		if overspend > 0 {
			return model.HealthInsight{
				Summary: "Budget adherence is the softest score this month.",
				Driver:  driver.Key,
				Nudge: fmt.Sprintf(
					"Trim about %s from essentials and desires to get back to plan and lift your score about %d points.",
					formatMoney(overspend, in.symbol), points),
			}
		}
		return model.HealthInsight{
			Summary: "Budget adherence is the softest score this month.",
			Driver:  driver.Key,
			Nudge:   "Keep essentials and desires within your plan to lift this score.",
		}

	case model.HealthKeyAllocation:
		points := in.maxAlloc - driver.Score
		over := maxPositiveDev(in.allocDevs)
		under := minNegativeDev(in.allocDevs)
		if over != nil && under != nil {
			pointsOver := int(math.Round(over.dev * 100))
			return model.HealthInsight{
				Summary: "Your category balance is the softest score this month.",
				Driver:  driver.Key,
				Nudge: fmt.Sprintf(
					"%s is running %d pts over its target share. Shifting spend toward %s could recover up to %d points.",
					over.label, pointsOver, under.label, points),
			}
		}
		// Defensive: unreachable when allocation is the driver below max, because
		// deviations over the surviving categories sum to 0, so an over-category
		// and an under-category always coexist.
		return model.HealthInsight{
			Summary: "Your category balance is the softest score this month.",
			Driver:  driver.Key,
			Nudge:   fmt.Sprintf("Rebalancing your categories could recover up to %d points.", points),
		}

	case model.HealthKeyStability:
		points := in.maxStability - driver.Score
		return model.HealthInsight{
			Summary: "Spending stability is the softest score this month.",
			Driver:  driver.Key,
			Nudge: fmt.Sprintf(
				"Steadier discretionary spending month to month could lift your score about %d points.", points),
		}
	}

	return model.HealthInsight{}
}

// selectDriver returns the lowest-scoring present component. Components are
// appended in precedence order (savings, budget, allocation), so
// picking the first strict minimum yields the required tie-break.
func selectDriver(components []model.HealthComponent) model.HealthComponent {
	driver := components[0]
	for _, component := range components[1:] {
		if component.Score < driver.Score {
			driver = component
		}
	}
	return driver
}

// allAtMax reports whether every present component is at its maximum.
func allAtMax(components []model.HealthComponent) bool {
	for _, component := range components {
		if component.Score < component.Max {
			return false
		}
	}
	return true
}
