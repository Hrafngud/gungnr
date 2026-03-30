package controller

import (
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/middleware"
	"go-notes/internal/service"
)

func (c *NetBirdController) logAudit(ctx *gin.Context, action, target string, metadata map[string]any) {
	if c.audit == nil {
		return
	}
	session, _ := middleware.SessionFromContext(ctx)
	_ = c.audit.Log(ctx.Request.Context(), service.AuditEntry{
		UserID:    session.UserID,
		UserLogin: session.Login,
		Action:    action,
		Target:    target,
		Metadata:  metadata,
	})
}

func netBirdModePlanAuditMetadata(plan service.NetBirdModePlan) map[string]any {
	return map[string]any{
		"currentMode":              plan.CurrentMode,
		"targetMode":               plan.TargetMode,
		"allowLocalhost":           plan.AllowLocalhost,
		"currentModeBProjectCount": len(plan.CurrentModeBProjectIDs),
		"targetModeBProjectCount":  len(plan.TargetModeBProjectIDs),
		"groups":                   len(plan.Catalog.Groups),
		"policies":                 len(plan.Catalog.Policies),
		"rebindings":               len(plan.ServiceRebindingOperations),
		"warnings":                 len(plan.Warnings),
	}
}

func netBirdModeApplyAuditMetadata(
	jobID uint,
	req service.NetBirdModeApplyRequest,
	targetMode service.NetBirdMode,
	usedStoredConfig bool,
) map[string]any {
	return map[string]any{
		"jobId":             jobID,
		"targetMode":        targetMode,
		"allowLocalhost":    req.AllowLocalhost,
		"apiBaseUrlSet":     req.APIBaseURL != "",
		"hostPeerIdSet":     req.HostPeerID != "",
		"adminPeerIdCount":  len(req.AdminPeerIDs),
		"modeBProjectCount": len(req.ModeBProjectIDs),
		"usedStoredConfig":  usedStoredConfig,
	}
}

func netBirdModeConfigUpdateAuditMetadata(
	req netBirdModeConfigUpsertRequest,
	updated service.NetBirdModeConfig,
) map[string]any {
	return map[string]any{
		"apiBaseUrlSet":     strings.TrimSpace(updated.APIBaseURL) != "",
		"apiTokenUpdated":   req.APIToken != nil && strings.TrimSpace(*req.APIToken) != "",
		"apiTokenSet":       updated.APITokenSet,
		"hostPeerIdSet":     strings.TrimSpace(updated.HostPeerID) != "",
		"adminPeerIdCount":  len(updated.AdminPeerIDs),
		"modeBProjectCount": len(updated.ModeBProjectIDs),
	}
}

func netBirdPolicyReapplyAuditMetadata(
	req service.NetBirdPolicyReapplyRequest,
	summary service.NetBirdPolicyReapplySummary,
) map[string]any {
	return map[string]any{
		"currentMode":             summary.CurrentMode,
		"defaultPolicyAction":     summary.DefaultPolicy.Action,
		"defaultPolicyResult":     summary.DefaultPolicy.Result.Result,
		"groupResultCounts":       summary.GroupResultCounts,
		"policyResultCounts":      summary.PolicyResultCounts,
		"warningCount":            len(summary.Warnings),
		"requestApiBaseUrlSet":    req.APIBaseURL != "",
		"requestHostPeerIdSet":    req.HostPeerID != "",
		"requestAdminPeerIdCount": len(req.AdminPeerIDs),
	}
}
