package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStabilityComponent_Steady(t *testing.T) {
	// Zero variation -> full marks and the steady detail.
	score, detail, present := stabilityComponent([]int64{80000, 80000, 80000}, weightStability)
	assert.True(t, present)
	assert.InDelta(t, weightStability, score, 0.0001)
	assert.Equal(t, stabilitySteadyDetail, detail)
}

func TestStabilityComponent_Volatile(t *testing.T) {
	// CoV at or beyond stabilityCoVCap (1.0) floors the score at 0.
	score, _, present := stabilityComponent([]int64{0, 0, 300000}, weightStability)
	assert.True(t, present)
	assert.InDelta(t, 0, score, 0.0001)
}

func TestStabilityComponent_ModerateVariation(t *testing.T) {
	// A modest spread scores between 0 and full, with a percentage detail.
	score, detail, present := stabilityComponent([]int64{90000, 100000, 110000}, weightStability)
	assert.True(t, present)
	assert.Greater(t, score, 0.0)
	assert.Less(t, score, weightStability)
	assert.Contains(t, detail, "% month to month")
}

func TestStabilityComponent_MeanZeroFullMarks(t *testing.T) {
	// All-zero desires -> treated as perfectly steady (no divide-by-zero).
	score, detail, present := stabilityComponent([]int64{0, 0, 0}, weightStability)
	assert.True(t, present)
	assert.InDelta(t, weightStability, score, 0.0001)
	assert.Equal(t, stabilitySteadyDetail, detail)
}

func TestStabilityComponent_HistoryGate(t *testing.T) {
	_, _, presentTwo := stabilityComponent([]int64{80000, 80000}, weightStability)
	assert.False(t, presentTwo, "fewer than stabilityMinMonths is dropped")

	_, _, presentThree := stabilityComponent([]int64{80000, 80000, 80000}, weightStability)
	assert.True(t, presentThree, "exactly stabilityMinMonths is present")
}
