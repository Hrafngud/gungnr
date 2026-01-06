package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/models"
	"go-notes/internal/repository"
	"go-notes/internal/service"
)

type JobsController struct {
	service *service.JobService
}

func NewJobsController(service *service.JobService) *JobsController {
	return &JobsController{service: service}
}

func (c *JobsController) Register(r gin.IRoutes) {
	r.GET("/jobs", c.List)
	r.GET("/jobs/:id", c.Get)
	r.GET("/jobs/:id/stream", c.Stream)
	r.POST("/jobs/:id/stop", c.Stop)
	r.POST("/jobs/:id/retry", c.Retry)
}

func (c *JobsController) List(ctx *gin.Context) {
	page := parsePositiveIntQuery(ctx, "page", 1)
	limit := parsePositiveIntQuery(ctx, "limit", 25)
	if limit > 100 {
		limit = 100
	}

	jobs, total, err := c.service.ListPage(ctx.Request.Context(), page, limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load jobs"})
		return
	}

	response := make([]jobResponse, 0, len(jobs))
	for _, job := range jobs {
		response = append(response, newJobResponse(job))
	}

	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"jobs":       response,
		"page":       page,
		"pageSize":   limit,
		"total":      total,
		"totalPages": totalPages,
	})
}

type jobDetailResponse struct {
	jobResponse
	LogLines []string `json:"logLines"`
}

func (c *JobsController) Get(ctx *gin.Context) {
	id, err := parseUintParam(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	job, err := c.service.Get(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	ctx.JSON(http.StatusOK, jobDetailResponse{
		jobResponse: newJobResponse(*job),
		LogLines:    c.service.LogLines(job),
	})
}

type stopJobRequest struct {
	Error string `json:"error"`
}

func (c *JobsController) Stop(ctx *gin.Context) {
	id, err := parseUintParam(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	var req stopJobRequest
	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
	}

	job, err := c.service.Stop(ctx.Request.Context(), id, req.Error)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrJobAlreadyFinished), errors.Is(err, service.ErrJobRunning), errors.Is(err, service.ErrJobNotStoppable):
			ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, repository.ErrNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stop job"})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"job": newJobResponse(*job)})
}

func (c *JobsController) Retry(ctx *gin.Context) {
	id, err := parseUintParam(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	job, err := c.service.Retry(ctx.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrJobAlreadyFinished), errors.Is(err, service.ErrJobRunning), errors.Is(err, service.ErrJobNotRetryable):
			ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, repository.ErrNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retry job"})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"job": newJobResponse(*job)})
}

func (c *JobsController) Stream(ctx *gin.Context) {
	id, err := parseUintParam(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := ctx.Writer.(http.Flusher)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	lastLen := parseOffset(ctx.Query("offset"))
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-ticker.C:
			job, err := c.service.Get(ctx.Request.Context(), id)
			if err != nil {
				sendEvent(ctx, flusher, "error", map[string]string{"message": "job not found"})
				return
			}
			lastLen = streamLogs(ctx, flusher, job, lastLen)
			if jobDone(job) {
				sendEvent(ctx, flusher, "done", map[string]string{"status": job.Status})
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
		sendEvent(ctx, flusher, "log", map[string]string{"line": trimmed})
	}
	return len(job.LogLines)
}

func sendEvent(ctx *gin.Context, flusher http.Flusher, event string, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(ctx.Writer, "event: %s\n", event)
	fmt.Fprintf(ctx.Writer, "data: %s\n\n", encoded)
	flusher.Flush()
}

func jobDone(job *models.Job) bool {
	if job == nil {
		return true
	}
	return job.Status == "completed" || job.Status == "failed"
}

func parseUintParam(raw string) (uint, error) {
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(value), nil
}

func parsePositiveIntQuery(ctx *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(ctx.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func parseOffset(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}
