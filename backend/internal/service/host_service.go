package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go-notes/internal/errs"
	infraclient "go-notes/internal/infra/client"
	"go-notes/internal/infra/contract"
	"go-notes/internal/jobs"
	"go-notes/internal/repository"
)

type DockerPortBinding struct {
	HostIP        string `json:"hostIp"`
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Proto         string `json:"proto"`
	Published     bool   `json:"published"`
}

type DockerContainer struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Image        string              `json:"image"`
	Status       string              `json:"status"`
	Ports        string              `json:"ports"`
	CreatedAt    string              `json:"createdAt"`
	RunningFor   string              `json:"runningFor"`
	Service      string              `json:"service"`
	Project      string              `json:"project"`
	PortBindings []DockerPortBinding `json:"portBindings"`
}

type DockerRuntimeInfo struct {
	ServerVersion   string   `json:"serverVersion,omitempty"`
	DockerRootDir   string   `json:"dockerRootDir,omitempty"`
	SecurityOptions []string `json:"securityOptions,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	Rootless        bool     `json:"rootless"`
	UsernsRemap     bool     `json:"usernsRemap"`
}

type HostService struct {
	templatesDir string
	projects     repository.ProjectRepository
	infraClient  hostInfraBridgeClient
}

type hostInfraBridgeClient interface {
	StopContainer(ctx context.Context, requestID, container string) (contract.Result, error)
	RestartContainer(ctx context.Context, requestID, container string) (contract.Result, error)
	RemoveContainer(ctx context.Context, requestID, container string, removeVolumes bool) (contract.Result, error)
	DockerListContainers(ctx context.Context, requestID string, includeAll bool) (contract.Result, error)
	DockerSystemDF(ctx context.Context, requestID string) (contract.Result, error)
	DockerListVolumes(ctx context.Context, requestID string) (contract.Result, error)
	DockerContainerLogs(ctx context.Context, requestID string, payload contract.DockerContainerLogsPayload) (contract.Result, error)
	DockerRuntimeCheck(ctx context.Context, requestID string) (contract.Result, error)
	HostRuntimeStats(ctx context.Context, requestID string) (contract.Result, error)
	HostRuntimeStream(ctx context.Context, requestID string) (contract.Result, error)
	ComposeUpStack(ctx context.Context, requestID string, payload contract.ComposeUpStackPayload) (contract.Result, error)
}

func NewHostService(templatesDir string, projects repository.ProjectRepository, infraClient hostInfraBridgeClient) *HostService {
	return &HostService{
		templatesDir: strings.TrimSpace(templatesDir),
		projects:     projects,
		infraClient:  infraClient,
	}
}

type ContainerLogsOptions struct {
	Tail       int
	Follow     bool
	Timestamps bool
}

type ContainerLogsWaiter interface {
	Wait() error
}

type containerLogsBridgeWaiter struct {
	done <-chan error
}

func (w containerLogsBridgeWaiter) Wait() error {
	return <-w.done
}

func (s *HostService) ListContainers(ctx context.Context, includeAll bool) ([]DockerContainer, error) {
	lines, err := s.readDockerPSLines(ctx, includeAll, errs.CodeHostDockerFailed, "failed to list docker containers")
	if err != nil {
		return nil, err
	}
	return parseDockerPSLinesToContainers(lines)
}

func (s *HostService) CountRunningContainers(ctx context.Context) (int, error) {
	lines, err := s.readDockerPSLines(ctx, false, errs.CodeHostDockerFailed, "failed to count running containers")
	if err != nil {
		return 0, err
	}
	return len(lines), nil
}

type DockerUsageEntry struct {
	Count int    `json:"count"`
	Size  string `json:"size"`
}

type DockerUsageProjectCounts struct {
	Containers int `json:"containers"`
	Images     int `json:"images"`
	Volumes    int `json:"volumes"`
}

type DockerUsageSummary struct {
	TotalSize     string                    `json:"totalSize"`
	Images        DockerUsageEntry          `json:"images"`
	Containers    DockerUsageEntry          `json:"containers"`
	Volumes       DockerUsageEntry          `json:"volumes"`
	BuildCache    DockerUsageEntry          `json:"buildCache,omitempty"`
	Project       string                    `json:"project,omitempty"`
	ProjectCounts *DockerUsageProjectCounts `json:"projectCounts,omitempty"`
}

type HostRuntimeResource struct {
	TotalBytes     int64   `json:"totalBytes"`
	UsedBytes      int64   `json:"usedBytes"`
	FreeBytes      int64   `json:"freeBytes"`
	AvailableBytes int64   `json:"availableBytes,omitempty"`
	UsedPercent    float64 `json:"usedPercent"`
	SpeedMTs       int     `json:"speedMTs,omitempty"`
}

type HostRuntimeCPU struct {
	Model    string  `json:"model"`
	Cores    int     `json:"cores"`
	Threads  int     `json:"threads"`
	SpeedMHz float64 `json:"speedMHz,omitempty"`
}

type HostRuntimeGPU struct {
	Model    string  `json:"model"`
	SpeedMHz float64 `json:"speedMHz,omitempty"`
}

type HostRuntimeMemorySnapshot struct {
	TotalBytes     int64 `json:"totalBytes"`
	FreeBytes      int64 `json:"freeBytes"`
	AvailableBytes int64 `json:"availableBytes,omitempty"`
	SpeedMTs       int   `json:"speedMTs,omitempty"`
}

type HostRuntimeWorkloadSnapshot struct {
	Containers        int     `json:"containers"`
	RunningContainers int     `json:"runningContainers"`
	DiskUsedBytes     int64   `json:"diskUsedBytes"`
	DiskSharePercent  float64 `json:"diskSharePercent"`
}

type HostRuntimeSnapshot struct {
	CollectedAt    string                                 `json:"collectedAt"`
	Hostname       string                                 `json:"hostname,omitempty"`
	UptimeSeconds  int64                                  `json:"uptimeSeconds"`
	UptimeHuman    string                                 `json:"uptimeHuman"`
	SystemImage    string                                 `json:"systemImage"`
	Kernel         string                                 `json:"kernel"`
	CPU            HostRuntimeCPU                         `json:"cpu"`
	GPU            *HostRuntimeGPU                        `json:"gpu,omitempty"`
	Memory         HostRuntimeMemorySnapshot              `json:"memory"`
	Disk           HostRuntimeResource                    `json:"disk"`
	Panel          HostRuntimeWorkloadSnapshot            `json:"panel"`
	Projects       HostRuntimeWorkloadSnapshot            `json:"projects"`
	ProjectsByName map[string]HostRuntimeWorkloadSnapshot `json:"projectsByName,omitempty"`
	Warnings       []string                               `json:"warnings,omitempty"`
}

type HostRuntimeHostStreamUsage struct {
	MemoryUsedBytes      int64   `json:"memoryUsedBytes"`
	MemoryUsedPercent    float64 `json:"memoryUsedPercent"`
	MemoryFreeBytes      int64   `json:"memoryFreeBytes"`
	MemoryAvailableBytes int64   `json:"memoryAvailableBytes,omitempty"`
}

type HostRuntimeWorkloadStreamUsage struct {
	CPUUsedPercent     float64 `json:"cpuUsedPercent"`
	MemoryUsedBytes    int64   `json:"memoryUsedBytes"`
	MemorySharePercent float64 `json:"memorySharePercent"`
}

type HostRuntimeStreamSample struct {
	CollectedAt    string                                    `json:"collectedAt"`
	Mode           string                                    `json:"mode"`
	IntervalMs     int                                       `json:"intervalMs"`
	Host           HostRuntimeHostStreamUsage                `json:"host"`
	Panel          HostRuntimeWorkloadStreamUsage            `json:"panel"`
	Projects       HostRuntimeWorkloadStreamUsage            `json:"projects"`
	ProjectsByName map[string]HostRuntimeWorkloadStreamUsage `json:"projectsByName,omitempty"`
	Warnings       []string                                  `json:"warnings,omitempty"`
}

const HostRuntimeStreamInterval = 100 * time.Millisecond

type dockerSystemDFLine struct {
	Type        string `json:"Type"`
	TotalCount  string `json:"TotalCount"`
	Active      string `json:"Active"`
	Size        string `json:"Size"`
	Reclaimable string `json:"Reclaimable"`
}

type dockerVolumeLine struct {
	Name   string `json:"Name"`
	Driver string `json:"Driver"`
	Labels string `json:"Labels"`
}

func (s *HostService) DockerUsage(ctx context.Context, project string) (DockerUsageSummary, error) {
	summary, totalBytes, err := s.readDockerUsage(ctx)
	if err != nil {
		return DockerUsageSummary{}, err
	}
	summary.TotalSize = formatDockerBytes(totalBytes)

	project = strings.TrimSpace(project)
	if project == "" {
		return summary, nil
	}
	summary.Project = project

	containers, err := s.listContainersForUsage(ctx, true)
	if err != nil {
		summary.ProjectCounts = &DockerUsageProjectCounts{}
		return summary, wrapDockerUsageProjectCountsDegraded(err)
	}

	normalizedProject := strings.ToLower(project)
	projectContainers := make([]DockerContainer, 0)
	for _, container := range containers {
		if strings.ToLower(strings.TrimSpace(container.Project)) == normalizedProject {
			projectContainers = append(projectContainers, container)
		}
	}

	imageSet := make(map[string]struct{})
	for _, container := range projectContainers {
		image := strings.TrimSpace(container.Image)
		if image != "" {
			imageSet[image] = struct{}{}
		}
	}

	volumes, err := s.listVolumes(ctx)
	if err != nil {
		summary.ProjectCounts = &DockerUsageProjectCounts{}
		return summary, wrapDockerUsageProjectCountsDegraded(err)
	}
	volumeCount := 0
	for _, volume := range volumes {
		labels := parseDockerLabels(volume.Labels)
		if strings.ToLower(labels["com.docker.compose.project"]) == normalizedProject {
			volumeCount++
		}
	}

	summary.ProjectCounts = &DockerUsageProjectCounts{
		Containers: len(projectContainers),
		Images:     len(imageSet),
		Volumes:    volumeCount,
	}
	return summary, nil
}

func (s *HostService) listContainersForUsage(ctx context.Context, includeAll bool) ([]DockerContainer, error) {
	lines, err := s.readDockerPSLines(ctx, includeAll, errs.CodeHostUsageFailed, "failed to load docker usage")
	if err != nil {
		return nil, err
	}
	return parseDockerPSLinesToContainers(lines)
}

func (s *HostService) readDockerPSLines(ctx context.Context, includeAll bool, code errs.Code, message string) ([]string, error) {
	if s.infraClient == nil {
		return nil, errs.WithDetails(
			errs.New(code, "infra bridge client unavailable"),
			map[string]any{"task_type": contract.TaskTypeDockerListContainers},
		)
	}

	result, err := s.infraClient.DockerListContainers(ctx, "", includeAll)
	if err != nil {
		return nil, bridgeTaskErrorWithCode(code, message, contract.TaskTypeDockerListContainers, "docker", err)
	}
	if err := bridgeResultErrorWithCode(code, message, contract.TaskTypeDockerListContainers, "docker", result); err != nil {
		return nil, err
	}
	return decodeBridgeLinesPayload(result)
}

func decodeBridgeLinesPayload(result contract.Result) ([]string, error) {
	if len(result.Data) == 0 {
		return []string{}, nil
	}
	rawLines, exists := result.Data["lines"]
	if !exists || rawLines == nil {
		return []string{}, nil
	}
	raw, err := json.Marshal(rawLines)
	if err != nil {
		return nil, fmt.Errorf("encode worker lines payload: %w", err)
	}
	var lines []string
	if err := json.Unmarshal(raw, &lines); err != nil {
		return nil, fmt.Errorf("decode worker lines payload: %w", err)
	}
	return lines, nil
}

func parseDockerPSLinesToContainers(lines []string) ([]DockerContainer, error) {
	containers := make([]DockerContainer, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry dockerPSLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse docker output: %w", err)
		}
		labels := parseDockerLabels(entry.Labels)
		containers = append(containers, DockerContainer{
			ID:           entry.ID,
			Name:         entry.Names,
			Image:        entry.Image,
			Status:       entry.Status,
			Ports:        entry.Ports,
			CreatedAt:    entry.CreatedAt,
			RunningFor:   entry.RunningFor,
			Service:      labels["com.docker.compose.service"],
			Project:      labels["com.docker.compose.project"],
			PortBindings: parseDockerPorts(entry.Ports),
		})
	}
	return containers, nil
}

func (s *HostService) RuntimeSnapshot(ctx context.Context) (HostRuntimeSnapshot, error) {
	if s.infraClient == nil {
		return HostRuntimeSnapshot{}, errs.New(errs.CodeHostStatsFailed, "infra bridge client unavailable")
	}

	result, err := s.infraClient.HostRuntimeStats(ctx, "")
	if err != nil {
		return HostRuntimeSnapshot{}, bridgeTaskErrorWithCode(errs.CodeHostStatsFailed, "failed to load host runtime snapshot", contract.TaskTypeHostRuntimeStats, "host", err)
	}
	if err := bridgeResultErrorWithCode(errs.CodeHostStatsFailed, "failed to load host runtime snapshot", contract.TaskTypeHostRuntimeStats, "host", result); err != nil {
		return HostRuntimeSnapshot{}, err
	}
	if len(result.Data) == 0 {
		return HostRuntimeSnapshot{}, errs.WithDetails(
			errs.New(errs.CodeHostStatsFailed, "host runtime snapshot payload is empty"),
			map[string]any{
				"task_type": contract.TaskTypeHostRuntimeStats,
				"intent_id": result.IntentID,
			},
		)
	}

	raw, err := json.Marshal(result.Data)
	if err != nil {
		return HostRuntimeSnapshot{}, errs.Wrap(errs.CodeHostStatsFailed, "failed to parse host runtime snapshot payload", err)
	}
	var stats HostRuntimeSnapshot
	if err := json.Unmarshal(raw, &stats); err != nil {
		return HostRuntimeSnapshot{}, errs.Wrap(errs.CodeHostStatsFailed, "failed to decode host runtime snapshot payload", err)
	}
	return stats, nil
}

func (s *HostService) RuntimeStreamSample(ctx context.Context) (HostRuntimeStreamSample, error) {
	if s.infraClient == nil {
		return HostRuntimeStreamSample{}, errs.New(errs.CodeHostStatsFailed, "infra bridge client unavailable")
	}

	result, err := s.infraClient.HostRuntimeStream(ctx, "")
	if err != nil {
		return HostRuntimeStreamSample{}, bridgeTaskErrorWithCode(errs.CodeHostStatsFailed, "failed to load host runtime stream sample", contract.TaskTypeHostRuntimeStream, "host", err)
	}
	if err := bridgeResultErrorWithCode(errs.CodeHostStatsFailed, "failed to load host runtime stream sample", contract.TaskTypeHostRuntimeStream, "host", result); err != nil {
		return HostRuntimeStreamSample{}, err
	}
	if len(result.Data) == 0 {
		return HostRuntimeStreamSample{}, errs.WithDetails(
			errs.New(errs.CodeHostStatsFailed, "host runtime stream payload is empty"),
			map[string]any{
				"task_type": contract.TaskTypeHostRuntimeStream,
				"intent_id": result.IntentID,
			},
		)
	}

	raw, err := json.Marshal(result.Data)
	if err != nil {
		return HostRuntimeStreamSample{}, errs.Wrap(errs.CodeHostStatsFailed, "failed to parse host runtime stream payload", err)
	}
	var sample HostRuntimeStreamSample
	if err := json.Unmarshal(raw, &sample); err != nil {
		return HostRuntimeStreamSample{}, errs.Wrap(errs.CodeHostStatsFailed, "failed to decode host runtime stream payload", err)
	}
	return sample, nil
}

func (s *HostService) DockerRuntime(ctx context.Context) (DockerRuntimeInfo, error) {
	if s.infraClient == nil {
		return DockerRuntimeInfo{}, errs.WithDetails(
			errs.New(errs.CodeHostDockerFailed, "infra bridge client unavailable"),
			map[string]any{"task_type": contract.TaskTypeDockerRuntimeCheck},
		)
	}

	result, err := s.infraClient.DockerRuntimeCheck(ctx, "")
	if err != nil {
		return DockerRuntimeInfo{}, bridgeTaskError("failed to inspect docker runtime", contract.TaskTypeDockerRuntimeCheck, "docker", err)
	}
	if err := bridgeResultError("failed to inspect docker runtime", contract.TaskTypeDockerRuntimeCheck, "docker", result); err != nil {
		return DockerRuntimeInfo{}, err
	}

	var payload struct {
		ServerVersion   string   `json:"server_version"`
		DockerRootDir   string   `json:"docker_root_dir"`
		SecurityOptions []string `json:"security_options"`
		Warnings        []string `json:"warnings"`
		Rootless        bool     `json:"rootless"`
		UsernsRemap     bool     `json:"userns_remap"`
	}
	if len(result.Data) > 0 {
		raw, marshalErr := json.Marshal(result.Data)
		if marshalErr != nil {
			return DockerRuntimeInfo{}, fmt.Errorf("encode docker runtime payload: %w", marshalErr)
		}
		if unmarshalErr := json.Unmarshal(raw, &payload); unmarshalErr != nil {
			return DockerRuntimeInfo{}, fmt.Errorf("decode docker runtime payload: %w", unmarshalErr)
		}
	}

	serverVersion := strings.TrimSpace(payload.ServerVersion)
	if serverVersion == "" {
		lines, linesErr := decodeBridgeLinesPayload(result)
		if linesErr != nil {
			return DockerRuntimeInfo{}, fmt.Errorf("decode docker runtime version payload: %w", linesErr)
		}
		if len(lines) > 0 {
			serverVersion = strings.TrimSpace(lines[0])
		}
	}

	return DockerRuntimeInfo{
		ServerVersion:   serverVersion,
		DockerRootDir:   strings.TrimSpace(payload.DockerRootDir),
		SecurityOptions: payload.SecurityOptions,
		Warnings:        payload.Warnings,
		Rootless:        payload.Rootless,
		UsernsRemap:     payload.UsernsRemap,
	}, nil
}

func (s *HostService) StartContainerLogs(ctx context.Context, container string, opts ContainerLogsOptions) (ContainerLogsWaiter, io.ReadCloser, error) {
	container = strings.TrimSpace(container)
	if container == "" {
		return nil, nil, fmt.Errorf("container is required")
	}
	if s.infraClient == nil {
		return nil, nil, fmt.Errorf("infra bridge client unavailable")
	}

	reader, writer := io.Pipe()
	done := make(chan error, 1)

	go func() {
		err := s.streamContainerLogs(ctx, container, opts, func(line string) error {
			_, writeErr := io.WriteString(writer, line+"\n")
			return writeErr
		})
		if err != nil {
			_ = writer.CloseWithError(err)
		} else {
			_ = writer.Close()
		}
		done <- err
	}()

	return containerLogsBridgeWaiter{done: done}, reader, nil
}

func (s *HostService) streamContainerLogs(ctx context.Context, container string, opts ContainerLogsOptions, emit func(string) error) error {
	tail := opts.Tail
	if tail <= 0 {
		tail = 200
	}
	if tail > 5000 {
		tail = 5000
	}

	firstFetch := true
	since := ""
	pollInterval := 1 * time.Second

	for {
		// Use current time as the next since-cursor to avoid duplicates while following.
		nextSince := time.Now().UTC().Format(time.RFC3339Nano)

		requestTail := 0
		if firstFetch {
			requestTail = tail
		}

		result, err := s.infraClient.DockerContainerLogs(ctx, "", contract.DockerContainerLogsPayload{
			Container:  container,
			Tail:       requestTail,
			Follow:     false,
			Timestamps: opts.Timestamps,
			Since:      since,
		})
		if err != nil {
			return fmt.Errorf("fetch docker logs via infra bridge: %w", err)
		}
		if result.Status == contract.StatusFailed {
			message := "host worker reported failure"
			if result.Error != nil && strings.TrimSpace(result.Error.Message) != "" {
				message = result.Error.Message
			}
			return fmt.Errorf("docker logs task failed: %s", message)
		}

		lines, err := decodeBridgeLinesPayload(result)
		if err != nil {
			return fmt.Errorf("decode docker logs payload: %w", err)
		}
		for _, line := range lines {
			if emitErr := emit(line); emitErr != nil {
				return emitErr
			}
		}

		if !opts.Follow {
			return nil
		}

		firstFetch = false
		since = nextSince
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(pollInterval):
		}
	}
}

func (s *HostService) StopContainer(ctx context.Context, container string) error {
	container = strings.TrimSpace(container)
	if container == "" {
		return fmt.Errorf("container is required")
	}
	if s.infraClient == nil {
		return fmt.Errorf("infra bridge client unavailable")
	}

	result, err := s.infraClient.StopContainer(ctx, "", container)
	if err != nil {
		return bridgeTaskError("failed to stop container", contract.TaskTypeDockerStopContainer, container, err)
	}
	return bridgeResultError("failed to stop container", contract.TaskTypeDockerStopContainer, container, result)
}

func (s *HostService) RestartContainer(ctx context.Context, container string) error {
	container = strings.TrimSpace(container)
	if container == "" {
		return fmt.Errorf("container is required")
	}
	if s.infraClient == nil {
		return fmt.Errorf("infra bridge client unavailable")
	}

	result, err := s.infraClient.RestartContainer(ctx, "", container)
	if err != nil {
		return bridgeTaskError("failed to restart container", contract.TaskTypeDockerRestartContainer, container, err)
	}
	return bridgeResultError("failed to restart container", contract.TaskTypeDockerRestartContainer, container, result)
}

func (s *HostService) RemoveContainer(ctx context.Context, container string, removeVolumes bool) error {
	container = strings.TrimSpace(container)
	if container == "" {
		return fmt.Errorf("container is required")
	}
	if s.infraClient == nil {
		return fmt.Errorf("infra bridge client unavailable")
	}

	result, err := s.infraClient.RemoveContainer(ctx, "", container, removeVolumes)
	if err != nil {
		return bridgeTaskError("failed to remove container", contract.TaskTypeDockerRemoveContainer, container, err)
	}
	return bridgeResultError("failed to remove container", contract.TaskTypeDockerRemoveContainer, container, result)
}

func (s *HostService) RestartProjectStack(ctx context.Context, project string) error {
	return s.restartProjectStack(ctx, "", project, nil)
}

func (s *HostService) RestartProjectStackWithLogger(ctx context.Context, requestID, project string, logger jobs.Logger) error {
	return s.restartProjectStack(ctx, requestID, project, logger)
}

func (s *HostService) restartProjectStack(ctx context.Context, requestID, project string, logger jobs.Logger) error {
	project = strings.TrimSpace(project)
	if project == "" || project == "." || project == ".." {
		return fmt.Errorf("invalid project name")
	}
	if s.infraClient == nil {
		return fmt.Errorf("infra bridge client unavailable")
	}

	hostLogf(logger, "submitting compose_up_stack intent via infra bridge for project %q", project)
	result, err := s.infraClient.ComposeUpStack(ctx, requestID, contract.ComposeUpStackPayload{
		Project:       project,
		Build:         true,
		ForceRecreate: true,
	})
	if err != nil {
		hostLogf(logger, "infra bridge compose_up_stack error: %v", err)
		return bridgeTaskError("restart compose stack failed", contract.TaskTypeComposeUpStack, project, err)
	}
	if err := bridgeResultError("restart compose stack failed", contract.TaskTypeComposeUpStack, project, result); err != nil {
		hostLogf(logger, "infra bridge compose_up_stack failed result: %v", err)
		return err
	}

	hostLogf(logger, "infra bridge compose_up_stack intent completed: intent_id=%s status=%s", result.IntentID, result.Status)
	if strings.TrimSpace(result.LogPath) != "" {
		hostLogf(logger, "infra bridge compose_up_stack log path: %s", result.LogPath)
	}
	return nil
}

func (s *HostService) resolveProjectDirFromRepository(ctx context.Context, baseDir, project string) (string, error) {
	if s.projects == nil {
		return "", fmt.Errorf("project repository unavailable")
	}

	pathFromRecord := func(name string) (string, error) {
		record, err := s.projects.GetByName(ctx, name)
		if err != nil {
			return "", err
		}
		return normalizeProjectPath(baseDir, record.Path)
	}

	if dir, err := pathFromRecord(project); err == nil {
		return dir, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return "", fmt.Errorf("load project record: %w", err)
	}

	lower := strings.ToLower(project)
	if lower != project {
		if dir, err := pathFromRecord(lower); err == nil {
			return dir, nil
		} else if !errors.Is(err, repository.ErrNotFound) {
			return "", fmt.Errorf("load project record: %w", err)
		}
	}

	projects, err := s.projects.List(ctx)
	if err != nil {
		return "", fmt.Errorf("list projects: %w", err)
	}
	for _, item := range projects {
		if strings.EqualFold(strings.TrimSpace(item.Name), project) {
			return normalizeProjectPath(baseDir, item.Path)
		}
	}

	return "", fmt.Errorf("project record missing: %q", project)
}

func normalizeProjectPath(baseDir, rawPath string) (string, error) {
	projectPath := strings.TrimSpace(rawPath)
	if projectPath == "" {
		return "", fmt.Errorf("project path is empty")
	}

	candidates := make([]string, 0, 3)
	if filepath.IsAbs(projectPath) {
		candidates = append(candidates, projectPath)
		if baseDir != "" {
			candidates = append(candidates, filepath.Join(baseDir, filepath.Base(projectPath)))
		}
	} else {
		candidates = append(candidates, projectPath)
		if baseDir != "" {
			candidates = append(candidates, filepath.Join(baseDir, projectPath))
		}
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		return candidate, nil
	}

	return "", fmt.Errorf("project directory not accessible from path %q", projectPath)
}

func buildRestartProjectError(attempts []string) error {
	if len(attempts) == 0 {
		return errs.New(errs.CodeHostDockerFailed, "restart compose stack failed")
	}
	message := "restart compose stack failed: " + attempts[len(attempts)-1]
	return errs.WithDetails(
		errs.New(errs.CodeHostDockerFailed, message),
		map[string]any{"attempts": attempts},
	)
}

func resolveProjectDirExact(baseDir, project string) (string, error) {
	if strings.TrimSpace(baseDir) == "" {
		return "", fmt.Errorf("templates directory not configured")
	}
	projectDir := filepath.Join(baseDir, project)
	info, err := os.Stat(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("project directory missing: %q", project)
		}
		return "", fmt.Errorf("check project directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project path is not a directory: %q", project)
	}
	return projectDir, nil
}

func resolveProjectDir(baseDir, project string) (string, error) {
	if strings.TrimSpace(baseDir) == "" {
		return "", fmt.Errorf("templates directory not configured")
	}
	projectDir := filepath.Join(baseDir, project)
	if info, err := os.Stat(projectDir); err == nil {
		if info.IsDir() {
			return projectDir, nil
		}
		return "", fmt.Errorf("project path is not a directory: %q", project)
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", fmt.Errorf("read templates directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), project) {
			return filepath.Join(baseDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("project directory missing: %q", project)
}

type composeProjectMeta struct {
	WorkingDir  string
	ConfigFiles []string
}

func readComposeProjectMeta(ctx context.Context, runtimeMetaClient infraDockerMetadataClient, project string) (composeProjectMeta, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return composeProjectMeta{}, fmt.Errorf("compose project is required")
	}
	if runtimeMetaClient == nil {
		return composeProjectMeta{}, fmt.Errorf("infra bridge client unavailable")
	}

	result, err := runtimeMetaClient.DockerListContainers(ctx, "", true)
	if err != nil {
		return composeProjectMeta{}, fmt.Errorf("docker list containers for compose metadata failed: %w", err)
	}
	if result.Status == contract.StatusFailed {
		message := "host worker reported failure"
		if result.Error != nil && strings.TrimSpace(result.Error.Message) != "" {
			message = result.Error.Message
		}
		return composeProjectMeta{}, fmt.Errorf("docker list containers for compose metadata failed: %s", message)
	}

	lines, err := decodeBridgeLinesPayload(result)
	if err != nil {
		return composeProjectMeta{}, fmt.Errorf("decode compose metadata payload: %w", err)
	}

	foundProjectContainer := false
	var parseErr error
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry dockerPSLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			parseErr = err
			continue
		}

		labels := parseDockerLabels(entry.Labels)
		if !strings.EqualFold(strings.TrimSpace(labels["com.docker.compose.project"]), project) {
			continue
		}
		foundProjectContainer = true

		workingDir := strings.TrimSpace(labels["com.docker.compose.project.working_dir"])
		configFilesRaw := strings.TrimSpace(labels["com.docker.compose.project.config_files"])
		if workingDir == "" && configFilesRaw == "" {
			continue
		}
		return composeProjectMeta{
			WorkingDir:  workingDir,
			ConfigFiles: splitComposeConfigFiles(configFilesRaw),
		}, nil
	}

	if parseErr != nil {
		return composeProjectMeta{}, fmt.Errorf("parse compose metadata payload: %w", parseErr)
	}
	if !foundProjectContainer {
		return composeProjectMeta{}, fmt.Errorf("compose project not found: %q", project)
	}
	return composeProjectMeta{}, fmt.Errorf("compose metadata labels unavailable for project %q", project)
}

func resolveComposeProjectDir(baseDir, project, workingDir string, configFiles []string) (string, error) {
	workingDir = strings.TrimSpace(workingDir)
	if workingDir != "" {
		if info, err := os.Stat(workingDir); err == nil && info.IsDir() {
			return workingDir, nil
		}
	}
	if baseDir == "" {
		if workingDir != "" {
			return "", fmt.Errorf("compose working directory is not accessible: %q", workingDir)
		}
		return "", fmt.Errorf("templates directory not configured")
	}

	if workingDir != "" {
		baseName := filepath.Base(workingDir)
		if baseName != "." && baseName != "/" {
			mapped := filepath.Join(baseDir, baseName)
			if info, err := os.Stat(mapped); err == nil && info.IsDir() {
				return mapped, nil
			}
		}
	}

	for _, configFile := range configFiles {
		dir := filepath.Dir(strings.TrimSpace(configFile))
		if dir == "" || dir == "." || dir == "/" {
			continue
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
		if baseDir != "" {
			mapped := filepath.Join(baseDir, filepath.Base(dir))
			if info, err := os.Stat(mapped); err == nil && info.IsDir() {
				return mapped, nil
			}
		}
	}

	return resolveProjectDir(baseDir, project)
}

func resolveComposeConfigFiles(projectDir string, configFiles []string) []string {
	if len(configFiles) == 0 {
		return nil
	}
	resolved := make([]string, 0, len(configFiles))
	seen := make(map[string]struct{})
	for _, raw := range configFiles {
		filePath := strings.TrimSpace(raw)
		if filePath == "" {
			continue
		}
		candidates := []string{filePath}
		if projectDir != "" {
			if filepath.IsAbs(filePath) {
				candidates = append(candidates, filepath.Join(projectDir, filepath.Base(filePath)))
			} else {
				candidates = append(candidates, filepath.Join(projectDir, filePath))
			}
		}
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
			if _, exists := seen[candidate]; exists {
				break
			}
			seen[candidate] = struct{}{}
			resolved = append(resolved, candidate)
			break
		}
	}
	if len(resolved) == 0 {
		return nil
	}
	return resolved
}

func splitComposeConfigFiles(raw string) []string {
	parts := strings.Split(raw, ",")
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		file := strings.TrimSpace(part)
		if file != "" {
			files = append(files, file)
		}
	}
	return files
}

func parseLines(raw []byte) []string {
	lines := strings.Split(string(raw), "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		item := strings.TrimSpace(line)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func hostLogf(logger jobs.Logger, format string, args ...any) {
	if logger == nil {
		return
	}
	logger.Logf(format, args...)
}

type dockerPSLine struct {
	ID         string `json:"ID"`
	Image      string `json:"Image"`
	Names      string `json:"Names"`
	Status     string `json:"Status"`
	Ports      string `json:"Ports"`
	CreatedAt  string `json:"CreatedAt"`
	RunningFor string `json:"RunningFor"`
	Labels     string `json:"Labels"`
}

func (s *HostService) readDockerUsage(ctx context.Context) (DockerUsageSummary, int64, error) {
	if s.infraClient == nil {
		return DockerUsageSummary{}, 0, errs.WithDetails(
			errs.New(errs.CodeHostUsageFailed, "infra bridge client unavailable"),
			map[string]any{"task_type": contract.TaskTypeDockerSystemDF},
		)
	}

	result, err := s.infraClient.DockerSystemDF(ctx, "")
	if err != nil {
		return DockerUsageSummary{}, 0, bridgeTaskErrorWithCode(errs.CodeHostUsageFailed, "failed to load docker usage", contract.TaskTypeDockerSystemDF, "docker", err)
	}
	if err := bridgeResultErrorWithCode(errs.CodeHostUsageFailed, "failed to load docker usage", contract.TaskTypeDockerSystemDF, "docker", result); err != nil {
		return DockerUsageSummary{}, 0, err
	}

	lines, err := decodeBridgeLinesPayload(result)
	if err != nil {
		return DockerUsageSummary{}, 0, errs.WithDetails(
			errs.Wrap(errs.CodeHostUsageFailed, "failed to decode docker usage payload", err),
			map[string]any{
				"task_type": contract.TaskTypeDockerSystemDF,
				"intent_id": result.IntentID,
			},
		)
	}

	var summary DockerUsageSummary
	var totalBytes int64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry dockerSystemDFLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return DockerUsageSummary{}, 0, fmt.Errorf("parse docker system df output: %w", err)
		}
		count := parseDockerCount(entry.TotalCount)
		sizeLabel := strings.TrimSpace(entry.Size)
		if bytes, ok := parseDockerSizeToBytes(sizeLabel); ok {
			totalBytes += bytes
		}
		switch strings.ToLower(entry.Type) {
		case "images":
			summary.Images = DockerUsageEntry{Count: count, Size: sizeLabel}
		case "containers":
			summary.Containers = DockerUsageEntry{Count: count, Size: sizeLabel}
		case "local volumes":
			summary.Volumes = DockerUsageEntry{Count: count, Size: sizeLabel}
		case "build cache":
			summary.BuildCache = DockerUsageEntry{Count: count, Size: sizeLabel}
		}
	}

	return summary, totalBytes, nil
}

func (s *HostService) listVolumes(ctx context.Context) ([]dockerVolumeLine, error) {
	if s.infraClient == nil {
		return nil, errs.WithDetails(
			errs.New(errs.CodeHostUsageFailed, "infra bridge client unavailable"),
			map[string]any{"task_type": contract.TaskTypeDockerListVolumes},
		)
	}

	result, err := s.infraClient.DockerListVolumes(ctx, "")
	if err != nil {
		return nil, bridgeTaskErrorWithCode(errs.CodeHostUsageFailed, "failed to load docker usage", contract.TaskTypeDockerListVolumes, "docker", err)
	}
	if err := bridgeResultErrorWithCode(errs.CodeHostUsageFailed, "failed to load docker usage", contract.TaskTypeDockerListVolumes, "docker", result); err != nil {
		return nil, err
	}

	lines, err := decodeBridgeLinesPayload(result)
	if err != nil {
		return nil, errs.WithDetails(
			errs.Wrap(errs.CodeHostUsageFailed, "failed to decode docker volume payload", err),
			map[string]any{
				"task_type": contract.TaskTypeDockerListVolumes,
				"intent_id": result.IntentID,
			},
		)
	}

	volumes := make([]dockerVolumeLine, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry dockerVolumeLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse docker volume output: %w", err)
		}
		volumes = append(volumes, entry)
	}
	return volumes, nil
}

func parseDockerLabels(raw string) map[string]string {
	labels := make(map[string]string)
	currentKey := ""
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if key, value, ok := parseDockerLabelEntry(entry); ok {
			currentKey = key
			labels[key] = value
			continue
		}

		if currentKey != "" {
			labels[currentKey] = labels[currentKey] + "," + entry
		}
	}
	return labels
}

func parseDockerLabelEntry(entry string) (string, string, bool) {
	keyRaw, valueRaw, ok := strings.Cut(entry, "=")
	if !ok {
		return "", "", false
	}
	key := strings.TrimSpace(keyRaw)
	if !isLikelyDockerLabelKey(key) {
		return "", "", false
	}
	return key, strings.TrimSpace(valueRaw), true
}

func isLikelyDockerLabelKey(key string) bool {
	if key == "" {
		return false
	}
	for idx, r := range key {
		if idx == 0 && !isASCIIAlphaNum(r) {
			return false
		}
		if isASCIIAlphaNum(r) {
			continue
		}
		switch r {
		case '.', '_', '-', '/':
			continue
		default:
			return false
		}
	}
	return true
}

func isASCIIAlphaNum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func parseDockerPorts(raw string) []DockerPortBinding {
	if strings.TrimSpace(raw) == "" {
		return []DockerPortBinding{}
	}

	var bindings []DockerPortBinding
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if strings.Contains(entry, "->") {
			parts := strings.Split(entry, "->")
			if len(parts) != 2 {
				continue
			}
			hostIP, hostPort := parseHostPort(parts[0])
			containerPort, proto := parseContainerPort(parts[1])
			bindings = append(bindings, DockerPortBinding{
				HostIP:        hostIP,
				HostPort:      hostPort,
				ContainerPort: containerPort,
				Proto:         proto,
				Published:     true,
			})
			continue
		}

		containerPort, proto := parseContainerPort(entry)
		bindings = append(bindings, DockerPortBinding{
			ContainerPort: containerPort,
			Proto:         proto,
			Published:     false,
		})
	}

	return bindings
}

func bridgeResultError(message string, taskType contract.TaskType, target string, result contract.Result) error {
	return bridgeResultErrorWithCode(errs.CodeHostDockerFailed, message, taskType, target, result)
}

func bridgeResultErrorWithCode(code errs.Code, message string, taskType contract.TaskType, target string, result contract.Result) error {
	if result.Status != contract.StatusFailed {
		return nil
	}
	failed := &infraclient.TaskFailedError{
		IntentID: result.IntentID,
		LogPath:  result.LogPath,
	}
	if result.Error != nil {
		failed.Code = result.Error.Code
		failed.Message = result.Error.Message
	}
	if strings.TrimSpace(failed.Message) == "" {
		failed.Message = "host worker reported failure"
	}
	return bridgeTaskErrorWithCode(code, message, taskType, target, failed)
}

func bridgeTaskError(message string, taskType contract.TaskType, target string, err error) error {
	return bridgeTaskErrorWithCode(errs.CodeHostDockerFailed, message, taskType, target, err)
}

func bridgeTaskErrorWithCode(code errs.Code, message string, taskType contract.TaskType, target string, err error) error {
	details := map[string]any{
		"task_type": taskType,
	}
	if strings.TrimSpace(target) != "" {
		details["target"] = target
	}

	var taskFailed *infraclient.TaskFailedError
	if errors.As(err, &taskFailed) {
		if strings.TrimSpace(taskFailed.IntentID) != "" {
			details["intent_id"] = taskFailed.IntentID
		}
		if strings.TrimSpace(taskFailed.Code) != "" {
			details["worker_error_code"] = taskFailed.Code
		}
		if strings.TrimSpace(taskFailed.LogPath) != "" {
			details["log_path"] = taskFailed.LogPath
		}
	}

	return errs.WithDetails(errs.Wrap(code, message, err), details)
}

func parseHostPort(raw string) (string, int) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", 0
	}
	lastColon := strings.LastIndex(trimmed, ":")
	if lastColon == -1 {
		return trimmed, 0
	}
	hostIP := strings.TrimSpace(trimmed[:lastColon])
	portStr := strings.TrimSpace(trimmed[lastColon+1:])
	port, _ := strconv.Atoi(portStr)
	return hostIP, port
}

func parseContainerPort(raw string) (int, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, ""
	}
	parts := strings.SplitN(trimmed, "/", 2)
	port, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	proto := ""
	if len(parts) == 2 {
		proto = strings.TrimSpace(parts[1])
	}
	return port, proto
}

func parseDockerCount(raw string) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return count
}

func parseDockerSizeToBytes(raw string) (int64, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || value == "0B" {
		return 0, true
	}
	var number float64
	var unit string
	if _, err := fmt.Sscanf(value, "%f%s", &number, &unit); err != nil {
		return 0, false
	}
	unit = strings.ToUpper(strings.TrimSpace(unit))
	multiplier := float64(1)
	switch unit {
	case "B":
		multiplier = 1
	case "KB", "KIB", "K":
		multiplier = 1024
	case "MB", "MIB", "M":
		multiplier = 1024 * 1024
	case "GB", "GIB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "TIB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, false
	}
	return int64(number * multiplier), true
}

func formatDockerBytes(value int64) string {
	if value <= 0 {
		return "0B"
	}
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%dB", value)
	}
	div, exp := int64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(value)/float64(div), "KMGTPE"[exp])
}
