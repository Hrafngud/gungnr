package controller

import (
	"bufio"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/service"
	"go-notes/internal/utils/httpx"
)

type ProjectsController struct {
	service   *service.ProjectService
	archive   *service.ProjectArchiveService
	workbench *service.WorkbenchService
	runtime   *service.ProjectRuntimeService
	env       *service.ProjectEnvService
	host      *service.HostService
	jobs      *service.JobService
	audit     *service.AuditService
}

func NewProjectsController(
	service *service.ProjectService,
	archive *service.ProjectArchiveService,
	workbench *service.WorkbenchService,
	runtime *service.ProjectRuntimeService,
	env *service.ProjectEnvService,
	host *service.HostService,
	jobs *service.JobService,
	audit *service.AuditService,
) *ProjectsController {
	return &ProjectsController{
		service:   service,
		archive:   archive,
		workbench: workbench,
		runtime:   runtime,
		env:       env,
		host:      host,
		jobs:      jobs,
		audit:     audit,
	}
}

func (c *ProjectsController) List(ctx *gin.Context) {
	if c.runtime != nil {
		summaries, err := c.runtime.ListSummaries(ctx.Request.Context())
		if err != nil {
			apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeProjectListFailed, "failed to load projects")
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"projects": newProjectResponsesFromSummaries(summaries)})
		return
	}

	projects, err := c.service.List(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeProjectListFailed, "failed to load projects")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"projects": newProjectResponsesFromModels(projects)})
}

func (c *ProjectsController) ListLocal(ctx *gin.Context) {
	projects, err := c.service.ListLocal(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeProjectLocalListFailed, err.Error())
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (c *ProjectsController) Detail(ctx *gin.Context) {
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.runtime == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectDetailFailed, "project runtime service unavailable", nil)
		return
	}

	detail, err := c.runtime.Detail(ctx.Request.Context(), project)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectDetailFailed, "failed to load project detail")
		return
	}

	ctx.JSON(http.StatusOK, detail)
}

func (c *ProjectsController) ListJobs(ctx *gin.Context) {
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.jobs == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectJobsFailed, "job service unavailable", nil)
		return
	}

	page := httpx.ParsePositiveIntQuery(ctx, "page", 1)
	limit := httpx.ParsePositiveIntQuery(ctx, "limit", 10)
	if limit > 100 {
		limit = 100
	}

	jobs, total, err := c.jobs.ListByProjectPage(ctx.Request.Context(), project, page, limit)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectJobsFailed, "failed to load project jobs")
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

func (c *ProjectsController) RestartStack(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.runtime == nil || c.jobs == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectStackFailed, "project restart service unavailable", nil)
		return
	}

	resolved, err := c.runtime.Resolve(ctx.Request.Context(), project)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectStackFailed, "failed to resolve project")
		return
	}

	job, err := c.jobs.Create(ctx.Request.Context(), service.JobTypeHostRestart, service.RestartProjectStackRequest{
		Project: resolved.NormalizedName,
	})
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeProjectStackFailed, "failed to queue project restart")
		return
	}

	c.logAudit(ctx, "project.stack.restart", resolved.NormalizedName, map[string]any{
		"project": resolved.NormalizedName,
		"jobId":   job.ID,
	})

	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job)})
}

func (c *ProjectsController) StopContainer(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}
	project, container, ok := c.parseProjectContainerAction(ctx)
	if !ok {
		return
	}
	if c.host == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectContainerFailed, "host service unavailable", nil)
		return
	}

	if err := c.host.StopContainer(ctx.Request.Context(), container); err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeProjectContainerFailed, "failed to stop project container")
		return
	}
	c.logAudit(ctx, "project.container.stop", container, map[string]any{
		"project":   project,
		"container": container,
	})
	ctx.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

func (c *ProjectsController) RestartContainer(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}
	project, container, ok := c.parseProjectContainerAction(ctx)
	if !ok {
		return
	}
	if c.host == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectContainerFailed, "host service unavailable", nil)
		return
	}

	if err := c.host.RestartContainer(ctx.Request.Context(), container); err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeProjectContainerFailed, "failed to restart project container")
		return
	}
	c.logAudit(ctx, "project.container.restart", container, map[string]any{
		"project":   project,
		"container": container,
	})
	ctx.JSON(http.StatusOK, gin.H{"status": "restarted"})
}

func (c *ProjectsController) RemoveContainer(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}
	project, req, ok := c.parseProjectRemoveContainerAction(ctx)
	if !ok {
		return
	}
	if c.host == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectContainerFailed, "host service unavailable", nil)
		return
	}

	if err := c.host.RemoveContainer(ctx.Request.Context(), req.Container, req.RemoveVolumes); err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeProjectContainerFailed, "failed to remove project container")
		return
	}
	c.logAudit(ctx, "project.container.remove", req.Container, map[string]any{
		"project":       project,
		"container":     req.Container,
		"removeVolumes": req.RemoveVolumes,
	})
	ctx.JSON(http.StatusOK, gin.H{"status": "removed"})
}

func (c *ProjectsController) StreamLogs(ctx *gin.Context) {
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	container := strings.TrimSpace(ctx.Query("container"))
	if container == "" {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidContainer, "container is required", nil)
		return
	}
	if !httpx.IsSafeRef(container) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidContainer, "invalid container name", nil)
		return
	}
	if c.runtime == nil || c.host == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectLogsFailed, "project logs service unavailable", nil)
		return
	}

	if _, err := c.runtime.EnsureContainerInProject(ctx.Request.Context(), project, container); err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectLogsFailed, "failed to resolve project container")
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
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeHostStreamUnsupported, "streaming unsupported", nil)
		return
	}

	waiter, stdout, err := c.host.StartContainerLogs(ctx.Request.Context(), container, opts)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeProjectLogsFailed, "failed to stream project container logs")
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
		httpx.SendSSEEvent(ctx, flusher, "error", gin.H{"code": errs.CodeProjectLogsFailed, "message": err.Error()})
		return
	}
	if err := waiter.Wait(); err != nil {
		httpx.SendSSEEvent(ctx, flusher, "error", gin.H{"code": errs.CodeProjectLogsFailed, "message": err.Error()})
		return
	}

	httpx.SendSSEEvent(ctx, flusher, "done", gin.H{"status": "closed"})
}

func (c *ProjectsController) ReadEnv(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.env == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectEnvReadFailed, "project env service unavailable", nil)
		return
	}

	env, err := c.env.Load(ctx.Request.Context(), project)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectEnvReadFailed, "failed to load project .env")
		return
	}

	c.logAudit(ctx, "project.env.read", project, map[string]any{
		"project":   project,
		"path":      env.Path,
		"exists":    env.Exists,
		"sizeBytes": env.SizeBytes,
	})

	ctx.JSON(http.StatusOK, gin.H{"env": env})
}

func (c *ProjectsController) WriteEnv(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.env == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectEnvWriteFailed, "project env service unavailable", nil)
		return
	}

	var req projectEnvWriteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}
	createBackup := true
	if req.CreateBackup != nil {
		createBackup = *req.CreateBackup
	}

	result, err := c.env.Save(ctx.Request.Context(), project, req.Content, createBackup)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectEnvWriteFailed, "failed to save project .env")
		return
	}

	c.logAudit(ctx, "project.env.write", project, map[string]any{
		"project":    project,
		"path":       result.Path,
		"sizeBytes":  result.SizeBytes,
		"backupPath": result.BackupPath,
	})

	ctx.JSON(http.StatusOK, gin.H{"env": result})
}

func (c *ProjectsController) CreateFromTemplate(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}
	var req service.CreateTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	job, err := c.service.CreateFromTemplate(ctx.Request.Context(), req)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeProjectCreateFailed, err.Error())
		return
	}

	subdomain := req.Subdomain
	if subdomain == "" {
		subdomain = req.Name
	}
	c.logAudit(ctx, "project.create_template", req.Name, map[string]any{
		"template":  req.Template,
		"subdomain": subdomain,
		"domain":    req.Domain,
		"proxyPort": req.ProxyPort,
		"dbPort":    req.DBPort,
		"jobId":     job.ID,
	})

	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job)})
}

func (c *ProjectsController) DeployExisting(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}
	var req service.DeployExistingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	job, err := c.service.DeployExisting(ctx.Request.Context(), req)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeProjectDeployFailed, err.Error())
		return
	}

	c.logAudit(ctx, "project.deploy_existing", req.Name, map[string]any{
		"subdomain": req.Subdomain,
		"domain":    req.Domain,
		"port":      req.Port,
		"jobId":     job.ID,
	})

	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job)})
}

func (c *ProjectsController) ForwardLocal(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}
	var req service.ForwardLocalRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	job, err := c.service.ForwardLocal(ctx.Request.Context(), req)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeProjectForwardFailed, err.Error())
		return
	}

	c.logAudit(ctx, "project.forward_local", req.Name, map[string]any{
		"subdomain": req.Subdomain,
		"domain":    req.Domain,
		"port":      req.Port,
		"jobId":     job.ID,
	})

	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job)})
}

func (c *ProjectsController) QuickService(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}
	var req service.QuickServiceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	job, hostPort, err := c.service.QuickService(ctx.Request.Context(), req)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeProjectQuickFailed, err.Error())
		return
	}
	exposureMode, exposureErr := service.NormalizeQuickServiceExposureMode(req.ExposureMode, req.Port)
	if exposureErr != nil {
		exposureMode = req.ExposureMode
	}

	c.logAudit(ctx, "project.quick_service", req.Subdomain, map[string]any{
		"domain":        req.Domain,
		"requestedPort": req.Port,
		"port":          hostPort,
		"jobId":         job.ID,
		"portAuto":      hostPort != 0 && hostPort != req.Port,
		"exposureMode":  exposureMode,
	})

	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job), "hostPort": hostPort})
}
