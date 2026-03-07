package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

const (
	workbenchModelVersion = 1

	workbenchImportReasonManual       = "manual"
	workbenchImportReasonAutoDeploy   = "auto_deploy"
	workbenchImportReasonAutoRedeploy = "auto_redeploy"
)

type WorkbenchStackModule struct {
	ModuleType  string `json:"moduleType"`
	ServiceName string `json:"serviceName,omitempty"`
}

type WorkbenchManagedService struct {
	EntryKey    string `json:"entryKey"`
	ServiceName string `json:"serviceName"`
}

type WorkbenchStackSnapshot struct {
	ProjectName       string                       `json:"projectName"`
	ProjectDir        string                       `json:"projectDir"`
	ComposePath       string                       `json:"composePath"`
	ModelVersion      int                          `json:"modelVersion"`
	Revision          int                          `json:"revision"`
	SourceFingerprint string                       `json:"sourceFingerprint"`
	Services          []WorkbenchComposeService    `json:"services"`
	Dependencies      []WorkbenchComposeDependency `json:"dependencies"`
	Ports             []WorkbenchComposePort       `json:"ports"`
	Resources         []WorkbenchComposeResource   `json:"resources"`
	NetworkRefs       []WorkbenchComposeNetworkRef `json:"networkRefs"`
	VolumeRefs        []WorkbenchComposeVolumeRef  `json:"volumeRefs"`
	EnvRefs           []WorkbenchComposeEnvRef     `json:"envRefs"`
	ManagedServices   []WorkbenchManagedService    `json:"managedServices"`
	Modules           []WorkbenchStackModule       `json:"modules"`
	Warnings          []WorkbenchComposeWarning    `json:"warnings"`
}

type workbenchStoredSnapshot = WorkbenchStackSnapshot

func (s *WorkbenchService) ImportComposeSnapshot(
	ctx context.Context,
	projectName string,
	reason string,
) (WorkbenchStackSnapshot, bool, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchStackSnapshot{}, false, err
	}
	if _, err := normalizeWorkbenchImportReason(reason); err != nil {
		return WorkbenchStackSnapshot{}, false, err
	}

	source, release, err := s.ResolveComposeSourceWithLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, false, err
	}
	defer release()

	parsed, err := s.ParseComposeCoreFromSource(source)
	if err != nil {
		return WorkbenchStackSnapshot{}, false, err
	}

	current, exists, err := s.loadStoredWorkbenchSnapshot(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, false, err
	}
	if exists && current.SourceFingerprint == parsed.SourceFingerprint {
		return current, false, nil
	}

	next := snapshotFromParsedCompose(parsed)
	if exists && current.Revision > 0 {
		next.Revision = current.Revision + 1
	}

	if err := s.saveWorkbenchSnapshot(ctx, normalizedProject, next); err != nil {
		return WorkbenchStackSnapshot{}, false, err
	}
	return next, true, nil
}

func normalizeWorkbenchImportReason(reason string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	if normalized == "" {
		return workbenchImportReasonManual, nil
	}
	switch normalized {
	case workbenchImportReasonManual, workbenchImportReasonAutoDeploy, workbenchImportReasonAutoRedeploy:
		return normalized, nil
	default:
		return "", errs.New(errs.CodeProjectInvalidBody, fmt.Sprintf("invalid workbench import reason %q", reason))
	}
}

func snapshotFromParsedCompose(parsed WorkbenchComposeParseResult) WorkbenchStackSnapshot {
	return normalizeWorkbenchStackSnapshot(WorkbenchStackSnapshot{
		ProjectName:       parsed.ProjectName,
		ProjectDir:        parsed.ProjectDir,
		ComposePath:       parsed.ComposePath,
		ModelVersion:      workbenchModelVersion,
		Revision:          1,
		SourceFingerprint: parsed.SourceFingerprint,
		Services:          append([]WorkbenchComposeService{}, parsed.Services...),
		Dependencies:      append([]WorkbenchComposeDependency{}, parsed.Dependencies...),
		Ports:             append([]WorkbenchComposePort{}, parsed.Ports...),
		Resources:         append([]WorkbenchComposeResource{}, parsed.Resources...),
		NetworkRefs:       append([]WorkbenchComposeNetworkRef{}, parsed.NetworkRefs...),
		VolumeRefs:        append([]WorkbenchComposeVolumeRef{}, parsed.VolumeRefs...),
		EnvRefs:           append([]WorkbenchComposeEnvRef{}, parsed.EnvRefs...),
		ManagedServices:   []WorkbenchManagedService{},
		Modules:           []WorkbenchStackModule{},
		Warnings:          append([]WorkbenchComposeWarning{}, parsed.Warnings...),
	})
}

func normalizeWorkbenchStackSnapshot(snapshot WorkbenchStackSnapshot) WorkbenchStackSnapshot {
	normalized := snapshot
	if normalized.ModelVersion <= 0 {
		normalized.ModelVersion = workbenchModelVersion
	}
	if normalized.Revision <= 0 {
		normalized.Revision = 1
	}
	normalized.ProjectName = strings.ToLower(strings.TrimSpace(normalized.ProjectName))
	normalized.ProjectDir = strings.TrimSpace(normalized.ProjectDir)
	normalized.ComposePath = strings.TrimSpace(normalized.ComposePath)
	normalized.SourceFingerprint = strings.TrimSpace(normalized.SourceFingerprint)

	if normalized.Services == nil {
		normalized.Services = []WorkbenchComposeService{}
	}
	if normalized.Dependencies == nil {
		normalized.Dependencies = []WorkbenchComposeDependency{}
	}
	if normalized.Ports == nil {
		normalized.Ports = []WorkbenchComposePort{}
	}
	if normalized.Resources == nil {
		normalized.Resources = []WorkbenchComposeResource{}
	}
	if normalized.NetworkRefs == nil {
		normalized.NetworkRefs = []WorkbenchComposeNetworkRef{}
	}
	if normalized.VolumeRefs == nil {
		normalized.VolumeRefs = []WorkbenchComposeVolumeRef{}
	}
	if normalized.EnvRefs == nil {
		normalized.EnvRefs = []WorkbenchComposeEnvRef{}
	}
	if normalized.ManagedServices == nil {
		normalized.ManagedServices = []WorkbenchManagedService{}
	}
	if normalized.Modules == nil {
		normalized.Modules = []WorkbenchStackModule{}
	}
	if normalized.Warnings == nil {
		normalized.Warnings = []WorkbenchComposeWarning{}
	}

	for idx := range normalized.Ports {
		normalized.Ports[idx] = normalizeWorkbenchComposePort(normalized.Ports[idx])
	}
	if len(normalized.ManagedServices) > 0 {
		cleaned := make([]WorkbenchManagedService, 0, len(normalized.ManagedServices))
		for _, managedService := range normalized.ManagedServices {
			entryKey := strings.ToLower(strings.TrimSpace(managedService.EntryKey))
			serviceName := strings.TrimSpace(managedService.ServiceName)
			if entryKey == "" || serviceName == "" {
				continue
			}
			cleaned = append(cleaned, WorkbenchManagedService{
				EntryKey:    entryKey,
				ServiceName: serviceName,
			})
		}
		normalized.ManagedServices = cleaned
	}

	sort.SliceStable(normalized.Services, func(i, j int) bool {
		return workbenchComposeServiceLess(normalized.Services[i], normalized.Services[j])
	})
	sort.SliceStable(normalized.Dependencies, func(i, j int) bool {
		return workbenchComposeDependencyLess(normalized.Dependencies[i], normalized.Dependencies[j])
	})
	sort.SliceStable(normalized.Ports, func(i, j int) bool {
		return workbenchComposePortLess(normalized.Ports[i], normalized.Ports[j])
	})
	sort.SliceStable(normalized.Resources, func(i, j int) bool {
		return workbenchComposeResourceLess(normalized.Resources[i], normalized.Resources[j])
	})
	sort.SliceStable(normalized.NetworkRefs, func(i, j int) bool {
		return workbenchComposeNetworkRefLess(normalized.NetworkRefs[i], normalized.NetworkRefs[j])
	})
	sort.SliceStable(normalized.VolumeRefs, func(i, j int) bool {
		return workbenchComposeVolumeRefLess(normalized.VolumeRefs[i], normalized.VolumeRefs[j])
	})
	sort.SliceStable(normalized.EnvRefs, func(i, j int) bool {
		return workbenchComposeEnvRefLess(normalized.EnvRefs[i], normalized.EnvRefs[j])
	})
	sort.SliceStable(normalized.ManagedServices, func(i, j int) bool {
		leftService := strings.ToLower(strings.TrimSpace(normalized.ManagedServices[i].ServiceName))
		rightService := strings.ToLower(strings.TrimSpace(normalized.ManagedServices[j].ServiceName))
		if leftService != rightService {
			return leftService < rightService
		}
		leftEntry := strings.ToLower(strings.TrimSpace(normalized.ManagedServices[i].EntryKey))
		rightEntry := strings.ToLower(strings.TrimSpace(normalized.ManagedServices[j].EntryKey))
		return leftEntry < rightEntry
	})
	sort.SliceStable(normalized.Modules, func(i, j int) bool {
		leftType := strings.ToLower(strings.TrimSpace(normalized.Modules[i].ModuleType))
		rightType := strings.ToLower(strings.TrimSpace(normalized.Modules[j].ModuleType))
		if leftType != rightType {
			return leftType < rightType
		}
		leftService := strings.ToLower(strings.TrimSpace(normalized.Modules[i].ServiceName))
		rightService := strings.ToLower(strings.TrimSpace(normalized.Modules[j].ServiceName))
		return leftService < rightService
	})
	sort.SliceStable(normalized.Warnings, func(i, j int) bool {
		return workbenchComposeWarningLess(normalized.Warnings[i], normalized.Warnings[j])
	})

	return normalized
}

func normalizeWorkbenchComposePort(port WorkbenchComposePort) WorkbenchComposePort {
	normalized := port
	normalized.ServiceName = strings.TrimSpace(normalized.ServiceName)
	normalized.HostPortRaw = strings.TrimSpace(normalized.HostPortRaw)
	normalized.Protocol = strings.ToLower(strings.TrimSpace(normalized.Protocol))
	if normalized.Protocol == "" {
		normalized.Protocol = "tcp"
	}
	normalized.HostIP = normalizeHostIP(strings.TrimSpace(normalized.HostIP))

	strategy := strings.ToLower(strings.TrimSpace(normalized.AssignmentStrategy))
	switch strategy {
	case workbenchPortStrategyAuto, workbenchPortStrategyManual:
		normalized.AssignmentStrategy = strategy
	default:
		normalized.AssignmentStrategy = ""
	}

	status := strings.ToLower(strings.TrimSpace(normalized.AllocationStatus))
	switch status {
	case workbenchPortAllocationAssigned, workbenchPortAllocationConflict, workbenchPortAllocationUnavailable:
		normalized.AllocationStatus = status
	default:
		normalized.AllocationStatus = ""
	}

	return normalized
}

func (s *WorkbenchService) loadStoredWorkbenchSnapshot(
	ctx context.Context,
	projectName string,
) (WorkbenchStackSnapshot, bool, error) {
	if s.settings == nil {
		return WorkbenchStackSnapshot{}, false, workbenchStorageError(projectName, "workbench settings storage is unavailable", nil)
	}

	stored, err := s.settings.Get(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return WorkbenchStackSnapshot{}, false, nil
		}
		return WorkbenchStackSnapshot{}, false, workbenchStorageError(projectName, "failed to load workbench settings payload", err)
	}
	if stored == nil {
		return WorkbenchStackSnapshot{}, false, nil
	}

	payload, err := loadSettingsEncryptedPayload(s.sessionSecret, stored.NetBirdConfigEncrypted)
	if err != nil {
		return WorkbenchStackSnapshot{}, false, workbenchStorageError(projectName, "failed to decode workbench settings payload", err)
	}
	if len(payload.Workbench) == 0 {
		return WorkbenchStackSnapshot{}, false, nil
	}

	snapshot, ok := payload.Workbench[projectName]
	if !ok {
		return WorkbenchStackSnapshot{}, false, nil
	}
	return normalizeWorkbenchStackSnapshot(snapshot), true, nil
}

func (s *WorkbenchService) saveWorkbenchSnapshot(
	ctx context.Context,
	projectName string,
	snapshot WorkbenchStackSnapshot,
) error {
	settingsWriteLock.Lock()
	defer settingsWriteLock.Unlock()

	if s.settings == nil {
		return workbenchStorageError(projectName, "workbench settings storage is unavailable", nil)
	}

	stored, err := s.settings.Get(ctx)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return workbenchStorageError(projectName, "failed to load workbench settings payload", err)
	}
	if stored == nil {
		stored = &models.Settings{}
	}

	payload, err := loadSettingsEncryptedPayload(s.sessionSecret, stored.NetBirdConfigEncrypted)
	if err != nil {
		return workbenchStorageError(projectName, "failed to decode workbench settings payload", err)
	}
	if payload.Workbench == nil {
		payload.Workbench = map[string]workbenchStoredSnapshot{}
	}
	normalized := normalizeWorkbenchStackSnapshot(snapshot)
	normalized.ProjectName = projectName
	payload.Workbench[projectName] = normalized

	encoded, err := encodeSettingsEncryptedPayload(s.sessionSecret, payload)
	if err != nil {
		return workbenchStorageError(projectName, "failed to encode workbench settings payload", err)
	}
	stored.NetBirdConfigEncrypted = encoded

	if err := s.settings.Save(ctx, stored); err != nil {
		return workbenchStorageError(projectName, "failed to persist workbench settings payload", err)
	}
	return nil
}

func workbenchStorageError(projectName, message string, cause error) error {
	details := map[string]any{
		"project": strings.ToLower(strings.TrimSpace(projectName)),
	}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	return errs.WithDetails(errs.Wrap(errs.CodeWorkbenchStorageFailed, message, cause), details)
}
