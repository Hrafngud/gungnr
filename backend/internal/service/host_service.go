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

func (s *HostService) ListContainers(ctx context.Context) ([]DockerContainer, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{json .}}")
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
