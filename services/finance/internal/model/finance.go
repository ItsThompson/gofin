package model

import "time"

// DefaultSettings represents a user's default budget configuration.
type DefaultSettings struct {
	UserID            string    `json:"userId"`
	BudgetAmount      int64     `json:"budgetAmount"`
	EssentialsPercent int32     `json:"essentialsPercent"`
	DesiresPercent    int32     `json:"desiresPercent"`
	SavingsPercent    int32     `json:"savingsPercent"`
	Currency          string    `json:"currency"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// Tag represents a user-owned expense tag.
type Tag struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	IsDefault bool      `json:"isDefault"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
