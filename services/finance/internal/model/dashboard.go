package model

// PeriodSummary is the aggregate dashboard response for a budget period.
// All monetary values are in minor units (cents).
type PeriodSummary struct {
	PeriodID     string          `json:"periodId"`
	Year         int32           `json:"year"`
	Month        int32           `json:"month"`
	TotalBudget  int64           `json:"totalBudget"`
	TotalSpent   int64           `json:"totalSpent"`
	Remaining    int64           `json:"remaining"`
	DaysInPeriod int32           `json:"daysInPeriod"`
	DaysElapsed  int32           `json:"daysElapsed"`
	// DailySpendRate is totalSpent / daysElapsed (cents). 0 when daysElapsed is 0.
	DailySpendRate int64          `json:"dailySpendRate"`
	// BudgetPace is remaining / daysRemaining (cents). 0 when no days remain.
	BudgetPace     int64          `json:"budgetPace"`
	// IsOnTrack is true when dailySpendRate <= idealDailyRate (totalBudget / daysInPeriod).
	IsOnTrack      bool           `json:"isOnTrack"`
	Essentials     CategorySummary `json:"essentials"`
	Desires        CategorySummary `json:"desires"`
	Savings        CategorySummary `json:"savings"`
}

// CategorySummary is the breakdown for one E/D/S category.
type CategorySummary struct {
	Allocated   int64   `json:"allocated"`
	Spent       int64   `json:"spent"`
	Remaining   int64   `json:"remaining"`
	PercentUsed float64 `json:"percentUsed"`
}

// TagSpending is a single tag's spending data for the tag spending chart.
type TagSpending struct {
	TagID          string  `json:"tagId"`
	TagName        string  `json:"tagName"`
	Amount         int64   `json:"amount"`
	PercentOfTotal float64 `json:"percentOfTotal"`
}

// CumulativeSpendPoint is one data point in the cumulative spend chart.
type CumulativeSpendPoint struct {
	Day    int32 `json:"day"`
	Actual int64 `json:"actual"`
	Ideal  int64 `json:"ideal"`
}
