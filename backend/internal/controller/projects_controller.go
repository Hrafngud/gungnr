package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/service"
)

type ProjectsController struct {
	service *service.ProjectService
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

func NewProjectsController(service *service.ProjectService) *ProjectsController {
	return &ProjectsController{service: service}
}

func (c *ProjectsController) Register(r gin.IRoutes) {
	r.GET("/projects", c.List)
	r.GET("/projects/local", c.ListLocal)
	r.POST("/projects/template", c.CreateFromTemplate)
	r.POST("/projects/existing", c.DeployExisting)
	r.POST("/projects/quick", c.QuickService)
}

func (c *ProjectsController) List(ctx *gin.Context) {
	projects, err := c.service.List(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load projects"})
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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (c *ProjectsController) CreateFromTemplate(ctx *gin.Context) {
	var req service.CreateTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	job, err := c.service.CreateFromTemplate(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job)})
}

func (c *ProjectsController) DeployExisting(ctx *gin.Context) {
	var req service.DeployExistingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	job, err := c.service.DeployExisting(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job)})
}

func (c *ProjectsController) QuickService(ctx *gin.Context) {
	var req service.QuickServiceRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	job, err := c.service.QuickService(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job)})
}
