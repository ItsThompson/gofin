package service

// DeletionJobSubmitter submits a deletion job for async processing.
// This interface decouples the DeletionService from the concrete deletion.Engine,
// allowing tests to stub the submission step.
type DeletionJobSubmitter interface {
	Submit(jobID, userID string)
}
