package models

import "time"

// JobResponse is the standard job shape used across multiple domains.
type JobResponse struct {
	ID         uint       `json:"id"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	Error      string     `json:"error"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// NewJobResponse builds a JobResponse from a Job model.
func NewJobResponse(job Job) JobResponse {
	return JobResponse{
		ID:         job.ID,
		Type:       job.Type,
		Status:     NormalizeJobStatus(job.Status),
		StartedAt:  job.StartedAt,
		FinishedAt: job.FinishedAt,
		Error:      job.Error,
		CreatedAt:  job.CreatedAt,
	}
}

// NormalizeJobStatus maps internal status names to API-facing names.
func NormalizeJobStatus(status string) string {
	if status == "pending_host" {
		return "pending"
	}
	return status
}

// PaginatedMeta is the standard pagination metadata.
type PaginatedMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}
