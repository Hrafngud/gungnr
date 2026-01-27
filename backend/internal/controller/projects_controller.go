package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/service"
)

type ProjectsController struct {
	service *service.ProjectService
	audit   *service.AuditService
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

func NewProjectsController(service *service.ProjectService, audit *service.AuditService) *ProjectsController {
	return &ProjectsController{service: service, audit: audit}
}

func (c *ProjectsController) Register(r gin.IRoutes) {
	r.GET("/projects", c.List)
	r.GET("/projects/local", c.ListLocal)
	r.POST("/projects/template", c.CreateFromTemplate)
	r.POST("/projects/existing", c.DeployExisting)
	r.POST("/projects/forward", c.ForwardLocal)
	r.POST("/projects/quick", c.QuickService)
}

func (c *ProjectsController) List(ctx *gin.Context) {
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

	job, err := c.service.QuickService(ctx.Request.Context(), req)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeProjectQuickFailed, err.Error())
		return
	}

	c.logAudit(ctx, "project.quick_service", req.Subdomain, map[string]any{
		"domain": req.Domain,
		"port":   req.Port,
		"jobId":  job.ID,
	})

	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job)})
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
