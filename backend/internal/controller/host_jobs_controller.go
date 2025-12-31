package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/middleware"
	"go-notes/internal/repository"
	"go-notes/internal/service"
)

type HostJobsController struct {
	service *service.HostJobService
	audit   *service.AuditService
}

func NewHostJobsController(service *service.HostJobService, audit *service.AuditService) *HostJobsController {
	return &HostJobsController{service: service, audit: audit}
}

func (c *HostJobsController) RegisterAuthed(r gin.IRoutes) {
	r.POST("/jobs/host-deploy", c.CreateHostDeploy)
}

func (c *HostJobsController) RegisterPublic(r gin.IRoutes) {
	r.GET("/host/jobs/:token", c.FetchHostJob)
	r.POST("/host/jobs/:token/logs", c.AppendHostLogs)
	r.POST("/host/jobs/:token/complete", c.CompleteHostJob)
}

type hostDeployResponse struct {
	Job       jobResponse `json:"job"`
	Token     string      `json:"token"`
	ExpiresAt *time.Time  `json:"expiresAt"`
	Action    string      `json:"action"`
}

func (c *HostJobsController) CreateHostDeploy(ctx *gin.Context) {
	var req service.HostDeployRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	job, token, expiresAt, err := c.service.CreateHostDeploy(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if c.audit != nil {
		session, _ := middleware.SessionFromContext(ctx)
		_ = c.audit.Log(ctx.Request.Context(), service.AuditEntry{
			UserID:    session.UserID,
			UserLogin: session.Login,
			Action:    "job.host_deploy",
			Target:    strings.TrimSpace(req.JobType),
			Metadata: map[string]any{
				"jobId":     job.ID,
				"expiresAt": expiresAt,
			},
		})
	}

	ctx.JSON(http.StatusAccepted, hostDeployResponse{
		Job:       newJobResponse(*job),
		Token:     token,
		ExpiresAt: expiresAt,
		Action:    strings.TrimSpace(req.JobType),
	})
}

type hostJobResponse struct {
	ID        uint            `json:"id"`
	Type      string          `json:"type"`
	Status    string          `json:"status"`
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
	StartedAt *time.Time      `json:"startedAt,omitempty"`
	ExpiresAt *time.Time      `json:"expiresAt,omitempty"`
}

func (c *HostJobsController) FetchHostJob(ctx *gin.Context) {
	token, ok := parseHostToken(ctx)
	if !ok {
		return
	}

	job, payload, err := c.service.ClaimJob(ctx.Request.Context(), token)
	if err != nil {
		respondHostJobError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, hostJobResponse{
		ID:        job.ID,
		Type:      job.Type,
		Status:    job.Status,
		Action:    payload.Action,
		Payload:   payload.Payload,
		CreatedAt: job.CreatedAt,
		StartedAt: job.StartedAt,
		ExpiresAt: job.HostTokenExpiresAt,
	})
}

type hostLogRequest struct {
	Line  string   `json:"line"`
	Lines []string `json:"lines"`
}

func (c *HostJobsController) AppendHostLogs(ctx *gin.Context) {
	token, ok := parseHostToken(ctx)
	if !ok {
		return
	}

	var req hostLogRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	lines := make([]string, 0, len(req.Lines)+1)
	if strings.TrimSpace(req.Line) != "" {
		lines = append(lines, req.Line)
	}
	for _, line := range req.Lines {
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "no log lines provided"})
		return
	}

	if _, err := c.service.AppendLogs(ctx.Request.Context(), token, lines); err != nil {
		respondHostJobError(ctx, err)
		return
	}

	ctx.JSON(http.StatusAccepted, gin.H{"status": "logged"})
}

type hostCompleteRequest struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func (c *HostJobsController) CompleteHostJob(ctx *gin.Context) {
	token, ok := parseHostToken(ctx)
	if !ok {
		return
	}

	var req hostCompleteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	job, err := c.service.CompleteJob(ctx.Request.Context(), token, req.Status, req.Error)
	if err != nil {
		respondHostJobError(ctx, err)
		return
	}

	if c.audit != nil {
		_ = c.audit.Log(ctx.Request.Context(), service.AuditEntry{
			UserLogin: "host-worker",
			Action:    "job.host_complete",
			Target:    fmt.Sprintf("job:%d", job.ID),
			Metadata: map[string]any{
				"status": job.Status,
				"error":  job.Error,
			},
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"job": newJobResponse(*job)})
}

var hostTokenPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func parseHostToken(ctx *gin.Context) (string, bool) {
	token := strings.TrimSpace(ctx.Param("token"))
	if token == "" || !hostTokenPattern.MatchString(token) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid host token"})
		return "", false
	}
	return token, true
}

func respondHostJobError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrHostTokenInvalid), errors.Is(err, repository.ErrNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": "host job not found"})
	case errors.Is(err, service.ErrHostTokenExpired):
		ctx.JSON(http.StatusGone, gin.H{"error": "host token expired"})
	case errors.Is(err, service.ErrHostTokenUsed):
		ctx.JSON(http.StatusGone, gin.H{"error": "host token already used"})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "host job request failed"})
	}
}
