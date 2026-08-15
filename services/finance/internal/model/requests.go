package model

// OnboardingRequest is the input for POST /api/finance/onboarding.
// Note: percentage fields intentionally omit binding:"required" because Gin's
// validator treats the int32 zero value as "missing", which would reject valid
// splits like 100/0/0. The sum-to-100 constraint is enforced by ValidateEDSSplit.
type OnboardingRequest struct {
	BudgetAmount      int64  `json:"budgetAmount"`
	EssentialsPercent int32  `json:"essentialsPercent"`
	DesiresPercent    int32  `json:"desiresPercent"`
	SavingsPercent    int32  `json:"savingsPercent"`
	Currency          string `json:"currency" binding:"required"`
}

// DefaultsResponse is the JSON body returned for default settings endpoints.
type DefaultsResponse struct {
	Defaults *DefaultSettings `json:"defaults"`
}

// CreatePeriodRequest is the input for POST /api/finance/periods.
// Note: percentage fields intentionally omit binding:"required" because Gin's
// validator treats the int32 zero value as "missing", which would reject valid
// splits like 100/0/0. The sum-to-100 constraint is enforced by ValidateEDSSplit.
// BudgetAmount is a pointer so binding:"required" rejects an absent field
// without rejecting a legitimate $0 budget (the int64 zero value).
type CreatePeriodRequest struct {
	Year              int32  `json:"year" binding:"required"`
	Month             int32  `json:"month" binding:"required"`
	BudgetAmount      *int64 `json:"budgetAmount" binding:"required"`
	ReportingCurrency string `json:"reportingCurrency" binding:"required"`
	EssentialsPercent int32  `json:"essentialsPercent"`
	DesiresPercent    int32  `json:"desiresPercent"`
	SavingsPercent    int32  `json:"savingsPercent"`
}

// PeriodResponse is the JSON body returned for period endpoints.
type PeriodResponse struct {
	Period *BudgetPeriod `json:"period"`
}

// PeriodListResponse is the JSON body returned for listing periods.
type PeriodListResponse struct {
	Periods []*BudgetPeriod `json:"periods"`
}

// TagListResponse is the JSON body returned for listing tags.
type TagListResponse struct {
	Tags []*Tag `json:"tags"`
}

// CreateTagRequest is the input for POST /api/finance/tags.
type CreateTagRequest struct {
	Name string `json:"name" binding:"required"`
}

// TagResponse is the JSON body returned for single tag endpoints.
type TagResponse struct {
	Tag *Tag `json:"tag"`
}

// UpdateTagRequest is the input for PUT /api/finance/tags/:id.
type UpdateTagRequest struct {
	Name string `json:"name" binding:"required"`
}

// UpdateDefaultsRequest is the input for PUT /api/finance/defaults.
// Note: percentage fields intentionally omit binding:"required" because Gin's
// validator treats the int32 zero value as "missing", which would reject valid
// splits like 100/0/0. The sum-to-100 constraint is enforced by ValidateEDSSplit.
type UpdateDefaultsRequest struct {
	BudgetAmount      int64  `json:"budgetAmount"`
	EssentialsPercent int32  `json:"essentialsPercent"`
	DesiresPercent    int32  `json:"desiresPercent"`
	SavingsPercent    int32  `json:"savingsPercent"`
	Currency          string `json:"currency" binding:"required"`
}

// SummaryResponse is the JSON body returned for GET /api/finance/summary.
type SummaryResponse struct {
	Summary *PeriodSummary `json:"summary"`
}

// TagSpendingResponse is the JSON body returned for GET /api/finance/spending/by-tag.
type TagSpendingResponse struct {
	TagSpending []TagSpending `json:"tagSpending"`
}

// CumulativeSpendResponse is the JSON body returned for GET /api/finance/spending/cumulative.
type CumulativeSpendResponse struct {
	Points []CumulativeSpendPoint `json:"points"`
}

// UpdatePeriodRequest is the input for PUT /api/finance/periods/:id.
// Note: percentage fields intentionally omit binding:"required" because Gin's
// validator treats the int32 zero value as "missing", which would reject valid
// splits like 100/0/0. The sum-to-100 constraint is enforced by ValidateEDSSplit.
type UpdatePeriodRequest struct {
	BudgetAmount      int64 `json:"budgetAmount"`
	EssentialsPercent int32 `json:"essentialsPercent"`
	DesiresPercent    int32 `json:"desiresPercent"`
	SavingsPercent    int32 `json:"savingsPercent"`
}

// HistoricalComparison is the response for the historical comparison widget.
type HistoricalComparison struct {
	// CurrentSpent is the total spent in the requested period (cents).
	CurrentSpent int64 `json:"currentSpent"`
	// PreviousSpent is the total spent in the period before (cents).
	PreviousSpent int64 `json:"previousSpent"`
	// PreviousReportingCurrency is the reporting currency of the previous period.
	// Empty when there is no previous period. When it differs from the current
	// period's reporting currency, ChangePercent and the amount delta are not
	// comparable and the frontend must guard the display.
	PreviousReportingCurrency string `json:"previousReportingCurrency"`
	// Comparable is false when the current and previous periods have different
	// reporting currencies, making amount comparisons invalid.
	Comparable bool `json:"comparable"`
	// RollingAverage is the average of the last 3 periods' totalSpent. Null if < 3 periods.
	RollingAverage *int64 `json:"rollingAverage"`
	// ChangePercent is the percentage change from previous period.
	// Only meaningful when Comparable is true.
	ChangePercent float64 `json:"changePercent"`
}

// HistoricalComparisonResponse is the JSON body returned for GET /api/finance/spending/comparison.
type HistoricalComparisonResponse struct {
	Comparison *HistoricalComparison `json:"comparison"`
}

// TrendPoint is a single monthly data point in the spending trends chart.
type TrendPoint struct {
	Year              int32   `json:"year"`
	Month             int32   `json:"month"`
	TotalSpent        int64   `json:"totalSpent"`
	BudgetAmount      int64   `json:"budgetAmount"`
	EssentialsSpent   int64   `json:"essentialsSpent"`
	DesiresSpent      int64   `json:"desiresSpent"`
	SavingsSpent      int64   `json:"savingsSpent"`
	EssentialsPercent float64 `json:"essentialsPercent"`
	DesiresPercent    float64 `json:"desiresPercent"`
	SavingsPercent    float64 `json:"savingsPercent"`
	// ReportingCurrency is the period's reporting currency for this data point.
	// Each trend point may have a different currency when the user has mixed-
	// currency periods; the frontend uses it to format each point correctly.
	ReportingCurrency string `json:"reportingCurrency"`
}

// TrendResponse is the JSON body returned for GET /api/finance/spending/trends.
type TrendResponse struct {
	Trends []TrendPoint `json:"trends"`
}
