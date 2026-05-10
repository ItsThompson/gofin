package model

import "time"

// CreateExportRequest is the parsed request for creating an export job.
// Currently empty since no body is required: the user ID comes from the header.
type CreateExportRequest struct{}

// JobResponse wraps a single job in the standard response envelope.
type JobResponse struct {
	Job *ExportJob `json:"job"`
}

// JobListResponse wraps a paginated list of jobs.
type JobListResponse struct {
	Data     []*ExportJob `json:"data"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	HasMore  bool         `json:"hasMore"`
}

// RateLimitedResponse is returned when the user has exceeded the export rate limit.
type RateLimitedResponse struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	RetryAfter time.Time `json:"retryAfter"`
}

// ApiError follows the standard error contract.
type ApiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Common error codes.
const (
	ErrUnauthorized        = "UNAUTHORIZED"
	ErrNotFound            = "NOT_FOUND"
	ErrInternalServerError = "INTERNAL_SERVER_ERROR"
	ErrRateLimited         = "RATE_LIMITED"
	ErrValidationError     = "VALIDATION_ERROR"
)
