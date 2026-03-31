package controller

import (
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/respond"
	"go-notes/internal/service"
	"go-notes/internal/utils/httpx"
)

type JobsController struct {
	service *service.JobService
}

func NewJobsController(service *service.JobService) *JobsController {
	return &JobsController{service: service}
}

func (c *JobsController) List(ctx *gin.Context) {
	page := httpx.ParsePositiveIntQuery(ctx, "page", 1)
	limit := httpx.ParsePositiveIntQuery(ctx, "limit", 25)
	if limit > 100 {
		limit = 100
	}

	jobs, total, err := c.service.ListPage(ctx.Request.Context(), page, limit)
	if err != nil {
		respond.Err(ctx, err, errs.CodeJobListFailed, "failed to load jobs")
		return
	}

	response := make([]models.JobResponse, 0, len(jobs))
	for _, job := range jobs {
		response = append(response, models.NewJobResponse(job))
	}

	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}

	respond.OK(ctx, gin.H{
		"jobs":       response,
		"page":       page,
		"pageSize":   limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

func (c *JobsController) Get(ctx *gin.Context) {
	id, err := httpx.ParseUintParam(ctx.Param("id"))
	if err != nil {
		respond.Err(ctx, errs.New(errs.CodeJobInvalidID, "invalid job id"), errs.CodeJobInvalidID, "invalid job id")
		return
	}

	job, err := c.service.Get(ctx.Request.Context(), id)
	if err != nil {
		respond.Err(ctx, err, errs.CodeJobNotFound, "job not found")
		return
	}

	respond.OK(ctx, models.JobDetailResponse{
		JobResponse: models.NewJobResponse(*job),
		LogLines:    c.service.LogLines(job),
	})
}

func (c *JobsController) Stop(ctx *gin.Context) {
	id, err := httpx.ParseUintParam(ctx.Param("id"))
	if err != nil {
		respond.Err(ctx, errs.New(errs.CodeJobInvalidID, "invalid job id"), errs.CodeJobInvalidID, "invalid job id")
		return
	}

	var req models.StopJobRequest
	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&req); err != nil {
			respond.Err(ctx, errs.New(errs.CodeJobInvalidBody, "invalid request body"), errs.CodeJobInvalidBody, "invalid request body")
			return
		}
	}

	job, err := c.service.Stop(ctx.Request.Context(), id, req.Error)
	if err != nil {
		respond.Err(ctx, err, errs.CodeJobStopFailed, "failed to stop job")
		return
	}

	respond.OK(ctx, gin.H{"job": models.NewJobResponse(*job)})
}

func (c *JobsController) Retry(ctx *gin.Context) {
	id, err := httpx.ParseUintParam(ctx.Param("id"))
	if err != nil {
		respond.Err(ctx, errs.New(errs.CodeJobInvalidID, "invalid job id"), errs.CodeJobInvalidID, "invalid job id")
		return
	}

	job, err := c.service.Retry(ctx.Request.Context(), id)
	if err != nil {
		respond.Err(ctx, err, errs.CodeJobRetryFailed, "failed to retry job")
		return
	}

	respond.OK(ctx, gin.H{"job": models.NewJobResponse(*job)})
}

func (c *JobsController) Stream(ctx *gin.Context) {
	id, err := httpx.ParseUintParam(ctx.Param("id"))
	if err != nil {
		respond.Err(ctx, errs.New(errs.CodeJobInvalidID, "invalid job id"), errs.CodeJobInvalidID, "invalid job id")
		return
	}

	httpx.SetSSEHeaders(ctx)

	flusher, ok := httpx.SSEFlusher(ctx)
	if !ok {
		respond.Err(ctx, errs.New(errs.CodeJobStreamUnsupported, "streaming unsupported"), errs.CodeJobStreamUnsupported, "streaming unsupported")
		return
	}

	lastLen := httpx.ParseOffset(ctx.Query("offset"))
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-ticker.C:
			job, err := c.service.Get(ctx.Request.Context(), id)
			if err != nil {
				httpx.SendSSEEvent(ctx, flusher, "error", map[string]any{"code": errs.CodeJobNotFound, "message": "job not found"})
				return
			}
			lastLen = streamLogs(ctx, flusher, job, lastLen)
			if jobDone(job) {
				httpx.SendSSEEvent(ctx, flusher, "done", map[string]string{"status": job.Status})
				return
			}
		}
	}
}

func streamLogs(ctx *gin.Context, flusher http.Flusher, job *models.Job, lastLen int) int {
	if job == nil || job.LogLines == "" {
		return lastLen
	}

	if len(job.LogLines) <= lastLen {
		return lastLen
	}

	if lastLen > len(job.LogLines) {
		lastLen = len(job.LogLines)
	}

	chunk := job.LogLines[lastLen:]
	lines := strings.Split(chunk, "\n")
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "" {
			continue
		}
		httpx.SendSSEEvent(ctx, flusher, "log", map[string]string{"line": trimmed})
	}
	return len(job.LogLines)
}

func jobDone(job *models.Job) bool {
	if job == nil {
		return true
	}
	return job.Status == "completed" || job.Status == "failed"
}
