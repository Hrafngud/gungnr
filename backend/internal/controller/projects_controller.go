package controller

import (
	"bufio"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

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

type projectResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	RepoURL   string    `json:"repoUrl"`
	Path      string    `json:"path"`
	ProxyPort int       `json:"proxyPort"`
	DBPort    int       `json:"dbPort"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type projectContainerActionRequest struct {
	Container string `json:"container"`
}

type projectRemoveContainerActionRequest struct {
	Container     string `json:"container"`
	RemoveVolumes bool   `json:"removeVolumes"`
}

type projectEnvWriteRequest struct {
	Content      string `json:"content"`
	CreateBackup *bool  `json:"createBackup,omitempty"`
}

type projectArchiveRequest struct {
	RemoveContainers *bool `json:"removeContainers,omitempty"`
	RemoveVolumes    *bool `json:"removeVolumes,omitempty"`
	RemoveIngress    *bool `json:"removeIngress,omitempty"`
	RemoveDNS        *bool `json:"removeDns,omitempty"`
}

type projectWorkbenchImportRequest struct {
	Reason string `json:"reason,omitempty"`
}

type projectWorkbenchPortMutationRequest struct {
	Selector       service.WorkbenchPortSelector `json:"selector"`
	Action         string                        `json:"action"`
	ManualHostPort *int                          `json:"manualHostPort,omitempty"`
}

type projectWorkbenchPortSuggestionRequest struct {
	Selector service.WorkbenchPortSelector `json:"selector"`
	Limit    int                           `json:"limit,omitempty"`
}

type projectWorkbenchResourceMutationRequest struct {
	Action            string   `json:"action"`
	LimitCPUs         *string  `json:"limitCpus,omitempty"`
	LimitMemory       *string  `json:"limitMemory,omitempty"`
	ReservationCPUs   *string  `json:"reservationCpus,omitempty"`
	ReservationMemory *string  `json:"reservationMemory,omitempty"`
	ClearFields       []string `json:"clearFields,omitempty"`
}

type projectWorkbenchOptionalServiceAddRequest struct {
	EntryKey string `json:"entryKey"`
}

type projectWorkbenchModuleMutationRequest struct {
	Selector service.WorkbenchModuleSelector `json:"selector"`
	Action   string                          `json:"action"`
}

type projectWorkbenchComposePreviewRequest struct {
	ExpectedRevision *int `json:"expectedRevision,omitempty"`
}

type projectWorkbenchComposeApplyRequest struct {
	ExpectedRevision          *int   `json:"expectedRevision,omitempty"`
	ExpectedSourceFingerprint string `json:"expectedSourceFingerprint,omitempty"`
}

type projectWorkbenchComposeRestoreRequest struct {
	BackupID string `json:"backupId"`
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

		response := make([]projectResponse, 0, len(summaries))
		for _, project := range summaries {
			response = append(response, projectResponse{
				ID:        project.ID,
				Name:      project.Name,
				RepoURL:   project.RepoURL,
				Path:      project.Path,
				ProxyPort: project.ProxyPort,
				DBPort:    project.DBPort,
				Status:    project.Status,
				CreatedAt: project.CreatedAt,
				UpdatedAt: project.UpdatedAt,
			})
		}

		ctx.JSON(http.StatusOK, gin.H{"projects": response})
		return
	}

	projects, err := c.service.List(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeProjectListFailed, "failed to load projects")
		return
	}

	response := make([]projectResponse, 0, len(projects))
	for _, project := range projects {
		response = append(response, projectResponse{
			ID:        project.ID,
			Name:      project.Name,
			RepoURL:   project.RepoURL,
			Path:      project.Path,
			ProxyPort: project.ProxyPort,
			DBPort:    project.DBPort,
			Status:    project.Status,
			CreatedAt: project.CreatedAt,
			UpdatedAt: project.UpdatedAt,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"projects": response})
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

func (c *ProjectsController) ArchivePlan(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.archive == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectArchivePlanFailed, "project archive service unavailable", nil)
		return
	}

	plan, err := c.archive.Plan(ctx.Request.Context(), project)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectArchivePlanFailed, "failed to build project archive plan")
		return
	}

	c.logAudit(ctx, "project.archive.plan", plan.Project.NormalizedName, map[string]any{
		"project":        plan.Project.NormalizedName,
		"containers":     len(plan.Containers),
		"hostnames":      len(plan.Hostnames),
		"ingressRules":   len(plan.Ingress),
		"dnsRecords":     len(plan.DNSRecords),
		"warningCount":   len(plan.Warnings),
		"defaultOptions": plan.Defaults,
	})

	ctx.JSON(http.StatusOK, gin.H{"plan": plan})
}

func (c *ProjectsController) Archive(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.archive == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectArchiveFailed, "project archive service unavailable", nil)
		return
	}

	options, ok := c.parseProjectArchiveRequest(ctx)
	if !ok {
		return
	}

	job, plan, err := c.archive.Queue(ctx.Request.Context(), project, options, service.ProjectArchiveActor{
		UserID: session.UserID,
		Login:  session.Login,
	})
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectArchiveFailed, "failed to queue project archive")
		return
	}

	targets := jobTargetsFromPlan(plan, options)
	c.logAudit(ctx, "project.archive.execute", plan.Project.NormalizedName, map[string]any{
		"project":          plan.Project.NormalizedName,
		"jobId":            job.ID,
		"removeContainers": options.RemoveContainers,
		"removeVolumes":    options.RemoveVolumes,
		"removeIngress":    options.RemoveIngress,
		"removeDns":        options.RemoveDNS,
		"targets": map[string]any{
			"containers": len(targets.Containers),
			"hostnames":  len(targets.Hostnames),
			"dnsRecords": len(targets.DNSRecords),
		},
		"warningCount": len(plan.Warnings),
	})

	ctx.JSON(http.StatusAccepted, gin.H{
		"job":  newJobResponse(*job),
		"plan": plan,
	})
}

func (c *ProjectsController) WorkbenchImport(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	req := projectWorkbenchImportRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	stack, changed, err := c.workbench.ImportComposeSnapshot(ctx.Request.Context(), project, req.Reason)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchImportFailed, "failed to import workbench snapshot")
		return
	}

	c.logAudit(ctx, "project.workbench.import", project, map[string]any{
		"project":      project,
		"reason":       strings.ToLower(strings.TrimSpace(req.Reason)),
		"changed":      changed,
		"idempotent":   !changed,
		"revision":     stack.Revision,
		"fingerprint":  stack.SourceFingerprint,
		"serviceCount": len(stack.Services),
		"warningCount": len(stack.Warnings),
	})

	ctx.JSON(http.StatusOK, gin.H{
		"stack":      stack,
		"changed":    changed,
		"idempotent": !changed,
	})
}

func (c *ProjectsController) WorkbenchSnapshot(ctx *gin.Context) {
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	stack, err := c.workbench.GetSnapshot(ctx.Request.Context(), project)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchReadFailed, "failed to load workbench snapshot")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"stack": stack,
	})
}

func (c *ProjectsController) WorkbenchGraph(ctx *gin.Context) {
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	stack, err := c.workbench.GetSnapshot(ctx.Request.Context(), project)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchReadFailed, "failed to load workbench snapshot")
		return
	}

	containers := []service.DockerContainer{}
	runtimeWarning := ""
	if c.runtime == nil {
		runtimeWarning = "runtime service unavailable; graph statuses are based on snapshot-only data"
	} else {
		detail, runtimeErr := c.runtime.Detail(ctx.Request.Context(), project)
		if runtimeErr != nil {
			runtimeWarning = "runtime container state unavailable; graph statuses are based on snapshot-only data"
		} else {
			containers = detail.Containers
		}
	}

	graph := c.workbench.BuildDependencyGraph(stack, containers)
	if runtimeWarning != "" {
		graph.Warnings = append(graph.Warnings, runtimeWarning)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"graph": graph,
	})
}

func (c *ProjectsController) WorkbenchCatalog(ctx *gin.Context) {
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	catalog, err := c.workbench.GetOptionalServiceCatalog(ctx.Request.Context(), project)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchReadFailed, "failed to load workbench optional-service catalog")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"catalog": catalog,
	})
}

func (c *ProjectsController) WorkbenchResolvePorts(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	req := struct{}{}
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	stack, summary, err := c.workbench.ResolveStoredSnapshotPorts(ctx.Request.Context(), project)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		errorCode, issueCount := workbenchErrorCodeAndIssueCount(err)
		c.logAudit(ctx, "project.workbench.ports.resolve", project, map[string]any{
			"project":           project,
			"success":           false,
			"changed":           false,
			"revision":          nil,
			"sourceFingerprint": "",
			"assigned":          0,
			"conflict":          0,
			"unavailable":       0,
			"issueCount":        issueCount,
			"errorCode":         errorCode,
		})
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchPortResolveFailed, "failed to resolve workbench ports")
		return
	}

	c.logAudit(ctx, "project.workbench.ports.resolve", project, map[string]any{
		"project":           project,
		"success":           true,
		"changed":           summary.Changed,
		"revision":          stack.Revision,
		"sourceFingerprint": stack.SourceFingerprint,
		"assigned":          summary.Assigned,
		"conflict":          summary.Conflict,
		"unavailable":       summary.Unavailable,
		"issueCount":        0,
		"errorCode":         "",
	})
	ctx.JSON(http.StatusOK, gin.H{
		"stack":   stack,
		"resolve": summary,
	})
}

func (c *ProjectsController) WorkbenchMutatePort(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	req := projectWorkbenchPortMutationRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	stack, summary, err := c.workbench.MutateStoredSnapshotPort(
		ctx.Request.Context(),
		project,
		service.WorkbenchPortMutationRequest{
			Selector:       req.Selector,
			Action:         req.Action,
			ManualHostPort: req.ManualHostPort,
		},
	)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		errorCode, issueCount := workbenchErrorCodeAndIssueCount(err)
		c.logAudit(ctx, "project.workbench.ports.mutate", project, map[string]any{
			"project":           project,
			"success":           false,
			"selector":          req.Selector,
			"action":            strings.ToLower(strings.TrimSpace(req.Action)),
			"changed":           false,
			"status":            "",
			"assignedHostPort":  nil,
			"revision":          nil,
			"sourceFingerprint": "",
			"issueCount":        issueCount,
			"errorCode":         errorCode,
		})
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchPortMutateFailed, "failed to mutate workbench port")
		return
	}

	c.logAudit(ctx, "project.workbench.ports.mutate", project, map[string]any{
		"project":           project,
		"success":           true,
		"selector":          summary.Selector,
		"action":            summary.Action,
		"changed":           summary.Changed,
		"status":            summary.Status,
		"assignedHostPort":  summary.AssignedHostPort,
		"revision":          stack.Revision,
		"sourceFingerprint": stack.SourceFingerprint,
		"issueCount":        0,
		"errorCode":         "",
	})
	ctx.JSON(http.StatusOK, gin.H{
		"stack":    stack,
		"mutation": summary,
	})
}

func (c *ProjectsController) WorkbenchSuggestPorts(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	req := projectWorkbenchPortSuggestionRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	stack, summary, err := c.workbench.SuggestStoredSnapshotHostPorts(
		ctx.Request.Context(),
		project,
		service.WorkbenchPortSuggestionRequest{
			Selector: req.Selector,
			Limit:    req.Limit,
		},
	)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		errorCode, issueCount := workbenchErrorCodeAndIssueCount(err)
		c.logAudit(ctx, "project.workbench.ports.suggest", project, map[string]any{
			"project":           project,
			"success":           false,
			"selector":          req.Selector,
			"limit":             req.Limit,
			"suggestionCount":   0,
			"revision":          nil,
			"sourceFingerprint": "",
			"issueCount":        issueCount,
			"errorCode":         errorCode,
		})
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchPortSuggestFailed, "failed to suggest workbench ports")
		return
	}

	c.logAudit(ctx, "project.workbench.ports.suggest", project, map[string]any{
		"project":           project,
		"success":           true,
		"selector":          summary.Selector,
		"limit":             summary.Limit,
		"suggestionCount":   summary.SuggestionCount,
		"revision":          stack.Revision,
		"sourceFingerprint": stack.SourceFingerprint,
		"issueCount":        0,
		"errorCode":         "",
	})
	ctx.JSON(http.StatusOK, gin.H{
		"stack":       stack,
		"suggestions": summary,
	})
}

func (c *ProjectsController) WorkbenchMutateResource(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	serviceName := strings.TrimSpace(ctx.Param("serviceName"))
	req := projectWorkbenchResourceMutationRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	stack, summary, err := c.workbench.MutateStoredSnapshotResource(
		ctx.Request.Context(),
		project,
		service.WorkbenchResourceMutationRequest{
			Selector: service.WorkbenchResourceSelector{
				ServiceName: serviceName,
			},
			Action:            req.Action,
			LimitCPUs:         req.LimitCPUs,
			LimitMemory:       req.LimitMemory,
			ReservationCPUs:   req.ReservationCPUs,
			ReservationMemory: req.ReservationMemory,
			ClearFields:       req.ClearFields,
		},
	)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		errorCode, issueCount := workbenchErrorCodeAndIssueCount(err)
		c.logAudit(ctx, "project.workbench.resources.mutate", project, map[string]any{
			"project":           project,
			"success":           false,
			"service":           serviceName,
			"action":            strings.ToLower(strings.TrimSpace(req.Action)),
			"changed":           false,
			"updatedFields":     []string{},
			"clearedFields":     []string{},
			"revision":          nil,
			"sourceFingerprint": "",
			"issueCount":        issueCount,
			"errorCode":         errorCode,
		})
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchResourceMutateFailed, "failed to mutate workbench resources")
		return
	}

	c.logAudit(ctx, "project.workbench.resources.mutate", project, map[string]any{
		"project":           project,
		"success":           true,
		"service":           summary.Selector.ServiceName,
		"action":            summary.Action,
		"changed":           summary.Changed,
		"updatedFields":     summary.UpdatedFields,
		"clearedFields":     summary.ClearedFields,
		"revision":          stack.Revision,
		"sourceFingerprint": stack.SourceFingerprint,
		"issueCount":        0,
		"errorCode":         "",
	})
	ctx.JSON(http.StatusOK, gin.H{
		"stack":    stack,
		"mutation": summary,
	})
}

func (c *ProjectsController) WorkbenchAddService(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	req := projectWorkbenchOptionalServiceAddRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	stack, summary, err := c.workbench.AddOptionalService(
		ctx.Request.Context(),
		project,
		service.WorkbenchOptionalServiceAddRequest{
			EntryKey: req.EntryKey,
		},
	)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		errorCode, issueCount := workbenchErrorCodeAndIssueCount(err)
		c.logAudit(ctx, "project.workbench.services.add", project, map[string]any{
			"project":           project,
			"success":           false,
			"entryKey":          strings.ToLower(strings.TrimSpace(req.EntryKey)),
			"serviceName":       "",
			"changed":           false,
			"previousCount":     0,
			"currentCount":      0,
			"revision":          nil,
			"sourceFingerprint": "",
			"issueCount":        issueCount,
			"errorCode":         errorCode,
		})
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchServiceMutateFailed, "failed to add workbench optional service")
		return
	}

	c.logAudit(ctx, "project.workbench.services.add", project, map[string]any{
		"project":           project,
		"success":           true,
		"entryKey":          summary.EntryKey,
		"serviceName":       summary.ServiceName,
		"changed":           summary.Changed,
		"previousCount":     summary.PreviousCount,
		"currentCount":      summary.CurrentCount,
		"revision":          stack.Revision,
		"sourceFingerprint": stack.SourceFingerprint,
		"issueCount":        0,
		"errorCode":         "",
	})
	ctx.JSON(http.StatusOK, gin.H{
		"stack":    stack,
		"mutation": summary,
	})
}

func (c *ProjectsController) WorkbenchRemoveService(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	serviceName := strings.TrimSpace(ctx.Param("serviceName"))
	stack, summary, err := c.workbench.RemoveOptionalService(ctx.Request.Context(), project, serviceName)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		errorCode, issueCount := workbenchErrorCodeAndIssueCount(err)
		c.logAudit(ctx, "project.workbench.services.remove", project, map[string]any{
			"project":           project,
			"success":           false,
			"entryKey":          "",
			"serviceName":       serviceName,
			"changed":           false,
			"previousCount":     0,
			"currentCount":      0,
			"revision":          nil,
			"sourceFingerprint": "",
			"issueCount":        issueCount,
			"errorCode":         errorCode,
		})
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchServiceMutateFailed, "failed to remove workbench optional service")
		return
	}

	c.logAudit(ctx, "project.workbench.services.remove", project, map[string]any{
		"project":           project,
		"success":           true,
		"entryKey":          summary.EntryKey,
		"serviceName":       summary.ServiceName,
		"changed":           summary.Changed,
		"previousCount":     summary.PreviousCount,
		"currentCount":      summary.CurrentCount,
		"revision":          stack.Revision,
		"sourceFingerprint": stack.SourceFingerprint,
		"issueCount":        0,
		"errorCode":         "",
	})
	ctx.JSON(http.StatusOK, gin.H{
		"stack":    stack,
		"mutation": summary,
	})
}

func (c *ProjectsController) WorkbenchMutateModule(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	req := projectWorkbenchModuleMutationRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	stack, summary, err := c.workbench.MutateLegacyModuleCompatibility(
		ctx.Request.Context(),
		project,
		service.WorkbenchModuleMutationRequest{
			Selector: req.Selector,
			Action:   req.Action,
		},
	)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		errorCode, issueCount := workbenchErrorCodeAndIssueCount(err)
		c.logAudit(ctx, "project.workbench.modules.mutate", project, map[string]any{
			"project":           project,
			"success":           false,
			"selector":          req.Selector,
			"action":            strings.ToLower(strings.TrimSpace(req.Action)),
			"changed":           false,
			"previousCount":     0,
			"currentCount":      0,
			"revision":          nil,
			"sourceFingerprint": "",
			"issueCount":        issueCount,
			"errorCode":         errorCode,
			"compatibilityPath": true,
		})
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchModuleMutateFailed, "failed to mutate workbench modules")
		return
	}

	c.logAudit(ctx, "project.workbench.modules.mutate", project, map[string]any{
		"project":           project,
		"success":           true,
		"selector":          summary.Selector,
		"action":            summary.Action,
		"changed":           summary.Changed,
		"previousCount":     summary.PreviousCount,
		"currentCount":      summary.CurrentCount,
		"revision":          stack.Revision,
		"sourceFingerprint": stack.SourceFingerprint,
		"issueCount":        0,
		"errorCode":         "",
		"compatibilityPath": true,
	})
	ctx.JSON(http.StatusOK, gin.H{
		"stack":    stack,
		"mutation": summary,
	})
}

func (c *ProjectsController) WorkbenchComposePreview(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	req := projectWorkbenchComposePreviewRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	preview, err := c.workbench.PreviewComposeFromStoredSnapshot(
		ctx.Request.Context(),
		project,
		service.WorkbenchComposePreviewRequest{
			ExpectedRevision: req.ExpectedRevision,
		},
	)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		c.logAudit(ctx, "project.workbench.compose.preview", project, workbenchComposePreviewAuditMetadata(project, req.ExpectedRevision, nil, err))
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchPreviewFailed, "failed to preview workbench compose")
		return
	}

	c.logAudit(ctx, "project.workbench.compose.preview", project, workbenchComposePreviewAuditMetadata(project, req.ExpectedRevision, &preview, nil))
	ctx.JSON(http.StatusOK, gin.H{"preview": preview})
}

func (c *ProjectsController) WorkbenchComposeApply(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	req := projectWorkbenchComposeApplyRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	result, err := c.workbench.ApplyComposeFromStoredSnapshot(
		ctx.Request.Context(),
		project,
		service.WorkbenchComposeApplyRequest{
			ExpectedRevision:          req.ExpectedRevision,
			ExpectedSourceFingerprint: req.ExpectedSourceFingerprint,
		},
	)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		c.logAudit(ctx, "project.workbench.compose.apply", project, workbenchComposeApplyAuditMetadata(project, req, nil, err))
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchApplyFailed, "failed to apply workbench compose")
		return
	}

	c.logAudit(ctx, "project.workbench.compose.apply", project, workbenchComposeApplyAuditMetadata(project, req, &result, nil))
	ctx.JSON(http.StatusOK, gin.H{"apply": result})
}

func (c *ProjectsController) WorkbenchComposeBackups(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	backups, err := c.workbench.ListComposeBackups(ctx.Request.Context(), project)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchReadFailed, "failed to load workbench compose backups")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"backups": backups})
}

func (c *ProjectsController) WorkbenchComposeRestore(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeProjectAdminRequired, "admin role required", nil)
		return
	}

	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return
	}
	if c.workbench == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeWorkbenchStorageFailed, "workbench service unavailable", nil)
		return
	}

	req := projectWorkbenchComposeRestoreRequest{}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return
	}

	result, err := c.workbench.RestoreComposeFromBackup(
		ctx.Request.Context(),
		project,
		service.WorkbenchComposeRestoreRequest{
			BackupID: req.BackupID,
		},
	)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		c.logAudit(ctx, "project.workbench.compose.restore", project, workbenchComposeRestoreAuditMetadata(project, req, nil, err))
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectWorkbenchRestoreFailed, "failed to restore workbench compose")
		return
	}

	c.logAudit(ctx, "project.workbench.compose.restore", project, workbenchComposeRestoreAuditMetadata(project, req, &result, nil))
	ctx.JSON(http.StatusOK, gin.H{"restore": result})
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

func (c *ProjectsController) parseProjectParam(ctx *gin.Context) (string, bool) {
	project := strings.TrimSpace(ctx.Param("name"))
	if project == "" {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidName, "project name is required", nil)
		return "", false
	}
	project = strings.ToLower(project)
	if err := service.ValidateProjectName(project); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidName, "project name must be lowercase alphanumerics or dashes", nil)
		return "", false
	}
	return project, true
}

func (c *ProjectsController) parseProjectContainerAction(ctx *gin.Context) (string, string, bool) {
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return "", "", false
	}
	var req projectContainerActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return "", "", false
	}
	container := strings.TrimSpace(req.Container)
	if container == "" {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidContainer, "container is required", nil)
		return "", "", false
	}
	if !httpx.IsSafeRef(container) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidContainer, "invalid container name", nil)
		return "", "", false
	}
	if c.runtime == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectContainerFailed, "project runtime service unavailable", nil)
		return "", "", false
	}
	resolvedContainer, err := c.runtime.EnsureContainerInProject(ctx.Request.Context(), project, container)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectContainerFailed, "project container validation failed")
		return "", "", false
	}
	return project, resolvedContainer, true
}

func (c *ProjectsController) parseProjectRemoveContainerAction(
	ctx *gin.Context,
) (string, projectRemoveContainerActionRequest, bool) {
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return "", projectRemoveContainerActionRequest{}, false
	}

	var req projectRemoveContainerActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return "", projectRemoveContainerActionRequest{}, false
	}

	container := strings.TrimSpace(req.Container)
	if container == "" {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidContainer, "container is required", nil)
		return "", projectRemoveContainerActionRequest{}, false
	}
	if !httpx.IsSafeRef(container) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidContainer, "invalid container name", nil)
		return "", projectRemoveContainerActionRequest{}, false
	}

	if c.runtime == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeProjectContainerFailed, "project runtime service unavailable", nil)
		return "", projectRemoveContainerActionRequest{}, false
	}
	resolvedContainer, err := c.runtime.EnsureContainerInProject(ctx.Request.Context(), project, container)
	if err != nil {
		status := projectHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeProjectContainerFailed, "project container validation failed")
		return "", projectRemoveContainerActionRequest{}, false
	}

	req.Container = resolvedContainer
	return project, req, true
}

func projectHTTPStatus(err error, fallback int) int {
	if err == nil {
		return fallback
	}

	var typed *errs.Error
	if !errors.As(err, &typed) {
		return fallback
	}

	switch typed.Code {
	case errs.CodeProjectInvalidBody,
		errs.CodeProjectInvalidName,
		errs.CodeProjectInvalidContainer,
		errs.CodeWorkbenchSourceInvalid,
		errs.CodeProjectEnvTooLarge:
		return http.StatusBadRequest
	case errs.CodeProjectNotFound, errs.CodeProjectContainerNotFound, errs.CodeWorkbenchSourceNotFound:
		return http.StatusNotFound
	case errs.CodeWorkbenchBackupNotFound:
		return http.StatusNotFound
	case errs.CodeWorkbenchLocked:
		return http.StatusConflict
	case errs.CodeWorkbenchStaleRevision, errs.CodeWorkbenchDriftDetected, errs.CodeWorkbenchBackupIntegrity:
		return http.StatusConflict
	case errs.CodeWorkbenchValidationFailed:
		return http.StatusUnprocessableEntity
	case errs.CodeProjectAdminRequired:
		return http.StatusForbidden
	default:
		return fallback
	}
}

func workbenchComposeApplyAuditMetadata(
	project string,
	req projectWorkbenchComposeApplyRequest,
	result *service.WorkbenchComposeApplyResult,
	opErr error,
) map[string]any {
	metadata := map[string]any{
		"project":                   project,
		"expectedRevision":          nil,
		"expectedSourceFingerprint": strings.TrimSpace(req.ExpectedSourceFingerprint),
		"success":                   opErr == nil,
		"blocked":                   false,
		"revision":                  nil,
		"sourceFingerprint":         "",
		"composePath":               "",
		"composeBytes":              0,
		"backupId":                  "",
		"backupSequence":            0,
		"retainedBackups":           0,
		"prunedBackups":             0,
		"issueCount":                0,
		"errorCode":                 "",
	}
	if req.ExpectedRevision != nil {
		metadata["expectedRevision"] = *req.ExpectedRevision
	}

	if result != nil {
		metadata["revision"] = result.Metadata.Revision
		metadata["sourceFingerprint"] = result.Metadata.SourceFingerprint
		metadata["composePath"] = result.Metadata.ComposePath
		metadata["composeBytes"] = result.ComposeBytes
		metadata["backupId"] = result.Backup.BackupID
		metadata["backupSequence"] = result.Backup.Sequence
		metadata["retainedBackups"] = result.Retention.RetainedCount
		metadata["prunedBackups"] = result.Retention.PrunedCount
		return metadata
	}

	var typed *errs.Error
	if !errors.As(opErr, &typed) {
		return metadata
	}

	metadata["errorCode"] = typed.Code
	switch typed.Code {
	case errs.CodeWorkbenchValidationFailed, errs.CodeWorkbenchStaleRevision, errs.CodeWorkbenchDriftDetected:
		metadata["blocked"] = true
	}

	details, ok := typed.Details.(map[string]any)
	if !ok {
		return metadata
	}

	if revision, ok := details["revision"]; ok {
		metadata["revision"] = revision
	}
	if fingerprint, ok := details["sourceFingerprint"].(string); ok {
		metadata["sourceFingerprint"] = fingerprint
	}
	if composePath, ok := details["composePath"].(string); ok {
		metadata["composePath"] = composePath
	}
	if issueCount, ok := details["issueCount"].(int); ok {
		metadata["issueCount"] = issueCount
	}
	return metadata
}

func workbenchComposeRestoreAuditMetadata(
	project string,
	req projectWorkbenchComposeRestoreRequest,
	result *service.WorkbenchComposeRestoreResult,
	opErr error,
) map[string]any {
	metadata := map[string]any{
		"project":             project,
		"backupId":            strings.TrimSpace(req.BackupID),
		"success":             opErr == nil,
		"blocked":             false,
		"revision":            nil,
		"sourceFingerprint":   "",
		"restoredFingerprint": "",
		"composePath":         "",
		"composeBytes":        0,
		"requiresImport":      false,
		"errorCode":           "",
	}

	if result != nil {
		metadata["revision"] = result.Metadata.Revision
		metadata["sourceFingerprint"] = result.Metadata.SourceFingerprint
		metadata["restoredFingerprint"] = result.Metadata.RestoredFingerprint
		metadata["composePath"] = result.Metadata.ComposePath
		metadata["composeBytes"] = result.ComposeBytes
		metadata["requiresImport"] = result.Metadata.RequiresImport
		return metadata
	}

	var typed *errs.Error
	if !errors.As(opErr, &typed) {
		return metadata
	}

	metadata["errorCode"] = typed.Code
	metadata["blocked"] = typed.Code == errs.CodeWorkbenchBackupIntegrity
	details, ok := typed.Details.(map[string]any)
	if !ok {
		return metadata
	}

	if revision, ok := details["revision"]; ok {
		metadata["revision"] = revision
	}
	if fingerprint, ok := details["sourceFingerprint"].(string); ok {
		metadata["sourceFingerprint"] = fingerprint
	}
	if composePath, ok := details["composePath"].(string); ok {
		metadata["composePath"] = composePath
	}
	return metadata
}

func workbenchComposePreviewAuditMetadata(
	project string,
	expectedRevision *int,
	preview *service.WorkbenchComposePreviewResult,
	opErr error,
) map[string]any {
	metadata := map[string]any{
		"project":           project,
		"expectedRevision":  nil,
		"success":           opErr == nil,
		"blocked":           false,
		"revision":          nil,
		"sourceFingerprint": "",
		"composeBytes":      0,
		"issueCount":        0,
		"errorCode":         "",
	}
	if expectedRevision != nil {
		metadata["expectedRevision"] = *expectedRevision
	}

	if preview != nil {
		metadata["revision"] = preview.Metadata.Revision
		metadata["sourceFingerprint"] = preview.Metadata.SourceFingerprint
		metadata["composeBytes"] = len(preview.Compose)
		return metadata
	}

	var typed *errs.Error
	if !errors.As(opErr, &typed) {
		return metadata
	}

	metadata["errorCode"] = typed.Code
	metadata["blocked"] = typed.Code == errs.CodeWorkbenchValidationFailed
	details, ok := typed.Details.(map[string]any)
	if !ok {
		return metadata
	}

	if revision, ok := details["revision"]; ok {
		metadata["revision"] = revision
	}
	if fingerprint, ok := details["sourceFingerprint"].(string); ok {
		metadata["sourceFingerprint"] = fingerprint
	}
	if issueCount, ok := details["issueCount"].(int); ok {
		metadata["issueCount"] = issueCount
	}
	return metadata
}

func workbenchErrorCodeAndIssueCount(opErr error) (errs.Code, int) {
	if opErr == nil {
		return "", 0
	}

	var typed *errs.Error
	if !errors.As(opErr, &typed) {
		return "", 0
	}

	issueCount := 0
	details, ok := typed.Details.(map[string]any)
	if ok {
		switch value := details["issueCount"].(type) {
		case int:
			issueCount = value
		case int32:
			issueCount = int(value)
		case int64:
			issueCount = int(value)
		case float64:
			issueCount = int(value)
		}
	}

	return typed.Code, issueCount
}

func (c *ProjectsController) logAudit(ctx *gin.Context, action, target string, metadata map[string]any) {
	if c.audit == nil {
		return
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

func (c *ProjectsController) parseProjectArchiveRequest(ctx *gin.Context) (service.ProjectArchiveOptions, bool) {
	options := service.DefaultProjectArchiveOptions()
	if ctx.Request.ContentLength == 0 {
		return options, true
	}

	var req projectArchiveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return options, true
		}
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "invalid request body", nil)
		return service.ProjectArchiveOptions{}, false
	}

	if req.RemoveContainers != nil {
		options.RemoveContainers = *req.RemoveContainers
	}
	if req.RemoveVolumes != nil {
		options.RemoveVolumes = *req.RemoveVolumes
	}
	if req.RemoveIngress != nil {
		options.RemoveIngress = *req.RemoveIngress
	}
	if req.RemoveDNS != nil {
		options.RemoveDNS = *req.RemoveDNS
	}
	if !options.RemoveContainers && options.RemoveVolumes {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeProjectInvalidBody, "removeVolumes requires removeContainers=true", nil)
		return service.ProjectArchiveOptions{}, false
	}
	return options, true
}

func jobTargetsFromPlan(plan service.ProjectArchivePlan, options service.ProjectArchiveOptions) service.ProjectArchiveTargets {
	targets := service.ProjectArchiveTargets{
		Containers:         []string{},
		Hostnames:          []string{},
		ExposureContainers: []string{},
		ExposureHostnames:  []string{},
		DNSRecords:         []service.ProjectArchiveDNSDeleteTarget{},
	}
	if options.RemoveContainers {
		for _, container := range plan.Containers {
			if strings.TrimSpace(container.Name) == "" {
				continue
			}
			targets.Containers = append(targets.Containers, container.Name)
		}
		for _, exposure := range plan.ServiceExposures {
			container := strings.TrimSpace(exposure.Container)
			if container == "" {
				continue
			}
			targets.ExposureContainers = append(targets.ExposureContainers, container)
		}
	}
	if options.RemoveIngress {
		targets.Hostnames = append(targets.Hostnames, plan.Hostnames...)
		for _, exposure := range plan.ServiceExposures {
			hostname := strings.ToLower(strings.TrimSpace(exposure.Hostname))
			if hostname == "" {
				continue
			}
			targets.ExposureHostnames = append(targets.ExposureHostnames, hostname)
		}
	}
	if options.RemoveDNS {
		for _, record := range plan.DNSRecords {
			if !record.DeleteEligible {
				continue
			}
			targets.DNSRecords = append(targets.DNSRecords, service.ProjectArchiveDNSDeleteTarget{
				ZoneID:   record.ZoneID,
				RecordID: record.ID,
				Hostname: record.Name,
				Content:  record.Content,
			})
		}
	}
	return targets
}
