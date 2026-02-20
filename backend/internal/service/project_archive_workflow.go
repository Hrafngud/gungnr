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

	removedContainers := 0
	if options.RemoveContainers {
		if w.host == nil {
			addArchiveWarning(warnings, "host service unavailable while removing project containers")
		} else {
			targetContainers := dedupeStrings(req.Targets.Containers)
			for _, container := range targetContainers {
				logger.Logf("removing project container %s (remove_volumes=%t)", container, options.RemoveVolumes)
				if err := w.host.RemoveContainer(ctx, container, options.RemoveVolumes); err != nil {
					addArchiveWarning(warnings, fmt.Sprintf("remove container %s failed: %v", container, err))
					logger.Logf("container removal warning: %v", err)
					continue
				}
				removedContainers++
			}
		}
	}

	runtimeCfg, err := w.resolveArchiveRuntimeConfig(ctx)
	if err != nil {
		return fmt.Errorf("resolve runtime config: %w", err)
	}

	remoteIngressRemoved := 0
	localIngressRemoved := 0
	if options.RemoveIngress {
		hostnames := dedupeHostnames(req.Targets.Hostnames)
		if len(hostnames) == 0 {
			addArchiveWarning(warnings, "ingress cleanup requested but no hostnames were targeted")
		} else {
			cfClient := cloudflare.NewClient(runtimeCfg)
			removedRemote, removeErr := cfClient.RemoveIngressHostnames(ctx, hostnames)
			if removeErr != nil {
				if errors.Is(removeErr, cloudflare.ErrTunnelNotRemote) {
					removedLocal, localErr := cloudflare.RemoveLocalIngressHostnames(runtimeCfg.CloudflaredConfig, hostnames)
					if localErr != nil {
						addArchiveWarning(warnings, fmt.Sprintf("remove local ingress rules failed: %v", localErr))
					} else {
						localIngressRemoved = len(removedLocal)
						logger.Logf("removed %d local ingress rules", localIngressRemoved)
						if localIngressRemoved > 0 {
							if restartErr := w.restartTunnelForArchive(ctx, logger, fmt.Sprintf("job-%d", job.ID), runtimeCfg.CloudflaredConfig); restartErr != nil {
								addArchiveWarning(warnings, fmt.Sprintf("cloudflared restart after ingress cleanup failed: %v", restartErr))
							}
						}
					}
				} else {
					addArchiveWarning(warnings, fmt.Sprintf("remove remote ingress rules failed: %v", removeErr))
				}
			} else {
				remoteIngressRemoved = len(removedRemote)
				logger.Logf("removed %d remote ingress rules", remoteIngressRemoved)
			}
		}
	}

	dnsRemoved := 0
	if options.RemoveDNS {
		targets := dedupeDNSDeleteTargets(req.Targets.DNSRecords)
		if len(targets) == 0 {
			addArchiveWarning(warnings, "DNS cleanup requested but no deletable records were targeted")
		} else {
			cfClient := cloudflare.NewClient(runtimeCfg)
			expectedTarget, expectedErr := cfClient.ExpectedTunnelCNAME(ctx)
			if expectedErr != nil {
				addArchiveWarning(warnings, fmt.Sprintf("resolve tunnel dns target failed: %v", expectedErr))
			}
			expectedTarget = strings.ToLower(strings.TrimSpace(expectedTarget))

			for _, target := range targets {
				if strings.TrimSpace(target.ZoneID) == "" || strings.TrimSpace(target.RecordID) == "" {
					addArchiveWarning(warnings, fmt.Sprintf("skip DNS record with incomplete target metadata: zone=%q id=%q", target.ZoneID, target.RecordID))
					continue
				}
				if expectedTarget == "" {
					addArchiveWarning(warnings, fmt.Sprintf("skip DNS record %s because expected tunnel target is unavailable", target.RecordID))
					continue
				}
				if strings.TrimSpace(target.Hostname) == "" {
					addArchiveWarning(warnings, fmt.Sprintf("skip DNS record %s because hostname metadata is missing", target.RecordID))
					continue
				}

				records, err := cfClient.ListDNSRecordsByName(ctx, target.Hostname, target.ZoneID)
				if err != nil {
					addArchiveWarning(warnings, fmt.Sprintf("list DNS records for %s failed: %v", target.Hostname, err))
					continue
				}

				record := findDNSRecordByID(records, target.RecordID)
				if record == nil {
					addArchiveWarning(warnings, fmt.Sprintf("skip DNS record %s because it no longer exists", target.RecordID))
					continue
				}
				if !strings.EqualFold(strings.TrimSpace(record.Type), "CNAME") {
					addArchiveWarning(warnings, fmt.Sprintf("skip DNS record %s because type is %s", target.RecordID, record.Type))
					continue
				}
				if strings.ToLower(strings.TrimSpace(record.Content)) != expectedTarget {
					addArchiveWarning(warnings, fmt.Sprintf("skip DNS record %s because target %s no longer matches %s", target.RecordID, strings.TrimSpace(record.Content), expectedTarget))
					continue
				}

				logger.Logf("deleting Cloudflare DNS record %s for %s", target.RecordID, target.Hostname)
				if err := cfClient.DeleteDNSRecord(ctx, target.ZoneID, target.RecordID); err != nil {
					addArchiveWarning(warnings, fmt.Sprintf("delete DNS record %s failed: %v", target.RecordID, err))
					continue
				}
				dnsRemoved++
			}
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
			"warnings":             sortedArchiveWarnings(warnings),
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

func findDNSRecordByID(records []cloudflare.DNSRecord, id string) *cloudflare.DNSRecord {
	trimmed := strings.TrimSpace(id)
	for _, record := range records {
		if strings.TrimSpace(record.ID) != trimmed {
			continue
		}
		recordCopy := record
		return &recordCopy
	}
	return nil
}
