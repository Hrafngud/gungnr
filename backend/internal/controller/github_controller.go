package controller

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/respond"
	"go-notes/internal/service"
)

type GitHubController struct {
	service *service.GitHubService
}

func NewGitHubController(service *service.GitHubService) *GitHubController {
	return &GitHubController{service: service}
}

func (c *GitHubController) Catalog(ctx *gin.Context) {
	if c.service == nil {
		respond.Err(ctx, errs.New(errs.CodeGitHubUnavailable, "github service unavailable"), errs.CodeGitHubUnavailable, "github service unavailable")
		return
	}

	catalog, err := c.service.Catalog(ctx.Request.Context())
	if err != nil {
		respond.Err(ctx, err, errs.CodeGitHubCatalog, "failed to load github catalog")
		return
	}

	respond.OK(ctx, gin.H{"catalog": catalog})
}
