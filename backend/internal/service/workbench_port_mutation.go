package service

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"go-notes/internal/errs"
)

const (
	workbenchPortMutationActionSetManual   = "set_manual"
	workbenchPortMutationActionClearManual = "clear_manual"

	workbenchPortSuggestionDefaultLimit = 10
	workbenchPortSuggestionMaxLimit     = 100
)

type WorkbenchPortSelector struct {
	ServiceName   string `json:"serviceName"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
	HostIP        string `json:"hostIp,omitempty"`
}

type WorkbenchPortMutationRequest struct {
	Selector       WorkbenchPortSelector `json:"selector"`
	Action         string                `json:"action"`
	ManualHostPort *int                  `json:"manualHostPort,omitempty"`
}

type WorkbenchPortMutationSummary struct {
	Changed           bool                  `json:"changed"`
	Action            string                `json:"action"`
	Selector          WorkbenchPortSelector `json:"selector"`
	Source            string                `json:"source,omitempty"`
	Status            string                `json:"status,omitempty"`
	Message           string                `json:"message,omitempty"`
	PreviousStrategy  string                `json:"previousStrategy,omitempty"`
	CurrentStrategy   string                `json:"currentStrategy,omitempty"`
	PreviousHostPort  *int                  `json:"previousHostPort,omitempty"`
	RequestedHostPort *int                  `json:"requestedHostPort,omitempty"`
	PreferredHostPort *int                  `json:"preferredHostPort,omitempty"`
	AssignedHostPort  *int                  `json:"assignedHostPort,omitempty"`
	Attempts          int                   `json:"attempts,omitempty"`
}

type WorkbenchPortSuggestionRequest struct {
	Selector WorkbenchPortSelector `json:"selector"`
	Limit    int                   `json:"limit,omitempty"`
}

type WorkbenchPortSuggestion struct {
	HostPort int `json:"hostPort"`
	Rank     int `json:"rank"`
}

type WorkbenchPortSuggestionSummary struct {
	Selector          WorkbenchPortSelector     `json:"selector"`
	Source            string                    `json:"source,omitempty"`
	PreferredHostPort *int                      `json:"preferredHostPort,omitempty"`
	CurrentHostPort   *int                      `json:"currentHostPort,omitempty"`
	CurrentStrategy   string                    `json:"currentStrategy,omitempty"`
	CurrentStatus     string                    `json:"currentStatus,omitempty"`
	Limit             int                       `json:"limit"`
	SuggestionCount   int                       `json:"suggestionCount"`
	Suggestions       []WorkbenchPortSuggestion `json:"suggestions"`
}

type workbenchHostPortScanner func(ctx context.Context) (map[int]struct{}, error)

func workbenchScanOccupiedHostPorts(ctx context.Context) (map[int]struct{}, error) {
	occupied := make(map[int]struct{})
	hostPorts, hostErr := listHostListeningPorts(ctx)
	for _, port := range hostPorts {
		occupied[port] = struct{}{}
	}
	dockerPorts, dockerErr := listDockerPublishedPorts(ctx)
	for _, port := range dockerPorts {
		occupied[port] = struct{}{}
	}

	if hostErr != nil && dockerErr != nil {
		return nil, fmt.Errorf("failed to inspect host listening ports")
	}
	return occupied, nil
}

func (s *WorkbenchService) MutateStoredSnapshotPort(
	ctx context.Context,
	projectName string,
	input WorkbenchPortMutationRequest,
) (WorkbenchStackSnapshot, WorkbenchPortMutationSummary, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchPortMutationSummary{}, err
	}

	normalizedInput, issues := normalizeWorkbenchPortMutationRequest(input)
	summary := WorkbenchPortMutationSummary{
		Action:            normalizedInput.Action,
		Selector:          normalizedInput.Selector,
		RequestedHostPort: cloneWorkbenchPortInt(normalizedInput.ManualHostPort),
	}
	if len(issues) > 0 {
		return WorkbenchStackSnapshot{}, summary, workbenchPortMutationValidationError(WorkbenchStackSnapshot{}, summary, issues)
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchPortMutationSummary{}, err
	}
	defer release()

	snapshot, exists, err := s.loadStoredWorkbenchSnapshot(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchPortMutationSummary{}, err
	}
	if !exists {
		return WorkbenchStackSnapshot{}, WorkbenchPortMutationSummary{}, errs.WithDetails(
			errs.New(errs.CodeWorkbenchSourceNotFound, fmt.Sprintf("workbench snapshot not found for project %q", normalizedProject)),
			map[string]any{
				"project": normalizedProject,
			},
		)
	}

	mutated, mutationSummary, mutationIssues := mutateWorkbenchSnapshotPort(snapshot, normalizedInput)
	if len(mutationIssues) > 0 {
		return mutated, mutationSummary, workbenchPortMutationValidationError(mutated, mutationSummary, mutationIssues)
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

func (s *WorkbenchService) SuggestStoredSnapshotHostPorts(
	ctx context.Context,
	projectName string,
	input WorkbenchPortSuggestionRequest,
) (WorkbenchStackSnapshot, WorkbenchPortSuggestionSummary, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchPortSuggestionSummary{}, err
	}

	normalizedInput, issues := normalizeWorkbenchPortSuggestionRequest(input)
	summary := WorkbenchPortSuggestionSummary{
		Selector:    normalizedInput.Selector,
		Limit:       normalizedInput.Limit,
		Suggestions: []WorkbenchPortSuggestion{},
	}
	if len(issues) > 0 {
		return WorkbenchStackSnapshot{}, summary, workbenchPortSuggestionValidationError(WorkbenchStackSnapshot{}, summary, issues)
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchPortSuggestionSummary{}, err
	}
	defer release()

	snapshot, exists, err := s.loadStoredWorkbenchSnapshot(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchPortSuggestionSummary{}, err
	}
	if !exists {
		return WorkbenchStackSnapshot{}, WorkbenchPortSuggestionSummary{}, errs.WithDetails(
			errs.New(errs.CodeWorkbenchSourceNotFound, fmt.Sprintf("workbench snapshot not found for project %q", normalizedProject)),
			map[string]any{
				"project": normalizedProject,
			},
		)
	}

	var occupiedHostPorts map[int]struct{}
	if s.hostPortScanner != nil {
		if scanned, scanErr := s.hostPortScanner(ctx); scanErr == nil {
			occupiedHostPorts = scanned
		}
	}

	resolved, suggestionSummary, suggestionIssues := suggestWorkbenchSnapshotPorts(snapshot, normalizedInput, occupiedHostPorts)
	if len(suggestionIssues) > 0 {
		return resolved, suggestionSummary, workbenchPortSuggestionValidationError(resolved, suggestionSummary, suggestionIssues)
	}
	return resolved, suggestionSummary, nil
}

func mutateWorkbenchSnapshotPort(
	snapshot WorkbenchStackSnapshot,
	input WorkbenchPortMutationRequest,
) (WorkbenchStackSnapshot, WorkbenchPortMutationSummary, []WorkbenchPortResolutionIssue) {
	normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
	beforePorts := append([]WorkbenchComposePort(nil), normalizedSnapshot.Ports...)

	summary := WorkbenchPortMutationSummary{
		Action:            input.Action,
		Selector:          input.Selector,
		RequestedHostPort: cloneWorkbenchPortInt(input.ManualHostPort),
	}

	targetIndex, issue := workbenchFindPortIndexBySelector(normalizedSnapshot.Ports, input.Selector, "$.selector", "WB-MUTATE")
	if issue != nil {
		return normalizedSnapshot, summary, []WorkbenchPortResolutionIssue{*issue}
	}

	target := normalizeWorkbenchComposePort(normalizedSnapshot.Ports[targetIndex])
	protocol := strings.ToLower(strings.TrimSpace(target.Protocol))
	if protocol == "" {
		protocol = "tcp"
	}
	hostIP := normalizeHostIP(strings.TrimSpace(target.HostIP))
	serviceName := strings.TrimSpace(target.ServiceName)

	summary.Source = workbenchPortSourceComposeHostPort
	summary.PreviousStrategy = strings.ToLower(strings.TrimSpace(target.AssignmentStrategy))
	if summary.PreviousStrategy == "" {
		summary.PreviousStrategy = workbenchPortStrategyAuto
	}
	summary.PreviousHostPort = cloneWorkbenchPortInt(target.HostPort)
	summary.CurrentStrategy = summary.PreviousStrategy

	reservedBindings := workbenchSnapshotReservedBindings(normalizedSnapshot, targetIndex)
	issues := []WorkbenchPortResolutionIssue{}

	switch input.Action {
	case workbenchPortMutationActionSetManual:
		requested := 0
		if input.ManualHostPort != nil {
			requested = *input.ManualHostPort
		}
		summary.RequestedHostPort = intPtr(requested)
		summary.PreferredHostPort = intPtr(requested)

		if workbenchHostPortConflicts(reservedBindings, protocol, hostIP, requested) {
			summary.Status = workbenchPortAllocationConflict
			summary.Message = fmt.Sprintf(
				"manual host port %d conflicts with an existing reservation for service %q",
				requested,
				serviceName,
			)
			summary.CurrentStrategy = workbenchPortStrategyManual
			issues = append(issues, WorkbenchPortResolutionIssue{
				Class:    workbenchPortIssueClassConflict,
				Code:     "WB-MUTATE-MANUAL-CONFLICT",
				Path:     "$.manualHostPort",
				Message:  summary.Message,
				Service:  serviceName,
				Protocol: protocol,
				HostIP:   hostIP,
				HostPort: strconv.Itoa(requested),
				Strategy: workbenchPortStrategyManual,
				Source:   workbenchPortSourceComposeHostPort,
			})
			break
		}

		target.HostPort = intPtr(requested)
		target.HostPortRaw = ""
		target.AssignmentStrategy = workbenchPortStrategyManual
		target.AllocationStatus = workbenchPortAllocationAssigned
		summary.Status = workbenchPortAllocationAssigned
		summary.AssignedHostPort = intPtr(requested)
		summary.CurrentStrategy = workbenchPortStrategyManual
		summary.Attempts = 1
	case workbenchPortMutationActionClearManual:
		candidate, candidateIssue := workbenchResolvePortCandidate(normalizedSnapshot, target, targetIndex)
		if candidateIssue != nil {
			summary.Status = workbenchPortAllocationUnavailable
			summary.Message = candidateIssue.Message
			issues = append(issues, workbenchPortResolutionIssueWithCodePrefix(*candidateIssue, "WB-MUTATE"))
			break
		}

		summary.Source = candidate.source
		summary.PreferredHostPort = intPtr(candidate.port)
		assigned, attempts, ok := workbenchFindAvailableHostPort(candidate.port, protocol, hostIP, reservedBindings)
		if !ok {
			summary.Status = workbenchPortAllocationUnavailable
			summary.Attempts = attempts
			summary.Message = fmt.Sprintf("no available host port from %d to 65535 for service %q", candidate.port, serviceName)
			issues = append(issues, WorkbenchPortResolutionIssue{
				Class:    workbenchPortIssueClassAllocation,
				Code:     "WB-MUTATE-PORT-UNAVAILABLE",
				Path:     candidate.path,
				Message:  summary.Message,
				Service:  serviceName,
				Protocol: protocol,
				HostIP:   hostIP,
				HostPort: strconv.Itoa(candidate.port),
				Strategy: workbenchPortStrategyAuto,
				Source:   candidate.source,
			})
			break
		}

		target.HostPort = intPtr(assigned)
		target.HostPortRaw = ""
		target.AssignmentStrategy = workbenchPortStrategyAuto
		target.AllocationStatus = workbenchPortAllocationAssigned
		summary.Status = workbenchPortAllocationAssigned
		summary.CurrentStrategy = workbenchPortStrategyAuto
		summary.AssignedHostPort = intPtr(assigned)
		summary.Attempts = attempts
	default:
		summary.Status = workbenchPortAllocationUnavailable
		summary.Message = fmt.Sprintf("invalid mutation action %q", input.Action)
		issues = append(issues, WorkbenchPortResolutionIssue{
			Class:   workbenchPortIssueClassSchema,
			Code:    "WB-MUTATE-ACTION-INVALID",
			Path:    "$.action",
			Message: summary.Message,
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchPortResolutionIssueLess(issues[i], issues[j])
		})
		return normalizedSnapshot, summary, issues
	}

	next := normalizedSnapshot
	next.Ports[targetIndex] = target
	next = normalizeWorkbenchStackSnapshot(next)

	summary.Changed = !reflect.DeepEqual(beforePorts, next.Ports)
	if summary.AssignedHostPort == nil {
		if selectedIndex, selectedIssue := workbenchFindPortIndexBySelector(next.Ports, input.Selector, "$.selector", "WB-MUTATE"); selectedIssue == nil {
			resolvedTarget := next.Ports[selectedIndex]
			summary.AssignedHostPort = cloneWorkbenchPortInt(resolvedTarget.HostPort)
			summary.CurrentStrategy = strings.ToLower(strings.TrimSpace(resolvedTarget.AssignmentStrategy))
			if summary.CurrentStrategy == "" {
				summary.CurrentStrategy = workbenchPortStrategyAuto
			}
			if summary.Status == "" {
				summary.Status = strings.ToLower(strings.TrimSpace(resolvedTarget.AllocationStatus))
			}
		}
	}
	return next, summary, nil
}

func suggestWorkbenchSnapshotPorts(
	snapshot WorkbenchStackSnapshot,
	input WorkbenchPortSuggestionRequest,
	occupiedHostPorts map[int]struct{},
) (WorkbenchStackSnapshot, WorkbenchPortSuggestionSummary, []WorkbenchPortResolutionIssue) {
	normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
	summary := WorkbenchPortSuggestionSummary{
		Selector:    input.Selector,
		Limit:       input.Limit,
		Suggestions: []WorkbenchPortSuggestion{},
	}

	targetIndex, issue := workbenchFindPortIndexBySelector(normalizedSnapshot.Ports, input.Selector, "$.selector", "WB-SUGGEST")
	if issue != nil {
		return normalizedSnapshot, summary, []WorkbenchPortResolutionIssue{*issue}
	}
	target := normalizeWorkbenchComposePort(normalizedSnapshot.Ports[targetIndex])
	summary.CurrentHostPort = cloneWorkbenchPortInt(target.HostPort)
	summary.CurrentStrategy = strings.ToLower(strings.TrimSpace(target.AssignmentStrategy))
	if summary.CurrentStrategy == "" {
		summary.CurrentStrategy = workbenchPortStrategyAuto
	}
	summary.CurrentStatus = strings.ToLower(strings.TrimSpace(target.AllocationStatus))

	candidate, candidateIssue := workbenchResolveSuggestionPortCandidate(normalizedSnapshot, target, targetIndex)
	if candidateIssue != nil {
		return normalizedSnapshot, summary, []WorkbenchPortResolutionIssue{
			workbenchPortResolutionIssueWithCodePrefix(*candidateIssue, "WB-SUGGEST"),
		}
	}

	summary.Source = candidate.source
	summary.PreferredHostPort = intPtr(candidate.port)

	reservedBindings := workbenchSnapshotReservedBindings(normalizedSnapshot, targetIndex)
	suggestions := make([]WorkbenchPortSuggestion, 0, input.Limit)
	rank := 1
	for candidatePort := candidate.port; candidatePort <= 65535 && len(suggestions) < input.Limit; candidatePort++ {
		if _, occupied := occupiedHostPorts[candidatePort]; occupied {
			continue
		}
		if workbenchHostPortConflicts(reservedBindings, target.Protocol, target.HostIP, candidatePort) {
			continue
		}
		suggestions = append(suggestions, WorkbenchPortSuggestion{
			HostPort: candidatePort,
			Rank:     rank,
		})
		rank++
	}

	summary.Suggestions = suggestions
	summary.SuggestionCount = len(suggestions)
	return normalizedSnapshot, summary, nil
}

func workbenchResolveSuggestionPortCandidate(
	snapshot WorkbenchStackSnapshot,
	port WorkbenchComposePort,
	index int,
) (workbenchPortResolveCandidate, *WorkbenchPortResolutionIssue) {
	path := fmt.Sprintf("$.ports[%d]", index)
	serviceName := strings.TrimSpace(port.ServiceName)
	strategy := strings.ToLower(strings.TrimSpace(port.AssignmentStrategy))
	if strategy != workbenchPortStrategyManual {
		strategy = workbenchPortStrategyAuto
	}
	protocol := strings.ToLower(strings.TrimSpace(port.Protocol))
	if protocol == "" {
		protocol = "tcp"
	}
	hostIP := normalizeHostIP(strings.TrimSpace(port.HostIP))

	if profilePort, ok := workbenchResolveServiceProfileBaselinePort(snapshot, serviceName); ok {
		return workbenchPortResolveCandidate{
			port:   profilePort,
			source: workbenchPortSourceServiceProfile,
			path:   path + ".serviceName",
		}, nil
	}

	if moduleDefault, ok := workbenchResolveModuleDefaultPort(snapshot.Modules, serviceName); ok {
		return workbenchPortResolveCandidate{
			port:   moduleDefault,
			source: workbenchPortSourceModuleDefault,
			path:   path + ".hostPort",
		}, nil
	}

	if port.ContainerPort < 1 || port.ContainerPort > 65535 {
		return workbenchPortResolveCandidate{}, &WorkbenchPortResolutionIssue{
			Class:    workbenchPortIssueClassSchema,
			Code:     "WB-RESOLVE-PORT-CONTAINER-RANGE",
			Path:     path + ".containerPort",
			Message:  fmt.Sprintf("service %q has invalid containerPort %d", serviceName, port.ContainerPort),
			Service:  serviceName,
			Protocol: protocol,
			HostIP:   hostIP,
			HostPort: strconv.Itoa(port.ContainerPort),
			Strategy: strategy,
			Source:   workbenchPortSourceContainerPort,
		}
	}

	return workbenchPortResolveCandidate{
		port:   port.ContainerPort,
		source: workbenchPortSourceContainerPort,
		path:   path + ".containerPort",
	}, nil
}

func workbenchResolveServiceProfileBaselinePort(snapshot WorkbenchStackSnapshot, serviceName string) (int, bool) {
	image := workbenchServiceImageForName(snapshot.Services, serviceName)
	if port, ok := workbenchServiceProfilePortFromImage(image); ok {
		return port, true
	}
	return workbenchServiceProfilePortFromName(serviceName)
}

func workbenchServiceImageForName(services []WorkbenchComposeService, serviceName string) string {
	normalizedName := strings.TrimSpace(serviceName)
	for _, service := range services {
		if strings.EqualFold(strings.TrimSpace(service.ServiceName), normalizedName) {
			return strings.ToLower(strings.TrimSpace(service.Image))
		}
	}
	return ""
}

func workbenchServiceProfilePortFromImage(image string) (int, bool) {
	normalizedImage := strings.ToLower(strings.TrimSpace(image))
	if normalizedImage == "" {
		return 0, false
	}

	candidates := []struct {
		keyword string
		port    int
	}{
		{keyword: "postgres", port: 5432},
		{keyword: "mysql", port: 3306},
		{keyword: "mariadb", port: 3306},
		{keyword: "redis", port: 6379},
		{keyword: "mongo", port: 27017},
		{keyword: "nginx", port: 80},
		{keyword: "traefik", port: 80},
		{keyword: "caddy", port: 80},
		{keyword: "httpd", port: 80},
		{keyword: "haproxy", port: 80},
		{keyword: "prometheus", port: 9090},
		{keyword: "minio", port: 9000},
	}

	for _, candidate := range candidates {
		if strings.Contains(normalizedImage, candidate.keyword) {
			return candidate.port, true
		}
	}
	return 0, false
}

func workbenchServiceProfilePortFromName(serviceName string) (int, bool) {
	normalizedName := strings.ToLower(strings.TrimSpace(serviceName))
	if normalizedName == "" {
		return 0, false
	}

	switch normalizedName {
	case "proxy", "ingress", "gateway", "edge", "web", "frontend", "nginx":
		return 80, true
	case "postgres", "postgresql", "db", "database":
		return 5432, true
	case "mysql", "mariadb":
		return 3306, true
	case "redis", "cache":
		return 6379, true
	case "mongo", "mongodb":
		return 27017, true
	case "prometheus", "metrics":
		return 9090, true
	case "minio", "storage", "s3":
		return 9000, true
	default:
		return 0, false
	}
}

func normalizeWorkbenchPortMutationRequest(input WorkbenchPortMutationRequest) (WorkbenchPortMutationRequest, []WorkbenchPortResolutionIssue) {
	normalized := input
	issues := []WorkbenchPortResolutionIssue{}

	normalizedSelector, selectorIssue := normalizeWorkbenchPortSelector(input.Selector, "$.selector", "WB-MUTATE")
	normalized.Selector = normalizedSelector
	if selectorIssue != nil {
		issues = append(issues, *selectorIssue)
	}

	normalized.Action = strings.ToLower(strings.TrimSpace(input.Action))
	switch normalized.Action {
	case workbenchPortMutationActionSetManual:
		if input.ManualHostPort == nil {
			issues = append(issues, WorkbenchPortResolutionIssue{
				Class:   workbenchPortIssueClassSchema,
				Code:    "WB-MUTATE-MANUAL-REQUIRED",
				Path:    "$.manualHostPort",
				Message: "manualHostPort is required when action is set_manual",
			})
		} else if *input.ManualHostPort < 1 || *input.ManualHostPort > 65535 {
			issues = append(issues, WorkbenchPortResolutionIssue{
				Class:    workbenchPortIssueClassSchema,
				Code:     "WB-MUTATE-MANUAL-RANGE",
				Path:     "$.manualHostPort",
				Message:  fmt.Sprintf("manualHostPort %d is out of range (1-65535)", *input.ManualHostPort),
				HostPort: strconv.Itoa(*input.ManualHostPort),
			})
		}
	case workbenchPortMutationActionClearManual:
		if input.ManualHostPort != nil {
			issues = append(issues, WorkbenchPortResolutionIssue{
				Class:    workbenchPortIssueClassSchema,
				Code:     "WB-MUTATE-MANUAL-UNEXPECTED",
				Path:     "$.manualHostPort",
				Message:  "manualHostPort must be omitted when action is clear_manual",
				HostPort: strconv.Itoa(*input.ManualHostPort),
			})
		}
		normalized.ManualHostPort = nil
	default:
		issues = append(issues, WorkbenchPortResolutionIssue{
			Class:   workbenchPortIssueClassSchema,
			Code:    "WB-MUTATE-ACTION-INVALID",
			Path:    "$.action",
			Message: fmt.Sprintf("invalid action %q; expected %q or %q", input.Action, workbenchPortMutationActionSetManual, workbenchPortMutationActionClearManual),
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchPortResolutionIssueLess(issues[i], issues[j])
		})
	}
	return normalized, issues
}

func normalizeWorkbenchPortSuggestionRequest(input WorkbenchPortSuggestionRequest) (WorkbenchPortSuggestionRequest, []WorkbenchPortResolutionIssue) {
	normalized := input
	issues := []WorkbenchPortResolutionIssue{}

	normalizedSelector, selectorIssue := normalizeWorkbenchPortSelector(input.Selector, "$.selector", "WB-SUGGEST")
	normalized.Selector = normalizedSelector
	if selectorIssue != nil {
		issues = append(issues, *selectorIssue)
	}

	normalized.Limit = input.Limit
	switch {
	case normalized.Limit == 0:
		normalized.Limit = workbenchPortSuggestionDefaultLimit
	case normalized.Limit < 0 || normalized.Limit > workbenchPortSuggestionMaxLimit:
		issues = append(issues, WorkbenchPortResolutionIssue{
			Class:    workbenchPortIssueClassSchema,
			Code:     "WB-SUGGEST-LIMIT-RANGE",
			Path:     "$.limit",
			Message:  fmt.Sprintf("limit %d is out of range (1-%d)", input.Limit, workbenchPortSuggestionMaxLimit),
			HostPort: strconv.Itoa(input.Limit),
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchPortResolutionIssueLess(issues[i], issues[j])
		})
	}
	return normalized, issues
}

func normalizeWorkbenchPortSelector(
	selector WorkbenchPortSelector,
	pathPrefix string,
	codePrefix string,
) (WorkbenchPortSelector, *WorkbenchPortResolutionIssue) {
	normalized := selector
	normalized.ServiceName = strings.TrimSpace(selector.ServiceName)
	normalized.ContainerPort = selector.ContainerPort
	normalized.Protocol = strings.ToLower(strings.TrimSpace(selector.Protocol))
	if normalized.Protocol == "" {
		normalized.Protocol = "tcp"
	}
	normalized.HostIP = normalizeHostIP(strings.TrimSpace(selector.HostIP))

	if normalized.ServiceName == "" {
		return normalized, &WorkbenchPortResolutionIssue{
			Class:   workbenchPortIssueClassSchema,
			Code:    codePrefix + "-SELECTOR-SERVICE-REQUIRED",
			Path:    pathPrefix + ".serviceName",
			Message: "selector.serviceName is required",
		}
	}
	if normalized.ContainerPort < 1 || normalized.ContainerPort > 65535 {
		return normalized, &WorkbenchPortResolutionIssue{
			Class:    workbenchPortIssueClassSchema,
			Code:     codePrefix + "-SELECTOR-CONTAINER-RANGE",
			Path:     pathPrefix + ".containerPort",
			Message:  fmt.Sprintf("selector.containerPort %d is out of range (1-65535)", normalized.ContainerPort),
			Service:  normalized.ServiceName,
			Protocol: normalized.Protocol,
			HostIP:   normalized.HostIP,
			HostPort: strconv.Itoa(normalized.ContainerPort),
		}
	}
	return normalized, nil
}

func workbenchFindPortIndexBySelector(
	ports []WorkbenchComposePort,
	selector WorkbenchPortSelector,
	pathPrefix string,
	codePrefix string,
) (int, *WorkbenchPortResolutionIssue) {
	matches := make([]int, 0, 2)
	for idx := range ports {
		if workbenchPortMatchesSelector(ports[idx], selector) {
			matches = append(matches, idx)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return -1, &WorkbenchPortResolutionIssue{
			Class:    workbenchPortIssueClassSchema,
			Code:     codePrefix + "-SELECTOR-NOT-FOUND",
			Path:     pathPrefix,
			Message:  fmt.Sprintf("selector did not match any stored port for service %q", selector.ServiceName),
			Service:  selector.ServiceName,
			Protocol: selector.Protocol,
			HostIP:   selector.HostIP,
			HostPort: strconv.Itoa(selector.ContainerPort),
		}
	}
	return -1, &WorkbenchPortResolutionIssue{
		Class:    workbenchPortIssueClassSchema,
		Code:     codePrefix + "-SELECTOR-AMBIGUOUS",
		Path:     pathPrefix,
		Message:  fmt.Sprintf("selector matched %d stored ports for service %q", len(matches), selector.ServiceName),
		Service:  selector.ServiceName,
		Protocol: selector.Protocol,
		HostIP:   selector.HostIP,
		HostPort: strconv.Itoa(selector.ContainerPort),
	}
}

func workbenchPortMatchesSelector(port WorkbenchComposePort, selector WorkbenchPortSelector) bool {
	normalized := normalizeWorkbenchComposePort(port)
	portProtocol := strings.ToLower(strings.TrimSpace(normalized.Protocol))
	if portProtocol == "" {
		portProtocol = "tcp"
	}
	if !strings.EqualFold(strings.TrimSpace(normalized.ServiceName), strings.TrimSpace(selector.ServiceName)) {
		return false
	}
	if normalized.ContainerPort != selector.ContainerPort {
		return false
	}
	if portProtocol != selector.Protocol {
		return false
	}
	portHostIP := normalizeHostIP(strings.TrimSpace(normalized.HostIP))
	selectorHostIP := normalizeHostIP(strings.TrimSpace(selector.HostIP))
	if portHostIP == selectorHostIP {
		return true
	}
	return workbenchIsWildcardHostIP(portHostIP) && workbenchIsWildcardHostIP(selectorHostIP)
}

func workbenchSnapshotReservedBindings(snapshot WorkbenchStackSnapshot, skipIndex int) []workbenchHostBinding {
	reserved := make([]workbenchHostBinding, 0, len(snapshot.Ports))
	for idx, port := range snapshot.Ports {
		if idx == skipIndex {
			continue
		}
		if port.HostPort == nil || *port.HostPort < 1 || *port.HostPort > 65535 {
			continue
		}
		protocol := strings.ToLower(strings.TrimSpace(port.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		reserved = append(reserved, workbenchHostBinding{
			serviceName: strings.TrimSpace(port.ServiceName),
			hostIP:      normalizeHostIP(strings.TrimSpace(port.HostIP)),
			hostPort:    strconv.Itoa(*port.HostPort),
			protocol:    protocol,
		})
	}
	return reserved
}

func workbenchPortResolutionIssueWithCodePrefix(issue WorkbenchPortResolutionIssue, prefix string) WorkbenchPortResolutionIssue {
	normalized := issue
	trimmed := strings.TrimSpace(strings.ToUpper(normalized.Code))
	if trimmed != "" {
		switch {
		case strings.HasPrefix(trimmed, "WB-RESOLVE-"):
			normalized.Code = strings.Replace(trimmed, "WB-RESOLVE-", prefix+"-", 1)
		default:
			normalized.Code = prefix + "-" + trimmed
		}
	}
	return normalized
}

func workbenchPortMutationValidationError(
	snapshot WorkbenchStackSnapshot,
	summary WorkbenchPortMutationSummary,
	issues []WorkbenchPortResolutionIssue,
) error {
	normalizedIssues := append([]WorkbenchPortResolutionIssue(nil), issues...)
	sort.SliceStable(normalizedIssues, func(i, j int) bool {
		return workbenchPortResolutionIssueLess(normalizedIssues[i], normalizedIssues[j])
	})
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchValidationFailed, "invalid workbench port mutation"),
		map[string]any{
			"project":           strings.TrimSpace(snapshot.ProjectName),
			"composePath":       strings.TrimSpace(snapshot.ComposePath),
			"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
			"revision":          snapshot.Revision,
			"action":            strings.TrimSpace(summary.Action),
			"selector":          summary.Selector,
			"issueCount":        len(normalizedIssues),
			"issues":            normalizedIssues,
			"summary":           summary,
		},
	)
}

func workbenchPortSuggestionValidationError(
	snapshot WorkbenchStackSnapshot,
	summary WorkbenchPortSuggestionSummary,
	issues []WorkbenchPortResolutionIssue,
) error {
	normalizedIssues := append([]WorkbenchPortResolutionIssue(nil), issues...)
	sort.SliceStable(normalizedIssues, func(i, j int) bool {
		return workbenchPortResolutionIssueLess(normalizedIssues[i], normalizedIssues[j])
	})
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchValidationFailed, "invalid workbench port suggestion request"),
		map[string]any{
			"project":           strings.TrimSpace(snapshot.ProjectName),
			"composePath":       strings.TrimSpace(snapshot.ComposePath),
			"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
			"revision":          snapshot.Revision,
			"selector":          summary.Selector,
			"limit":             summary.Limit,
			"issueCount":        len(normalizedIssues),
			"issues":            normalizedIssues,
			"summary":           summary,
		},
	)
}
