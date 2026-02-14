package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
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
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeGitHubUnavailable, "github service unavailable", nil)
		return
	}

	catalog, err := c.service.Catalog(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeGitHubCatalog, "failed to load github catalog")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"catalog": catalog})
}
