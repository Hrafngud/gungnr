package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/service"
)

type GitHubController struct {
	service *service.GitHubService
}

func NewGitHubController(service *service.GitHubService) *GitHubController {
	return &GitHubController{service: service}
}

func (c *GitHubController) Register(r gin.IRoutes) {
	r.GET("/github/catalog", c.Catalog)
}

func (c *GitHubController) Catalog(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "github service unavailable"})
		return
	}

	catalog, err := c.service.Catalog(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load github catalog"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"catalog": catalog})
}
