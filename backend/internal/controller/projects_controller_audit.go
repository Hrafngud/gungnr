package controller

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/middleware"
	"go-notes/internal/models"
	"go-notes/internal/service"
)

func workbenchComposeApplyAuditMetadata(
	project string,
	req models.ProjectWorkbenchComposeApplyRequest,
	result *service.WorkbenchComposeApplyResult,
	opErr error,
) map[string]any {
	metadata := map[string]any{
		"project":                   project,
		"expectedRevision":          nil,
		"expectedSourceFingerprint": strings.TrimSpace(req.ExpectedSourceFingerprint),
		"success":                   opErr == nil,
		"blocked":                   false,
		"revision":                  nil,
		"sourceFingerprint":         "",
		"composePath":               "",
		"composeBytes":              0,
		"backupId":                  "",
		"backupSequence":            0,
		"retainedBackups":           0,
		"prunedBackups":             0,
		"issueCount":                0,
		"errorCode":                 "",
	}
	if req.ExpectedRevision != nil {
		metadata["expectedRevision"] = *req.ExpectedRevision
	}

	if result != nil {
		metadata["revision"] = result.Metadata.Revision
		metadata["sourceFingerprint"] = result.Metadata.SourceFingerprint
		metadata["composePath"] = result.Metadata.ComposePath
		metadata["composeBytes"] = result.ComposeBytes
		metadata["backupId"] = result.Backup.BackupID
		metadata["backupSequence"] = result.Backup.Sequence
		metadata["retainedBackups"] = result.Retention.RetainedCount
		metadata["prunedBackups"] = result.Retention.PrunedCount
		return metadata
	}

	var typed *errs.Error
	if !errors.As(opErr, &typed) {
		return metadata
	}

	metadata["errorCode"] = typed.Code
	switch typed.Code {
	case errs.CodeWorkbenchValidationFailed, errs.CodeWorkbenchStaleRevision, errs.CodeWorkbenchDriftDetected:
		metadata["blocked"] = true
	}

	details, ok := typed.Details.(map[string]any)
	if !ok {
		return metadata
	}

	if revision, ok := details["revision"]; ok {
		metadata["revision"] = revision
	}
	if fingerprint, ok := details["sourceFingerprint"].(string); ok {
		metadata["sourceFingerprint"] = fingerprint
	}
	if composePath, ok := details["composePath"].(string); ok {
		metadata["composePath"] = composePath
	}
	if issueCount, ok := details["issueCount"].(int); ok {
		metadata["issueCount"] = issueCount
	}
	return metadata
}

func workbenchComposeRestoreAuditMetadata(
	project string,
	req models.ProjectWorkbenchComposeRestoreRequest,
	result *service.WorkbenchComposeRestoreResult,
	opErr error,
) map[string]any {
	metadata := map[string]any{
		"project":             project,
		"backupId":            strings.TrimSpace(req.BackupID),
		"success":             opErr == nil,
		"blocked":             false,
		"revision":            nil,
		"sourceFingerprint":   "",
		"restoredFingerprint": "",
		"composePath":         "",
		"composeBytes":        0,
		"requiresImport":      false,
		"errorCode":           "",
	}

	if result != nil {
		metadata["revision"] = result.Metadata.Revision
		metadata["sourceFingerprint"] = result.Metadata.SourceFingerprint
		metadata["restoredFingerprint"] = result.Metadata.RestoredFingerprint
		metadata["composePath"] = result.Metadata.ComposePath
		metadata["composeBytes"] = result.ComposeBytes
		metadata["requiresImport"] = result.Metadata.RequiresImport
		return metadata
	}

	var typed *errs.Error
	if !errors.As(opErr, &typed) {
		return metadata
	}

	metadata["errorCode"] = typed.Code
	metadata["blocked"] = typed.Code == errs.CodeWorkbenchBackupIntegrity
	details, ok := typed.Details.(map[string]any)
	if !ok {
		return metadata
	}

	if revision, ok := details["revision"]; ok {
		metadata["revision"] = revision
	}
	if fingerprint, ok := details["sourceFingerprint"].(string); ok {
		metadata["sourceFingerprint"] = fingerprint
	}
	if composePath, ok := details["composePath"].(string); ok {
		metadata["composePath"] = composePath
	}
	return metadata
}

func workbenchComposePreviewAuditMetadata(
	project string,
	expectedRevision *int,
	preview *service.WorkbenchComposePreviewResult,
	opErr error,
) map[string]any {
	metadata := map[string]any{
		"project":           project,
		"expectedRevision":  nil,
		"success":           opErr == nil,
		"blocked":           false,
		"revision":          nil,
		"sourceFingerprint": "",
		"composeBytes":      0,
		"issueCount":        0,
		"errorCode":         "",
	}
	if expectedRevision != nil {
		metadata["expectedRevision"] = *expectedRevision
	}

	if preview != nil {
		metadata["revision"] = preview.Metadata.Revision
		metadata["sourceFingerprint"] = preview.Metadata.SourceFingerprint
		metadata["composeBytes"] = len(preview.Compose)
		return metadata
	}

	var typed *errs.Error
	if !errors.As(opErr, &typed) {
		return metadata
	}

	metadata["errorCode"] = typed.Code
	metadata["blocked"] = typed.Code == errs.CodeWorkbenchValidationFailed
	details, ok := typed.Details.(map[string]any)
	if !ok {
		return metadata
	}

	if revision, ok := details["revision"]; ok {
		metadata["revision"] = revision
	}
	if fingerprint, ok := details["sourceFingerprint"].(string); ok {
		metadata["sourceFingerprint"] = fingerprint
	}
	if issueCount, ok := details["issueCount"].(int); ok {
		metadata["issueCount"] = issueCount
	}
	return metadata
}

func workbenchErrorCodeAndIssueCount(opErr error) (errs.Code, int) {
	if opErr == nil {
		return "", 0
	}

	var typed *errs.Error
	if !errors.As(opErr, &typed) {
		return "", 0
	}

	issueCount := 0
	details, ok := typed.Details.(map[string]any)
	if ok {
		switch value := details["issueCount"].(type) {
		case int:
			issueCount = value
		case int32:
			issueCount = int(value)
		case int64:
			issueCount = int(value)
		case float64:
			issueCount = int(value)
		}
	}

	return typed.Code, issueCount
}

func (c *ProjectsController) logAudit(ctx *gin.Context, action, target string, metadata map[string]any) {
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

func jobTargetsFromPlan(plan service.ProjectArchivePlan, options service.ProjectArchiveOptions) service.ProjectArchiveTargets {
	targets := service.ProjectArchiveTargets{
		Containers:         []string{},
		Hostnames:          []string{},
		ExposureContainers: []string{},
		ExposureHostnames:  []string{},
		DNSRecords:         []service.ProjectArchiveDNSDeleteTarget{},
	}
	if options.RemoveContainers {
		for _, container := range plan.Containers {
			if strings.TrimSpace(container.Name) == "" {
				continue
			}
			targets.Containers = append(targets.Containers, container.Name)
		}
		for _, exposure := range plan.ServiceExposures {
			container := strings.TrimSpace(exposure.Container)
			if container == "" {
				continue
			}
			targets.ExposureContainers = append(targets.ExposureContainers, container)
		}
	}
	if options.RemoveIngress {
		targets.Hostnames = append(targets.Hostnames, plan.Hostnames...)
		for _, exposure := range plan.ServiceExposures {
			hostname := strings.ToLower(strings.TrimSpace(exposure.Hostname))
			if hostname == "" {
				continue
			}
			targets.ExposureHostnames = append(targets.ExposureHostnames, hostname)
		}
	}
	if options.RemoveDNS {
		for _, record := range plan.DNSRecords {
			if !record.DeleteEligible {
				continue
			}
			targets.DNSRecords = append(targets.DNSRecords, service.ProjectArchiveDNSDeleteTarget{
				ZoneID:   record.ZoneID,
				RecordID: record.ID,
				Hostname: record.Name,
				Content:  record.Content,
			})
		}
	}
	return targets
}
