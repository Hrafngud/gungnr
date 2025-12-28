package controller

import (
	"time"

	"go-notes/internal/models"
)

type jobResponse struct {
	ID         uint       `json:"id"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	Error      string     `json:"error"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func newJobResponse(job models.Job) jobResponse {
	return jobResponse{
		ID:         job.ID,
		Type:       job.Type,
		Status:     job.Status,
		StartedAt:  job.StartedAt,
		FinishedAt: job.FinishedAt,
		Error:      job.Error,
		CreatedAt:  job.CreatedAt,
	}
}
