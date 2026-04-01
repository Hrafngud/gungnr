package controller

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/models"
	"go-notes/internal/respond"
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
		respond.Err(ctx, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable"), errs.CodeNetBirdUnavailable, "netbird service unavailable")
		return
	}

	status, err := c.service.Status(ctx.Request.Context())
	if err != nil {
		respond.ErrStatus(ctx, netBirdHTTPStatus(err, http.StatusInternalServerError), err, errs.CodeNetBirdStatusFailed, "failed to load netbird status")
		return
	}

	respond.OK(ctx, gin.H{"status": status})
}

func (c *NetBirdController) ACLGraph(ctx *gin.Context) {
	c.Graph(ctx)
}

func (c *NetBirdController) Graph(ctx *gin.Context) {
	if c.service == nil {
		respond.Err(ctx, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable"), errs.CodeNetBirdUnavailable, "netbird service unavailable")
		return
	}

	graph, err := c.service.ACLGraph(ctx.Request.Context())
	if err != nil {
		respond.ErrStatus(ctx, netBirdHTTPStatus(err, http.StatusInternalServerError), err, errs.CodeNetBirdACLGraphFailed, "failed to load netbird acl graph")
		return
	}

	respond.OK(ctx, gin.H{"graph": graph})
}

func (c *NetBirdController) PlanMode(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeNetBirdAdminRequired, "admin role required"), errs.CodeNetBirdAdminRequired, "admin role required")
		return
	}
	if c.service == nil {
		respond.Err(ctx, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable"), errs.CodeNetBirdUnavailable, "netbird service unavailable")
		return
	}

	var req models.NetBirdModePlanRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeNetBirdInvalidBody, "invalid request body"), errs.CodeNetBirdInvalidBody, "invalid request body")
		return
	}

	plan, err := c.service.PlanMode(ctx.Request.Context(), req.TargetMode, req.AllowLocalhost, req.ModeBProjectIDs)
	if err != nil {
		respond.ErrStatus(ctx, netBirdHTTPStatus(err, http.StatusInternalServerError), err, errs.CodeNetBirdPlanFailed, "failed to build netbird mode plan")
		return
	}

	c.logAudit(ctx, "netbird.mode.plan", string(plan.TargetMode), netBirdModePlanAuditMetadata(plan))
	respond.OK(ctx, gin.H{"plan": plan})
}

func (c *NetBirdController) ApplyMode(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeNetBirdAdminRequired, "admin role required"), errs.CodeNetBirdAdminRequired, "admin role required")
		return
	}
	if c.service == nil {
		respond.Err(ctx, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable"), errs.CodeNetBirdUnavailable, "netbird service unavailable")
		return
	}
	if c.jobs == nil {
		respond.Err(ctx, errs.New(errs.CodeNetBirdUnavailable, "job service unavailable"), errs.CodeNetBirdUnavailable, "job service unavailable")
		return
	}

	var req service.NetBirdModeApplyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeNetBirdInvalidBody, "invalid request body"), errs.CodeNetBirdInvalidBody, "invalid request body")
		return
	}
	req = service.NormalizeNetBirdModeApplyRequest(req)
	inlineConfigRequest := req

	targetMode, err := service.ParseNetBirdMode(req.TargetMode)
	if err != nil {
		respond.Err(ctx, err, errs.CodeNetBirdInvalidMode, "invalid netbird target mode")
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
			respond.Err(ctx, resolveErr, errs.CodeNetBirdApplyFailed, "failed to resolve netbird mode config")
			return
		}
		req = resolvedReq
		usedStoredConfig = usedStored
	}

	if req.APIToken == "" {
		respond.Err(ctx, errs.New(errs.CodeNetBirdInvalidBody, "apiToken is required; save NetBird mode config first or provide apiToken in request"), errs.CodeNetBirdInvalidBody, "apiToken is required; save NetBird mode config first or provide apiToken in request")
		return
	}
	if targetMode != service.NetBirdModeLegacy {
		if req.HostPeerID == "" {
			respond.Err(ctx, errs.New(errs.CodeNetBirdInvalidBody, "hostPeerId is required for this mode"), errs.CodeNetBirdInvalidBody, "hostPeerId is required for this mode")
			return
		}
		if len(req.AdminPeerIDs) == 0 {
			respond.Err(ctx, errs.New(errs.CodeNetBirdInvalidBody, "adminPeerIds is required for this mode"), errs.CodeNetBirdInvalidBody, "adminPeerIds is required for this mode")
			return
		}
	}
	if c.settings != nil && inlineConfigProvided {
		update := buildNetBirdConfigUpdateFromApplyRequest(inlineConfigRequest, targetMode)
		_, upsertErr := c.settings.UpsertNetBirdModeConfig(ctx.Request.Context(), update)
		if upsertErr != nil {
			respond.Err(ctx, upsertErr, errs.CodeNetBirdApplyFailed, "failed to persist netbird mode config")
			return
		}
	}

	jobPayload := service.BuildNetBirdModeApplyJobRequest(req, service.NetBirdModeApplyActor{
		UserID: session.UserID,
		Login:  session.Login,
	})
	job, err := c.jobs.Create(ctx.Request.Context(), service.JobTypeNetBirdModeApply, jobPayload)
	if err != nil {
		respond.Err(ctx, err, errs.CodeNetBirdApplyFailed, "failed to queue netbird mode apply job")
		return
	}

	c.logAudit(ctx, "netbird.mode.apply", string(targetMode), netBirdModeApplyAuditMetadata(job.ID, req, targetMode, usedStoredConfig))
	respond.Accepted(ctx, gin.H{"job": models.NewJobResponse(*job)})
}

func (c *NetBirdController) ModeConfig(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeNetBirdAdminRequired, "admin role required"), errs.CodeNetBirdAdminRequired, "admin role required")
		return
	}
	if c.settings == nil {
		respond.Err(ctx, errs.New(errs.CodeNetBirdUnavailable, "settings service unavailable"), errs.CodeNetBirdUnavailable, "settings service unavailable")
		return
	}

	config, err := c.settings.GetNetBirdModeConfig(ctx.Request.Context())
	if err != nil {
		respond.Err(ctx, err, errs.CodeNetBirdUnavailable, "failed to load netbird mode config")
		return
	}

	respond.OK(ctx, gin.H{"config": config})
}

func (c *NetBirdController) UpdateModeConfig(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeNetBirdAdminRequired, "admin role required"), errs.CodeNetBirdAdminRequired, "admin role required")
		return
	}
	if c.settings == nil {
		respond.Err(ctx, errs.New(errs.CodeNetBirdUnavailable, "settings service unavailable"), errs.CodeNetBirdUnavailable, "settings service unavailable")
		return
	}

	var req models.NetBirdModeConfigUpsertRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respond.Err(ctx, errs.New(errs.CodeNetBirdInvalidBody, "invalid request body"), errs.CodeNetBirdInvalidBody, "invalid request body")
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
		respond.Err(ctx, err, errs.CodeNetBirdUnavailable, "failed to persist netbird mode config")
		return
	}

	c.logAudit(ctx, "netbird.mode.config.update", "settings", netBirdModeConfigUpdateAuditMetadata(req, updated))
	respond.OK(ctx, gin.H{"config": updated})
}

func (c *NetBirdController) ReapplyPolicies(ctx *gin.Context) {
	session, ok := middleware.SessionFromContext(ctx)
	if !ok || !isAdminRole(session.Role) {
		respond.Err(ctx, errs.New(errs.CodeNetBirdAdminRequired, "admin role required"), errs.CodeNetBirdAdminRequired, "admin role required")
		return
	}
	if c.service == nil {
		respond.Err(ctx, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable"), errs.CodeNetBirdUnavailable, "netbird service unavailable")
		return
	}

	var req service.NetBirdPolicyReapplyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		respond.Err(ctx, errs.New(errs.CodeNetBirdInvalidBody, "invalid request body"), errs.CodeNetBirdInvalidBody, "invalid request body")
		return
	}
	req = service.NormalizeNetBirdPolicyReapplyRequest(req)

	summary, err := c.service.ReapplyPolicies(ctx.Request.Context(), req)
	if err != nil {
		respond.Err(ctx, err, errs.CodeNetBirdReapplyFailed, "failed to reapply netbird policies")
		return
	}

	c.logAudit(ctx, "netbird.policy.reapply", string(summary.CurrentMode), netBirdPolicyReapplyAuditMetadata(req, summary))
	respond.OK(ctx, gin.H{"summary": summary})
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
