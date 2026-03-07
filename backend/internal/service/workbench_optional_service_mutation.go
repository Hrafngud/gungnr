package service

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"go-notes/internal/errs"
)

const (
	workbenchOptionalServiceMutationActionAdd    = "add"
	workbenchOptionalServiceMutationActionRemove = "remove"
)

type WorkbenchOptionalServiceAddRequest struct {
	EntryKey string `json:"entryKey"`
}

type WorkbenchOptionalServiceMutationSummary struct {
	Changed                bool     `json:"changed"`
	Action                 string   `json:"action"`
	EntryKey               string   `json:"entryKey,omitempty"`
	ServiceName            string   `json:"serviceName,omitempty"`
	PreviousCount          int      `json:"previousCount"`
	CurrentCount           int      `json:"currentCount"`
	ComposeGenerationReady bool     `json:"composeGenerationReady"`
	Notes                  []string `json:"notes,omitempty"`
}

func (s *WorkbenchService) AddOptionalService(
	ctx context.Context,
	projectName string,
	input WorkbenchOptionalServiceAddRequest,
) (WorkbenchStackSnapshot, WorkbenchOptionalServiceMutationSummary, error) {
	return s.mutateOptionalService(ctx, projectName, workbenchOptionalServiceMutationActionAdd, strings.TrimSpace(input.EntryKey))
}

func (s *WorkbenchService) RemoveOptionalService(
	ctx context.Context,
	projectName string,
	serviceName string,
) (WorkbenchStackSnapshot, WorkbenchOptionalServiceMutationSummary, error) {
	return s.mutateOptionalService(ctx, projectName, workbenchOptionalServiceMutationActionRemove, strings.TrimSpace(serviceName))
}

func (s *WorkbenchService) mutateOptionalService(
	ctx context.Context,
	projectName string,
	action string,
	target string,
) (WorkbenchStackSnapshot, WorkbenchOptionalServiceMutationSummary, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchOptionalServiceMutationSummary{}, err
	}

	normalizedAction := strings.ToLower(strings.TrimSpace(action))
	summary, issues := normalizeWorkbenchOptionalServiceMutation(normalizedAction, target)
	if len(issues) > 0 {
		return WorkbenchStackSnapshot{}, summary, workbenchOptionalServiceMutationValidationError(WorkbenchStackSnapshot{}, summary, issues)
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchOptionalServiceMutationSummary{}, err
	}
	defer release()

	snapshot, exists, err := s.loadStoredWorkbenchSnapshot(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchOptionalServiceMutationSummary{}, err
	}
	if !exists {
		return WorkbenchStackSnapshot{}, WorkbenchOptionalServiceMutationSummary{}, errs.WithDetails(
			errs.New(errs.CodeWorkbenchSourceNotFound, fmt.Sprintf("workbench snapshot not found for project %q", normalizedProject)),
			map[string]any{
				"project": normalizedProject,
			},
		)
	}

	mutated, mutationSummary, mutationIssues := mutateWorkbenchOptionalService(snapshot, summary)
	if len(mutationIssues) > 0 {
		return mutated, mutationSummary, workbenchOptionalServiceMutationValidationError(mutated, mutationSummary, mutationIssues)
	}
	if !mutationSummary.Changed {
		return mutated, mutationSummary, nil
	}

	if mutated.Revision <= 0 {
		mutated.Revision = 1
	}
	mutated.Revision++
	if err := s.saveWorkbenchSnapshot(ctx, normalizedProject, mutated); err != nil {
		return mutated, mutationSummary, err
	}
	return mutated, mutationSummary, nil
}

func normalizeWorkbenchOptionalServiceMutation(
	action string,
	target string,
) (WorkbenchOptionalServiceMutationSummary, []WorkbenchMutationIssue) {
	summary := WorkbenchOptionalServiceMutationSummary{
		Action:                 action,
		ComposeGenerationReady: true,
		Notes: []string{
			"Compose preview/apply now renders catalog-managed services from frozen backend catalog definitions.",
			"Port resolution now includes baseline container-port planning for catalog-managed services.",
		},
	}
	issues := []WorkbenchMutationIssue{}

	switch action {
	case workbenchOptionalServiceMutationActionAdd:
		summary.EntryKey = strings.ToLower(strings.TrimSpace(target))
		if summary.EntryKey == "" {
			issues = append(issues, WorkbenchMutationIssue{
				Class:    workbenchMutationIssueClassSchema,
				Code:     "WB-OPTIONAL-SERVICE-KEY-REQUIRED",
				Path:     "$.entryKey",
				Message:  "entryKey is required",
				EntryKey: summary.EntryKey,
				Action:   action,
			})
			break
		}
		if definition, ok := workbenchOptionalServiceDefinitionByKey(summary.EntryKey); ok {
			summary.ServiceName = definition.defaultServiceName
		}
	case workbenchOptionalServiceMutationActionRemove:
		summary.ServiceName = strings.TrimSpace(target)
		if summary.ServiceName == "" {
			issues = append(issues, WorkbenchMutationIssue{
				Class:   workbenchMutationIssueClassSchema,
				Code:    "WB-OPTIONAL-SERVICE-SERVICE-REQUIRED",
				Path:    "$.serviceName",
				Message: "serviceName is required",
				Service: summary.ServiceName,
				Action:  action,
			})
		}
	default:
		issues = append(issues, WorkbenchMutationIssue{
			Class:   workbenchMutationIssueClassSchema,
			Code:    "WB-OPTIONAL-SERVICE-ACTION-INVALID",
			Path:    "$.action",
			Message: fmt.Sprintf("invalid action %q; expected %q or %q", action, workbenchOptionalServiceMutationActionAdd, workbenchOptionalServiceMutationActionRemove),
			Action:  action,
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchMutationIssueLess(issues[i], issues[j])
		})
	}
	return summary, issues
}

func mutateWorkbenchOptionalService(
	snapshot WorkbenchStackSnapshot,
	summary WorkbenchOptionalServiceMutationSummary,
) (WorkbenchStackSnapshot, WorkbenchOptionalServiceMutationSummary, []WorkbenchMutationIssue) {
	normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
	beforeManagedServices := append([]WorkbenchManagedService(nil), normalizedSnapshot.ManagedServices...)
	issues := []WorkbenchMutationIssue{}
	next := normalizedSnapshot

	switch summary.Action {
	case workbenchOptionalServiceMutationActionAdd:
		definition, ok := workbenchOptionalServiceDefinitionByKey(summary.EntryKey)
		if !ok {
			issues = append(issues, WorkbenchMutationIssue{
				Class:    workbenchMutationIssueClassSchema,
				Code:     "WB-OPTIONAL-SERVICE-KEY-UNSUPPORTED",
				Path:     "$.entryKey",
				Message:  fmt.Sprintf("optional service entry %q is not supported", summary.EntryKey),
				EntryKey: summary.EntryKey,
				Action:   summary.Action,
			})
			break
		}

		summary.EntryKey = definition.key
		summary.ServiceName = definition.defaultServiceName
		summary.PreviousCount = workbenchCountManagedOptionalServicesByEntryKey(normalizedSnapshot.ManagedServices, definition.key)
		if summary.PreviousCount > 0 {
			issues = append(issues, WorkbenchMutationIssue{
				Class:    workbenchMutationIssueClassConflict,
				Code:     "WB-OPTIONAL-SERVICE-DUPLICATE",
				Path:     "$.entryKey",
				Message:  fmt.Sprintf("optional service %q is already managed in the stored snapshot", definition.key),
				EntryKey: definition.key,
				Service:  definition.defaultServiceName,
				Action:   summary.Action,
			})
			break
		}
		if workbenchSnapshotHasService(normalizedSnapshot.Services, definition.defaultServiceName) ||
			workbenchFindManagedOptionalServiceByServiceName(normalizedSnapshot.ManagedServices, definition.defaultServiceName) >= 0 {
			issues = append(issues, WorkbenchMutationIssue{
				Class:    workbenchMutationIssueClassConflict,
				Code:     "WB-OPTIONAL-SERVICE-SERVICE-NAME-CONFLICT",
				Path:     "$.entryKey",
				Message:  fmt.Sprintf("service name %q is already present in the stored snapshot", definition.defaultServiceName),
				EntryKey: definition.key,
				Service:  definition.defaultServiceName,
				Action:   summary.Action,
			})
			break
		}

		next.ManagedServices = append(next.ManagedServices, WorkbenchManagedService{
			EntryKey:    definition.key,
			ServiceName: definition.defaultServiceName,
		})
	case workbenchOptionalServiceMutationActionRemove:
		index := workbenchFindManagedOptionalServiceByServiceName(normalizedSnapshot.ManagedServices, summary.ServiceName)
		if index < 0 {
			code := "WB-OPTIONAL-SERVICE-NOT-FOUND"
			message := fmt.Sprintf("catalog-managed service %q is not present in the stored snapshot", summary.ServiceName)
			if workbenchSnapshotHasService(normalizedSnapshot.Services, summary.ServiceName) {
				code = "WB-OPTIONAL-SERVICE-COMPOSE-OWNED"
				message = fmt.Sprintf("service %q comes from imported compose state and is not catalog-managed", summary.ServiceName)
			}
			issues = append(issues, WorkbenchMutationIssue{
				Class:   workbenchMutationIssueClassSchema,
				Code:    code,
				Path:    "$.serviceName",
				Message: message,
				Service: summary.ServiceName,
				Action:  summary.Action,
			})
			break
		}

		target := normalizedSnapshot.ManagedServices[index]
		summary.EntryKey = target.EntryKey
		summary.ServiceName = target.ServiceName
		summary.PreviousCount = workbenchCountManagedOptionalServicesByServiceName(normalizedSnapshot.ManagedServices, target.ServiceName)

		filtered := make([]WorkbenchManagedService, 0, len(next.ManagedServices))
		for _, managedService := range next.ManagedServices {
			if strings.EqualFold(strings.TrimSpace(managedService.ServiceName), target.ServiceName) {
				continue
			}
			filtered = append(filtered, managedService)
		}
		next.ManagedServices = filtered
		next.Ports = workbenchFilterPortsByServiceName(next.Ports, target.ServiceName)
		next.Resources = workbenchFilterResourcesByServiceName(next.Resources, target.ServiceName)
		next.Dependencies = workbenchFilterDependenciesByServiceName(next.Dependencies, target.ServiceName)
		next.NetworkRefs = workbenchFilterNetworkRefsByServiceName(next.NetworkRefs, target.ServiceName)
		next.VolumeRefs = workbenchFilterVolumeRefsByServiceName(next.VolumeRefs, target.ServiceName)
		next.EnvRefs = workbenchFilterEnvRefsByServiceName(next.EnvRefs, target.ServiceName)
	default:
		issues = append(issues, WorkbenchMutationIssue{
			Class:   workbenchMutationIssueClassSchema,
			Code:    "WB-OPTIONAL-SERVICE-ACTION-INVALID",
			Path:    "$.action",
			Message: fmt.Sprintf("invalid action %q", summary.Action),
			Action:  summary.Action,
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchMutationIssueLess(issues[i], issues[j])
		})
		return normalizedSnapshot, summary, issues
	}

	next = normalizeWorkbenchStackSnapshot(next)
	summary.CurrentCount = workbenchCountManagedOptionalServicesByServiceName(next.ManagedServices, summary.ServiceName)
	summary.Changed = !reflect.DeepEqual(beforeManagedServices, next.ManagedServices)
	return next, summary, nil
}

func workbenchFilterPortsByServiceName(ports []WorkbenchComposePort, serviceName string) []WorkbenchComposePort {
	filtered := make([]WorkbenchComposePort, 0, len(ports))
	for _, port := range ports {
		if strings.EqualFold(strings.TrimSpace(port.ServiceName), strings.TrimSpace(serviceName)) {
			continue
		}
		filtered = append(filtered, port)
	}
	return filtered
}

func workbenchFilterResourcesByServiceName(resources []WorkbenchComposeResource, serviceName string) []WorkbenchComposeResource {
	filtered := make([]WorkbenchComposeResource, 0, len(resources))
	for _, resource := range resources {
		if strings.EqualFold(strings.TrimSpace(resource.ServiceName), strings.TrimSpace(serviceName)) {
			continue
		}
		filtered = append(filtered, resource)
	}
	return filtered
}

func workbenchFilterDependenciesByServiceName(dependencies []WorkbenchComposeDependency, serviceName string) []WorkbenchComposeDependency {
	filtered := make([]WorkbenchComposeDependency, 0, len(dependencies))
	for _, dependency := range dependencies {
		if strings.EqualFold(strings.TrimSpace(dependency.ServiceName), strings.TrimSpace(serviceName)) {
			continue
		}
		filtered = append(filtered, dependency)
	}
	return filtered
}

func workbenchFilterNetworkRefsByServiceName(networkRefs []WorkbenchComposeNetworkRef, serviceName string) []WorkbenchComposeNetworkRef {
	filtered := make([]WorkbenchComposeNetworkRef, 0, len(networkRefs))
	for _, networkRef := range networkRefs {
		if strings.EqualFold(strings.TrimSpace(networkRef.ServiceName), strings.TrimSpace(serviceName)) {
			continue
		}
		filtered = append(filtered, networkRef)
	}
	return filtered
}

func workbenchFilterVolumeRefsByServiceName(volumeRefs []WorkbenchComposeVolumeRef, serviceName string) []WorkbenchComposeVolumeRef {
	filtered := make([]WorkbenchComposeVolumeRef, 0, len(volumeRefs))
	for _, volumeRef := range volumeRefs {
		if strings.EqualFold(strings.TrimSpace(volumeRef.ServiceName), strings.TrimSpace(serviceName)) {
			continue
		}
		filtered = append(filtered, volumeRef)
	}
	return filtered
}

func workbenchFilterEnvRefsByServiceName(envRefs []WorkbenchComposeEnvRef, serviceName string) []WorkbenchComposeEnvRef {
	filtered := make([]WorkbenchComposeEnvRef, 0, len(envRefs))
	for _, envRef := range envRefs {
		if strings.EqualFold(strings.TrimSpace(envRef.ServiceName), strings.TrimSpace(serviceName)) {
			continue
		}
		filtered = append(filtered, envRef)
	}
	return filtered
}

func workbenchCountManagedOptionalServicesByEntryKey(services []WorkbenchManagedService, entryKey string) int {
	count := 0
	for _, managedService := range services {
		if strings.EqualFold(strings.TrimSpace(managedService.EntryKey), strings.TrimSpace(entryKey)) {
			count++
		}
	}
	return count
}

func workbenchCountManagedOptionalServicesByServiceName(services []WorkbenchManagedService, serviceName string) int {
	count := 0
	for _, managedService := range services {
		if strings.EqualFold(strings.TrimSpace(managedService.ServiceName), strings.TrimSpace(serviceName)) {
			count++
		}
	}
	return count
}

func workbenchFindManagedOptionalServiceByServiceName(services []WorkbenchManagedService, serviceName string) int {
	target := strings.TrimSpace(serviceName)
	if target == "" {
		return -1
	}
	for idx := range services {
		if strings.EqualFold(strings.TrimSpace(services[idx].ServiceName), target) {
			return idx
		}
	}
	return -1
}

func workbenchOptionalServiceMutationValidationError(
	snapshot WorkbenchStackSnapshot,
	summary WorkbenchOptionalServiceMutationSummary,
	issues []WorkbenchMutationIssue,
) error {
	normalizedIssues := append([]WorkbenchMutationIssue(nil), issues...)
	sort.SliceStable(normalizedIssues, func(i, j int) bool {
		return workbenchMutationIssueLess(normalizedIssues[i], normalizedIssues[j])
	})
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchValidationFailed, "invalid workbench optional-service mutation"),
		map[string]any{
			"project":           strings.TrimSpace(snapshot.ProjectName),
			"composePath":       strings.TrimSpace(snapshot.ComposePath),
			"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
			"revision":          snapshot.Revision,
			"action":            strings.TrimSpace(summary.Action),
			"entryKey":          strings.TrimSpace(summary.EntryKey),
			"serviceName":       strings.TrimSpace(summary.ServiceName),
			"issueCount":        len(normalizedIssues),
			"issues":            normalizedIssues,
			"summary":           summary,
		},
	)
}
