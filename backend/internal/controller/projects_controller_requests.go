package controller

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
	"go-notes/internal/service"
	"go-notes/internal/utils/httpx"
)

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
