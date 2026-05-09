package model

import "time"

// Job status constants.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// ExportJob represents a data export job.
type ExportJob struct {
	ID            string     `json:"id"`
	UserID        string     `json:"userId"`
	Status        string     `json:"status"`
	Error         *string    `json:"error"`
	FileSizeBytes *int64     `json:"fileSizeBytes"`
	CreatedAt     time.Time  `json:"createdAt"`
	CompletedAt   *time.Time `json:"completedAt"`
	UpdatedAt     time.Time  `json:"-"`
}

// RecoverableJob holds the minimal fields needed to re-submit a job on startup.
type RecoverableJob struct {
	ID     string
	UserID string
}
