package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
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

type HostService struct{}

func NewHostService() *HostService {
	return &HostService{}
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
	return s.runDockerCommand(ctx, "stop", container)
}

func (s *HostService) RestartContainer(ctx context.Context, container string) error {
	return s.runDockerCommand(ctx, "restart", container)
}

func (s *HostService) RemoveContainer(ctx context.Context, container string, removeVolumes bool) error {
	args := []string{"rm", "-f"}
	if removeVolumes {
		args = append(args, "-v")
	}
	args = append(args, container)
	return s.runDockerCommand(ctx, args...)
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

func (s *HostService) runDockerCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
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
