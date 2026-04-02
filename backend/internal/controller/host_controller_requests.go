package controller

import (
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/respond"
	"go-notes/internal/utils/httpx"
)

func (c *HostController) parseContainerAction(ctx *gin.Context) (string, bool) {
	var req models.ContainerActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidBody, "invalid request body"), errs.CodeHostInvalidBody, "invalid request body")
		return "", false
	}
	container := strings.TrimSpace(req.Container)
	if container == "" {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidContainer, "container is required"), errs.CodeHostInvalidContainer, "container is required")
		return "", false
	}
	if !httpx.IsSafeRef(container) {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidContainer, "invalid container name"), errs.CodeHostInvalidContainer, "invalid container name")
		return "", false
	}
	return container, true
}

func (c *HostController) parseRemoveContainerAction(ctx *gin.Context) (models.RemoveContainerRequest, bool) {
	var req models.RemoveContainerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidBody, "invalid request body"), errs.CodeHostInvalidBody, "invalid request body")
		return models.RemoveContainerRequest{}, false
	}
	container := strings.TrimSpace(req.Container)
	if container == "" {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidContainer, "container is required"), errs.CodeHostInvalidContainer, "container is required")
		return models.RemoveContainerRequest{}, false
	}
	if !httpx.IsSafeRef(container) {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidContainer, "invalid container name"), errs.CodeHostInvalidContainer, "invalid container name")
		return models.RemoveContainerRequest{}, false
	}
	req.Container = container
	return req, true
}

func (c *HostController) parseProjectAction(ctx *gin.Context) (string, bool) {
	var req models.ProjectActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidBody, "invalid request body"), errs.CodeHostInvalidBody, "invalid request body")
		return "", false
	}
	project := strings.TrimSpace(req.Project)
	if project == "" {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidProject, "project is required"), errs.CodeHostInvalidProject, "project is required")
		return "", false
	}
	if project == "." || project == ".." || !httpx.IsSafeRef(project) {
		respond.Err(ctx, errs.New(errs.CodeHostInvalidProject, "invalid project name"), errs.CodeHostInvalidProject, "invalid project name")
		return "", false
	}
	return project, true
}
