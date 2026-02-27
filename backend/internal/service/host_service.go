package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

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

type HostService struct {
	templatesDir string
	projects     repository.ProjectRepository
	infraClient  hostInfraBridgeClient
}

type hostInfraBridgeClient interface {
	StopContainer(ctx context.Context, requestID, container string) (contract.Result, error)
	RestartContainer(ctx context.Context, requestID, container string) (contract.Result, error)
	RemoveContainer(ctx context.Context, requestID, container string, removeVolumes bool) (contract.Result, error)
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

func (s *HostService) ListContainers(ctx context.Context, includeAll bool) ([]DockerContainer, error) {
	args := []string{"ps"}
	if includeAll {
		args = append(args, "-a")
	}
	args = append(args, "--format", "{{json .}}")
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var containers []DockerContainer

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan docker output: %w", err)
	}

	return containers, nil
}

func (s *HostService) CountRunningContainers(ctx context.Context) (int, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.ID}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("docker ps failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	count := 0
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
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

	containers, err := s.ListContainers(ctx, true)
	if err != nil {
		return DockerUsageSummary{}, err
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
		return DockerUsageSummary{}, err
	}
	volumeCount := 0
	for _, volume := range volumes {
		labels := parseDockerLabels(volume.Labels)
		if strings.ToLower(labels["com.docker.compose.project"]) == normalizedProject {
			volumeCount++
		}
	}

	summary.Project = project
	summary.ProjectCounts = &DockerUsageProjectCounts{
		Containers: len(projectContainers),
		Images:     len(imageSet),
		Volumes:    volumeCount,
	}
	return summary, nil
}

func (s *HostService) StartContainerLogs(ctx context.Context, container string, opts ContainerLogsOptions) (*exec.Cmd, io.ReadCloser, error) {
	args := []string{"logs"}
	if opts.Follow {
		args = append(args, "-f")
	}
	if opts.Timestamps {
		args = append(args, "--timestamps")
	}
	if opts.Tail > 0 {
		args = append(args, "--tail", strconv.Itoa(opts.Tail))
	}
	args = append(args, container)

	cmd := exec.CommandContext(ctx, "docker", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("attach docker logs: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start docker logs: %w", err)
	}
	return cmd, stdout, nil
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
		Project: project,
		Build:   true,
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

func (s *HostService) readComposeProjectMeta(ctx context.Context, project string) (composeProjectMeta, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "-q", "--filter", "label=com.docker.compose.project="+project)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return composeProjectMeta{}, fmt.Errorf("docker ps compose metadata failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	containerIDs := parseLines(output)
	if len(containerIDs) == 0 {
		return composeProjectMeta{}, fmt.Errorf("compose project not found: %q", project)
	}

	var lastErr error
	for _, containerID := range containerIDs {
		labels, err := inspectContainerLabels(ctx, containerID)
		if err != nil {
			lastErr = err
			continue
		}
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
	if lastErr != nil {
		return composeProjectMeta{}, fmt.Errorf("inspect compose metadata failed: %w", lastErr)
	}
	return composeProjectMeta{}, fmt.Errorf("compose metadata labels unavailable for project %q", project)
}

func inspectContainerLabels(ctx context.Context, containerID string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .Config.Labels}}", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker inspect labels failed for %s: %w: %s", containerID, err, strings.TrimSpace(string(output)))
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" || trimmed == "null" {
		return map[string]string{}, nil
	}
	labels := map[string]string{}
	if err := json.Unmarshal([]byte(trimmed), &labels); err != nil {
		return nil, fmt.Errorf("parse docker inspect labels for %s: %w", containerID, err)
	}
	return labels, nil
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

func (s *HostService) composeUp(ctx context.Context, projectDir string, configFiles []string, logger jobs.Logger) error {
	args := []string{"compose"}
	for _, configFile := range configFiles {
		args = append(args, "-f", configFile)
	}
	args = append(args, "up", "--build", "-d")

	hostLogf(logger, "running command in %s: docker %s", projectDir, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	logCommandOutput(logger, output)
	if err != nil {
		return fmt.Errorf("docker %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func hostLogf(logger jobs.Logger, format string, args ...any) {
	if logger == nil {
		return
	}
	logger.Logf(format, args...)
}

func logCommandOutput(logger jobs.Logger, output []byte) {
	if logger == nil {
		return
	}
	for _, line := range parseLines(output) {
		logger.Log(line)
	}
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
	cmd := exec.CommandContext(ctx, "docker", "system", "df", "--format", "{{json .}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return DockerUsageSummary{}, 0, fmt.Errorf("docker system df failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	var summary DockerUsageSummary
	var totalBytes int64

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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

	if err := scanner.Err(); err != nil {
		return DockerUsageSummary{}, 0, fmt.Errorf("scan docker system df output: %w", err)
	}

	return summary, totalBytes, nil
}

func (s *HostService) listVolumes(ctx context.Context) ([]dockerVolumeLine, error) {
	cmd := exec.CommandContext(ctx, "docker", "volume", "ls", "--format", "{{json .}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker volume ls failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	volumes := make([]dockerVolumeLine, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry dockerVolumeLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse docker volume output: %w", err)
		}
		volumes = append(volumes, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan docker volume output: %w", err)
	}
	return volumes, nil
}

func parseDockerLabels(raw string) map[string]string {
	labels := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" {
			labels[key] = value
		}
	}
	return labels
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
	return bridgeTaskError(message, taskType, target, failed)
}

func bridgeTaskError(message string, taskType contract.TaskType, target string, err error) error {
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

	return errs.WithDetails(errs.Wrap(errs.CodeHostDockerFailed, message, err), details)
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
