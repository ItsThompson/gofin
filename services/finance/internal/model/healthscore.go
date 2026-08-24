package model

// FormulaVersion is the health-score formula version. A closed month recomputes
// to the same numbers only within the same version. It is serialized into the
// response so persisted or recomputed scores stay comparable across formula
// changes. This version includes the spending-stability sub-score in the weighting.
const FormulaVersion int32 = 3

// Health-score component keys. These are domain truths shared by the compute
// logic, the insight driver, and the frontend, so they live on the model.
const (
	HealthKeySavings    = "savings_achievement"
	HealthKeyBudget     = "budget_adherence"
	HealthKeyAllocation = "allocation_balance"
	HealthKeyStability  = "spending_stability"
)

// Health-score band names.
const (
	HealthBandGreen = "green"
	HealthBandAmber = "amber"
	HealthBandRed   = "red"
)

// Band thresholds (inclusive lower bounds on the total).
const (
	healthBandGreenMin int32 = 80
	healthBandAmberMin int32 = 55
)

// HealthComponent is one contributing sub-score (savings, budget adherence, or
// allocation balance). Score is in [0, Max]; Max is the component's weight.
type HealthComponent struct {
	Key    string `json:"key"`
	Score  int32  `json:"score"`
	Max    int32  `json:"max"`
	Detail string `json:"detail"`
}

// HealthInsight is the rules-based plain-English read. Driver names the lowest
// present component (empty when every component is at its max).
type HealthInsight struct {
	Summary string `json:"summary"`
	Driver  string `json:"driver"`
	Nudge   string `json:"nudge"`
}

// HealthScore is the monthly financial health score returned by
// GET /api/finance/health-score. All amounts feeding it are minor units
// (cents). Total is the sum of the present component scores and is structurally
// in [0,100]. ConfigureBudget is set (with the numeric fields left zero) when
// the period has no budget configured.
type HealthScore struct {
	Year              int32             `json:"year"`
	Month             int32             `json:"month"`
	Total             int32             `json:"total"`
	Band              string            `json:"band"`
	Provisional       bool              `json:"provisional"`
	FormulaVersion    int32             `json:"formulaVersion"`
	ReportingCurrency string            `json:"reportingCurrency"`
	Components        []HealthComponent `json:"components"`
	Insight           HealthInsight     `json:"insight"`
	ConfigureBudget   bool              `json:"configureBudget,omitempty"`
}

// HealthScoreResponse is the JSON body returned for GET /api/finance/health-score.
type HealthScoreResponse struct {
	HealthScore *HealthScore `json:"healthScore"`
}

// HealthScoreTrendPoint is one month in the health-score trend sparkline. It
// carries only the denormalized scalars the sparkline needs, not the full
// component breakdown, so the trend read stays cheap.
type HealthScoreTrendPoint struct {
	Year              int32  `json:"year"`
	Month             int32  `json:"month"`
	Total             int32  `json:"total"`
	Band              string `json:"band"`
	Provisional       bool   `json:"provisional"`
	FormulaVersion    int32  `json:"formulaVersion"`
	ReportingCurrency string `json:"reportingCurrency"`
}

// HealthScoreTrendResponse is the JSON body returned for
// GET /api/finance/health-score/trend.
type HealthScoreTrendResponse struct {
	Trends []HealthScoreTrendPoint `json:"trends"`
}

// Band maps a total to its color band. Green >= 80, amber 55-79, red <= 54.
func Band(total int32) string {
	switch {
	case total >= healthBandGreenMin:
		return HealthBandGreen
	case total >= healthBandAmberMin:
		return HealthBandAmber
	default:
		return HealthBandRed
	}
}
