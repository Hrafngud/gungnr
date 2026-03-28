package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"
	"go-notes/internal/jobs"
)

const (
	defaultQuickServiceImage         = "excalidraw/excalidraw:latest"
	defaultQuickServiceContainerPort = 80
	defaultComposeUpWaitTimeout      = 30 * time.Minute
)

type DockerRunner struct {
	infra dockerRunnerInfraClient
}

type dockerRunnerInfraClient interface {
	infraPortProbeClient
	DockerRuntimeCheck(ctx context.Context, requestID string) (contract.Result, error)
	DockerListContainers(ctx context.Context, requestID string, includeAll bool) (contract.Result, error)
	DockerRunQuickService(ctx context.Context, requestID string, payload contract.DockerRunQuickServicePayload) (contract.Result, error)
	ComposeUpStack(ctx context.Context, requestID string, payload contract.ComposeUpStackPayload) (contract.Result, error)
}

type DockerRunRequest struct {
	Image         string `json:"image"`
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	ContainerName string `json:"containerName,omitempty"`
	ExposureMode  string `json:"exposureMode,omitempty"`
}

type DockerComposeRequest struct {
	ProjectDir string `json:"projectDir"`
}

func NewDockerRunner(infra dockerRunnerInfraClient) *DockerRunner {
	return &DockerRunner{infra: infra}
}

func (r *DockerRunner) RunContainer(ctx context.Context, logger jobs.Logger, req DockerRunRequest) error {
	if err := r.ensureDocker(ctx); err != nil {
		return err
	}

	exposureMode, err := normalizeQuickServiceExposureRequest(req.ExposureMode, req.HostPort)
	if err != nil {
		return err
	}

	image := strings.TrimSpace(req.Image)
	if image == "" {
		return fmt.Errorf("docker image is required")
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

	if err := ValidatePort(req.HostPort); err != nil {
		return err
	}
	if inUse, err := r.isPortInUse(ctx, req.HostPort); err != nil {
		return err
	} else if inUse {
		return fmt.Errorf("host port %d is already in use", req.HostPort)
	}

	containers, err := r.listContainers(ctx)
	if err != nil {
		return err
	}
	names := make(map[string]struct{}, len(containers))
	for _, container := range containers {
		trimmed := strings.TrimSpace(container.Name)
		if trimmed == "" {
			continue
		}
		names[trimmed] = struct{}{}
	}
	uniqueName := ensureUniqueContainerName(name, names)
	if uniqueName != name {
		logger.Logf("container name %q already exists; using %q", name, uniqueName)
		name = uniqueName
	}

	networkName := inferQuickServiceNetworkName(containers)
	capabilityProfile := "none"
	if containerPort < 1024 {
		capabilityProfile = "NET_BIND_SERVICE"
	}
	logger.Logf(
		"quick-service policy: exposure=%s network=%s publish=%s:%d restart=unless-stopped no-new-privileges=true cap-drop=ALL cap-add=%s pids-limit=%d memory=%s cpus=%s",
		exposureMode,
		networkName,
		contract.QuickServicePublishLoopbackHost,
		req.HostPort,
		capabilityProfile,
		contract.QuickServiceDefaultPIDsLimit,
		contract.QuickServiceDefaultMemory,
		contract.QuickServiceDefaultCPUs,
	)
	logger.Logf("quick-service publish: %s:%d -> %d/tcp", contract.QuickServicePublishLoopbackHost, req.HostPort, containerPort)
	logger.Logf("starting docker container %s (%s)", name, image)
	result, err := r.infra.DockerRunQuickService(ctx, "", contract.DockerRunQuickServicePayload{
		Image:         image,
		HostPort:      req.HostPort,
		ContainerPort: containerPort,
		ContainerName: name,
		ExposureMode:  exposureMode,
		NetworkName:   networkName,
		PublishHost:   contract.QuickServicePublishLoopbackHost,
	})
	if err != nil {
		return bridgeTaskError("failed to start docker container", contract.TaskTypeDockerRunQuickService, name, err)
	}
	if err := bridgeResultError("failed to start docker container", contract.TaskTypeDockerRunQuickService, name, result); err != nil {
		return err
	}
	logBridgeResultTail(logger, result)
	return nil
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
	project := strings.TrimSpace(filepath.Base(dir))
	if project == "" || project == "." || project == string(filepath.Separator) {
		project = "compose-stack"
	}

	composeCtx, cancel := withComposeUpWaitTimeout(ctx)
	defer cancel()

	result, err := r.infra.ComposeUpStack(composeCtx, "", contract.ComposeUpStackPayload{
		Project:    project,
		ProjectDir: dir,
		Build:      true,
	})
	if err != nil {
		return bridgeTaskError("failed to start docker compose stack", contract.TaskTypeComposeUpStack, project, err)
	}
	if err := bridgeResultError("failed to start docker compose stack", contract.TaskTypeComposeUpStack, project, result); err != nil {
		return err
	}
	logBridgeResultTail(logger, result)
	return nil
}

func (r *DockerRunner) ensureDocker(ctx context.Context) error {
	if r.infra == nil {
		return fmt.Errorf("infra bridge client unavailable")
	}
	result, err := r.infra.DockerRuntimeCheck(ctx, "")
	if err != nil {
		return bridgeTaskError("docker unavailable", contract.TaskTypeDockerRuntimeCheck, "docker", err)
	}
	if err := bridgeResultError("docker unavailable", contract.TaskTypeDockerRuntimeCheck, "docker", result); err != nil {
		return err
	}
	return nil
}

func (r *DockerRunner) containerExists(ctx context.Context, name string) (bool, error) {
	names, err := r.listContainerNames(ctx)
	if err != nil {
		return false, err
	}
	_, ok := names[name]
	return ok, nil
}

func (r *DockerRunner) listContainerNames(ctx context.Context) (map[string]struct{}, error) {
	containers, err := r.listContainers(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{})
	for _, container := range containers {
		trimmed := strings.TrimSpace(container.Name)
		if trimmed == "" {
			continue
		}
		names[trimmed] = struct{}{}
	}
	return names, nil
}

func (r *DockerRunner) listContainers(ctx context.Context) ([]DockerContainer, error) {
	if r.infra == nil {
		return nil, fmt.Errorf("infra bridge client unavailable")
	}
	result, err := r.infra.DockerListContainers(ctx, "", true)
	if err != nil {
		return nil, bridgeTaskError("docker ps failed", contract.TaskTypeDockerListContainers, "docker", err)
	}
	if err := bridgeResultError("docker ps failed", contract.TaskTypeDockerListContainers, "docker", result); err != nil {
		return nil, err
	}

	lines, err := decodeBridgeLinesPayload(result)
	if err != nil {
		return nil, fmt.Errorf("decode docker ps payload: %w", err)
	}
	containers, err := parseDockerPSLinesToContainers(lines)
	if err != nil {
		return nil, fmt.Errorf("parse docker ps payload: %w", err)
	}
	return containers, nil
}

func (r *DockerRunner) isPortInUse(ctx context.Context, port int) (bool, error) {
	if err := ValidatePort(port); err != nil {
		return false, err
	}

	if ports, err := listDockerPublishedPorts(ctx, r.infra); err == nil {
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
		return errs.New(errs.CodeContainerName, fmt.Sprintf("container name %q is invalid; use letters, numbers, '.', '_' or '-'", name))
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

func ensureUniqueContainerName(base string, existing map[string]struct{}) string {
	if _, ok := existing[base]; !ok {
		return base
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if _, ok := existing[candidate]; !ok {
			return candidate
		}
	}
}

func logBridgeResultTail(logger jobs.Logger, result contract.Result) {
	if logger == nil {
		return
	}
	for _, line := range result.LogTail {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		logger.Log(trimmed)
	}
}

func withComposeUpWaitTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultComposeUpWaitTimeout)
}
