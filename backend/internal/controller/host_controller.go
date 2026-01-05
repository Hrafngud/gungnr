package controller

import (
	"bufio"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/middleware"
	"go-notes/internal/service"
)

type HostController struct {
	service *service.HostService
	audit   *service.AuditService
}

func NewHostController(service *service.HostService, audit *service.AuditService) *HostController {
	return &HostController{service: service, audit: audit}
}

func (c *HostController) Register(r gin.IRoutes) {
	r.GET("/host/docker", c.ListDocker)
	r.GET("/host/docker/usage", c.DockerUsage)
	r.GET("/host/docker/logs", c.StreamDockerLogs)
	r.POST("/host/docker/stop", c.StopDocker)
	r.POST("/host/docker/restart", c.RestartDocker)
	r.POST("/host/docker/remove", c.RemoveDocker)
}

func (c *HostController) ListDocker(ctx *gin.Context) {
	containers, err := c.service.ListContainers(ctx.Request.Context(), true)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"containers": containers})
}

func (c *HostController) DockerUsage(ctx *gin.Context) {
	project := strings.TrimSpace(ctx.Query("project"))
	if project != "" && !isSafeContainerRef(project) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid project name"})
		return
	}
	usage, err := c.service.DockerUsage(ctx.Request.Context(), project)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"summary": usage})
}

func (c *HostController) StreamDockerLogs(ctx *gin.Context) {
	container := strings.TrimSpace(ctx.Query("container"))
	if container == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "container is required"})
		return
	}
	if !isSafeContainerRef(container) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid container name"})
		return
	}

	opts := service.ContainerLogsOptions{
		Tail:       clampInt(parseIntQuery(ctx, "tail", 200), 1, 5000),
		Follow:     parseBoolQuery(ctx, "follow", true),
		Timestamps: parseBoolQuery(ctx, "timestamps", true),
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

	cmd, stdout, err := c.service.StartContainerLogs(ctx.Request.Context(), container, opts)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer stdout.Close()

	scanner := bufio.NewScanner(stdout)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			continue
		}
		sendEvent(ctx, flusher, "log", gin.H{"line": line})
	}

	if err := scanner.Err(); err != nil {
		sendEvent(ctx, flusher, "error", gin.H{"message": err.Error()})
		return
	}
	if err := cmd.Wait(); err != nil {
		sendEvent(ctx, flusher, "error", gin.H{"message": err.Error()})
		return
	}
	sendEvent(ctx, flusher, "done", gin.H{"status": "closed"})
}

type containerActionRequest struct {
	Container string `json:"container"`
}

type removeContainerRequest struct {
	Container     string `json:"container"`
	RemoveVolumes bool   `json:"removeVolumes"`
}

func (c *HostController) StopDocker(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	container, ok := c.parseContainerAction(ctx)
	if !ok {
		return
	}
	if err := c.service.StopContainer(ctx.Request.Context(), container); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.logAudit(ctx, "host.container.stop", container, nil)
	ctx.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

func (c *HostController) RestartDocker(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	container, ok := c.parseContainerAction(ctx)
	if !ok {
		return
	}
	if err := c.service.RestartContainer(ctx.Request.Context(), container); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.logAudit(ctx, "host.container.restart", container, nil)
	ctx.JSON(http.StatusOK, gin.H{"status": "restarted"})
}

func (c *HostController) RemoveDocker(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	req, ok := c.parseRemoveContainerAction(ctx)
	if !ok {
		return
	}
	if err := c.service.RemoveContainer(ctx.Request.Context(), req.Container, req.RemoveVolumes); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.logAudit(ctx, "host.container.remove", req.Container, map[string]any{
		"removeVolumes": req.RemoveVolumes,
	})
	ctx.JSON(http.StatusOK, gin.H{"status": "removed"})
}

func parseBoolQuery(ctx *gin.Context, key string, fallback bool) bool {
	raw := strings.TrimSpace(ctx.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseIntQuery(ctx *gin.Context, key string, fallback int) int {
	raw := strings.TrimSpace(ctx.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

var dockerRefPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

func isSafeContainerRef(value string) bool {
	return dockerRefPattern.MatchString(value)
}

func (c *HostController) parseContainerAction(ctx *gin.Context) (string, bool) {
	var req containerActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return "", false
	}
	container := strings.TrimSpace(req.Container)
	if container == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "container is required"})
		return "", false
	}
	if !isSafeContainerRef(container) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid container name"})
		return "", false
	}
	return container, true
}

func (c *HostController) parseRemoveContainerAction(ctx *gin.Context) (removeContainerRequest, bool) {
	var req removeContainerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return removeContainerRequest{}, false
	}
	container := strings.TrimSpace(req.Container)
	if container == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "container is required"})
		return removeContainerRequest{}, false
	}
	if !isSafeContainerRef(container) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid container name"})
		return removeContainerRequest{}, false
	}
	req.Container = container
	return req, true
}

func (c *HostController) logAudit(ctx *gin.Context, action, target string, metadata map[string]any) {
	if c.audit == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	if _, ok := metadata["container"]; !ok {
		metadata["container"] = target
	}
	session, _ := middleware.SessionFromContext(ctx)
	_ = c.audit.Log(ctx.Request.Context(), service.AuditEntry{
		UserID:    session.UserID,
		UserLogin: session.Login,
		Action:    action,
		Target:    target,
		Metadata:  metadata,
	})
}
