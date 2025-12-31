package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-notes/internal/jobs"
	"go-notes/internal/models"
)

type DockerWorkflows struct {
	runner *DockerRunner
}

func NewDockerWorkflows(runner *DockerRunner) *DockerWorkflows {
	return &DockerWorkflows{runner: runner}
}

func (w *DockerWorkflows) Register(runner *jobs.Runner) {
	if runner == nil {
		return
	}
	runner.Register(JobTypeDockerRun, w.handleDockerRun)
	runner.Register(JobTypeDockerCompose, w.handleDockerCompose)
}

func (w *DockerWorkflows) handleDockerRun(ctx context.Context, job models.Job, logger jobs.Logger) error {
	if w.runner == nil {
		return fmt.Errorf("docker runner unavailable")
	}
	var req DockerRunRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse docker run request: %w", err)
	}
	req.Image = strings.TrimSpace(req.Image)
	req.ContainerName = strings.TrimSpace(req.ContainerName)
	if req.ContainerPort == 0 {
		req.ContainerPort = defaultQuickServiceContainerPort
	}
	if req.ContainerName != "" {
		if err := validateContainerName(req.ContainerName); err != nil {
			return err
		}
	}
	return w.runner.RunContainer(ctx, logger, req)
}

func (w *DockerWorkflows) handleDockerCompose(ctx context.Context, job models.Job, logger jobs.Logger) error {
	if w.runner == nil {
		return fmt.Errorf("docker runner unavailable")
	}
	var req DockerComposeRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse docker compose request: %w", err)
	}
	req.ProjectDir = strings.TrimSpace(req.ProjectDir)
	return w.runner.ComposeUp(ctx, logger, req)
}
