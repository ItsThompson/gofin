package model

import "time"

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

// Datarights-specific error codes. The shared codes (UNAUTHORIZED, NOT_FOUND,
// VALIDATION_ERROR, INTERNAL_SERVER_ERROR) are sourced from the apierr package;
// only the codes unique to datarights are declared here.
const (
	ErrInvalidCredentials = "INVALID_CREDENTIALS"
	ErrRateLimited        = "RATE_LIMITED"
	ErrProtectedUser      = "PROTECTED_USER"
	ErrExportConflict     = "EXPORT_CONFLICT"
	ErrServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// CreateDeletionRequest is the parsed request body for creating a deletion job.
type CreateDeletionRequest struct {
	UserID   string `json:"userId" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// DeletionJobResponse wraps a single deletion job in the response envelope.
type DeletionJobResponse struct {
	ID          string     `json:"id"`
	UserID      string     `json:"userId"`
	Status      string     `json:"status"`
	Error       *string    `json:"error"`
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt"`
}
