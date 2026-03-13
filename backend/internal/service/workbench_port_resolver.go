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
	workbenchPortStrategyAuto   = "auto"
	workbenchPortStrategyManual = "manual"

	workbenchPortAllocationAssigned    = "assigned"
	workbenchPortAllocationConflict    = "conflict"
	workbenchPortAllocationUnavailable = "unavailable"

	workbenchPortSourceComposeHostPort = "compose_host_port"
	workbenchPortSourceModuleDefault   = "module_default"
	workbenchPortSourceContainerPort   = "container_port"
	workbenchPortSourceServiceProfile  = "service_profile"
)

const (
	workbenchPortIssueClassSchema     = "schema"
	workbenchPortIssueClassConflict   = "port_conflict"
	workbenchPortIssueClassAllocation = "allocation"
)

type WorkbenchPortResolutionSummary struct {
	Changed     bool                          `json:"changed"`
	Assigned    int                           `json:"assigned"`
	Conflict    int                           `json:"conflict"`
	Unavailable int                           `json:"unavailable"`
	Outcomes    []WorkbenchPortResolveOutcome `json:"outcomes"`
}

type WorkbenchPortResolveOutcome struct {
	ServiceName          string `json:"serviceName"`
	ContainerPort        int    `json:"containerPort"`
	Protocol             string `json:"protocol"`
	HostIP               string `json:"hostIp,omitempty"`
	RequestedHostPort    *int   `json:"requestedHostPort,omitempty"`
	RequestedHostPortRaw string `json:"requestedHostPortRaw,omitempty"`
	PreferredHostPort    *int   `json:"preferredHostPort,omitempty"`
	AssignedHostPort     *int   `json:"assignedHostPort,omitempty"`
	Status               string `json:"status"`
	Strategy             string `json:"strategy"`
	Source               string `json:"source"`
	Attempts             int    `json:"attempts,omitempty"`
	Message              string `json:"message,omitempty"`
}

type WorkbenchPortResolutionIssue struct {
	Class    string `json:"class"`
	Code     string `json:"code"`
	Path     string `json:"path"`
	Message  string `json:"message"`
	Service  string `json:"service,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	HostIP   string `json:"hostIp,omitempty"`
	HostPort string `json:"hostPort,omitempty"`
	Strategy string `json:"strategy,omitempty"`
	Source   string `json:"source,omitempty"`
}

type workbenchPortResolveCandidate struct {
	port   int
	source string
	path   string
}

func (s *WorkbenchService) ResolveStoredSnapshotPorts(
	ctx context.Context,
	projectName string,
) (WorkbenchStackSnapshot, WorkbenchPortResolutionSummary, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchPortResolutionSummary{}, err
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchPortResolutionSummary{}, err
	}
	defer release()

	snapshot, exists, err := s.loadStoredWorkbenchSnapshot(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, WorkbenchPortResolutionSummary{}, err
	}
	if !exists {
		return WorkbenchStackSnapshot{}, WorkbenchPortResolutionSummary{}, errs.WithDetails(
			errs.New(errs.CodeWorkbenchSourceNotFound, fmt.Sprintf("workbench snapshot not found for project %q", normalizedProject)),
			map[string]any{
				"project": normalizedProject,
			},
		)
	}

	resolved, summary, err := resolveWorkbenchSnapshotPorts(snapshot)
	if err != nil {
		return resolved, summary, err
	}

	if !summary.Changed {
		return resolved, summary, nil
	}

	if resolved.Revision <= 0 {
		resolved.Revision = 1
	}
	resolved.Revision++

	if err := s.saveWorkbenchSnapshot(ctx, normalizedProject, resolved); err != nil {
		return resolved, summary, err
	}
	return resolved, summary, nil
}

func resolveWorkbenchSnapshotPorts(
	snapshot WorkbenchStackSnapshot,
) (WorkbenchStackSnapshot, WorkbenchPortResolutionSummary, error) {
	normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
	outcomes := make([]WorkbenchPortResolveOutcome, 0, len(normalizedSnapshot.Ports))
	issues := []WorkbenchPortResolutionIssue{}
	resolvedPorts := append([]WorkbenchComposePort(nil), normalizedSnapshot.Ports...)

	addIssue := func(issue WorkbenchPortResolutionIssue) {
		issue.Class = strings.ToLower(strings.TrimSpace(issue.Class))
		issue.Code = strings.ToUpper(strings.TrimSpace(issue.Code))
		issue.Path = strings.TrimSpace(issue.Path)
		issue.Message = strings.TrimSpace(issue.Message)
		issue.Service = strings.TrimSpace(issue.Service)
		issue.Protocol = strings.ToLower(strings.TrimSpace(issue.Protocol))
		issue.HostIP = normalizeHostIP(strings.TrimSpace(issue.HostIP))
		issue.HostPort = strings.TrimSpace(issue.HostPort)
		issue.Strategy = strings.ToLower(strings.TrimSpace(issue.Strategy))
		issue.Source = strings.TrimSpace(issue.Source)
		if issue.Class == "" || issue.Code == "" || issue.Path == "" || issue.Message == "" {
			return
		}
		issues = append(issues, issue)
	}

	managedServiceModels, managedServiceIssues := workbenchBuildManagedServiceModels(normalizedSnapshot)
	for _, issue := range workbenchManagedServiceIssuesToPortResolution(managedServiceIssues) {
		addIssue(issue)
	}

	serviceSet := make(map[string]struct{}, len(normalizedSnapshot.Services)+len(managedServiceModels))
	for _, svc := range normalizedSnapshot.Services {
		name := strings.TrimSpace(svc.ServiceName)
		if name == "" {
			continue
		}
		serviceSet[name] = struct{}{}
	}
	for _, managedService := range managedServiceModels {
		name := strings.TrimSpace(managedService.ServiceName)
		if name == "" {
			continue
		}
		serviceSet[name] = struct{}{}
		for _, port := range managedService.Ports {
			if workbenchHasPortMapping(resolvedPorts, port.ServiceName, port.ContainerPort, port.Protocol, port.HostIP) {
				continue
			}
			resolvedPorts = append(resolvedPorts, normalizeWorkbenchComposePort(port))
		}
	}

	reservedBindings := []workbenchHostBinding{}
	for idx, port := range resolvedPorts {
		path := fmt.Sprintf("$.ports[%d]", idx)
		serviceName := strings.TrimSpace(port.ServiceName)
		protocol := strings.ToLower(strings.TrimSpace(port.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		hostIP := normalizeHostIP(strings.TrimSpace(port.HostIP))
		strategy := strings.ToLower(strings.TrimSpace(port.AssignmentStrategy))
		if strategy != workbenchPortStrategyManual {
			strategy = workbenchPortStrategyAuto
		}

		outcome := WorkbenchPortResolveOutcome{
			ServiceName:          serviceName,
			ContainerPort:        port.ContainerPort,
			Protocol:             protocol,
			HostIP:               hostIP,
			RequestedHostPort:    cloneWorkbenchPortInt(port.HostPort),
			RequestedHostPortRaw: strings.TrimSpace(port.HostPortRaw),
			Status:               workbenchPortAllocationUnavailable,
			Strategy:             strategy,
		}

		if _, exists := serviceSet[serviceName]; !exists {
			outcome.Message = fmt.Sprintf("port entry references unknown service %q", serviceName)
			outcome.Source = workbenchPortSourceComposeHostPort
			port.AllocationStatus = workbenchPortAllocationUnavailable
			port.AssignmentStrategy = strategy
			resolvedPorts[idx] = port
			outcomes = append(outcomes, outcome)
			addIssue(WorkbenchPortResolutionIssue{
				Class:    workbenchPortIssueClassSchema,
				Code:     "WB-RESOLVE-PORT-SERVICE-UNKNOWN",
				Path:     path + ".serviceName",
				Message:  outcome.Message,
				Service:  serviceName,
				Protocol: protocol,
				HostIP:   hostIP,
				Strategy: strategy,
			})
			continue
		}

		candidate, issue := workbenchResolvePortCandidate(normalizedSnapshot, port, idx)
		if issue != nil {
			outcome.Message = issue.Message
			outcome.Source = issue.Source
			if candidate.port > 0 {
				outcome.PreferredHostPort = intPtr(candidate.port)
			}
			port.AllocationStatus = workbenchPortAllocationUnavailable
			port.AssignmentStrategy = strategy
			resolvedPorts[idx] = port
			outcomes = append(outcomes, outcome)
			addIssue(*issue)
			continue
		}

		outcome.Source = candidate.source
		outcome.PreferredHostPort = intPtr(candidate.port)

		if strategy == workbenchPortStrategyManual {
			if workbenchHostPortConflicts(reservedBindings, protocol, hostIP, candidate.port) {
				outcome.Status = workbenchPortAllocationConflict
				outcome.Message = fmt.Sprintf(
					"manual host port %d conflicts with an existing reservation for service %q",
					candidate.port,
					serviceName,
				)
				port.AllocationStatus = workbenchPortAllocationConflict
				port.AssignmentStrategy = workbenchPortStrategyManual
				resolvedPorts[idx] = port
				outcomes = append(outcomes, outcome)
				addIssue(WorkbenchPortResolutionIssue{
					Class:    workbenchPortIssueClassConflict,
					Code:     "WB-RESOLVE-MANUAL-CONFLICT",
					Path:     candidate.path,
					Message:  outcome.Message,
					Service:  serviceName,
					Protocol: protocol,
					HostIP:   hostIP,
					HostPort: strconv.Itoa(candidate.port),
					Strategy: workbenchPortStrategyManual,
					Source:   candidate.source,
				})
				continue
			}

			assigned := candidate.port
			outcome.Status = workbenchPortAllocationAssigned
			outcome.Attempts = 1
			outcome.AssignedHostPort = intPtr(assigned)
			port.HostPort = intPtr(assigned)
			port.HostPortRaw = ""
			port.AssignmentStrategy = workbenchPortStrategyManual
			port.AllocationStatus = workbenchPortAllocationAssigned
			resolvedPorts[idx] = port
			outcomes = append(outcomes, outcome)
			reservedBindings = append(reservedBindings, workbenchHostBinding{
				serviceName: serviceName,
				hostIP:      hostIP,
				hostPort:    strconv.Itoa(assigned),
				protocol:    protocol,
			})
			continue
		}

		assigned, attempts, ok := workbenchFindAvailableHostPort(candidate.port, protocol, hostIP, reservedBindings)
		if !ok {
			outcome.Status = workbenchPortAllocationUnavailable
			outcome.Attempts = attempts
			outcome.Message = fmt.Sprintf("no available host port from %d to 65535 for service %q", candidate.port, serviceName)
			port.AllocationStatus = workbenchPortAllocationUnavailable
			port.AssignmentStrategy = workbenchPortStrategyAuto
			resolvedPorts[idx] = port
			outcomes = append(outcomes, outcome)
			addIssue(WorkbenchPortResolutionIssue{
				Class:    workbenchPortIssueClassAllocation,
				Code:     "WB-RESOLVE-PORT-UNAVAILABLE",
				Path:     candidate.path,
				Message:  outcome.Message,
				Service:  serviceName,
				Protocol: protocol,
				HostIP:   hostIP,
				HostPort: strconv.Itoa(candidate.port),
				Strategy: workbenchPortStrategyAuto,
				Source:   candidate.source,
			})
			continue
		}

		outcome.Status = workbenchPortAllocationAssigned
		outcome.Attempts = attempts
		outcome.AssignedHostPort = intPtr(assigned)
		port.HostPort = intPtr(assigned)
		port.HostPortRaw = ""
		port.AssignmentStrategy = workbenchPortStrategyAuto
		port.AllocationStatus = workbenchPortAllocationAssigned
		resolvedPorts[idx] = port
		outcomes = append(outcomes, outcome)
		reservedBindings = append(reservedBindings, workbenchHostBinding{
			serviceName: serviceName,
			hostIP:      hostIP,
			hostPort:    strconv.Itoa(assigned),
			protocol:    protocol,
		})
	}

	resolved := normalizedSnapshot
	resolved.Ports = resolvedPorts

	assignedCount, conflictCount, unavailableCount := workbenchSummarizePortOutcomes(outcomes)
	summary := WorkbenchPortResolutionSummary{
		Changed:     !reflect.DeepEqual(normalizedSnapshot.Ports, resolvedPorts),
		Assigned:    assignedCount,
		Conflict:    conflictCount,
		Unavailable: unavailableCount,
		Outcomes:    outcomes,
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchPortResolutionIssueLess(issues[i], issues[j])
		})
		return resolved, summary, workbenchPortResolutionValidationError(resolved, summary, issues)
	}

	return resolved, summary, nil
}

func workbenchResolvePortCandidate(
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

	if port.HostPort != nil {
		if *port.HostPort < 1 || *port.HostPort > 65535 {
			return workbenchPortResolveCandidate{}, &WorkbenchPortResolutionIssue{
				Class:    workbenchPortIssueClassSchema,
				Code:     "WB-RESOLVE-PORT-REQUESTED-RANGE",
				Path:     path + ".hostPort",
				Message:  fmt.Sprintf("service %q has invalid requested hostPort %d", serviceName, *port.HostPort),
				Service:  serviceName,
				Protocol: protocol,
				HostIP:   hostIP,
				HostPort: strconv.Itoa(*port.HostPort),
				Strategy: strategy,
				Source:   workbenchPortSourceComposeHostPort,
			}
		}
		return workbenchPortResolveCandidate{
			port:   *port.HostPort,
			source: workbenchPortSourceComposeHostPort,
			path:   path + ".hostPort",
		}, nil
	}

	if raw := strings.TrimSpace(port.HostPortRaw); raw != "" {
		return workbenchPortResolveCandidate{}, &WorkbenchPortResolutionIssue{
			Class:    workbenchPortIssueClassSchema,
			Code:     "WB-RESOLVE-PORT-REQUESTED-UNSUPPORTED",
			Path:     path + ".hostPortRaw",
			Message:  fmt.Sprintf("service %q requested host port %q is unsupported by resolver baseline", serviceName, raw),
			Service:  serviceName,
			Protocol: protocol,
			HostIP:   hostIP,
			HostPort: raw,
			Strategy: strategy,
			Source:   workbenchPortSourceComposeHostPort,
		}
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
		path:   path + ".hostPort",
	}, nil
}

func workbenchResolveModuleDefaultPort(modules []WorkbenchStackModule, serviceName string) (int, bool) {
	normalizedService := strings.TrimSpace(serviceName)
	best := 0
	hasDefault := false
	for _, module := range modules {
		if strings.TrimSpace(module.ServiceName) != normalizedService {
			continue
		}
		candidate, ok := workbenchModuleDefaultPort(module.ModuleType)
		if !ok {
			continue
		}
		if !hasDefault || candidate < best {
			hasDefault = true
			best = candidate
		}
	}
	return best, hasDefault
}

func workbenchModuleDefaultPort(moduleType string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(moduleType)) {
	case "redis":
		return 6379, true
	default:
		return 0, false
	}
}

func workbenchHostPortConflicts(reserved []workbenchHostBinding, protocol, hostIP string, hostPort int) bool {
	candidate := workbenchHostBinding{
		hostIP:   normalizeHostIP(strings.TrimSpace(hostIP)),
		hostPort: strconv.Itoa(hostPort),
		protocol: strings.ToLower(strings.TrimSpace(protocol)),
	}
	for _, existing := range reserved {
		if workbenchHostBindingConflicts(existing, candidate) {
			return true
		}
	}
	return false
}

func workbenchFindAvailableHostPort(
	start int,
	protocol string,
	hostIP string,
	reserved []workbenchHostBinding,
) (int, int, bool) {
	if start < 1 || start > 65535 {
		return 0, 0, false
	}

	attempts := 0
	for candidate := start; candidate <= 65535; candidate++ {
		attempts++
		if workbenchHostPortConflicts(reserved, protocol, hostIP, candidate) {
			continue
		}
		return candidate, attempts, true
	}
	return 0, attempts, false
}

func workbenchSummarizePortOutcomes(outcomes []WorkbenchPortResolveOutcome) (assigned, conflict, unavailable int) {
	for _, outcome := range outcomes {
		switch strings.ToLower(strings.TrimSpace(outcome.Status)) {
		case workbenchPortAllocationAssigned:
			assigned++
		case workbenchPortAllocationConflict:
			conflict++
		case workbenchPortAllocationUnavailable:
			unavailable++
		default:
			unavailable++
		}
	}
	return assigned, conflict, unavailable
}

func workbenchPortResolutionValidationError(
	snapshot WorkbenchStackSnapshot,
	summary WorkbenchPortResolutionSummary,
	issues []WorkbenchPortResolutionIssue,
) error {
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchValidationFailed, "invalid workbench snapshot for host-port resolution"),
		map[string]any{
			"project":           strings.TrimSpace(snapshot.ProjectName),
			"composePath":       strings.TrimSpace(snapshot.ComposePath),
			"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
			"revision":          snapshot.Revision,
			"issueCount":        len(issues),
			"issues":            append([]WorkbenchPortResolutionIssue(nil), issues...),
			"outcomeCount":      len(summary.Outcomes),
			"outcomes":          append([]WorkbenchPortResolveOutcome(nil), summary.Outcomes...),
			"assigned":          summary.Assigned,
			"conflict":          summary.Conflict,
			"unavailable":       summary.Unavailable,
		},
	)
}

func workbenchPortResolutionIssueLess(left, right WorkbenchPortResolutionIssue) bool {
	leftClass := strings.ToLower(strings.TrimSpace(left.Class))
	rightClass := strings.ToLower(strings.TrimSpace(right.Class))
	if leftClass != rightClass {
		return leftClass < rightClass
	}

	leftCode := strings.ToUpper(strings.TrimSpace(left.Code))
	rightCode := strings.ToUpper(strings.TrimSpace(right.Code))
	if leftCode != rightCode {
		return leftCode < rightCode
	}

	leftPath := strings.TrimSpace(left.Path)
	rightPath := strings.TrimSpace(right.Path)
	if leftPath != rightPath {
		return leftPath < rightPath
	}

	leftService := strings.ToLower(strings.TrimSpace(left.Service))
	rightService := strings.ToLower(strings.TrimSpace(right.Service))
	if leftService != rightService {
		return leftService < rightService
	}

	leftProtocol := strings.ToLower(strings.TrimSpace(left.Protocol))
	rightProtocol := strings.ToLower(strings.TrimSpace(right.Protocol))
	if leftProtocol != rightProtocol {
		return leftProtocol < rightProtocol
	}

	leftHostIP := strings.ToLower(strings.TrimSpace(left.HostIP))
	rightHostIP := strings.ToLower(strings.TrimSpace(right.HostIP))
	if leftHostIP != rightHostIP {
		return leftHostIP < rightHostIP
	}

	leftHostPort := strings.TrimSpace(left.HostPort)
	rightHostPort := strings.TrimSpace(right.HostPort)
	if leftHostPort != rightHostPort {
		return leftHostPort < rightHostPort
	}

	return strings.TrimSpace(left.Message) < strings.TrimSpace(right.Message)
}

func cloneWorkbenchPortInt(value *int) *int {
	if value == nil {
		return nil
	}
	return intPtr(*value)
}

func intPtr(value int) *int {
	port := value
	return &port
}
