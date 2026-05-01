package model

// OnboardingRequest is the input for POST /api/finance/onboarding.
type OnboardingRequest struct {
	BudgetAmount      int64  `json:"budgetAmount"`
	EssentialsPercent int32  `json:"essentialsPercent" binding:"required"`
	DesiresPercent    int32  `json:"desiresPercent" binding:"required"`
	SavingsPercent    int32  `json:"savingsPercent" binding:"required"`
	Currency          string `json:"currency" binding:"required"`
}

// DefaultsResponse is the JSON body returned for default settings endpoints.
type DefaultsResponse struct {
	Defaults *DefaultSettings `json:"defaults"`
}
