package controller

import (
	"bufio"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/models"
	"go-notes/internal/respond"
	"go-notes/internal/service"
	"go-notes/internal/utils/httpx"
)

type HostController struct {
	service *service.HostService
	jobs    *service.JobService
	audit   *service.AuditService
}

func NewHostController(service *service.HostService, jobs *service.JobService, audit *service.AuditService) *HostController {
	return &HostController{service: service, jobs: jobs, audit: audit}
}

func (c *HostController) ListDocker(ctx *gin.Context) {
	containers, err := c.service.ListContainers(ctx.Request.Context(), true)
	if err != nil {
		respond.Err(ctx, err, errs.CodeHostDockerFailed, "failed to list docker containers")
		return
	}
	respond.OK(ctx, gin.H{"containers": containers})
}

func (c *HostController) DockerUsage(ctx *gin.Context) {
	project := strings.TrimSpace(ctx.Query("project"))
	if project != "" && !httpx.IsSafeRef(project) {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidProject, "invalid project name"), errs.CodeHostInvalidProject, "invalid project name")
		return
	}
	usage, err := c.service.DockerUsage(ctx.Request.Context(), project)
	if err != nil {
		respond.Err(ctx, err, errs.CodeHostUsageFailed, "failed to load docker usage")
		return
	}
	respond.OK(ctx, gin.H{"summary": usage})
}

func (c *HostController) RuntimeStats(ctx *gin.Context) {
	stats, err := c.service.RuntimeStats(ctx.Request.Context())
	if err != nil {
		respond.Err(ctx, err, errs.CodeHostStatsFailed, "failed to load host runtime stats")
		return
	}
	respond.OK(ctx, gin.H{"stats": stats})
}

func (c *HostController) StreamDockerLogs(ctx *gin.Context) {
	container := strings.TrimSpace(ctx.Query("container"))
	if container == "" {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidContainer, "container is required"), errs.CodeHostInvalidContainer, "container is required")
		return
	}
	if !httpx.IsSafeRef(container) {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidContainer, "invalid container name"), errs.CodeHostInvalidContainer, "invalid container name")
		return
	}

	opts := service.ContainerLogsOptions{
		Tail:       httpx.ClampInt(httpx.ParseIntQuery(ctx, "tail", 200), 1, 5000),
		Follow:     httpx.ParseBoolQuery(ctx, "follow", true),
		Timestamps: httpx.ParseBoolQuery(ctx, "timestamps", true),
	}

	httpx.SetSSEHeaders(ctx)

	flusher, ok := httpx.SSEFlusher(ctx)
	if !ok {
		respond.Err(ctx, errs.New(errs.CodeHostStreamUnsupported, "streaming unsupported"), errs.CodeHostStreamUnsupported, "streaming unsupported")
		return
	}

	waiter, stdout, err := c.service.StartContainerLogs(ctx.Request.Context(), container, opts)
	if err != nil {
		respond.Err(ctx, err, errs.CodeHostLogsFailed, "failed to stream docker logs")
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
		httpx.SendSSEEvent(ctx, flusher, "log", gin.H{"line": line})
	}

	if err := scanner.Err(); err != nil {
		httpx.SendSSEEvent(ctx, flusher, "error", gin.H{"code": errs.CodeHostLogsFailed, "message": err.Error()})
		return
	}
	if err := waiter.Wait(); err != nil {
		httpx.SendSSEEvent(ctx, flusher, "error", gin.H{"code": errs.CodeHostLogsFailed, "message": err.Error()})
		return
	}
	httpx.SendSSEEvent(ctx, flusher, "done", gin.H{"status": "closed"})
}

func (c *HostController) StopDocker(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeHostAdminRequired, "admin role required"), errs.CodeHostAdminRequired, "admin role required")
		return
	}
	container, ok := c.parseContainerAction(ctx)
	if !ok {
		return
	}
	if err := c.service.StopContainer(ctx.Request.Context(), container); err != nil {
		respond.Err(ctx, err, errs.CodeHostDockerFailed, "failed to stop container")
		return
	}
	c.logAudit(ctx, "host.container.stop", container, nil)
	respond.OK(ctx, gin.H{"status": "stopped"})
}

func (c *HostController) RestartDocker(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeHostAdminRequired, "admin role required"), errs.CodeHostAdminRequired, "admin role required")
		return
	}
	container, ok := c.parseContainerAction(ctx)
	if !ok {
		return
	}
	if err := c.service.RestartContainer(ctx.Request.Context(), container); err != nil {
		respond.Err(ctx, err, errs.CodeHostDockerFailed, "failed to restart container")
		return
	}
	c.logAudit(ctx, "host.container.restart", container, nil)
	respond.OK(ctx, gin.H{"status": "restarted"})
}

func (c *HostController) RemoveDocker(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeHostAdminRequired, "admin role required"), errs.CodeHostAdminRequired, "admin role required")
		return
	}
	req, ok := c.parseRemoveContainerAction(ctx)
	if !ok {
		return
	}
	if err := c.service.RemoveContainer(ctx.Request.Context(), req.Container, req.RemoveVolumes); err != nil {
		respond.Err(ctx, err, errs.CodeHostDockerFailed, "failed to remove container")
		return
	}
	c.logAudit(ctx, "host.container.remove", req.Container, map[string]any{
		"removeVolumes": req.RemoveVolumes,
	})
	respond.OK(ctx, gin.H{"status": "removed"})
}

func (c *HostController) RestartDockerProject(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeHostAdminRequired, "admin role required"), errs.CodeHostAdminRequired, "admin role required")
		return
	}
	project, ok := c.parseProjectAction(ctx)
	if !ok {
		return
	}
	if c.jobs == nil {
		respond.Err(ctx, errs.New(errs.CodeHostDockerFailed, "job service unavailable"), errs.CodeHostDockerFailed, "job service unavailable")
		return
	}
	job, err := c.jobs.Create(ctx.Request.Context(), service.JobTypeHostRestart, service.RestartProjectStackRequest{
		Project: project,
	})
	if err != nil {
		respond.Err(ctx, err, errs.CodeHostDockerFailed, "failed to queue project restart")
		return
	}
	c.logAudit(ctx, "host.project.restart", project, map[string]any{
		"project":   project,
		"operation": "docker_compose_up_build_async",
		"jobId":     job.ID,
	})
	respond.Accepted(ctx, gin.H{"job": models.NewJobResponse(*job)})
}
