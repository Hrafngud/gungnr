package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/service"
)

type JobsController struct {
	service *service.JobService
}

type jobResponse struct {
	ID         uint       `json:"id"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	Error      string     `json:"error"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func NewJobsController(service *service.JobService) *JobsController {
	return &JobsController{service: service}
}

func (c *JobsController) Register(r gin.IRoutes) {
	r.GET("/jobs", c.List)
}

func (c *JobsController) List(ctx *gin.Context) {
	jobs, err := c.service.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load jobs"})
		return
	}

	response := make([]jobResponse, 0, len(jobs))
	for _, job := range jobs {
		response = append(response, jobResponse{
			ID:         job.ID,
			Type:       job.Type,
			Status:     job.Status,
			StartedAt:  job.StartedAt,
			FinishedAt: job.FinishedAt,
			Error:      job.Error,
			CreatedAt:  job.CreatedAt,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"jobs": response})
}
