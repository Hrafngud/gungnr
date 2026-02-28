package service

import (
	"context"
	"fmt"
	"strings"

	"go-notes/internal/errs"
)

func (s *NetBirdService) ReapplyPolicies(ctx context.Context, input NetBirdPolicyReapplyRequest) (NetBirdPolicyReapplySummary, error) {
	if s == nil || s.projects == nil {
		return NetBirdPolicyReapplySummary{}, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable")
	}

	request := NormalizeNetBirdPolicyReapplyRequest(input)
	usedStoredConfig := false
	if s.settings != nil {
		resolvedReq, usedStored, err := s.settings.ResolveNetBirdModeApplyRequest(ctx, NetBirdModeApplyRequest{
			APIBaseURL:   request.APIBaseURL,
			APIToken:     request.APIToken,
			HostPeerID:   request.HostPeerID,
			AdminPeerIDs: request.AdminPeerIDs,
		})
		if err != nil {
			return NetBirdPolicyReapplySummary{}, errs.Wrap(errs.CodeNetBirdReapplyFailed, "failed to resolve saved netbird mode config", err)
		}
		request = NormalizeNetBirdPolicyReapplyRequest(NetBirdPolicyReapplyRequest{
			APIBaseURL:   resolvedReq.APIBaseURL,
			APIToken:     resolvedReq.APIToken,
			HostPeerID:   resolvedReq.HostPeerID,
			AdminPeerIDs: resolvedReq.AdminPeerIDs,
		})
		usedStoredConfig = usedStored
	}
	runtimeState := s.resolveNetBirdRuntimeState(ctx)
	currentMode := runtimeState.EffectiveMode
	panelPort, panelPortFallback := resolvePanelPort(s.cfg.Port)
	projectInputs, projectWarnings, err := s.loadProjectCatalogInputs(ctx)
	if err != nil {
		return NetBirdPolicyReapplySummary{}, errs.Wrap(errs.CodeNetBirdReapplyFailed, "failed to load netbird policy reapply project catalog", err)
	}
	runtimeModeProjects := []NetBirdProjectCatalogInput{}
	runtimeProjectWarnings := []string{}
	if currentMode == NetBirdModeB {
		runtimeModeProjects, runtimeProjectWarnings = selectModeBProjects(projectInputs, runtimeState.EffectiveModeBProjectIDs)
	}
	catalog := BuildNetBirdCatalog(NetBirdCatalogInput{
		Mode:      currentMode,
		PanelPort: panelPort,
		Projects:  runtimeModeProjects,
	})

	warnings := make([]string, 0, 7+len(projectWarnings)+len(runtimeState.Warnings))
	warnings = append(warnings, runtimeState.Warnings...)
	if usedStoredConfig {
		warnings = append(warnings, "Missing policy reapply context was populated from saved NetBird mode config.")
	}
	if panelPortFallback {
		warnings = append(warnings, "Panel port was not a valid integer; policy reapply used default port 8080.")
	}
	warnings = append(warnings, projectWarnings...)
	warnings = append(warnings, runtimeProjectWarnings...)

	if netBirdReapplyNeedsContext(currentMode, request) {
		snapshot, err := s.latestModeApplySnapshot(ctx)
		if err != nil {
			return NetBirdPolicyReapplySummary{}, errs.Wrap(errs.CodeNetBirdReapplyFailed, "failed to load latest netbird mode apply context", err)
		}
		if snapshot.RequestParseError != nil {
			warnings = append(warnings, "Latest NetBird mode apply request could not be parsed; automatic reapply context fill may be incomplete.")
		}
		if snapshot.RequestParsed {
			var usedLatest bool
			request, usedLatest = fillNetBirdReapplyFromSnapshot(request, snapshot.Request)
			if usedLatest {
				warnings = append(warnings, "Missing policy reapply context was populated from the latest NetBird mode apply job.")
			}
		}
	}

	if request.APIToken == "" {
		return NetBirdPolicyReapplySummary{}, errs.New(errs.CodeNetBirdInvalidBody, "apiToken is required")
	}
	if currentMode != NetBirdModeLegacy {
		if request.HostPeerID == "" {
			return NetBirdPolicyReapplySummary{}, errs.New(errs.CodeNetBirdInvalidBody, fmt.Sprintf("hostPeerId is required for current mode %q", currentMode))
		}
		if len(request.AdminPeerIDs) == 0 {
			return NetBirdPolicyReapplySummary{}, errs.New(errs.CodeNetBirdInvalidBody, fmt.Sprintf("adminPeerIds is required for current mode %q", currentMode))
		}
	}

	defaultPolicyAction := netBirdDefaultPolicyActionDisable
	if currentMode == NetBirdModeLegacy {
		defaultPolicyAction = netBirdDefaultPolicyActionNone
	}

	reconcileResult, err := s.ReconcileManagedCatalogWithToken(ctx, request.APIBaseURL, request.APIToken, NetBirdReconcileInput{
		Catalog:             catalog,
		HostPeerID:          request.HostPeerID,
		AdminPeerIDs:        request.AdminPeerIDs,
		DefaultPolicyAction: defaultPolicyAction,
	})
	if err != nil {
		return NetBirdPolicyReapplySummary{}, errs.Wrap(errs.CodeNetBirdReapplyFailed, "failed to reapply netbird managed policies", err)
	}

	return NetBirdPolicyReapplySummary{
		CurrentMode: currentMode,
		DefaultPolicy: NetBirdDefaultPolicySummary{
			Action: defaultPolicyAction,
			Result: reconcileResult.DefaultPolicy,
		},
		GroupResultCounts:  countNetBirdResults(reconcileResult.GroupOperations),
		PolicyResultCounts: countNetBirdResults(reconcileResult.PolicyOperations),
		GroupOperations:    reconcileResult.GroupOperations,
		PolicyOperations:   reconcileResult.PolicyOperations,
		Warnings:           warnings,
	}, nil
}

func netBirdReapplyNeedsContext(mode NetBirdMode, request NetBirdPolicyReapplyRequest) bool {
	if strings.TrimSpace(request.APIToken) == "" {
		return true
	}
	if mode == NetBirdModeLegacy {
		return false
	}
	if strings.TrimSpace(request.HostPeerID) == "" {
		return true
	}
	return len(request.AdminPeerIDs) == 0
}

func fillNetBirdReapplyFromSnapshot(request NetBirdPolicyReapplyRequest, source NetBirdModeApplyJobRequest) (NetBirdPolicyReapplyRequest, bool) {
	merged := request
	used := false

	if strings.TrimSpace(merged.APIBaseURL) == "" {
		if value := strings.TrimSpace(source.APIBaseURL); value != "" {
			merged.APIBaseURL = value
			used = true
		}
	}
	if strings.TrimSpace(merged.APIToken) == "" {
		if value := strings.TrimSpace(source.APIToken); value != "" {
			merged.APIToken = value
			used = true
		}
	}
	if strings.TrimSpace(merged.HostPeerID) == "" {
		if value := strings.TrimSpace(source.HostPeerID); value != "" {
			merged.HostPeerID = value
			used = true
		}
	}
	if len(merged.AdminPeerIDs) == 0 && len(source.AdminPeerIDs) > 0 {
		merged.AdminPeerIDs = append([]string(nil), source.AdminPeerIDs...)
		used = true
	}

	return NormalizeNetBirdPolicyReapplyRequest(merged), used
}
