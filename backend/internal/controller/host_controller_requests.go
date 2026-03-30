package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
	"go-notes/internal/utils/httpx"
)

type containerActionRequest struct {
	Container string `json:"container"`
}

type removeContainerRequest struct {
	Container     string `json:"container"`
	RemoveVolumes bool   `json:"removeVolumes"`
}

type projectActionRequest struct {
	Project string `json:"project"`
}

func (c *HostController) parseContainerAction(ctx *gin.Context) (string, bool) {
	var req containerActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeHostInvalidBody, "invalid request body", nil)
		return "", false
	}
	container := strings.TrimSpace(req.Container)
	if container == "" {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeHostInvalidContainer, "container is required", nil)
		return "", false
	}
	if !httpx.IsSafeRef(container) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeHostInvalidContainer, "invalid container name", nil)
		return "", false
	}
	return container, true
}

func (c *HostController) parseRemoveContainerAction(ctx *gin.Context) (removeContainerRequest, bool) {
	var req removeContainerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeHostInvalidBody, "invalid request body", nil)
		return removeContainerRequest{}, false
	}
	container := strings.TrimSpace(req.Container)
	if container == "" {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeHostInvalidContainer, "container is required", nil)
		return removeContainerRequest{}, false
	}
	if !httpx.IsSafeRef(container) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeHostInvalidContainer, "invalid container name", nil)
		return removeContainerRequest{}, false
	}
	req.Container = container
	return req, true
}

func (c *HostController) parseProjectAction(ctx *gin.Context) (string, bool) {
	var req projectActionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeHostInvalidBody, "invalid request body", nil)
		return "", false
	}
	project := strings.TrimSpace(req.Project)
	if project == "" {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeHostInvalidProject, "project is required", nil)
		return "", false
	}
	if project == "." || project == ".." || !httpx.IsSafeRef(project) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeHostInvalidProject, "invalid project name", nil)
		return "", false
	}
	return project, true
}
