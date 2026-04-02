package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"go-notes/internal/config"
	"go-notes/internal/infra/contract"
	"go-notes/internal/integrations/cloudflare"
	"go-notes/internal/jobs"
	"go-notes/internal/models"
	"go-notes/internal/validate"
)

type projectArchiveServiceExposureSummary struct {
	TargetContainers  int
	RemovedContainers int
	SkippedContainers int
	FailedContainers  int

	TargetHostnames      int
	RemovedIngressRemote int
	RemovedIngressLocal  int
	SkippedIngress       int
	FailedIngress        int

	TargetDNS  int
	RemovedDNS int
	SkippedDNS int
	FailedDNS  int
}

type projectArchiveStepStatus string

const (
	projectArchiveStepStatusCompleted      projectArchiveStepStatus = "completed"
	projectArchiveStepStatusPartialFailure projectArchiveStepStatus = "partial_failure"
	projectArchiveStepStatusSkipped        projectArchiveStepStatus = "skipped"
)

func (w *ProjectWorkflows) handleProjectArchive(ctx context.Context, job models.Job, logger jobs.Logger) error {
	var req ProjectArchiveJobRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse project archive request: %w", err)
	}

	req.Project = strings.ToLower(strings.TrimSpace(req.Project))
	if err := validate.ProjectName(req.Project); err != nil {
		return err
	}

	if w.projects == nil {
		return fmt.Errorf("project repository unavailable")
	}

	options := normalizeArchiveOptions(req.Options)
	warnings := make(map[string]struct{})

	targetContainers := dedupeStrings(req.Targets.Containers)
	exposureContainers := dedupeStrings(req.Targets.ExposureContainers)
	exposureHostnames := dedupeHostnames(req.Targets.ExposureHostnames)
	exposureContainerSet := stringSliceSet(exposureContainers)
	exposureHostnameSet := stringSliceSet(exposureHostnames)
	exposureSummary := projectArchiveServiceExposureSummary{
		TargetContainers: len(exposureContainers),
		TargetHostnames:  len(exposureHostnames),
	}

	projectContainerTargets := countStringsExcludingSet(targetContainers, exposureContainerSet)
	removedContainers := 0
	projectContainersRemoved := 0
	projectContainersFailed := 0
	logProjectArchiveStepStart(
		logger,
		"containers",
		"remove_containers=%t remove_volumes=%t project_targets=%d exposure_targets=%d",
		options.RemoveContainers,
		options.RemoveVolumes,
		projectContainerTargets,
		exposureSummary.TargetContainers,
	)
	if options.RemoveContainers {
		if w.host == nil {
			addArchiveWarning(warnings, "host service unavailable while removing project containers")
			projectContainersFailed = projectContainerTargets
			exposureSummary.FailedContainers += exposureSummary.TargetContainers
		} else {
			targetContainerSet := stringSliceSet(targetContainers)
			for container := range exposureContainerSet {
				if _, ok := targetContainerSet[container]; ok {
					continue
				}
				exposureSummary.SkippedContainers++
			}

			for _, container := range targetContainers {
				_, isExposureContainer := exposureContainerSet[container]
				if isExposureContainer {
					logger.Logf("removing service-exposure container %s (remove_volumes=%t)", container, options.RemoveVolumes)
				} else {
					logger.Logf("removing project container %s (remove_volumes=%t)", container, options.RemoveVolumes)
				}
				if err := w.host.RemoveContainer(ctx, container, options.RemoveVolumes); err != nil {
					addArchiveWarning(warnings, fmt.Sprintf("remove container %s failed: %v", container, err))
					logger.Logf("container removal warning: %v", err)
					if isExposureContainer {
						exposureSummary.FailedContainers++
					} else {
						projectContainersFailed++
					}
					continue
				}
				removedContainers++
				if isExposureContainer {
					exposureSummary.RemovedContainers++
				} else {
					projectContainersRemoved++
				}
			}
		}
	} else {
		exposureSummary.SkippedContainers = exposureSummary.TargetContainers
		logger.Logf("service-exposure container cleanup skipped because removeContainers=false (%d target(s))", exposureSummary.TargetContainers)
	}
	containersStepStatus := projectArchiveStepStatusCompleted
	if !options.RemoveContainers {
		containersStepStatus = projectArchiveStepStatusSkipped
	} else if w.host == nil || projectContainersFailed > 0 || exposureSummary.FailedContainers > 0 {
		containersStepStatus = projectArchiveStepStatusPartialFailure
	}
	logProjectArchiveStepResult(
		logger,
		"containers",
		containersStepStatus,
		"project_targets=%d project_removed=%d project_failed=%d exposure_targets=%d exposure_removed=%d exposure_skipped=%d exposure_failed=%d total_removed=%d",
		projectContainerTargets,
		projectContainersRemoved,
		projectContainersFailed,
		exposureSummary.TargetContainers,
		exposureSummary.RemovedContainers,
		exposureSummary.SkippedContainers,
		exposureSummary.FailedContainers,
		removedContainers,
	)

	runtimeCfg, err := w.resolveArchiveRuntimeConfig(ctx)
	if err != nil {
		return fmt.Errorf("resolve runtime config: %w", err)
	}

	hostnames := dedupeHostnames(req.Targets.Hostnames)
	ingressTargets := normalizeIngressDeleteTargets(req.Targets.IngressRules)
	remoteTargets := filterIngressDeleteTargetsBySource(ingressTargets, "remote")
	localTargets := filterIngressDeleteTargetsBySource(ingressTargets, "local")
	projectHostnameTargets := countStringsExcludingSet(hostnames, exposureHostnameSet)
	remoteIngressRemoved := 0
	localIngressRemoved := 0
	ingressStepFailed := false
	tunnelRestarted := false
	tunnelRestartFailed := false
	logProjectArchiveStepStart(
		logger,
		"ingress",
		"remove_ingress=%t project_hostnames=%d exposure_hostnames=%d target_rules=%d remote_rules=%d local_rules=%d",
		options.RemoveIngress,
		projectHostnameTargets,
		exposureSummary.TargetHostnames,
		len(ingressTargets),
		len(remoteTargets),
		len(localTargets),
	)
	if options.RemoveIngress {
		targetHostnameSet := stringSliceSet(hostnames)
		for hostname := range exposureHostnameSet {
			if _, ok := targetHostnameSet[hostname]; ok {
				continue
			}
			exposureSummary.SkippedIngress++
		}

		if len(ingressTargets) == 0 {
			addArchiveWarning(warnings, "ingress cleanup requested but no planned ingress rules were targeted")
			exposureSummary.SkippedIngress += exposureSummary.TargetHostnames
		} else {
			cfClient := cloudflare.NewClient(runtimeCfg)
			remoteTunnelMismatch := false

			if len(remoteTargets) > 0 {
				removedRemote, removeErr := cfClient.RemoveIngressRules(ctx, remoteTargets)
				if removeErr != nil {
					if errors.Is(removeErr, cloudflare.ErrTunnelNotRemote) {
						remoteTunnelMismatch = true
					} else {
						addArchiveWarning(warnings, fmt.Sprintf("remove remote ingress rules failed: %v", removeErr))
						exposureSummary.FailedIngress += countHostnamesInSet(hostnames, exposureHostnameSet)
						ingressStepFailed = true
					}
				} else {
					remoteIngressRemoved = len(removedRemote)
					exposureSummary.RemovedIngressRemote += countIngressRulesForHostnames(removedRemote, exposureHostnameSet)
					logger.Logf("removed %d remote ingress rules", remoteIngressRemoved)
				}
			}

			if len(localTargets) > 0 {
				removedLocal, localErr := cloudflare.RemoveLocalIngressRules(runtimeCfg.CloudflaredConfig, localTargets)
				if localErr != nil {
					addArchiveWarning(warnings, fmt.Sprintf("remove local ingress rules failed: %v", localErr))
					exposureSummary.FailedIngress += countHostnamesInSet(hostnames, exposureHostnameSet)
					ingressStepFailed = true
				} else {
					localIngressRemoved = len(removedLocal)
					exposureSummary.RemovedIngressLocal += countIngressRulesForHostnames(removedLocal, exposureHostnameSet)
					logger.Logf("removed %d local ingress rules", localIngressRemoved)
					if localIngressRemoved > 0 {
						if restartErr := w.restartTunnelForArchive(ctx, logger, fmt.Sprintf("job-%d", job.ID), runtimeCfg.CloudflaredConfig); restartErr != nil {
							addArchiveWarning(warnings, fmt.Sprintf("cloudflared restart after ingress cleanup failed: %v", restartErr))
							tunnelRestartFailed = true
							ingressStepFailed = true
						} else {
							tunnelRestarted = true
						}
					}
				}
			}

			if len(remoteTargets) == 0 && len(localTargets) == 0 {
				addArchiveWarning(warnings, "ingress cleanup requested but no source-scoped ingress rules were targeted")
				exposureSummary.SkippedIngress += exposureSummary.TargetHostnames
			} else if remoteTunnelMismatch && len(localTargets) == 0 {
				addArchiveWarning(warnings, "remote ingress cleanup skipped because the tunnel is locally managed and no planned local ingress rules were available")
				exposureSummary.FailedIngress += countHostnamesInSet(hostnames, exposureHostnameSet)
				ingressStepFailed = true
			}
		}
	} else {
		exposureSummary.SkippedIngress = exposureSummary.TargetHostnames
		logger.Logf("service-exposure ingress cleanup skipped because removeIngress=false (%d target(s))", exposureSummary.TargetHostnames)
	}

	if exposureSummary.TargetHostnames > 0 {
		remainingIngress := exposureSummary.TargetHostnames -
			exposureSummary.RemovedIngressRemote -
			exposureSummary.RemovedIngressLocal -
			exposureSummary.FailedIngress -
			exposureSummary.SkippedIngress
		if remainingIngress > 0 {
			exposureSummary.SkippedIngress += remainingIngress
		}
	}
	ingressStepStatus := projectArchiveStepStatusCompleted
	if !options.RemoveIngress {
		ingressStepStatus = projectArchiveStepStatusSkipped
	} else if ingressStepFailed {
		ingressStepStatus = projectArchiveStepStatusPartialFailure
	}
	logProjectArchiveStepResult(
		logger,
		"ingress",
		ingressStepStatus,
		"project_hostnames=%d exposure_hostnames=%d target_rules=%d remote_rules=%d local_rules=%d removed_remote=%d removed_local=%d exposure_removed_remote=%d exposure_removed_local=%d exposure_skipped=%d exposure_failed=%d tunnel_restarted=%t tunnel_restart_failed=%t",
		projectHostnameTargets,
		exposureSummary.TargetHostnames,
		len(ingressTargets),
		len(remoteTargets),
		len(localTargets),
		remoteIngressRemoved,
		localIngressRemoved,
		exposureSummary.RemovedIngressRemote,
		exposureSummary.RemovedIngressLocal,
		exposureSummary.SkippedIngress,
		exposureSummary.FailedIngress,
		tunnelRestarted,
		tunnelRestartFailed,
	)

	dnsRemoved := 0
	dnsTargets := dedupeDNSDeleteTargets(req.Targets.DNSRecords)
	exposureSummary.TargetDNS = countExposureDNSTargets(dnsTargets, exposureHostnameSet)
	dnsStepFailed := false
	dnsSkippedRecords := 0
	dnsFailedRecords := 0
	expectedTargetResolved := false
	logProjectArchiveStepStart(
		logger,
		"dns",
		"remove_dns=%t target_records=%d exposure_records=%d",
		options.RemoveDNS,
		len(dnsTargets),
		exposureSummary.TargetDNS,
	)
	if options.RemoveDNS {
		if len(dnsTargets) == 0 {
			addArchiveWarning(warnings, "DNS cleanup requested but no deletable records were targeted")
			exposureSummary.SkippedDNS += exposureSummary.TargetDNS
		} else {
			cfClient := cloudflare.NewClient(runtimeCfg)
			expectedTarget, expectedErr := cfClient.ExpectedTunnelCNAME(ctx)
			if expectedErr != nil {
				addArchiveWarning(warnings, fmt.Sprintf("resolve tunnel dns target failed: %v", expectedErr))
				dnsStepFailed = true
			} else {
				expectedTargetResolved = true
			}
			expectedTarget = strings.ToLower(strings.TrimSpace(expectedTarget))

			for _, target := range dnsTargets {
				targetHostname := strings.ToLower(strings.TrimSpace(target.Hostname))
				_, isExposureTarget := exposureHostnameSet[targetHostname]

				deleteResult, err := cfClient.DeleteTunnelCNAMERecord(ctx, target.ZoneID, target.RecordID, target.Hostname, expectedTarget)
				if err != nil {
					addArchiveWarning(warnings, fmt.Sprintf("delete DNS record %s failed: %v", target.RecordID, err))
					dnsFailedRecords++
					dnsStepFailed = true
					if isExposureTarget {
						exposureSummary.FailedDNS++
					}
					continue
				}
				if !deleteResult.Deleted {
					addArchiveWarning(warnings, fmt.Sprintf("skip DNS record %s because %s", target.RecordID, deleteResult.SkipReason))
					dnsSkippedRecords++
					if isExposureTarget {
						exposureSummary.SkippedDNS++
					}
					continue
				}

				logger.Logf("deleting Cloudflare DNS record %s for %s", target.RecordID, target.Hostname)
				dnsRemoved++
				if isExposureTarget {
					exposureSummary.RemovedDNS++
				}
			}
		}
	} else {
		exposureSummary.SkippedDNS = exposureSummary.TargetDNS
		logger.Logf("service-exposure DNS cleanup skipped because removeDns=false (%d target(s))", exposureSummary.TargetDNS)
	}

	if exposureSummary.TargetDNS > 0 {
		remainingDNS := exposureSummary.TargetDNS - exposureSummary.RemovedDNS - exposureSummary.FailedDNS - exposureSummary.SkippedDNS
		if remainingDNS > 0 {
			exposureSummary.SkippedDNS += remainingDNS
		}
	}
	dnsStepStatus := projectArchiveStepStatusCompleted
	if !options.RemoveDNS {
		dnsStepStatus = projectArchiveStepStatusSkipped
	} else if dnsStepFailed {
		dnsStepStatus = projectArchiveStepStatusPartialFailure
	}
	logProjectArchiveStepResult(
		logger,
		"dns",
		dnsStepStatus,
		"target_records=%d removed=%d skipped=%d failed=%d exposure_records=%d exposure_removed=%d exposure_skipped=%d exposure_failed=%d expected_target_resolved=%t",
		len(dnsTargets),
		dnsRemoved,
		dnsSkippedRecords,
		dnsFailedRecords,
		exposureSummary.TargetDNS,
		exposureSummary.RemovedDNS,
		exposureSummary.SkippedDNS,
		exposureSummary.FailedDNS,
		expectedTargetResolved,
	)

	statusPersisted := false
	auditLogged := false
	statusAuditStepFailed := false
	logProjectArchiveStepStart(logger, "status_audit", "project=%s audit_enabled=%t", req.Project, w.audit != nil)
	projectRecord, err := lookupProjectRecord(ctx, w.projects, req.Project)
	if err != nil {
		addArchiveWarning(warnings, fmt.Sprintf("project status update skipped: resolve project record failed: %v", err))
		statusAuditStepFailed = true
	} else if projectRecord == nil {
		addArchiveWarning(warnings, fmt.Sprintf("project status update skipped: no project database row found for %s", req.Project))
		statusAuditStepFailed = true
	} else {
		projectRecord.Status = "archived"
		if err := w.projects.Update(ctx, projectRecord); err != nil {
			addArchiveWarning(warnings, fmt.Sprintf("project status update failed: %v", err))
			statusAuditStepFailed = true
		} else {
			statusPersisted = true
		}
	}

	statusAuditStepStatusForAudit := projectArchiveStepStatusCompleted
	if statusAuditStepFailed {
		statusAuditStepStatusForAudit = projectArchiveStepStatusPartialFailure
	}
	sortedWarnings := sortedArchiveWarnings(warnings)
	auditWarningCount := 0

	if w.audit != nil {
		completionSummary := map[string]any{
			"outcome":      projectArchiveExecutionOutcome(containersStepStatus, ingressStepStatus, dnsStepStatus, statusAuditStepStatusForAudit, len(sortedWarnings)),
			"warningCount": len(sortedWarnings),
			"steps": map[string]any{
				"containers": map[string]any{
					"status":               string(containersStepStatus),
					"projectTargetCount":   projectContainerTargets,
					"projectRemovedCount":  projectContainersRemoved,
					"projectFailedCount":   projectContainersFailed,
					"exposureTargetCount":  exposureSummary.TargetContainers,
					"exposureRemovedCount": exposureSummary.RemovedContainers,
					"exposureSkippedCount": exposureSummary.SkippedContainers,
					"exposureFailedCount":  exposureSummary.FailedContainers,
					"totalRemovedCount":    removedContainers,
					"removeContainers":     options.RemoveContainers,
					"removeVolumes":        options.RemoveVolumes,
				},
				"ingress": map[string]any{
					"status":                 string(ingressStepStatus),
					"projectHostnameCount":   projectHostnameTargets,
					"exposureHostnameCount":  exposureSummary.TargetHostnames,
					"targetRuleCount":        len(ingressTargets),
					"remoteRuleCount":        len(remoteTargets),
					"localRuleCount":         len(localTargets),
					"removedRemoteRuleCount": remoteIngressRemoved,
					"removedLocalRuleCount":  localIngressRemoved,
					"tunnelRestarted":        tunnelRestarted,
					"tunnelRestartFailed":    tunnelRestartFailed,
					"removeIngress":          options.RemoveIngress,
				},
				"dns": map[string]any{
					"status":                 string(dnsStepStatus),
					"targetRecordCount":      len(dnsTargets),
					"removedRecordCount":     dnsRemoved,
					"skippedRecordCount":     dnsSkippedRecords,
					"failedRecordCount":      dnsFailedRecords,
					"exposureTargetCount":    exposureSummary.TargetDNS,
					"exposureRemovedCount":   exposureSummary.RemovedDNS,
					"exposureSkippedCount":   exposureSummary.SkippedDNS,
					"exposureFailedCount":    exposureSummary.FailedDNS,
					"expectedTargetResolved": expectedTargetResolved,
					"removeDns":              options.RemoveDNS,
				},
				"statusAudit": map[string]any{
					"status":          string(statusAuditStepStatusForAudit),
					"statusPersisted": statusPersisted,
					"auditLogged":     true,
				},
			},
		}
		metadata := map[string]any{
			"project":              req.Project,
			"jobId":                job.ID,
			"removeContainers":     options.RemoveContainers,
			"removeVolumes":        options.RemoveVolumes,
			"removeIngress":        options.RemoveIngress,
			"removeDns":            options.RemoveDNS,
			"removedContainers":    removedContainers,
			"removedIngressRemote": remoteIngressRemoved,
			"removedIngressLocal":  localIngressRemoved,
			"removedDnsRecords":    dnsRemoved,
			"statusPersisted":      statusPersisted,
			"serviceExposureCleanup": map[string]any{
				"targetContainers":     exposureSummary.TargetContainers,
				"removedContainers":    exposureSummary.RemovedContainers,
				"skippedContainers":    exposureSummary.SkippedContainers,
				"failedContainers":     exposureSummary.FailedContainers,
				"targetHostnames":      exposureSummary.TargetHostnames,
				"removedIngressRemote": exposureSummary.RemovedIngressRemote,
				"removedIngressLocal":  exposureSummary.RemovedIngressLocal,
				"skippedIngress":       exposureSummary.SkippedIngress,
				"failedIngress":        exposureSummary.FailedIngress,
				"targetDnsRecords":     exposureSummary.TargetDNS,
				"removedDnsRecords":    exposureSummary.RemovedDNS,
				"skippedDnsRecords":    exposureSummary.SkippedDNS,
				"failedDnsRecords":     exposureSummary.FailedDNS,
			},
			"completionSummary": completionSummary,
			"warnings":          sortedWarnings,
		}
		if err := w.audit.Log(ctx, AuditEntry{
			UserID:    req.RequestedBy.UserID,
			UserLogin: req.RequestedBy.Login,
			Action:    "project.archive.completed",
			Target:    req.Project,
			Metadata:  metadata,
		}); err != nil {
			statusAuditStepFailed = true
			auditWarningCount = 1
			logger.Logf("audit warning: failed to write archive completion event: %v", err)
		} else {
			auditLogged = true
		}
	}

	statusAuditStepStatus := projectArchiveStepStatusCompleted
	if statusAuditStepFailed {
		statusAuditStepStatus = projectArchiveStepStatusPartialFailure
	}
	logProjectArchiveStepResult(
		logger,
		"status_audit",
		statusAuditStepStatus,
		"status_persisted=%t audit_logged=%t",
		statusPersisted,
		auditLogged,
	)

	if exposureSummary.TargetContainers+exposureSummary.TargetHostnames+exposureSummary.TargetDNS == 0 {
		logger.Log("service-exposure cleanup summary: no resolved forward_local/quick_service targets")
	} else {
		logger.Logf(
			"service-exposure cleanup summary: containers target=%d removed=%d skipped=%d failed=%d | ingress target=%d removed_remote=%d removed_local=%d skipped=%d failed=%d | dns target=%d removed=%d skipped=%d failed=%d",
			exposureSummary.TargetContainers,
			exposureSummary.RemovedContainers,
			exposureSummary.SkippedContainers,
			exposureSummary.FailedContainers,
			exposureSummary.TargetHostnames,
			exposureSummary.RemovedIngressRemote,
			exposureSummary.RemovedIngressLocal,
			exposureSummary.SkippedIngress,
			exposureSummary.FailedIngress,
			exposureSummary.TargetDNS,
			exposureSummary.RemovedDNS,
			exposureSummary.SkippedDNS,
			exposureSummary.FailedDNS,
		)
	}

	totalWarningCount := len(sortedWarnings) + auditWarningCount
	overallOutcome := projectArchiveExecutionOutcome(containersStepStatus, ingressStepStatus, dnsStepStatus, statusAuditStepStatus, totalWarningCount)
	logger.Logf(
		"archive completion summary: outcome=%s warnings=%d steps=containers:%s ingress:%s dns:%s status_audit:%s",
		overallOutcome,
		totalWarningCount,
		containersStepStatus,
		ingressStepStatus,
		dnsStepStatus,
		statusAuditStepStatus,
	)
	switch overallOutcome {
	case "partial_failure":
		logger.Logf("archive completed with partial failures for project %s (warning_count=%d)", req.Project, totalWarningCount)
	case "completed_with_warnings":
		logger.Logf("archive completed with warnings for project %s (%d warning(s))", req.Project, totalWarningCount)
	default:
		logger.Logf("archive completed for project %s", req.Project)
	}
	for _, warning := range sortedWarnings {
		logger.Logf("warning: %s", warning)
	}

	return nil
}

func (w *ProjectWorkflows) resolveArchiveRuntimeConfig(ctx context.Context) (config.Config, error) {
	if w.settings == nil {
		return w.cfg, nil
	}
	return w.settings.ResolveConfig(ctx)
}

func (w *ProjectWorkflows) restartTunnelForArchive(
	ctx context.Context,
	logger jobs.Logger,
	requestID string,
	configPath string,
) error {
	if strings.TrimSpace(configPath) == "" {
		return fmt.Errorf("cloudflared config path is empty")
	}
	if w.infraClient == nil {
		return fmt.Errorf("infra bridge client unavailable")
	}

	result, err := w.infraClient.RestartTunnel(ctx, strings.TrimSpace(requestID), configPath)
	if err != nil {
		return err
	}
	logger.Logf("infra bridge restart_tunnel intent completed: intent_id=%s status=%s", result.IntentID, result.Status)
	if strings.TrimSpace(result.LogPath) != "" {
		logger.Logf("infra bridge restart_tunnel log path: %s", result.LogPath)
	}
	if result.Error != nil {
		return fmt.Errorf("cloudflared restart failed (%s): %s", strings.TrimSpace(result.Error.Code), strings.TrimSpace(result.Error.Message))
	}
	if result.Status != contract.StatusSucceeded {
		return fmt.Errorf("cloudflared restart reported non-success status %q", result.Status)
	}
	return nil
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}

func dedupeHostnames(values []string) []string {
	trimmed := dedupeStrings(values)
	for i := range trimmed {
		trimmed[i] = strings.ToLower(strings.TrimSpace(trimmed[i]))
	}
	return dedupeStrings(trimmed)
}

func dedupeDNSDeleteTargets(values []ProjectArchiveDNSDeleteTarget) []ProjectArchiveDNSDeleteTarget {
	seen := make(map[string]struct{})
	result := make([]ProjectArchiveDNSDeleteTarget, 0, len(values))
	for _, entry := range values {
		zoneID := strings.TrimSpace(entry.ZoneID)
		recordID := strings.TrimSpace(entry.RecordID)
		key := zoneID + ":" + recordID
		if zoneID == "" || recordID == "" {
			key = zoneID + ":" + recordID + ":" + strings.ToLower(strings.TrimSpace(entry.Hostname))
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		entry.ZoneID = zoneID
		entry.RecordID = recordID
		entry.Hostname = strings.ToLower(strings.TrimSpace(entry.Hostname))
		entry.Content = strings.TrimSpace(entry.Content)
		result = append(result, entry)
	}
	return result
}

func normalizeIngressDeleteTargets(values []ProjectArchiveIngressDeleteTarget) []ProjectArchiveIngressDeleteTarget {
	result := make([]ProjectArchiveIngressDeleteTarget, 0, len(values))
	for _, entry := range values {
		hostname := strings.ToLower(strings.TrimSpace(entry.Hostname))
		source := strings.ToLower(strings.TrimSpace(entry.Source))
		service := strings.TrimSpace(entry.Service)
		if hostname == "" {
			continue
		}
		entry.Hostname = hostname
		entry.Service = service
		entry.Source = source
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Hostname == result[j].Hostname {
			if result[i].Service == result[j].Service {
				return result[i].Source < result[j].Source
			}
			return result[i].Service < result[j].Service
		}
		return result[i].Hostname < result[j].Hostname
	})
	return result
}

func filterIngressDeleteTargetsBySource(values []ProjectArchiveIngressDeleteTarget, source string) []cloudflare.IngressRule {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		return []cloudflare.IngressRule{}
	}
	result := make([]cloudflare.IngressRule, 0, len(values))
	for _, entry := range values {
		if strings.ToLower(strings.TrimSpace(entry.Source)) != source {
			continue
		}
		hostname := strings.ToLower(strings.TrimSpace(entry.Hostname))
		if hostname == "" {
			continue
		}
		result = append(result, cloudflare.IngressRule{
			Hostname: hostname,
			Service:  strings.TrimSpace(entry.Service),
		})
	}
	return result
}

func stringSliceSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result[trimmed] = struct{}{}
	}
	return result
}

func countHostnamesInSet(hostnames []string, targets map[string]struct{}) int {
	if len(hostnames) == 0 || len(targets) == 0 {
		return 0
	}
	count := 0
	for _, hostname := range hostnames {
		normalized := strings.ToLower(strings.TrimSpace(hostname))
		if normalized == "" {
			continue
		}
		if _, ok := targets[normalized]; ok {
			count++
		}
	}
	return count
}

func countIngressRulesForHostnames(rules []cloudflare.IngressRule, targets map[string]struct{}) int {
	if len(rules) == 0 || len(targets) == 0 {
		return 0
	}
	count := 0
	for _, rule := range rules {
		hostname := strings.ToLower(strings.TrimSpace(rule.Hostname))
		if hostname == "" {
			continue
		}
		if _, ok := targets[hostname]; ok {
			count++
		}
	}
	return count
}

func countExposureDNSTargets(targets []ProjectArchiveDNSDeleteTarget, exposureHostnames map[string]struct{}) int {
	if len(targets) == 0 || len(exposureHostnames) == 0 {
		return 0
	}
	count := 0
	for _, target := range targets {
		hostname := strings.ToLower(strings.TrimSpace(target.Hostname))
		if hostname == "" {
			continue
		}
		if _, ok := exposureHostnames[hostname]; ok {
			count++
		}
	}
	return count
}

func countStringsExcludingSet(values []string, excluded map[string]struct{}) int {
	if len(values) == 0 {
		return 0
	}
	count := 0
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := excluded[trimmed]; ok {
			continue
		}
		count++
	}
	return count
}

func projectArchiveExecutionOutcome(
	containers projectArchiveStepStatus,
	ingress projectArchiveStepStatus,
	dns projectArchiveStepStatus,
	statusAudit projectArchiveStepStatus,
	warningCount int,
) string {
	for _, status := range []projectArchiveStepStatus{containers, ingress, dns, statusAudit} {
		if status == projectArchiveStepStatusPartialFailure {
			return "partial_failure"
		}
	}
	if warningCount > 0 {
		return "completed_with_warnings"
	}
	return "completed"
}

func logProjectArchiveStepStart(logger jobs.Logger, step string, format string, args ...any) {
	if logger == nil {
		return
	}
	message := strings.TrimSpace(fmt.Sprintf(format, args...))
	if message == "" {
		logger.Logf("archive step %s: start", step)
		return
	}
	logger.Logf("archive step %s: start %s", step, message)
}

func logProjectArchiveStepResult(logger jobs.Logger, step string, status projectArchiveStepStatus, format string, args ...any) {
	if logger == nil {
		return
	}
	message := strings.TrimSpace(fmt.Sprintf(format, args...))
	if message == "" {
		logger.Logf("archive step %s: result=%s", step, status)
		return
	}
	logger.Logf("archive step %s: result=%s %s", step, status, message)
}
