package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-notes/internal/jobs"
	"go-notes/internal/models"
)

const netBirdDefaultPolicyActionNone = "none"

type NetBirdWorkflows struct {
	netbird *NetBirdService
	audit   *AuditService
}

func NewNetBirdWorkflows(netbird *NetBirdService, audit *AuditService) *NetBirdWorkflows {
	return &NetBirdWorkflows{
		netbird: netbird,
		audit:   audit,
	}
}

func (w *NetBirdWorkflows) Register(runner *jobs.Runner) {
	if runner == nil {
		return
	}
	runner.Register(JobTypeNetBirdModeApply, w.handleModeApply)
}

func (w *NetBirdWorkflows) handleModeApply(ctx context.Context, job models.Job, logger jobs.Logger) error {
	if w.netbird == nil {
		return fmt.Errorf("netbird service unavailable")
	}

	var req NetBirdModeApplyJobRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse netbird mode apply request: %w", err)
	}
	req = normalizeNetBirdModeApplyJobRequest(req)

	targetMode, err := ParseNetBirdMode(req.TargetMode)
	if err != nil {
		return err
	}
	if req.APIToken == "" {
		return fmt.Errorf("apiToken is required")
	}
	if targetMode != NetBirdModeLegacy {
		if req.HostPeerID == "" {
			return fmt.Errorf("hostPeerId is required for target mode %q", targetMode)
		}
		if len(req.AdminPeerIDs) == 0 {
			return fmt.Errorf("adminPeerIds is required for target mode %q", targetMode)
		}
	}

	logger.Logf("step 1/3: building mode plan for target mode %s", targetMode)
	plan, err := w.netbird.PlanMode(ctx, string(targetMode), req.AllowLocalhost)
	if err != nil {
		return err
	}
	logger.Logf(
		"plan ready: groups=%d policies=%d rebindings=%d warnings=%d",
		len(plan.Catalog.Groups),
		len(plan.Catalog.Policies),
		len(plan.ServiceRebindingOperations),
		len(plan.Warnings),
	)

	defaultPolicyAction := netBirdDefaultPolicyActionDisable
	if plan.TargetMode == NetBirdModeLegacy {
		defaultPolicyAction = netBirdDefaultPolicyActionNone
	}

	logger.Log("step 2/3: reconciling managed groups and policies with NetBird API")
	reconcileResult, err := w.netbird.ReconcileManagedCatalogWithToken(ctx, req.APIBaseURL, req.APIToken, NetBirdReconcileInput{
		Catalog:             plan.Catalog,
		HostPeerID:          req.HostPeerID,
		AdminPeerIDs:        req.AdminPeerIDs,
		DefaultPolicyAction: defaultPolicyAction,
	})
	if err != nil {
		return err
	}
	groupCounts := countNetBirdResults(reconcileResult.GroupOperations)
	policyCounts := countNetBirdResults(reconcileResult.PolicyOperations)
	logger.Logf(
		"reconcile results: groups(created=%d updated=%d deleted=%d unchanged=%d) policies(created=%d updated=%d deleted=%d unchanged=%d)",
		groupCounts.Created,
		groupCounts.Updated,
		groupCounts.Deleted,
		groupCounts.Unchanged,
		policyCounts.Created,
		policyCounts.Updated,
		policyCounts.Deleted,
		policyCounts.Unchanged,
	)

	summary := NetBirdModeApplySummary{
		CurrentMode:         plan.CurrentMode,
		TargetMode:          plan.TargetMode,
		AllowLocalhost:      plan.AllowLocalhost,
		DefaultPolicyAction: defaultPolicyAction,
		Plan:                plan,
		Reconcile:           reconcileResult,
		GroupResultCounts:   groupCounts,
		PolicyResultCounts:  policyCounts,
		Warnings:            append([]string(nil), plan.Warnings...),
		RequestedBy:         req.RequestedBy,
		RequestedAt:         req.RequestedAt,
		CompletedAt:         time.Now().UTC(),
	}

	logger.Log("step 3/3: writing deterministic mode apply summary payload")
	summaryRaw, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("encode mode apply summary: %w", err)
	}
	logger.Logf("netbird_mode_apply_summary=%s", string(summaryRaw))

	if w.audit != nil {
		metadata := map[string]any{
			"jobId":               job.ID,
			"targetMode":          summary.TargetMode,
			"allowLocalhost":      summary.AllowLocalhost,
			"defaultPolicyAction": summary.DefaultPolicyAction,
			"groups":              len(summary.Plan.Catalog.Groups),
			"policies":            len(summary.Plan.Catalog.Policies),
			"groupResultCounts":   summary.GroupResultCounts,
			"policyResultCounts":  summary.PolicyResultCounts,
			"warnings":            summary.Warnings,
		}
		if err := w.audit.Log(ctx, AuditEntry{
			UserID:    summary.RequestedBy.UserID,
			UserLogin: summary.RequestedBy.Login,
			Action:    "netbird.mode.apply.completed",
			Target:    string(summary.TargetMode),
			Metadata:  metadata,
		}); err != nil {
			logger.Logf("audit warning: failed to write netbird mode apply completion event: %v", err)
		}
	}

	return nil
}

func normalizeNetBirdModeApplyJobRequest(input NetBirdModeApplyJobRequest) NetBirdModeApplyJobRequest {
	input.TargetMode = strings.ToLower(strings.TrimSpace(input.TargetMode))
	input.APIBaseURL = strings.TrimSpace(input.APIBaseURL)
	input.APIToken = strings.TrimSpace(input.APIToken)
	input.HostPeerID = strings.TrimSpace(input.HostPeerID)
	input.AdminPeerIDs = normalizeStringList(input.AdminPeerIDs)
	input.RequestedBy.Login = strings.TrimSpace(input.RequestedBy.Login)
	return input
}

func countNetBirdResults(ops []NetBirdReconcileOperation) NetBirdOperationCounts {
	counts := NetBirdOperationCounts{}
	for _, op := range ops {
		switch strings.ToLower(strings.TrimSpace(op.Result)) {
		case netBirdResultCreated:
			counts.Created++
		case netBirdResultUpdated:
			counts.Updated++
		case netBirdResultDeleted:
			counts.Deleted++
		default:
			counts.Unchanged++
		}
	}
	return counts
}
