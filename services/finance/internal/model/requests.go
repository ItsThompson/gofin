package model

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

type DefaultsResponse struct {
	Defaults *DefaultSettings `json:"defaults"`
}

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

type PeriodResponse struct {
	Period *BudgetPeriod `json:"period"`
}

type PeriodListResponse struct {
	Periods []*BudgetPeriod `json:"periods"`
}

type TagListResponse struct {
	Tags []*Tag `json:"tags"`
}

type CreateTagRequest struct {
	Name string `json:"name" binding:"required"`
}

type TagResponse struct {
	Tag *Tag `json:"tag"`
}

type UpdateTagRequest struct {
	Name string `json:"name" binding:"required"`
}

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

type SummaryResponse struct {
	Summary *PeriodSummary `json:"summary"`
}

type TagSpendingResponse struct {
	TagSpending []TagSpending `json:"tagSpending"`
}

type CumulativeSpendResponse struct {
	Points []CumulativeSpendPoint `json:"points"`
}

// Note: percentage fields intentionally omit binding:"required" because Gin's
// validator treats the int32 zero value as "missing", which would reject valid
// splits like 100/0/0. The sum-to-100 constraint is enforced by ValidateEDSSplit.
type UpdatePeriodRequest struct {
	BudgetAmount      int64 `json:"budgetAmount"`
	EssentialsPercent int32 `json:"essentialsPercent"`
	DesiresPercent    int32 `json:"desiresPercent"`
	SavingsPercent    int32 `json:"savingsPercent"`
}

type HistoricalComparison struct {
	CurrentSpent  int64 `json:"currentSpent"`
	PreviousSpent int64 `json:"previousSpent"`
	// Empty when there is no previous period. When it differs from the current
	// period's reporting currency, ChangePercent and the amount delta are not
	// comparable and the frontend must guard the display.
	PreviousReportingCurrency string `json:"previousReportingCurrency"`
	// Comparable is false when the current and previous periods have different
	// reporting currencies, making amount comparisons invalid.
	Comparable     bool   `json:"comparable"`
	RollingAverage *int64 `json:"rollingAverage"`
	// Only meaningful when Comparable is true.
	ChangePercent float64 `json:"changePercent"`
}

type HistoricalComparisonResponse struct {
	Comparison *HistoricalComparison `json:"comparison"`
}

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
	// Each trend point may have a different currency when the user has mixed-
	// currency periods; the frontend uses it to format each point correctly.
	ReportingCurrency string `json:"reportingCurrency"`
}

type TrendResponse struct {
	Trends []TrendPoint `json:"trends"`
}
