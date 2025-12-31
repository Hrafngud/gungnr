package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"syscall"

	"go-notes/internal/jobs"
)

const (
	defaultQuickServiceImage         = "excalidraw/excalidraw:latest"
	defaultQuickServiceContainerPort = 80
)

type DockerRunner struct{}

type DockerRunRequest struct {
	Image         string `json:"image"`
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	ContainerName string `json:"containerName,omitempty"`
}

type DockerComposeRequest struct {
	ProjectDir string `json:"projectDir"`
}

func NewDockerRunner() *DockerRunner {
	return &DockerRunner{}
}

func (r *DockerRunner) RunContainer(ctx context.Context, logger jobs.Logger, req DockerRunRequest) error {
	if err := r.ensureDocker(ctx); err != nil {
		return err
	}

	image := strings.TrimSpace(req.Image)
	if image == "" {
		return fmt.Errorf("docker image is required")
	}
	if err := ValidatePort(req.HostPort); err != nil {
		return err
	}
	containerPort := req.ContainerPort
	if containerPort == 0 {
		containerPort = defaultQuickServiceContainerPort
	}
	if err := ValidatePort(containerPort); err != nil {
		return err
	}

	name := strings.TrimSpace(req.ContainerName)
	if name == "" {
		name = inferContainerName(image)
	} else if err := validateContainerName(name); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("container name is required")
	}

	if inUse, err := isPortInUse(ctx, req.HostPort); err != nil {
		return err
	} else if inUse {
		return fmt.Errorf("host port %d is already in use", req.HostPort)
	}

	exists, err := r.containerExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("container name %q already exists", name)
	}

	logger.Logf("starting docker container %s (%s)", name, image)
	args := []string{
		"run",
		"-d",
		"--restart",
		"unless-stopped",
		"-p",
		fmt.Sprintf("%d:%d", req.HostPort, containerPort),
		"--name",
		name,
		image,
	}

	return runLoggedCommand(ctx, logger, "", nil, "docker", args...)
}

func (r *DockerRunner) ComposeUp(ctx context.Context, logger jobs.Logger, req DockerComposeRequest) error {
	if err := r.ensureDocker(ctx); err != nil {
		return err
	}

	dir := strings.TrimSpace(req.ProjectDir)
	if dir == "" {
		return fmt.Errorf("project directory is required")
	}

	logger.Log("starting docker compose stack")
	return runLoggedCommand(ctx, logger, dir, nil, "docker", "compose", "up", "--build", "-d")
}

func (r *DockerRunner) ensureDocker(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker unavailable: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (r *DockerRunner) containerExists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("docker ps failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

func isPortInUse(ctx context.Context, port int) (bool, error) {
	if err := ValidatePort(port); err != nil {
		return false, err
	}

	if ports, err := listDockerPublishedPorts(ctx); err == nil {
		for _, published := range ports {
			if published == port {
				return true, nil
			}
		}
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		if isPermissionError(err) {
			return false, nil
		}
		return true, nil
	}
	if err := ln.Close(); err != nil {
		return true, nil
	}
	return false, nil
}

func isPermissionError(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		err = opErr.Err
	}
	return errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM)
}

var containerNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

func validateContainerName(name string) error {
	if !containerNamePattern.MatchString(name) {
		return fmt.Errorf("container name %q is invalid; use letters, numbers, '.', '_' or '-'", name)
	}
	return nil
}

func inferContainerName(image string) string {
	trimmed := strings.TrimSpace(image)
	if trimmed == "" {
		return ""
	}
	last := path.Base(trimmed)
	if idx := strings.Index(last, "@"); idx != -1 {
		last = last[:idx]
	}
	if idx := strings.LastIndex(last, ":"); idx != -1 {
		last = last[:idx]
	}
	return sanitizeContainerName(last)
}

func sanitizeContainerName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	for _, ch := range trimmed {
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteRune(ch)
		case ch >= 'A' && ch <= 'Z':
			b.WriteRune(ch + ('a' - 'A'))
		case ch >= '0' && ch <= '9':
			b.WriteRune(ch)
		case ch == '.' || ch == '_' || ch == '-':
			b.WriteRune(ch)
		default:
			b.WriteByte('-')
		}
	}
	sanitized := strings.Trim(b.String(), "._-")
	if sanitized == "" {
		return ""
	}
	if err := validateContainerName(sanitized); err != nil {
		return ""
	}
	return sanitized
}
