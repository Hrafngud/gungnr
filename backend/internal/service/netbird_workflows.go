package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"go-notes/internal/errs"
	"go-notes/internal/jobs"
	"go-notes/internal/models"
)

const (
	netBirdDefaultPolicyActionNone = "none"
	netBirdExecutionSucceeded      = "succeeded"
	netBirdExecutionFailed         = "failed"
	netBirdExecutionSkipped        = "skipped"
)

var netBirdPanelServiceCandidates = map[string]struct{}{
	"proxy": {},
	"nginx": {},
}

type NetBirdWorkflows struct {
	netbird *NetBirdService
	host    *HostService
	audit   *AuditService
}

func NewNetBirdWorkflows(netbird *NetBirdService, host *HostService, audit *AuditService) *NetBirdWorkflows {
	return &NetBirdWorkflows{
		netbird: netbird,
		host:    host,
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

	logger.Logf("step 1/5: building mode plan for target mode %s", targetMode)
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

	logger.Log("step 2/5: reconciling managed groups and policies with NetBird API")
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

	logger.Log("step 3/5: executing service rebinding operations through bridge-backed host primitives")
	rebindingExecution, rebindingErr := w.executeServiceRebindings(ctx, job, plan.ServiceRebindingOperations, logger)
	if rebindingErr != nil {
		logger.Logf("service rebinding completed with failures: %v", rebindingErr)
	}

	logger.Log("step 4/5: executing deterministic redeploy targets")
	redeployExecution, redeployErr := w.executeRedeployTargets(ctx, job, plan.RedeployTargets, rebindingExecution, logger)
	if redeployErr != nil {
		logger.Logf("redeploy execution completed with failures: %v", redeployErr)
	}

	warnings := append([]string(nil), plan.Warnings...)
	warnings = appendNetBirdExecutionWarnings(warnings, rebindingExecution, redeployExecution)

	summary := NetBirdModeApplySummary{
		CurrentMode:         plan.CurrentMode,
		TargetMode:          plan.TargetMode,
		AllowLocalhost:      plan.AllowLocalhost,
		DefaultPolicyAction: defaultPolicyAction,
		Plan:                plan,
		Reconcile:           reconcileResult,
		RebindingExecution:  rebindingExecution,
		RedeployExecution:   redeployExecution,
		GroupResultCounts:   groupCounts,
		PolicyResultCounts:  policyCounts,
		Warnings:            warnings,
		RequestedBy:         req.RequestedBy,
		RequestedAt:         req.RequestedAt,
		CompletedAt:         time.Now().UTC(),
	}

	logger.Log("step 5/5: writing deterministic mode apply summary payload")
	summaryRaw, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("encode mode apply summary: %w", err)
	}
	logger.Logf("netbird_mode_apply_summary=%s", string(summaryRaw))

	applyExecutionErr := joinNetBirdExecutionErrors(rebindingErr, redeployErr)
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
			"rebindingCounts":     summary.RebindingExecution.Counts,
			"redeployCounts":      summary.RedeployExecution.Counts,
			"executionFailed":     applyExecutionErr != nil,
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

	if applyExecutionErr != nil {
		return applyExecutionErr
	}

	return nil
}

func (w *NetBirdWorkflows) executeServiceRebindings(
	ctx context.Context,
	job models.Job,
	operations []NetBirdServiceRebindingOperation,
	logger jobs.Logger,
) (NetBirdRebindingExecutionSummary, error) {
	summary := NetBirdRebindingExecutionSummary{
		Operations: make([]NetBirdRebindingExecutionOperation, 0, len(operations)),
	}
	if len(operations) == 0 {
		logger.Log("no service rebinding operations required")
		return summary, nil
	}

	failures := 0
	for idx, op := range operations {
		requestID := fmt.Sprintf("job-%d-netbird-rebind-%02d", job.ID, idx+1)
		entry := NetBirdRebindingExecutionOperation{
			Service:       op.Service,
			ProjectID:     op.ProjectID,
			ProjectName:   strings.TrimSpace(op.ProjectName),
			Port:          op.Port,
			FromListeners: append([]string(nil), op.FromListeners...),
			ToListeners:   append([]string(nil), op.ToListeners...),
			Reason:        strings.TrimSpace(op.Reason),
			Result:        netBirdExecutionSucceeded,
			RequestID:     requestID,
		}

		logger.Logf(
			"rebinding %d/%d: service=%s project=%q port=%d from=%s to=%s",
			idx+1,
			len(operations),
			op.Service,
			strings.TrimSpace(op.ProjectName),
			op.Port,
			strings.Join(op.FromListeners, ","),
			strings.Join(op.ToListeners, ","),
		)

		execErr := w.executeSingleRebinding(ctx, requestID, op, logger)
		if execErr != nil {
			failures++
			entry.Result = netBirdExecutionFailed
			entry.Message = execErr.Error()
			entry.Diagnostics = netBirdExecutionDiagnosticsFromError(execErr)
			logger.Logf("rebinding failed for service=%s project=%q: %v", op.Service, strings.TrimSpace(op.ProjectName), execErr)
		} else {
			entry.Message = "listener rebinding operation applied through bridge-backed service execution"
			logger.Logf("rebinding completed for service=%s project=%q", op.Service, strings.TrimSpace(op.ProjectName))
		}

		summary.Operations = append(summary.Operations, entry)
	}

	summary.Counts = countRebindingExecutionResults(summary.Operations)
	if failures > 0 {
		return summary, fmt.Errorf("%d service rebinding operation(s) failed", failures)
	}
	return summary, nil
}

func (w *NetBirdWorkflows) executeSingleRebinding(
	ctx context.Context,
	requestID string,
	op NetBirdServiceRebindingOperation,
	logger jobs.Logger,
) error {
	if w.host == nil {
		return fmt.Errorf("host service unavailable")
	}

	switch strings.ToLower(strings.TrimSpace(op.Service)) {
	case "panel":
		return w.redeployPanel(ctx, requestID, logger)
	case "project_ingress":
		project := strings.TrimSpace(op.ProjectName)
		if project == "" {
			return fmt.Errorf("projectName is required for service %q", op.Service)
		}
		return w.host.RestartProjectStackWithLogger(ctx, requestID, project, logger)
	default:
		return fmt.Errorf("unsupported rebinding service %q", strings.TrimSpace(op.Service))
	}
}

func (w *NetBirdWorkflows) executeRedeployTargets(
	ctx context.Context,
	job models.Job,
	targets NetBirdRedeployTargets,
	rebinding NetBirdRebindingExecutionSummary,
	logger jobs.Logger,
) (NetBirdRedeployExecutionSummary, error) {
	summary := NetBirdRedeployExecutionSummary{
		Projects: make([]NetBirdRedeployExecutionTarget, 0, len(targets.Projects)),
	}
	panelCovered, projectCoverage := rebindingSuccessCoverage(rebinding)
	failures := 0

	if !targets.Panel && len(targets.Projects) == 0 {
		logger.Log("no redeploy targets required")
		return summary, nil
	}

	if targets.Panel {
		requestID := fmt.Sprintf("job-%d-netbird-redeploy-panel", job.ID)
		panelEntry := NetBirdRedeployExecutionTarget{
			Service:   "panel",
			Reason:    "Panel listener binding change requires panel redeploy.",
			Result:    netBirdExecutionSucceeded,
			RequestID: requestID,
		}
		if panelCovered {
			panelEntry.Result = netBirdExecutionSkipped
			panelEntry.Message = "panel redeploy already applied during service rebinding execution"
			logger.Log("redeploy panel skipped: already applied during rebinding step")
		} else {
			logger.Log("redeploy panel: restarting panel ingress container(s)")
			if err := w.redeployPanel(ctx, requestID, logger); err != nil {
				failures++
				panelEntry.Result = netBirdExecutionFailed
				panelEntry.Message = err.Error()
				panelEntry.Diagnostics = netBirdExecutionDiagnosticsFromError(err)
				logger.Logf("redeploy panel failed: %v", err)
			} else {
				panelEntry.Message = "panel redeploy completed through bridge-backed restart"
				logger.Log("redeploy panel completed")
			}
		}
		summary.Panel = &panelEntry
	}

	for idx, target := range targets.Projects {
		requestID := fmt.Sprintf("job-%d-netbird-redeploy-project-%02d", job.ID, idx+1)
		if target.ProjectID != 0 {
			requestID = fmt.Sprintf("job-%d-netbird-redeploy-project-%d", job.ID, target.ProjectID)
		}
		entry := NetBirdRedeployExecutionTarget{
			Service:     "project_ingress",
			ProjectID:   target.ProjectID,
			ProjectName: strings.TrimSpace(target.ProjectName),
			Port:        target.Port,
			Reason:      strings.TrimSpace(target.Reason),
			Result:      netBirdExecutionSucceeded,
			RequestID:   requestID,
		}

		key := netBirdProjectTargetKey(target.ProjectID, target.ProjectName)
		if _, covered := projectCoverage[key]; covered {
			entry.Result = netBirdExecutionSkipped
			entry.Message = "project redeploy already applied during service rebinding execution"
			logger.Logf("redeploy project %q skipped: already applied during rebinding step", strings.TrimSpace(target.ProjectName))
			summary.Projects = append(summary.Projects, entry)
			continue
		}

		logger.Logf("redeploy project %q (%d/%d)", strings.TrimSpace(target.ProjectName), idx+1, len(targets.Projects))
		if w.host == nil {
			failures++
			entry.Result = netBirdExecutionFailed
			entry.Message = "host service unavailable"
			logger.Logf("redeploy project %q failed: host service unavailable", strings.TrimSpace(target.ProjectName))
			summary.Projects = append(summary.Projects, entry)
			continue
		}
		if strings.TrimSpace(target.ProjectName) == "" {
			failures++
			entry.Result = netBirdExecutionFailed
			entry.Message = "projectName is required for project redeploy target"
			logger.Log("redeploy project failed: missing projectName in redeploy target")
			summary.Projects = append(summary.Projects, entry)
			continue
		}

		if err := w.host.RestartProjectStackWithLogger(ctx, requestID, target.ProjectName, logger); err != nil {
			failures++
			entry.Result = netBirdExecutionFailed
			entry.Message = err.Error()
			entry.Diagnostics = netBirdExecutionDiagnosticsFromError(err)
			logger.Logf("redeploy project %q failed: %v", strings.TrimSpace(target.ProjectName), err)
		} else {
			entry.Message = "project redeploy completed through bridge-backed compose restart"
			logger.Logf("redeploy project %q completed", strings.TrimSpace(target.ProjectName))
		}
		summary.Projects = append(summary.Projects, entry)
	}

	summary.Counts = countRedeployExecutionResults(summary)
	if failures > 0 {
		return summary, fmt.Errorf("%d redeploy target(s) failed", failures)
	}
	return summary, nil
}

func (w *NetBirdWorkflows) redeployPanel(ctx context.Context, requestID string, logger jobs.Logger) error {
	if w.host == nil {
		return fmt.Errorf("host service unavailable")
	}

	containers, err := w.host.ListContainers(ctx, true)
	if err != nil {
		return fmt.Errorf("list containers for panel redeploy: %w", err)
	}
	composeProject, err := resolveCurrentComposeProject(containers)
	if err != nil {
		return err
	}
	targets := selectPanelIngressContainers(containers, composeProject)
	if len(targets) == 0 {
		return fmt.Errorf("panel ingress container not found for compose project %q", composeProject)
	}

	for idx, container := range targets {
		logger.Logf(
			"panel redeploy (%s) %d/%d: restarting container=%s service=%s compose_project=%s",
			requestID,
			idx+1,
			len(targets),
			container.Name,
			container.Service,
			container.Project,
		)
		if err := w.host.RestartContainer(ctx, container.Name); err != nil {
			return fmt.Errorf("restart panel container %q: %w", container.Name, err)
		}
	}
	return nil
}

func resolveCurrentComposeProject(containers []DockerContainer) (string, error) {
	runtimeRef := strings.TrimSpace(os.Getenv("HOSTNAME"))
	if runtimeRef == "" {
		if hostName, err := os.Hostname(); err == nil {
			runtimeRef = strings.TrimSpace(hostName)
		}
	}

	if runtimeRef != "" {
		for _, container := range containers {
			if !matchesRuntimeContainerRef(container, runtimeRef) {
				continue
			}
			project := strings.TrimSpace(container.Project)
			if project != "" {
				return project, nil
			}
		}
	}

	apiProjects := make(map[string]struct{})
	for _, container := range containers {
		if !strings.EqualFold(strings.TrimSpace(container.Service), "api") {
			continue
		}
		project := strings.TrimSpace(container.Project)
		if project == "" {
			continue
		}
		apiProjects[project] = struct{}{}
	}
	if len(apiProjects) == 1 {
		for project := range apiProjects {
			return project, nil
		}
	}

	projects := make([]string, 0, len(apiProjects))
	for project := range apiProjects {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	if len(projects) > 0 {
		return "", fmt.Errorf("unable to resolve panel compose project from runtime container; candidate api projects: %s", strings.Join(projects, ", "))
	}
	return "", errors.New("unable to resolve panel compose project for mode apply execution")
}

func matchesRuntimeContainerRef(container DockerContainer, ref string) bool {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if ref == "" {
		return false
	}
	containerID := strings.ToLower(strings.TrimSpace(container.ID))
	containerName := strings.ToLower(strings.TrimSpace(container.Name))
	if containerID == ref || containerName == ref {
		return true
	}
	if strings.HasPrefix(containerID, ref) || strings.HasPrefix(ref, containerID) {
		return true
	}
	return false
}

func selectPanelIngressContainers(containers []DockerContainer, composeProject string) []DockerContainer {
	composeProject = strings.TrimSpace(composeProject)
	selected := make([]DockerContainer, 0)
	for _, container := range containers {
		if !strings.EqualFold(strings.TrimSpace(container.Project), composeProject) {
			continue
		}
		service := strings.ToLower(strings.TrimSpace(container.Service))
		if _, ok := netBirdPanelServiceCandidates[service]; !ok {
			continue
		}
		selected = append(selected, container)
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Name < selected[j].Name
	})
	return selected
}

func countRebindingExecutionResults(ops []NetBirdRebindingExecutionOperation) NetBirdExecutionCounts {
	counts := NetBirdExecutionCounts{}
	for _, op := range ops {
		applyNetBirdExecutionResultCount(&counts, op.Result)
	}
	return counts
}

func countRedeployExecutionResults(summary NetBirdRedeployExecutionSummary) NetBirdExecutionCounts {
	counts := NetBirdExecutionCounts{}
	if summary.Panel != nil {
		applyNetBirdExecutionResultCount(&counts, summary.Panel.Result)
	}
	for _, entry := range summary.Projects {
		applyNetBirdExecutionResultCount(&counts, entry.Result)
	}
	return counts
}

func applyNetBirdExecutionResultCount(counts *NetBirdExecutionCounts, result string) {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case netBirdExecutionFailed:
		counts.Failed++
	case netBirdExecutionSkipped:
		counts.Skipped++
	default:
		counts.Succeeded++
	}
}

func rebindingSuccessCoverage(summary NetBirdRebindingExecutionSummary) (bool, map[string]struct{}) {
	panelCovered := false
	projects := make(map[string]struct{})
	for _, op := range summary.Operations {
		if strings.ToLower(strings.TrimSpace(op.Result)) != netBirdExecutionSucceeded {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(op.Service)) {
		case "panel":
			panelCovered = true
		case "project_ingress":
			projects[netBirdProjectTargetKey(op.ProjectID, op.ProjectName)] = struct{}{}
		}
	}
	return panelCovered, projects
}

func netBirdProjectTargetKey(projectID uint, projectName string) string {
	return fmt.Sprintf("%d|%s", projectID, strings.ToLower(strings.TrimSpace(projectName)))
}

func appendNetBirdExecutionWarnings(
	base []string,
	rebinding NetBirdRebindingExecutionSummary,
	redeploy NetBirdRedeployExecutionSummary,
) []string {
	warnings := append([]string(nil), base...)
	for _, op := range rebinding.Operations {
		if strings.ToLower(strings.TrimSpace(op.Result)) != netBirdExecutionFailed {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"Service rebinding failed for service=%s project=%q: %s",
			op.Service,
			strings.TrimSpace(op.ProjectName),
			strings.TrimSpace(op.Message),
		))
	}
	if redeploy.Panel != nil && strings.ToLower(strings.TrimSpace(redeploy.Panel.Result)) == netBirdExecutionFailed {
		warnings = append(warnings, fmt.Sprintf("Panel redeploy failed: %s", strings.TrimSpace(redeploy.Panel.Message)))
	}
	for _, entry := range redeploy.Projects {
		if strings.ToLower(strings.TrimSpace(entry.Result)) != netBirdExecutionFailed {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"Project redeploy failed for %q: %s",
			strings.TrimSpace(entry.ProjectName),
			strings.TrimSpace(entry.Message),
		))
	}
	return warnings
}

func netBirdExecutionDiagnosticsFromError(err error) *NetBirdExecutionDiagnostics {
	typed, ok := errs.From(err)
	if !ok || typed == nil || typed.Details == nil {
		return nil
	}
	details, ok := typed.Details.(map[string]any)
	if !ok {
		return nil
	}

	diagnostics := &NetBirdExecutionDiagnostics{
		IntentID:        netBirdDetailString(details["intent_id"]),
		WorkerErrorCode: netBirdDetailString(details["worker_error_code"]),
		LogPath:         netBirdDetailString(details["log_path"]),
	}
	if diagnostics.IntentID == "" && diagnostics.WorkerErrorCode == "" && diagnostics.LogPath == "" {
		return nil
	}
	return diagnostics
}

func netBirdDetailString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func joinNetBirdExecutionErrors(errsIn ...error) error {
	messages := make([]string, 0, len(errsIn))
	for _, candidate := range errsIn {
		if candidate == nil {
			continue
		}
		message := strings.TrimSpace(candidate.Error())
		if message == "" {
			continue
		}
		messages = append(messages, message)
	}
	if len(messages) == 0 {
		return nil
	}
	return errors.New(strings.Join(messages, "; "))
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
