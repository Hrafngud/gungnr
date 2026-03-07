package service

import (
	"context"
	"net/url"
	"strings"
)

const (
	workbenchOptionalServiceStatusAvailable                          = "available"
	workbenchOptionalServiceStatusComposePresent                     = "compose_present"
	workbenchOptionalServiceStatusLegacyModuleOnly                   = "legacy_module_only"
	workbenchOptionalServiceStatusComposePresentWithLegacy           = "compose_present_with_legacy_module"
	workbenchOptionalServiceStatusCatalogManaged                     = "catalog_managed"
	workbenchOptionalServiceStatusCatalogManagedWithCompose          = "catalog_managed_with_compose"
	workbenchOptionalServiceStatusCatalogManagedWithLegacy           = "catalog_managed_with_legacy_module"
	workbenchOptionalServiceStatusCatalogManagedWithComposeAndLegacy = "catalog_managed_with_compose_and_legacy_module"
	workbenchOptionalServiceLegacyModulesStatusEmpty                 = "empty"
	workbenchOptionalServiceLegacyModulesStatusPresent               = "present"
	workbenchOptionalServiceCurrentStateUnmanaged                    = "unmanaged"
	workbenchOptionalServiceCurrentStateLegacyModules                = "legacy_modules"
	workbenchOptionalServiceCurrentStateCatalogManaged               = "catalog_managed"
	workbenchOptionalServiceTargetStateCatalogManaged                = "catalog_managed"
	workbenchOptionalServiceMatchReasonServiceName                   = "service_name"
	workbenchOptionalServiceMatchReasonImageRepository               = "image_repository"
)

// WorkbenchOptionalServiceCatalog freezes the transition contract for the
// future multi-service catalog without pretending the old Redis-only module
// metadata is already catalog-managed.
type WorkbenchOptionalServiceCatalog struct {
	ProjectName       string                                 `json:"projectName"`
	SnapshotImported  bool                                   `json:"snapshotImported"`
	SnapshotRevision  int                                    `json:"snapshotRevision"`
	SourceFingerprint string                                 `json:"sourceFingerprint,omitempty"`
	Entries           []WorkbenchOptionalServiceCatalogEntry `json:"entries"`
	LegacyModules     WorkbenchLegacyModuleCatalog           `json:"legacyModules"`
}

type WorkbenchOptionalServiceCatalogEntry struct {
	Key                  string                               `json:"key"`
	DisplayName          string                               `json:"displayName"`
	Description          string                               `json:"description"`
	Category             string                               `json:"category"`
	DefaultServiceName   string                               `json:"defaultServiceName"`
	SuggestedImage       string                               `json:"suggestedImage"`
	DefaultContainerPort int                                  `json:"defaultContainerPort"`
	Availability         WorkbenchOptionalServiceAvailability `json:"availability"`
	Transition           WorkbenchOptionalServiceTransition   `json:"transition"`
}

type WorkbenchOptionalServiceAvailability struct {
	Status          string                                 `json:"status"`
	ComposeServices []WorkbenchOptionalServiceComposeMatch `json:"composeServices"`
	LegacyModules   []WorkbenchStackModule                 `json:"legacyModules"`
	ManagedServices []WorkbenchManagedService              `json:"managedServices"`
}

type WorkbenchOptionalServiceComposeMatch struct {
	ServiceName string `json:"serviceName"`
	Image       string `json:"image,omitempty"`
	MatchReason string `json:"matchReason"`
}

type WorkbenchOptionalServiceTransition struct {
	ReadOnly               bool     `json:"readOnly"`
	MutationReady          bool     `json:"mutationReady"`
	ComposeGenerationReady bool     `json:"composeGenerationReady"`
	CurrentState           string   `json:"currentState"`
	TargetState            string   `json:"targetState"`
	LegacyModuleType       string   `json:"legacyModuleType,omitempty"`
	LegacyMutationPath     string   `json:"legacyMutationPath,omitempty"`
	Notes                  []string `json:"notes"`
}

type WorkbenchLegacyModuleCatalog struct {
	Status               string                 `json:"status"`
	SupportedModuleTypes []string               `json:"supportedModuleTypes"`
	MutationPath         string                 `json:"mutationPath"`
	Records              []WorkbenchStackModule `json:"records"`
	Notes                []string               `json:"notes"`
}

type workbenchOptionalServiceDefinition struct {
	key                  string
	displayName          string
	description          string
	category             string
	defaultServiceName   string
	suggestedImage       string
	defaultContainerPort int
	serviceNameHints     []string
	imageNameHints       []string
	legacyModuleType     string
	runtime              workbenchOptionalServiceRuntimeDefinition
}

var workbenchOptionalServiceDefinitions = []workbenchOptionalServiceDefinition{
	{
		key:                  "redis",
		displayName:          "Redis",
		description:          "Cache and queue baseline for app-side state or transient data.",
		category:             "cache",
		defaultServiceName:   "redis",
		suggestedImage:       "redis:7-alpine",
		defaultContainerPort: 6379,
		serviceNameHints:     []string{"redis"},
		imageNameHints:       []string{"redis"},
		legacyModuleType:     "redis",
		runtime: workbenchOptionalServiceRuntimeDefinition{
			image:         "redis:7-alpine",
			restartPolicy: "unless-stopped",
			ports: []workbenchOptionalServicePortDefinition{
				{containerPort: 6379, protocol: "tcp"},
			},
		},
	},
	{
		key:                  "nginx",
		displayName:          "Nginx",
		description:          "Edge proxy baseline for static delivery, reverse proxying, or custom ingress.",
		category:             "edge",
		defaultServiceName:   "nginx",
		suggestedImage:       "nginx:stable-alpine",
		defaultContainerPort: 80,
		serviceNameHints:     []string{"nginx"},
		imageNameHints:       []string{"nginx"},
		runtime: workbenchOptionalServiceRuntimeDefinition{
			image:         "nginx:stable-alpine",
			restartPolicy: "unless-stopped",
			ports: []workbenchOptionalServicePortDefinition{
				{containerPort: 80, protocol: "tcp"},
			},
		},
	},
	{
		key:                  "prometheus",
		displayName:          "Prometheus",
		description:          "Metrics scraper baseline for service and infrastructure observability.",
		category:             "observability",
		defaultServiceName:   "prometheus",
		suggestedImage:       "prom/prometheus:latest",
		defaultContainerPort: 9090,
		serviceNameHints:     []string{"prometheus"},
		imageNameHints:       []string{"prometheus"},
		runtime: workbenchOptionalServiceRuntimeDefinition{
			image:         "prom/prometheus:latest",
			restartPolicy: "unless-stopped",
			ports: []workbenchOptionalServicePortDefinition{
				{containerPort: 9090, protocol: "tcp"},
			},
		},
	},
	{
		key:                  "minio",
		displayName:          "MinIO",
		description:          "Object-storage baseline for S3-compatible local buckets and assets.",
		category:             "storage",
		defaultServiceName:   "minio",
		suggestedImage:       "minio/minio:latest",
		defaultContainerPort: 9000,
		serviceNameHints:     []string{"minio"},
		imageNameHints:       []string{"minio"},
		runtime: workbenchOptionalServiceRuntimeDefinition{
			image:         "minio/minio:latest",
			restartPolicy: "unless-stopped",
			command:       []string{"server", "/data", "--console-address", ":9001"},
			environment: map[string]string{
				"MINIO_ROOT_PASSWORD": "${MINIO_ROOT_PASSWORD:-minioadmin123}",
				"MINIO_ROOT_USER":     "${MINIO_ROOT_USER:-minioadmin}",
			},
			ports: []workbenchOptionalServicePortDefinition{
				{containerPort: 9000, protocol: "tcp"},
				{containerPort: 9001, protocol: "tcp"},
			},
		},
	},
}

func (s *WorkbenchService) GetOptionalServiceCatalog(
	ctx context.Context,
	projectName string,
) (WorkbenchOptionalServiceCatalog, error) {
	snapshot, err := s.GetSnapshot(ctx, projectName)
	if err != nil {
		return WorkbenchOptionalServiceCatalog{}, err
	}

	normalizedProject := strings.ToLower(strings.TrimSpace(snapshot.ProjectName))
	legacyPath := "/api/v1/projects/" + url.PathEscape(normalizedProject) + "/workbench/modules"
	legacyRecords := workbenchNormalizeLegacyModuleRecords(snapshot.Modules)

	catalog := WorkbenchOptionalServiceCatalog{
		ProjectName:       normalizedProject,
		SnapshotImported:  snapshot.Revision > 0 && strings.TrimSpace(snapshot.SourceFingerprint) != "",
		SnapshotRevision:  snapshot.Revision,
		SourceFingerprint: strings.TrimSpace(snapshot.SourceFingerprint),
		Entries:           make([]WorkbenchOptionalServiceCatalogEntry, 0, len(workbenchOptionalServiceDefinitions)),
		LegacyModules: WorkbenchLegacyModuleCatalog{
			Status:               workbenchOptionalServiceLegacyModulesStatusEmpty,
			SupportedModuleTypes: []string{"redis"},
			MutationPath:         legacyPath,
			Records:              legacyRecords,
			Notes: []string{
				"Legacy /workbench/modules mutations remain available during the transition.",
				"Legacy module records are surfaced separately so callers do not mistake them for catalog-managed optional services.",
			},
		},
	}
	if len(legacyRecords) > 0 {
		catalog.LegacyModules.Status = workbenchOptionalServiceLegacyModulesStatusPresent
	}

	for _, definition := range workbenchOptionalServiceDefinitions {
		composeMatches := workbenchMatchOptionalServiceCompose(snapshot.Services, definition)
		legacyMatches := workbenchMatchOptionalServiceLegacyModules(legacyRecords, definition)
		managedMatches := workbenchMatchOptionalManagedServices(snapshot.ManagedServices, definition)
		transitionNotes := []string{
			"Catalog add/remove mutations are available against the stored snapshot.",
			"Compose preview/apply now renders catalog-managed services from frozen backend catalog definitions.",
			"Port resolution now includes baseline container-port planning for catalog-managed services.",
		}
		currentState := workbenchOptionalServiceCurrentStateUnmanaged
		legacyModuleType := ""
		legacyMutationPath := ""
		if len(managedMatches) > 0 {
			currentState = workbenchOptionalServiceCurrentStateCatalogManaged
			transitionNotes = append(transitionNotes,
				"Catalog-managed records are tracked separately from imported compose services and legacy module annotations.",
			)
		}
		if definition.legacyModuleType != "" {
			legacyModuleType = definition.legacyModuleType
			legacyMutationPath = legacyPath
			if currentState == workbenchOptionalServiceCurrentStateUnmanaged {
				currentState = workbenchOptionalServiceCurrentStateLegacyModules
			}
			transitionNotes = append(transitionNotes,
				"Legacy Redis module annotations remain separate metadata on existing services; they are not silently migrated into catalog-managed records.",
			)
			if len(managedMatches) > 0 && len(legacyMatches) > 0 {
				transitionNotes = append(transitionNotes,
					"Legacy module records and catalog-managed services can coexist temporarily during the transition.",
				)
			}
		}

		catalog.Entries = append(catalog.Entries, WorkbenchOptionalServiceCatalogEntry{
			Key:                  definition.key,
			DisplayName:          definition.displayName,
			Description:          definition.description,
			Category:             definition.category,
			DefaultServiceName:   definition.defaultServiceName,
			SuggestedImage:       definition.suggestedImage,
			DefaultContainerPort: definition.defaultContainerPort,
			Availability: WorkbenchOptionalServiceAvailability{
				Status:          workbenchOptionalServiceAvailabilityStatus(composeMatches, legacyMatches, managedMatches),
				ComposeServices: composeMatches,
				LegacyModules:   legacyMatches,
				ManagedServices: managedMatches,
			},
			Transition: WorkbenchOptionalServiceTransition{
				ReadOnly:               false,
				MutationReady:          true,
				ComposeGenerationReady: true,
				CurrentState:           currentState,
				TargetState:            workbenchOptionalServiceTargetStateCatalogManaged,
				LegacyModuleType:       legacyModuleType,
				LegacyMutationPath:     legacyMutationPath,
				Notes:                  transitionNotes,
			},
		})
	}

	return catalog, nil
}

func workbenchOptionalServiceAvailabilityStatus(
	composeMatches []WorkbenchOptionalServiceComposeMatch,
	legacyMatches []WorkbenchStackModule,
	managedMatches []WorkbenchManagedService,
) string {
	switch {
	case len(managedMatches) > 0 && len(composeMatches) > 0 && len(legacyMatches) > 0:
		return workbenchOptionalServiceStatusCatalogManagedWithComposeAndLegacy
	case len(managedMatches) > 0 && len(composeMatches) > 0:
		return workbenchOptionalServiceStatusCatalogManagedWithCompose
	case len(managedMatches) > 0 && len(legacyMatches) > 0:
		return workbenchOptionalServiceStatusCatalogManagedWithLegacy
	case len(managedMatches) > 0:
		return workbenchOptionalServiceStatusCatalogManaged
	case len(composeMatches) > 0 && len(legacyMatches) > 0:
		return workbenchOptionalServiceStatusComposePresentWithLegacy
	case len(composeMatches) > 0:
		return workbenchOptionalServiceStatusComposePresent
	case len(legacyMatches) > 0:
		return workbenchOptionalServiceStatusLegacyModuleOnly
	default:
		return workbenchOptionalServiceStatusAvailable
	}
}

func workbenchNormalizeLegacyModuleRecords(modules []WorkbenchStackModule) []WorkbenchStackModule {
	if len(modules) == 0 {
		return []WorkbenchStackModule{}
	}
	records := make([]WorkbenchStackModule, 0, len(modules))
	for _, module := range modules {
		moduleType := strings.ToLower(strings.TrimSpace(module.ModuleType))
		serviceName := strings.TrimSpace(module.ServiceName)
		if moduleType == "" || serviceName == "" {
			continue
		}
		records = append(records, WorkbenchStackModule{
			ModuleType:  moduleType,
			ServiceName: serviceName,
		})
	}
	return records
}

func workbenchOptionalServiceDefinitionByKey(key string) (workbenchOptionalServiceDefinition, bool) {
	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	for _, definition := range workbenchOptionalServiceDefinitions {
		if definition.key == normalizedKey {
			return definition, true
		}
	}
	return workbenchOptionalServiceDefinition{}, false
}

func workbenchOptionalServiceDefinitionByServiceName(serviceName string) (workbenchOptionalServiceDefinition, bool) {
	normalizedServiceName := strings.ToLower(strings.TrimSpace(serviceName))
	for _, definition := range workbenchOptionalServiceDefinitions {
		if strings.ToLower(strings.TrimSpace(definition.defaultServiceName)) == normalizedServiceName {
			return definition, true
		}
	}
	return workbenchOptionalServiceDefinition{}, false
}

func workbenchMatchOptionalServiceCompose(
	services []WorkbenchComposeService,
	definition workbenchOptionalServiceDefinition,
) []WorkbenchOptionalServiceComposeMatch {
	matches := make([]WorkbenchOptionalServiceComposeMatch, 0)
	for _, service := range services {
		serviceName := strings.TrimSpace(service.ServiceName)
		image := strings.TrimSpace(service.Image)
		if serviceName == "" {
			continue
		}

		matchReason := ""
		if workbenchOptionalServiceMatchesHint(serviceName, definition.serviceNameHints) {
			matchReason = workbenchOptionalServiceMatchReasonServiceName
		} else if workbenchOptionalServiceMatchesHint(workbenchOptionalServiceImageName(image), definition.imageNameHints) {
			matchReason = workbenchOptionalServiceMatchReasonImageRepository
		}
		if matchReason == "" {
			continue
		}

		matches = append(matches, WorkbenchOptionalServiceComposeMatch{
			ServiceName: serviceName,
			Image:       image,
			MatchReason: matchReason,
		})
	}
	return matches
}

func workbenchMatchOptionalServiceLegacyModules(
	modules []WorkbenchStackModule,
	definition workbenchOptionalServiceDefinition,
) []WorkbenchStackModule {
	if definition.legacyModuleType == "" {
		return []WorkbenchStackModule{}
	}

	matches := make([]WorkbenchStackModule, 0)
	for _, module := range modules {
		if !strings.EqualFold(strings.TrimSpace(module.ModuleType), definition.legacyModuleType) {
			continue
		}
		matches = append(matches, module)
	}
	return matches
}

func workbenchMatchOptionalManagedServices(
	services []WorkbenchManagedService,
	definition workbenchOptionalServiceDefinition,
) []WorkbenchManagedService {
	matches := make([]WorkbenchManagedService, 0)
	for _, managedService := range services {
		if !strings.EqualFold(strings.TrimSpace(managedService.EntryKey), definition.key) {
			continue
		}
		matches = append(matches, WorkbenchManagedService{
			EntryKey:    strings.ToLower(strings.TrimSpace(managedService.EntryKey)),
			ServiceName: strings.TrimSpace(managedService.ServiceName),
		})
	}
	return matches
}

func workbenchOptionalServiceMatchesHint(value string, hints []string) bool {
	normalizedValue := strings.ToLower(strings.TrimSpace(value))
	if normalizedValue == "" {
		return false
	}
	for _, hint := range hints {
		if normalizedValue == strings.ToLower(strings.TrimSpace(hint)) {
			return true
		}
	}
	return false
}

func workbenchOptionalServiceImageName(image string) string {
	normalized := strings.TrimSpace(strings.ToLower(image))
	if normalized == "" {
		return ""
	}
	if idx := strings.Index(normalized, "@"); idx >= 0 {
		normalized = normalized[:idx]
	}
	lastSlash := strings.LastIndex(normalized, "/")
	lastColon := strings.LastIndex(normalized, ":")
	if lastColon > lastSlash {
		normalized = normalized[:lastColon]
	}
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		return normalized[idx+1:]
	}
	return normalized
}
