package controller

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/service"
)

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

	ctx.JSON(http.StatusOK, gin.H{"stack": stack})
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

	ctx.JSON(http.StatusOK, gin.H{"graph": graph})
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

	ctx.JSON(http.StatusOK, gin.H{"catalog": catalog})
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
