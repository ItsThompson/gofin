package service

import (
	"fmt"
	"math"
)

// categoryDev is one E/D/S category's signed proportional deviation from its
// target share (actualProp - targetProp). Positive means over its target share.
type categoryDev struct {
	label string
	dev   float64
}

const allocBalancedDetail = "Balanced across categories"

// savingsComponent computes savings achievement (Formula B). The caller drops
// this component when savingsTarget is 0, so savingsTarget is > 0 here. The
// ratio caps at 1.0: beating the target earns full marks, never a bonus.
func savingsComponent(savingsActual, savingsTarget int64, weight float64, symbol string) (float64, string) {
	ratio := float64(savingsActual) / float64(savingsTarget)
	if ratio > 1.0 {
		ratio = 1.0
	}
	scoreFloat := weight * ratio
	detail := fmt.Sprintf("Saved %s of %s target", formatMoney(savingsActual, symbol), formatMoney(savingsTarget, symbol))
	return scoreFloat, detail
}

// budgetComponent computes budget adherence (Formula C). Spending at or under
// the combined essentials+desires target earns full marks; the factor ramps
// linearly to 0 at budgetFloorRatio (150%) of target and stays 0 beyond it.
func budgetComponent(edActual, combinedTarget int64, weight float64, symbol string) (float64, string) {
	scoreFloat := weight * budgetFactor(edActual, combinedTarget)
	detail := fmt.Sprintf("Spent %s of %s plan", formatMoney(edActual, symbol), formatMoney(combinedTarget, symbol))
	return scoreFloat, detail
}

// budgetFactor is the adherence factor in [0,1]. With no E/D budget only zero
// spend is on plan; otherwise it is full at or under target and ramps linearly
// to 0 at budgetFloorRatio.
func budgetFactor(edActual, combinedTarget int64) float64 {
	if combinedTarget == 0 {
		if edActual == 0 {
			return 1.0
		}
		return 0.0
	}
	ratio := float64(edActual) / float64(combinedTarget)
	switch {
	case ratio <= 1.0:
		return 1.0
	case ratio >= budgetFloorRatio:
		return 0.0
	default:
		return (budgetFloorRatio - ratio) / (budgetFloorRatio - 1.0)
	}
}

// allocationComponent computes allocation balance (Formula A). It compares
// actual category proportions against target proportions over the surviving
// categories (essentials/desires only when savings is dropped, else all three),
// applies the asymmetric penalty coefficients, and returns the score, a detail
// string, and the per-category deviations (reused by the insight builder). Zero
// spend over the surviving categories earns full marks (no drift yet).
func allocationComponent(
	essentialsActual, desiresActual, savingsActual int64,
	essentialsPct, desiresPct, savingsPct int32,
	savingsDropped bool,
	weight float64,
) (float64, string, []categoryDev) {
	percentSum := essentialsPct + desiresPct
	spendDenom := essentialsActual + desiresActual
	if !savingsDropped {
		percentSum += savingsPct
		spendDenom += savingsActual
	}

	if spendDenom == 0 {
		return weight, allocBalancedDetail, nil
	}

	targetProp := func(pct int32) float64 { return float64(pct) / float64(percentSum) }
	actualProp := func(actual int64) float64 { return float64(actual) / float64(spendDenom) }

	devEssentials := actualProp(essentialsActual) - targetProp(essentialsPct)
	devDesires := actualProp(desiresActual) - targetProp(desiresPct)

	wdev := coefEssentials * math.Abs(devEssentials)
	if devDesires > 0 {
		wdev += coefDesiresOver * math.Abs(devDesires)
	} else {
		wdev += coefDesiresUnder * math.Abs(devDesires)
	}

	devs := []categoryDev{{label: "Essentials", dev: devEssentials}, {label: "Desires", dev: devDesires}}

	if !savingsDropped {
		devSavings := actualProp(savingsActual) - targetProp(savingsPct)
		if devSavings < 0 {
			wdev += coefSavingsUnder * math.Abs(devSavings)
		} else {
			wdev += coefSavingsOver * math.Abs(devSavings)
		}
		devs = append(devs, categoryDev{label: "Savings", dev: devSavings})
	}

	normalized := wdev / allocDevCap
	if normalized > 1.0 {
		normalized = 1.0
	}
	scoreFloat := weight * (1.0 - normalized)
	return scoreFloat, allocationDetail(devs), devs
}

// allocationDetail names the category most over its target share, or reports a
// balanced allocation when no category is meaningfully over (rounds to 0 pts).
func allocationDetail(devs []categoryDev) string {
	over := maxPositiveDev(devs)
	if over == nil {
		return allocBalancedDetail
	}
	points := int(math.Round(over.dev * 100))
	if points <= 0 {
		return allocBalancedDetail
	}
	return fmt.Sprintf("%s %d pts over target share", over.label, points)
}

// maxPositiveDev returns the category most over its target share, or nil when
// none is over.
func maxPositiveDev(devs []categoryDev) *categoryDev {
	var best *categoryDev
	for i := range devs {
		if devs[i].dev > 0 && (best == nil || devs[i].dev > best.dev) {
			best = &devs[i]
		}
	}
	return best
}

// minNegativeDev returns the category most under its target share, or nil when
// none is under.
func minNegativeDev(devs []categoryDev) *categoryDev {
	var best *categoryDev
	for i := range devs {
		if devs[i].dev < 0 && (best == nil || devs[i].dev < best.dev) {
			best = &devs[i]
		}
	}
	return best
}
