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
type CreatePeriodRequest struct {
	Year              int32 `json:"year" binding:"required"`
	Month             int32 `json:"month" binding:"required"`
	BudgetAmount      int64 `json:"budgetAmount"`
	EssentialsPercent int32 `json:"essentialsPercent"`
	DesiresPercent    int32 `json:"desiresPercent"`
	SavingsPercent    int32 `json:"savingsPercent"`
}

// PeriodResponse is the JSON body returned for period endpoints.
type PeriodResponse struct {
	Period *BudgetPeriod `json:"period"`
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
