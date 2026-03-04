package service

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"go-notes/internal/config"
	"go-notes/internal/errs"
	"go-notes/internal/repository"
)

const workbenchModelVersion = 1

type WorkbenchService struct {
	cfg      config.Config
	projects repository.ProjectRepository
	settings *SettingsService
}

type WorkbenchSnapshot struct {
	Project           WorkbenchSnapshotProject    `json:"project"`
	ModelVersion      int                         `json:"modelVersion"`
	Revision          int                         `json:"revision"`
	SourceFingerprint *string                     `json:"sourceFingerprint"`
	Services          []WorkbenchSnapshotService  `json:"services"`
	Ports             []WorkbenchSnapshotPort     `json:"ports"`
	Resources         []WorkbenchSnapshotResource `json:"resources"`
	Modules           []WorkbenchSnapshotModule   `json:"modules"`
	Warnings          []WorkbenchSnapshotWarning  `json:"warnings"`
}

type WorkbenchSnapshotProject struct {
	Name           string `json:"name"`
	NormalizedName string `json:"normalizedName"`
	Path           string `json:"path"`
	ComposePath    string `json:"composePath"`
}

type WorkbenchSnapshotService struct {
	ServiceName    string   `json:"serviceName"`
	Classification string   `json:"classification,omitempty"`
	Image          string   `json:"image,omitempty"`
	DependsOn      []string `json:"dependsOn"`
}

type WorkbenchSnapshotPort struct {
	ServiceName   string `json:"serviceName"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIp"`
	HostPort      int    `json:"hostPort"`
}

type WorkbenchSnapshotResource struct {
	ServiceName       string `json:"serviceName"`
	CPULimit          string `json:"cpuLimit,omitempty"`
	CPUReservation    string `json:"cpuReservation,omitempty"`
	MemoryLimit       string `json:"memoryLimit,omitempty"`
	MemoryReservation string `json:"memoryReservation,omitempty"`
}

type WorkbenchSnapshotModule struct {
	ModuleType  string `json:"moduleType"`
	ServiceName string `json:"serviceName"`
	Source      string `json:"source,omitempty"`
}

type WorkbenchSnapshotWarning struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type workbenchStoredSnapshot struct {
	ComposePath       string                      `json:"composePath,omitempty"`
	ModelVersion      int                         `json:"modelVersion,omitempty"`
	Revision          int                         `json:"revision,omitempty"`
	SourceFingerprint string                      `json:"sourceFingerprint,omitempty"`
	Services          []WorkbenchSnapshotService  `json:"services,omitempty"`
	Ports             []WorkbenchSnapshotPort     `json:"ports,omitempty"`
	Resources         []WorkbenchSnapshotResource `json:"resources,omitempty"`
	Modules           []WorkbenchSnapshotModule   `json:"modules,omitempty"`
	Warnings          []WorkbenchSnapshotWarning  `json:"warnings,omitempty"`
}

func NewWorkbenchService(
	cfg config.Config,
	projects repository.ProjectRepository,
	settings *SettingsService,
) *WorkbenchService {
	return &WorkbenchService{
		cfg:      cfg,
		projects: projects,
		settings: settings,
	}
}

func (s *WorkbenchService) GetSnapshot(ctx context.Context, projectName string) (WorkbenchSnapshot, error) {
	resolved, err := resolveProjectPath(ctx, s.projects, s.cfg.TemplatesDir, projectName)
	if err != nil {
		return WorkbenchSnapshot{}, err
	}

	if s.settings == nil {
		return WorkbenchSnapshot{}, errs.New(errs.CodeWorkbenchStorageFailed, "workbench settings adapter unavailable")
	}

	stored, exists, err := s.settings.loadWorkbenchStoredSnapshot(ctx, resolved.NormalizedName)
	if err != nil {
		return WorkbenchSnapshot{}, errs.Wrap(errs.CodeWorkbenchStorageFailed, "load workbench snapshot failed", err)
	}
	if !exists {
		stored = workbenchStoredSnapshot{}
	}

	normalized := normalizeWorkbenchStoredSnapshot(stored)
	composePath := resolveWorkbenchComposePath(normalized.ComposePath, resolved)

	return WorkbenchSnapshot{
		Project: WorkbenchSnapshotProject{
			Name:           resolved.RequestedName,
			NormalizedName: resolved.NormalizedName,
			Path:           resolved.ProjectDir,
			ComposePath:    composePath,
		},
		ModelVersion:      normalized.ModelVersion,
		Revision:          normalized.Revision,
		SourceFingerprint: nullableTrimmedString(normalized.SourceFingerprint),
		Services:          append([]WorkbenchSnapshotService(nil), normalized.Services...),
		Ports:             append([]WorkbenchSnapshotPort(nil), normalized.Ports...),
		Resources:         append([]WorkbenchSnapshotResource(nil), normalized.Resources...),
		Modules:           append([]WorkbenchSnapshotModule(nil), normalized.Modules...),
		Warnings:          append([]WorkbenchSnapshotWarning(nil), normalized.Warnings...),
	}, nil
}

func normalizeWorkbenchStoredSnapshot(input workbenchStoredSnapshot) workbenchStoredSnapshot {
	normalized := workbenchStoredSnapshot{
		ComposePath:       strings.TrimSpace(input.ComposePath),
		ModelVersion:      input.ModelVersion,
		Revision:          input.Revision,
		SourceFingerprint: strings.TrimSpace(input.SourceFingerprint),
		Services:          normalizeWorkbenchServices(input.Services),
		Ports:             normalizeWorkbenchPorts(input.Ports),
		Resources:         normalizeWorkbenchResources(input.Resources),
		Modules:           normalizeWorkbenchModules(input.Modules),
		Warnings:          normalizeWorkbenchWarnings(input.Warnings),
	}
	if normalized.ModelVersion <= 0 {
		normalized.ModelVersion = workbenchModelVersion
	}
	if normalized.Revision < 0 {
		normalized.Revision = 0
	}
	return normalized
}

func normalizeWorkbenchServices(input []WorkbenchSnapshotService) []WorkbenchSnapshotService {
	if len(input) == 0 {
		return []WorkbenchSnapshotService{}
	}
	normalized := make([]WorkbenchSnapshotService, 0, len(input))
	for _, service := range input {
		dependsOn := normalizeStringList(service.DependsOn)
		normalized = append(normalized, WorkbenchSnapshotService{
			ServiceName:    strings.TrimSpace(service.ServiceName),
			Classification: strings.TrimSpace(service.Classification),
			Image:          strings.TrimSpace(service.Image),
			DependsOn:      dependsOn,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		left := strings.ToLower(normalized[i].ServiceName)
		right := strings.ToLower(normalized[j].ServiceName)
		if left == right {
			return normalized[i].ServiceName < normalized[j].ServiceName
		}
		return left < right
	})
	return normalized
}

func normalizeWorkbenchPorts(input []WorkbenchSnapshotPort) []WorkbenchSnapshotPort {
	if len(input) == 0 {
		return []WorkbenchSnapshotPort{}
	}
	normalized := make([]WorkbenchSnapshotPort, 0, len(input))
	for _, port := range input {
		normalized = append(normalized, WorkbenchSnapshotPort{
			ServiceName:   strings.TrimSpace(port.ServiceName),
			ContainerPort: port.ContainerPort,
			Protocol:      strings.ToLower(strings.TrimSpace(port.Protocol)),
			HostIP:        strings.TrimSpace(port.HostIP),
			HostPort:      port.HostPort,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		left := normalized[i]
		right := normalized[j]
		if left.ServiceName != right.ServiceName {
			return left.ServiceName < right.ServiceName
		}
		if left.ContainerPort != right.ContainerPort {
			return left.ContainerPort < right.ContainerPort
		}
		if left.Protocol != right.Protocol {
			return left.Protocol < right.Protocol
		}
		if left.HostIP != right.HostIP {
			return left.HostIP < right.HostIP
		}
		return left.HostPort < right.HostPort
	})
	return normalized
}

func normalizeWorkbenchResources(input []WorkbenchSnapshotResource) []WorkbenchSnapshotResource {
	if len(input) == 0 {
		return []WorkbenchSnapshotResource{}
	}
	normalized := make([]WorkbenchSnapshotResource, 0, len(input))
	for _, resource := range input {
		normalized = append(normalized, WorkbenchSnapshotResource{
			ServiceName:       strings.TrimSpace(resource.ServiceName),
			CPULimit:          strings.TrimSpace(resource.CPULimit),
			CPUReservation:    strings.TrimSpace(resource.CPUReservation),
			MemoryLimit:       strings.TrimSpace(resource.MemoryLimit),
			MemoryReservation: strings.TrimSpace(resource.MemoryReservation),
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].ServiceName < normalized[j].ServiceName
	})
	return normalized
}

func normalizeWorkbenchModules(input []WorkbenchSnapshotModule) []WorkbenchSnapshotModule {
	if len(input) == 0 {
		return []WorkbenchSnapshotModule{}
	}
	normalized := make([]WorkbenchSnapshotModule, 0, len(input))
	for _, module := range input {
		normalized = append(normalized, WorkbenchSnapshotModule{
			ModuleType:  strings.TrimSpace(module.ModuleType),
			ServiceName: strings.TrimSpace(module.ServiceName),
			Source:      strings.TrimSpace(module.Source),
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].ModuleType == normalized[j].ModuleType {
			return normalized[i].ServiceName < normalized[j].ServiceName
		}
		return normalized[i].ModuleType < normalized[j].ModuleType
	})
	return normalized
}

func normalizeWorkbenchWarnings(input []WorkbenchSnapshotWarning) []WorkbenchSnapshotWarning {
	if len(input) == 0 {
		return []WorkbenchSnapshotWarning{}
	}
	normalized := make([]WorkbenchSnapshotWarning, 0, len(input))
	for _, warning := range input {
		normalized = append(normalized, WorkbenchSnapshotWarning{
			Code:    strings.TrimSpace(warning.Code),
			Path:    strings.TrimSpace(warning.Path),
			Message: strings.TrimSpace(warning.Message),
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Code == normalized[j].Code {
			return normalized[i].Path < normalized[j].Path
		}
		return normalized[i].Code < normalized[j].Code
	})
	return normalized
}

func resolveWorkbenchComposePath(stored string, resolved projectPathResolution) string {
	if len(resolved.ComposeFiles) > 0 {
		return resolved.ComposeFiles[0]
	}
	trimmedStored := strings.TrimSpace(stored)
	if trimmedStored != "" {
		return trimmedStored
	}
	return filepath.Join(resolved.ProjectDir, "docker-compose.yml")
}

func nullableTrimmedString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeWorkbenchProjectKey(projectName string) string {
	return strings.ToLower(strings.TrimSpace(projectName))
}
