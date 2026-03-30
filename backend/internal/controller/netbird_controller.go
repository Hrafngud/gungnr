package controller

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/service"
)

type NetBirdController struct {
	service  *service.NetBirdService
	settings *service.SettingsService
	jobs     *service.JobService
	audit    *service.AuditService
}

func NewNetBirdController(service *service.NetBirdService, settings *service.SettingsService, jobs *service.JobService, audit *service.AuditService) *NetBirdController {
	return &NetBirdController{
		service:  service,
		settings: settings,
		jobs:     jobs,
		audit:    audit,
	}
}

func (c *NetBirdController) Status(ctx *gin.Context) {
	if c.service == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeNetBirdUnavailable, "netbird service unavailable", nil)
		return
	}

	status, err := c.service.Status(ctx.Request.Context())
	if err != nil {
		httpStatus := netBirdHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, httpStatus, err, errs.CodeNetBirdStatusFailed, "failed to load netbird status")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": status})
}

func (c *NetBirdController) ACLGraph(ctx *gin.Context) {
	c.Graph(ctx)
}

func (c *NetBirdController) Graph(ctx *gin.Context) {
	if c.service == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeNetBirdUnavailable, "netbird service unavailable", nil)
		return
	}

	graph, err := c.service.ACLGraph(ctx.Request.Context())
	if err != nil {
		httpStatus := netBirdHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, httpStatus, err, errs.CodeNetBirdACLGraphFailed, "failed to load netbird acl graph")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"graph": graph})
}

func (c *NetBirdController) PlanMode(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeNetBirdAdminRequired, "admin role required", nil)
		return
	}
	if c.service == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeNetBirdUnavailable, "netbird service unavailable", nil)
		return
	}

	var req netBirdModePlanRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeNetBirdInvalidBody, "invalid request body", nil)
		return
	}

	plan, err := c.service.PlanMode(ctx.Request.Context(), req.TargetMode, req.AllowLocalhost, req.ModeBProjectIDs)
	if err != nil {
		status := netBirdHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeNetBirdPlanFailed, "failed to build netbird mode plan")
		return
	}

	c.logAudit(ctx, "netbird.mode.plan", string(plan.TargetMode), netBirdModePlanAuditMetadata(plan))
	ctx.JSON(http.StatusOK, gin.H{"plan": plan})
}

func (c *NetBirdController) ApplyMode(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeNetBirdAdminRequired, "admin role required", nil)
		return
	}
	if c.service == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeNetBirdUnavailable, "netbird service unavailable", nil)
		return
	}
	if c.jobs == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeNetBirdUnavailable, "job service unavailable", nil)
		return
	}

	var req service.NetBirdModeApplyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeNetBirdInvalidBody, "invalid request body", nil)
		return
	}
	req = service.NormalizeNetBirdModeApplyRequest(req)
	inlineConfigRequest := req

	targetMode, err := service.ParseNetBirdMode(req.TargetMode)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeNetBirdInvalidMode, "invalid netbird target mode")
		return
	}
	inlineConfigProvided := inlineConfigRequest.APIToken != "" ||
		inlineConfigRequest.APIBaseURL != "" ||
		inlineConfigRequest.HostPeerID != "" ||
		len(inlineConfigRequest.AdminPeerIDs) > 0 ||
		len(inlineConfigRequest.ModeBProjectIDs) > 0 ||
		targetMode == service.NetBirdModeB

	usedStoredConfig := false
	if c.settings != nil {
		resolvedReq, usedStored, resolveErr := c.settings.ResolveNetBirdModeApplyRequest(ctx.Request.Context(), req)
		if resolveErr != nil {
			apierror.RespondWithError(ctx, http.StatusInternalServerError, resolveErr, errs.CodeNetBirdApplyFailed, "failed to resolve netbird mode config")
			return
		}
		req = resolvedReq
		usedStoredConfig = usedStored
	}

	if req.APIToken == "" {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeNetBirdInvalidBody, "apiToken is required; save NetBird mode config first or provide apiToken in request", nil)
		return
	}
	if targetMode != service.NetBirdModeLegacy {
		if req.HostPeerID == "" {
			apierror.Respond(ctx, http.StatusBadRequest, errs.CodeNetBirdInvalidBody, "hostPeerId is required for this mode", nil)
			return
		}
		if len(req.AdminPeerIDs) == 0 {
			apierror.Respond(ctx, http.StatusBadRequest, errs.CodeNetBirdInvalidBody, "adminPeerIds is required for this mode", nil)
			return
		}
	}
	if c.settings != nil && inlineConfigProvided {
		update := buildNetBirdConfigUpdateFromApplyRequest(inlineConfigRequest, targetMode)
		_, upsertErr := c.settings.UpsertNetBirdModeConfig(ctx.Request.Context(), update)
		if upsertErr != nil {
			apierror.RespondWithError(ctx, http.StatusInternalServerError, upsertErr, errs.CodeNetBirdApplyFailed, "failed to persist netbird mode config")
			return
		}
	}

	jobPayload := service.BuildNetBirdModeApplyJobRequest(req, service.NetBirdModeApplyActor{
		UserID: session.UserID,
		Login:  session.Login,
	})
	job, err := c.jobs.Create(ctx.Request.Context(), service.JobTypeNetBirdModeApply, jobPayload)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeNetBirdApplyFailed, "failed to queue netbird mode apply job")
		return
	}

	c.logAudit(ctx, "netbird.mode.apply", string(targetMode), netBirdModeApplyAuditMetadata(job.ID, req, targetMode, usedStoredConfig))
	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job)})
}

func (c *NetBirdController) ModeConfig(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeNetBirdAdminRequired, "admin role required", nil)
		return
	}
	if c.settings == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeNetBirdUnavailable, "settings service unavailable", nil)
		return
	}

	config, err := c.settings.GetNetBirdModeConfig(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeNetBirdUnavailable, "failed to load netbird mode config")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"config": config})
}

func (c *NetBirdController) UpdateModeConfig(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeNetBirdAdminRequired, "admin role required", nil)
		return
	}
	if c.settings == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeNetBirdUnavailable, "settings service unavailable", nil)
		return
	}

	var req netBirdModeConfigUpsertRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeNetBirdInvalidBody, "invalid request body", nil)
		return
	}

	updated, err := c.settings.UpsertNetBirdModeConfig(ctx.Request.Context(), service.NetBirdModeConfigUpdate{
		APIBaseURL:      req.APIBaseURL,
		APIToken:        req.APIToken,
		HostPeerID:      req.HostPeerID,
		AdminPeerIDs:    req.AdminPeerIDs,
		ModeBProjectIDs: req.ModeBProjectIDs,
	})
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeNetBirdUnavailable, "failed to persist netbird mode config")
		return
	}

	c.logAudit(ctx, "netbird.mode.config.update", "settings", netBirdModeConfigUpdateAuditMetadata(req, updated))
	ctx.JSON(http.StatusOK, gin.H{"config": updated})
}

func (c *NetBirdController) ReapplyPolicies(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		apierror.Respond(ctx, http.StatusForbidden, errs.CodeNetBirdAdminRequired, "admin role required", nil)
		return
	}
	if c.service == nil {
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeNetBirdUnavailable, "netbird service unavailable", nil)
		return
	}

	var req service.NetBirdPolicyReapplyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeNetBirdInvalidBody, "invalid request body", nil)
		return
	}
	req = service.NormalizeNetBirdPolicyReapplyRequest(req)

	summary, err := c.service.ReapplyPolicies(ctx.Request.Context(), req)
	if err != nil {
		status := netBirdHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeNetBirdReapplyFailed, "failed to reapply netbird policies")
		return
	}

	c.logAudit(ctx, "netbird.policy.reapply", string(summary.CurrentMode), netBirdPolicyReapplyAuditMetadata(req, summary))
	ctx.JSON(http.StatusOK, gin.H{"summary": summary})
}

func buildNetBirdConfigUpdateFromApplyRequest(req service.NetBirdModeApplyRequest, targetMode service.NetBirdMode) service.NetBirdModeConfigUpdate {
	update := service.NetBirdModeConfigUpdate{}
	if req.APIBaseURL != "" {
		value := req.APIBaseURL
		update.APIBaseURL = &value
	}
	if req.APIToken != "" {
		value := req.APIToken
		update.APIToken = &value
	}
	if req.HostPeerID != "" {
		value := req.HostPeerID
		update.HostPeerID = &value
	}
	if len(req.AdminPeerIDs) > 0 {
		value := append([]string(nil), req.AdminPeerIDs...)
		update.AdminPeerIDs = &value
	}
	if len(req.ModeBProjectIDs) > 0 || targetMode == service.NetBirdModeB {
		value := append([]uint(nil), req.ModeBProjectIDs...)
		update.ModeBProjectIDs = &value
	}
	return update
}
