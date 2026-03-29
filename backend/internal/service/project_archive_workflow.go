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

func (w *ProjectWorkflows) handleProjectArchive(ctx context.Context, job models.Job, logger jobs.Logger) error {
	var req ProjectArchiveJobRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse project archive request: %w", err)
	}

	req.Project = strings.ToLower(strings.TrimSpace(req.Project))
	if err := ValidateProjectName(req.Project); err != nil {
		return err
	}

	if w.projects == nil {
		return fmt.Errorf("project repository unavailable")
	}

	options := normalizeArchiveOptions(req.Options)
	warnings := make(map[string]struct{})

	exposureContainers := dedupeStrings(req.Targets.ExposureContainers)
	exposureHostnames := dedupeHostnames(req.Targets.ExposureHostnames)
	exposureContainerSet := stringSliceSet(exposureContainers)
	exposureHostnameSet := stringSliceSet(exposureHostnames)
	exposureSummary := projectArchiveServiceExposureSummary{
		TargetContainers: len(exposureContainers),
		TargetHostnames:  len(exposureHostnames),
	}

	removedContainers := 0
	if options.RemoveContainers {
		if w.host == nil {
			addArchiveWarning(warnings, "host service unavailable while removing project containers")
			exposureSummary.FailedContainers += exposureSummary.TargetContainers
		} else {
			targetContainers := dedupeStrings(req.Targets.Containers)
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
					}
					continue
				}
				removedContainers++
				if isExposureContainer {
					exposureSummary.RemovedContainers++
				}
			}
		}
	} else if exposureSummary.TargetContainers > 0 {
		exposureSummary.SkippedContainers = exposureSummary.TargetContainers
		logger.Logf("service-exposure container cleanup skipped because removeContainers=false (%d target(s))", exposureSummary.TargetContainers)
	}

	runtimeCfg, err := w.resolveArchiveRuntimeConfig(ctx)
	if err != nil {
		return fmt.Errorf("resolve runtime config: %w", err)
	}

	remoteIngressRemoved := 0
	localIngressRemoved := 0
	if options.RemoveIngress {
		hostnames := dedupeHostnames(req.Targets.Hostnames)
		targetHostnameSet := stringSliceSet(hostnames)
		for hostname := range exposureHostnameSet {
			if _, ok := targetHostnameSet[hostname]; ok {
				continue
			}
			exposureSummary.SkippedIngress++
		}

		if len(hostnames) == 0 {
			addArchiveWarning(warnings, "ingress cleanup requested but no hostnames were targeted")
			exposureSummary.SkippedIngress += exposureSummary.TargetHostnames
		} else {
			cfClient := cloudflare.NewClient(runtimeCfg)
			removedRemote, removeErr := cfClient.RemoveIngressHostnames(ctx, hostnames)
			if removeErr != nil {
				if errors.Is(removeErr, cloudflare.ErrTunnelNotRemote) {
					removedLocal, localErr := cloudflare.RemoveLocalIngressHostnames(runtimeCfg.CloudflaredConfig, hostnames)
					if localErr != nil {
						addArchiveWarning(warnings, fmt.Sprintf("remove local ingress rules failed: %v", localErr))
						exposureSummary.FailedIngress += countHostnamesInSet(hostnames, exposureHostnameSet)
					} else {
						localIngressRemoved = len(removedLocal)
						exposureSummary.RemovedIngressLocal += countIngressRulesForHostnames(removedLocal, exposureHostnameSet)
						logger.Logf("removed %d local ingress rules", localIngressRemoved)
						if localIngressRemoved > 0 {
							if restartErr := w.restartTunnelForArchive(ctx, logger, fmt.Sprintf("job-%d", job.ID), runtimeCfg.CloudflaredConfig); restartErr != nil {
								addArchiveWarning(warnings, fmt.Sprintf("cloudflared restart after ingress cleanup failed: %v", restartErr))
							}
						}
					}
				} else {
					addArchiveWarning(warnings, fmt.Sprintf("remove remote ingress rules failed: %v", removeErr))
					exposureSummary.FailedIngress += countHostnamesInSet(hostnames, exposureHostnameSet)
				}
			} else {
				remoteIngressRemoved = len(removedRemote)
				exposureSummary.RemovedIngressRemote += countIngressRulesForHostnames(removedRemote, exposureHostnameSet)
				logger.Logf("removed %d remote ingress rules", remoteIngressRemoved)
			}
		}
	} else if exposureSummary.TargetHostnames > 0 {
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

	dnsRemoved := 0
	dnsTargets := dedupeDNSDeleteTargets(req.Targets.DNSRecords)
	exposureSummary.TargetDNS = countExposureDNSTargets(dnsTargets, exposureHostnameSet)
	if options.RemoveDNS {
		if len(dnsTargets) == 0 {
			addArchiveWarning(warnings, "DNS cleanup requested but no deletable records were targeted")
			exposureSummary.SkippedDNS += exposureSummary.TargetDNS
		} else {
			cfClient := cloudflare.NewClient(runtimeCfg)
			expectedTarget, expectedErr := cfClient.ExpectedTunnelCNAME(ctx)
			if expectedErr != nil {
				addArchiveWarning(warnings, fmt.Sprintf("resolve tunnel dns target failed: %v", expectedErr))
			}
			expectedTarget = strings.ToLower(strings.TrimSpace(expectedTarget))

			for _, target := range dnsTargets {
				targetHostname := strings.ToLower(strings.TrimSpace(target.Hostname))
				_, isExposureTarget := exposureHostnameSet[targetHostname]

				deleteResult, err := cfClient.DeleteTunnelCNAMERecord(ctx, target.ZoneID, target.RecordID, target.Hostname, expectedTarget)
				if err != nil {
					addArchiveWarning(warnings, fmt.Sprintf("delete DNS record %s failed: %v", target.RecordID, err))
					if isExposureTarget {
						exposureSummary.FailedDNS++
					}
					continue
				}
				if !deleteResult.Deleted {
					addArchiveWarning(warnings, fmt.Sprintf("skip DNS record %s because %s", target.RecordID, deleteResult.SkipReason))
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
	} else if exposureSummary.TargetDNS > 0 {
		exposureSummary.SkippedDNS = exposureSummary.TargetDNS
		logger.Logf("service-exposure DNS cleanup skipped because removeDns=false (%d target(s))", exposureSummary.TargetDNS)
	}

	if exposureSummary.TargetDNS > 0 {
		remainingDNS := exposureSummary.TargetDNS - exposureSummary.RemovedDNS - exposureSummary.FailedDNS - exposureSummary.SkippedDNS
		if remainingDNS > 0 {
			exposureSummary.SkippedDNS += remainingDNS
		}
	}

	statusPersisted := false
	projectRecord, err := lookupProjectRecord(ctx, w.projects, req.Project)
	if err != nil {
		addArchiveWarning(warnings, fmt.Sprintf("project status update skipped: resolve project record failed: %v", err))
	} else if projectRecord == nil {
		addArchiveWarning(warnings, fmt.Sprintf("project status update skipped: no project database row found for %s", req.Project))
	} else {
		projectRecord.Status = "archived"
		if err := w.projects.Update(ctx, projectRecord); err != nil {
			addArchiveWarning(warnings, fmt.Sprintf("project status update failed: %v", err))
		} else {
			statusPersisted = true
		}
	}

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

	if len(warnings) == 0 {
		logger.Logf("archive completed for project %s", req.Project)
	} else {
		sortedWarnings := sortedArchiveWarnings(warnings)
		logger.Logf("archive completed with warnings for project %s (%d warning(s))", req.Project, len(sortedWarnings))
		for _, warning := range sortedWarnings {
			logger.Logf("warning: %s", warning)
		}
	}

	if w.audit != nil {
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
			"warnings": sortedArchiveWarnings(warnings),
		}
		if err := w.audit.Log(ctx, AuditEntry{
			UserID:    req.RequestedBy.UserID,
			UserLogin: req.RequestedBy.Login,
			Action:    "project.archive.completed",
			Target:    req.Project,
			Metadata:  metadata,
		}); err != nil {
			logger.Logf("audit warning: failed to write archive completion event: %v", err)
		}
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
