package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/service"
)

type NetBirdController struct {
	service *service.NetBirdService
	jobs    *service.JobService
	audit   *service.AuditService
}

type netBirdModePlanRequest struct {
	TargetMode     string `json:"targetMode"`
	AllowLocalhost bool   `json:"allowLocalhost"`
}

func NewNetBirdController(service *service.NetBirdService, jobs *service.JobService, audit *service.AuditService) *NetBirdController {
	return &NetBirdController{
		service: service,
		jobs:    jobs,
		audit:   audit,
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

	plan, err := c.service.PlanMode(ctx.Request.Context(), req.TargetMode, req.AllowLocalhost)
	if err != nil {
		status := netBirdHTTPStatus(err, http.StatusInternalServerError)
		apierror.RespondWithError(ctx, status, err, errs.CodeNetBirdPlanFailed, "failed to build netbird mode plan")
		return
	}

	c.logAudit(ctx, session.UserID, session.Login, plan)
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

	targetMode, err := service.ParseNetBirdMode(req.TargetMode)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusBadRequest, err, errs.CodeNetBirdInvalidMode, "invalid netbird target mode")
		return
	}
	if req.APIToken == "" {
		apierror.Respond(ctx, http.StatusBadRequest, errs.CodeNetBirdInvalidBody, "apiToken is required", nil)
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

	jobPayload := service.BuildNetBirdModeApplyJobRequest(req, service.NetBirdModeApplyActor{
		UserID: session.UserID,
		Login:  session.Login,
	})
	job, err := c.jobs.Create(ctx.Request.Context(), service.JobTypeNetBirdModeApply, jobPayload)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeNetBirdApplyFailed, "failed to queue netbird mode apply job")
		return
	}

	if c.audit != nil {
		_ = c.audit.Log(ctx.Request.Context(), service.AuditEntry{
			UserID:    session.UserID,
			UserLogin: session.Login,
			Action:    "netbird.mode.apply",
			Target:    string(targetMode),
			Metadata: map[string]any{
				"jobId":            job.ID,
				"targetMode":       targetMode,
				"allowLocalhost":   req.AllowLocalhost,
				"apiBaseUrlSet":    req.APIBaseURL != "",
				"hostPeerIdSet":    req.HostPeerID != "",
				"adminPeerIdCount": len(req.AdminPeerIDs),
			},
		})
	}

	ctx.JSON(http.StatusAccepted, gin.H{"job": newJobResponse(*job)})
}

func (c *NetBirdController) logAudit(ctx *gin.Context, userID uint, userLogin string, plan service.NetBirdModePlan) {
	if c.audit == nil {
		return
	}
	_ = c.audit.Log(ctx.Request.Context(), service.AuditEntry{
		UserID:    userID,
		UserLogin: userLogin,
		Action:    "netbird.mode.plan",
		Target:    string(plan.TargetMode),
		Metadata: map[string]any{
			"currentMode":    plan.CurrentMode,
			"targetMode":     plan.TargetMode,
			"allowLocalhost": plan.AllowLocalhost,
			"groups":         len(plan.Catalog.Groups),
			"policies":       len(plan.Catalog.Policies),
			"rebindings":     len(plan.ServiceRebindingOperations),
			"warnings":       len(plan.Warnings),
		},
	})
}

func netBirdHTTPStatus(err error, fallback int) int {
	typed, ok := errs.From(err)
	if !ok {
		return fallback
	}
	switch typed.Code {
	case errs.CodeNetBirdInvalidMode, errs.CodeNetBirdInvalidBody:
		return http.StatusBadRequest
	case errs.CodeNetBirdUnavailable:
		return http.StatusInternalServerError
	case errs.CodeNetBirdStatusFailed, errs.CodeNetBirdACLGraphFailed, errs.CodeNetBirdPlanFailed:
		return http.StatusBadGateway
	case errs.CodeNetBirdApplyFailed:
		return http.StatusInternalServerError
	default:
		if strings.HasPrefix(string(typed.Code), "NETBIRD-400") {
			return http.StatusBadRequest
		}
		return fallback
	}
}
