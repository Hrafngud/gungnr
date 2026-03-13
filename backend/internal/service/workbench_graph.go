package service

import (
	"fmt"
	"sort"
	"strings"
)

const (
	workbenchGraphNodeStatusRunning  = "running"
	workbenchGraphNodeStatusDegraded = "degraded"
	workbenchGraphNodeStatusFailed   = "failed"
	workbenchGraphNodeStatusMissing  = "missing"
	workbenchGraphNodeStatusUnknown  = "unknown"
)

type WorkbenchDependencyGraph struct {
	ProjectName       string                    `json:"projectName"`
	Revision          int                       `json:"revision"`
	SourceFingerprint string                    `json:"sourceFingerprint"`
	Nodes             []WorkbenchDependencyNode `json:"nodes"`
	Edges             []WorkbenchDependencyEdge `json:"edges"`
	Warnings          []string                  `json:"warnings"`
}

type WorkbenchDependencyNode struct {
	ServiceName    string `json:"serviceName"`
	Status         string `json:"status"`
	StatusText     string `json:"statusText"`
	ContainerCount int    `json:"containerCount"`
	RunningCount   int    `json:"runningCount"`
	HealthyCount   int    `json:"healthyCount"`
	FailedCount    int    `json:"failedCount"`
}

type WorkbenchDependencyEdge struct {
	Key           string `json:"key"`
	FromService   string `json:"fromService"`
	ToService     string `json:"toService"`
	SourceStatus  string `json:"sourceStatus"`
	FailureSource bool   `json:"failureSource"`
}

type workbenchServiceRuntimeStatus struct {
	status         string
	statusText     string
	containerCount int
	runningCount   int
	healthyCount   int
	failedCount    int
}

func (s *WorkbenchService) BuildDependencyGraph(
	snapshot WorkbenchStackSnapshot,
	containers []DockerContainer,
) WorkbenchDependencyGraph {
	normalized := normalizeWorkbenchStackSnapshot(snapshot)

	serviceNames := workbenchGraphServiceNames(normalized)
	containersByService := workbenchGroupContainersByService(containers)
	statusByService := make(map[string]string, len(serviceNames))

	nodes := make([]WorkbenchDependencyNode, 0, len(serviceNames))
	for _, serviceName := range serviceNames {
		key := strings.ToLower(strings.TrimSpace(serviceName))
		status := workbenchRuntimeStatusFromContainers(containersByService[key])
		statusByService[key] = status.status
		nodes = append(nodes, WorkbenchDependencyNode{
			ServiceName:    serviceName,
			Status:         status.status,
			StatusText:     status.statusText,
			ContainerCount: status.containerCount,
			RunningCount:   status.runningCount,
			HealthyCount:   status.healthyCount,
			FailedCount:    status.failedCount,
		})
	}

	indexByService := make(map[string]string, len(serviceNames))
	for _, serviceName := range serviceNames {
		indexByService[strings.ToLower(strings.TrimSpace(serviceName))] = serviceName
	}

	edges := make([]WorkbenchDependencyEdge, 0, len(normalized.Dependencies))
	edgeSet := make(map[string]struct{}, len(normalized.Dependencies))
	for _, dependency := range normalized.Dependencies {
		toService := workbenchResolveGraphServiceName(indexByService, dependency.ServiceName)
		fromService := workbenchResolveGraphServiceName(indexByService, dependency.DependsOn)
		if toService == "" || fromService == "" {
			continue
		}

		edgeKey := strings.ToLower(fromService + "->" + toService)
		if _, exists := edgeSet[edgeKey]; exists {
			continue
		}
		edgeSet[edgeKey] = struct{}{}

		sourceStatus := statusByService[strings.ToLower(strings.TrimSpace(fromService))]
		if sourceStatus == "" {
			sourceStatus = workbenchGraphNodeStatusUnknown
		}

		edges = append(edges, WorkbenchDependencyEdge{
			Key:           fmt.Sprintf("%s->%s", fromService, toService),
			FromService:   fromService,
			ToService:     toService,
			SourceStatus:  sourceStatus,
			FailureSource: sourceStatus == workbenchGraphNodeStatusFailed || sourceStatus == workbenchGraphNodeStatusMissing,
		})
	}

	sort.SliceStable(edges, func(i, j int) bool {
		leftFrom := strings.ToLower(strings.TrimSpace(edges[i].FromService))
		rightFrom := strings.ToLower(strings.TrimSpace(edges[j].FromService))
		if leftFrom != rightFrom {
			return leftFrom < rightFrom
		}
		leftTo := strings.ToLower(strings.TrimSpace(edges[i].ToService))
		rightTo := strings.ToLower(strings.TrimSpace(edges[j].ToService))
		return leftTo < rightTo
	})

	return WorkbenchDependencyGraph{
		ProjectName:       normalized.ProjectName,
		Revision:          normalized.Revision,
		SourceFingerprint: normalized.SourceFingerprint,
		Nodes:             nodes,
		Edges:             edges,
		Warnings:          []string{},
	}
}

func workbenchGraphServiceNames(snapshot WorkbenchStackSnapshot) []string {
	names := make([]string, 0, len(snapshot.Services)+len(snapshot.Dependencies)*2)
	seen := make(map[string]struct{}, len(snapshot.Services)+len(snapshot.Dependencies)*2)

	addName := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		names = append(names, trimmed)
	}

	for _, service := range snapshot.Services {
		addName(service.ServiceName)
	}
	for _, dependency := range snapshot.Dependencies {
		addName(dependency.ServiceName)
		addName(dependency.DependsOn)
	}

	sort.SliceStable(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names
}

func workbenchGroupContainersByService(containers []DockerContainer) map[string][]DockerContainer {
	grouped := make(map[string][]DockerContainer)
	for _, container := range containers {
		serviceName := strings.ToLower(strings.TrimSpace(container.Service))
		if serviceName == "" {
			continue
		}
		grouped[serviceName] = append(grouped[serviceName], container)
	}
	return grouped
}

func workbenchResolveGraphServiceName(index map[string]string, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if canonical, exists := index[strings.ToLower(trimmed)]; exists {
		return canonical
	}
	return trimmed
}

func workbenchRuntimeStatusFromContainers(containers []DockerContainer) workbenchServiceRuntimeStatus {
	if len(containers) == 0 {
		return workbenchServiceRuntimeStatus{
			status:         workbenchGraphNodeStatusMissing,
			statusText:     "no containers",
			containerCount: 0,
		}
	}

	runningCount := 0
	healthyCount := 0
	failedCount := 0
	statuses := make([]string, 0, len(containers))
	seenStatuses := make(map[string]struct{}, len(containers))

	for _, container := range containers {
		status := strings.TrimSpace(container.Status)
		if status == "" {
			status = workbenchGraphNodeStatusUnknown
		}

		normalizedStatus := strings.ToLower(status)
		if _, exists := seenStatuses[normalizedStatus]; !exists {
			seenStatuses[normalizedStatus] = struct{}{}
			statuses = append(statuses, status)
		}

		if isRunningContainerStatus(status) {
			runningCount++
			if isHealthyContainerStatus(status) {
				healthyCount++
			}
		}

		if workbenchContainerFailureStatus(status) {
			failedCount++
		}
	}

	sort.Strings(statuses)
	status := workbenchGraphNodeStatusUnknown
	switch {
	case runningCount == len(containers) && healthyCount == len(containers):
		status = workbenchGraphNodeStatusRunning
	case runningCount == 0 && failedCount > 0:
		status = workbenchGraphNodeStatusFailed
	case failedCount > 0 || runningCount < len(containers) || healthyCount < runningCount:
		status = workbenchGraphNodeStatusDegraded
	case runningCount > 0:
		status = workbenchGraphNodeStatusRunning
	}

	statusText := strings.Join(statuses, ", ")
	if strings.TrimSpace(statusText) == "" {
		statusText = workbenchGraphNodeStatusUnknown
	}

	return workbenchServiceRuntimeStatus{
		status:         status,
		statusText:     statusText,
		containerCount: len(containers),
		runningCount:   runningCount,
		healthyCount:   healthyCount,
		failedCount:    failedCount,
	}
}

func workbenchContainerFailureStatus(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		return false
	}

	return strings.Contains(normalized, "unhealthy") ||
		strings.Contains(normalized, "exited") ||
		strings.Contains(normalized, "dead") ||
		strings.Contains(normalized, "oom") ||
		strings.Contains(normalized, "error") ||
		strings.Contains(normalized, "restart") ||
		strings.Contains(normalized, "paused") ||
		strings.HasPrefix(normalized, "created")
}
