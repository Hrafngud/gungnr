package controller

import (
	"bufio"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/service"
)

type HostController struct {
	service *service.HostService
}

func NewHostController(service *service.HostService) *HostController {
	return &HostController{service: service}
}

func (c *HostController) Register(r gin.IRoutes) {
	r.GET("/host/docker", c.ListDocker)
	r.GET("/host/docker/logs", c.StreamDockerLogs)
}

func (c *HostController) ListDocker(ctx *gin.Context) {
	containers, err := c.service.ListContainers(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"containers": containers})
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
