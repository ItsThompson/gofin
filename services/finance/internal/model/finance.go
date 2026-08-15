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

// BudgetPeriod represents a single monthly budget period.
type BudgetPeriod struct {
	ID                string    `json:"id"`
	UserID            string    `json:"userId"`
	Year              int32     `json:"year"`
	Month             int32     `json:"month"`
	BudgetAmount      int64     `json:"budgetAmount"`
	ReportingCurrency string    `json:"reportingCurrency"`
	EssentialsPercent int32     `json:"essentialsPercent"`
	DesiresPercent    int32     `json:"desiresPercent"`
	SavingsPercent    int32     `json:"savingsPercent"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// PeriodContext is the read-only period state needed before expense writes.
type PeriodContext struct {
	PeriodID          string
	UserID            string
	Year              int32
	Month             int32
	ReportingCurrency string
	IsLocked          bool
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
