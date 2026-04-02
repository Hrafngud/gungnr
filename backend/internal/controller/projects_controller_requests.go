package controller

import (
	"errors"
	"io"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/respond"
	"go-notes/internal/service"
	"go-notes/internal/utils/httpx"
	"go-notes/internal/validate"
)

func (c *ProjectsController) parseProjectParam(ctx *gin.Context) (string, bool) {
	project := strings.TrimSpace(ctx.Param("name"))
	if project == "" {
		respond.Err(ctx, errs.New(errs.CodeProjectInvalidName, "project name is required"), errs.CodeProjectInvalidName, "project name is required")
		return "", false
	}
	project = strings.ToLower(project)
	if err := validate.ProjectName(project); err != nil {
		respond.Err(ctx, errs.New(errs.CodeProjectInvalidName, "project name must be lowercase alphanumerics or dashes"), errs.CodeProjectInvalidName, "project name must be lowercase alphanumerics or dashes")
		return "", false
	}
	return project, true
}

func (c *ProjectsController) parseProjectContainerAction(ctx *gin.Context) (string, string, bool) {
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return "", "", false
	}

	var req models.ProjectContainerActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeProjectInvalidBody, "invalid request body"), errs.CodeProjectInvalidBody, "invalid request body")
		return "", "", false
	}

	container := strings.TrimSpace(req.Container)
	if container == "" {
		respond.Err(ctx, errs.New(errs.CodeProjectInvalidContainer, "container is required"), errs.CodeProjectInvalidContainer, "container is required")
		return "", "", false
	}
	if !httpx.IsSafeRef(container) {
		respond.Err(ctx, errs.New(errs.CodeProjectInvalidContainer, "invalid container name"), errs.CodeProjectInvalidContainer, "invalid container name")
		return "", "", false
	}
	if c.runtime == nil {
		respond.Err(ctx, errs.New(errs.CodeProjectContainerFailed, "project runtime service unavailable"), errs.CodeProjectContainerFailed, "project runtime service unavailable")
		return "", "", false
	}

	resolvedContainer, err := c.runtime.EnsureContainerInProject(ctx.Request.Context(), project, container)
	if err != nil {
		respond.Err(ctx, err, errs.CodeProjectContainerFailed, "project container validation failed")
		return "", "", false
	}
	return project, resolvedContainer, true
}

func (c *ProjectsController) parseProjectRemoveContainerAction(
	ctx *gin.Context,
) (string, models.ProjectRemoveContainerActionRequest, bool) {
	project, ok := c.parseProjectParam(ctx)
	if !ok {
		return "", models.ProjectRemoveContainerActionRequest{}, false
	}

	var req models.ProjectRemoveContainerActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeProjectInvalidBody, "invalid request body"), errs.CodeProjectInvalidBody, "invalid request body")
		return "", models.ProjectRemoveContainerActionRequest{}, false
	}

	container := strings.TrimSpace(req.Container)
	if container == "" {
		respond.Err(ctx, errs.New(errs.CodeProjectInvalidContainer, "container is required"), errs.CodeProjectInvalidContainer, "container is required")
		return "", models.ProjectRemoveContainerActionRequest{}, false
	}
	if !httpx.IsSafeRef(container) {
		respond.Err(ctx, errs.New(errs.CodeProjectInvalidContainer, "invalid container name"), errs.CodeProjectInvalidContainer, "invalid container name")
		return "", models.ProjectRemoveContainerActionRequest{}, false
	}
	if c.runtime == nil {
		respond.Err(ctx, errs.New(errs.CodeProjectContainerFailed, "project runtime service unavailable"), errs.CodeProjectContainerFailed, "project runtime service unavailable")
		return "", models.ProjectRemoveContainerActionRequest{}, false
	}

	resolvedContainer, err := c.runtime.EnsureContainerInProject(ctx.Request.Context(), project, container)
	if err != nil {
		respond.Err(ctx, err, errs.CodeProjectContainerFailed, "project container validation failed")
		return "", models.ProjectRemoveContainerActionRequest{}, false
	}

	req.Container = resolvedContainer
	return project, req, true
}

func (c *ProjectsController) parseProjectArchiveRequest(ctx *gin.Context) (service.ProjectArchiveOptions, bool) {
	options := service.DefaultProjectArchiveOptions()
	if ctx.Request.ContentLength == 0 {
		return options, true
	}

	var req models.ProjectArchiveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return options, true
		}
		respond.Err(ctx, errs.New(errs.CodeProjectInvalidBody, "invalid request body"), errs.CodeProjectInvalidBody, "invalid request body")
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
		respond.Err(ctx, errs.New(errs.CodeProjectInvalidBody, "removeVolumes requires removeContainers=true"), errs.CodeProjectInvalidBody, "removeVolumes requires removeContainers=true")
		return service.ProjectArchiveOptions{}, false
	}
	return options, true
}
