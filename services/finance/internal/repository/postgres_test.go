package repository

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/db"
	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

// fullHealthScore is a complete v2 score (components AND insight), used to verify
// the JSONB round-trip preserves everything, especially the insight.
func fullHealthScore() *model.HealthScore {
	return &model.HealthScore{
		Year: 2026, Month: 3, Total: 77, Band: model.HealthBandAmber,
		Provisional: false, FormulaVersion: model.FormulaVersion,
		Components: []model.HealthComponent{
			{Key: model.HealthKeySavings, Score: 13, Max: 25, Detail: "Saved $300 of $600 target"},
			{Key: model.HealthKeyBudget, Score: 25, Max: 25, Detail: "Spent $2,350 of $2,400 plan"},
			{Key: model.HealthKeyAllocation, Score: 25, Max: 30, Detail: "Desires 6 pts over target share"},
			{Key: model.HealthKeyStability, Score: 14, Max: 20, Detail: "Desires spend varied ~29% month to month"},
		},
		Insight: model.HealthInsight{
			Summary: "Savings is the softest score this month.",
			Driver:  model.HealthKeySavings,
			Nudge:   "Move an extra $300 to savings to reach your target and lift your score about 12 points.",
		},
	}
}

// TestHealthScoreJSONBRoundTrip is the DB-free half of the persistence check: the score column is
// stored as marshaled JSON (UpsertHealthScore) and read back via
// dbHealthScoreToModel, so marshaling then unmarshaling must reproduce the full
// score, insight included. A live-Postgres deep-equal upsert/read is gated on
// TEST_DATABASE_URL (see services/dbmigrate for the gated-integration pattern).
func TestHealthScoreJSONBRoundTrip(t *testing.T) {
	score := fullHealthScore()

	payload, err := json.Marshal(score)
	require.NoError(t, err)

	got, err := dbHealthScoreToModel(db.FinanceHealthScore{
		Year:           score.Year,
		Month:          score.Month,
		Total:          score.Total,
		Band:           score.Band,
		Score:          payload,
		FormulaVersion: score.FormulaVersion,
	})
	require.NoError(t, err)

	assert.Equal(t, score, got, "JSONB round-trip must preserve the full score")
	assert.Equal(t, score.Insight, got.Insight, "insight must survive the round-trip (resolves B1)")
	require.Len(t, got.Components, 4, "all components survive the round-trip")
}
