package service

import (
	"fmt"
	"math"
)

const stabilitySteadyDetail = "Desires spend held steady month to month"

// stabilityComponent computes the spending-stability sub-score (Formula G) from a
// window of recent closed months' discretionary (desires) totals. Stability
// rewards a low coefficient of variation (CoV = sample standard deviation /
// mean): steady month-to-month desires spend earns full marks and the factor
// ramps linearly to 0 at stabilityCoVCap. A zero mean (no desires spend at all)
// is treated as perfectly steady. The window must hold at least
// stabilityMinMonths entries; below that the component is not present (the
// building-baseline state), reported by the present return being false.
func stabilityComponent(desiresByMonth []int64, weight float64) (float64, string, bool) {
	if len(desiresByMonth) < stabilityMinMonths {
		return 0, "", false
	}

	mean := meanInt64(desiresByMonth)
	if mean == 0 {
		return weight, stabilitySteadyDetail, true
	}

	cov := sampleStdDev(desiresByMonth, mean) / mean
	ratio := 1.0 - cov/stabilityCoVCap
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return weight * ratio, stabilityDetail(cov), true
}

// stabilityDetail renders the CoV as a rounded percentage, or the steady message
// when it rounds to zero.
func stabilityDetail(cov float64) string {
	pct := int(math.Round(cov * 100))
	if pct <= 0 {
		return stabilitySteadyDetail
	}
	return fmt.Sprintf("Desires spend varied ~%d%% month to month", pct)
}

// meanInt64 returns the arithmetic mean of the values as a float. len(values) > 0
// is guaranteed by the stabilityMinMonths gate in the sole caller.
func meanInt64(values []int64) float64 {
	var sum int64
	for _, value := range values {
		sum += value
	}
	return float64(sum) / float64(len(values))
}

// sampleStdDev returns the sample standard deviation (n-1 denominator) of the
// values about the supplied mean. len(values) >= 2 is guaranteed by the
// stabilityMinMonths gate in the sole caller.
func sampleStdDev(values []int64, mean float64) float64 {
	var sumSquares float64
	for _, value := range values {
		delta := float64(value) - mean
		sumSquares += delta * delta
	}
	return math.Sqrt(sumSquares / float64(len(values)-1))
}
