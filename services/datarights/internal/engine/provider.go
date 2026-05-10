package engine

import "context"

// DataProvider supplies one category of user data for export.
// Each provider fetches data from its upstream service, transforms it into
// human-readable CSV rows, and returns headers and rows as string slices.
type DataProvider interface {
	// Name returns the filename (without extension) for this provider's CSV.
	Name() string

	// Headers returns the CSV column headers.
	Headers() []string

	// Collect fetches all data for the given user and returns rows.
	// Each row is a string slice matching the Headers() order.
	Collect(ctx context.Context, userID string) ([][]string, error)
}
