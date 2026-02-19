package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-notes/internal/jobs"
	"go-notes/internal/models"
	"go-notes/internal/utils/httpx"
)

type RestartProjectStackRequest struct {
	Project string `json:"project"`
}

type HostWorkflows struct {
	host *HostService
}

func NewHostWorkflows(host *HostService) *HostWorkflows {
	return &HostWorkflows{host: host}
}

func (w *HostWorkflows) Register(runner *jobs.Runner) {
	if runner == nil {
		return
	}
	runner.Register(JobTypeHostRestart, w.handleRestartProjectStack)
}

func (w *HostWorkflows) handleRestartProjectStack(ctx context.Context, job models.Job, logger jobs.Logger) error {
	if w.host == nil {
		return fmt.Errorf("host service unavailable")
	}
	var req RestartProjectStackRequest
	if err := json.Unmarshal([]byte(job.Input), &req); err != nil {
		return fmt.Errorf("parse restart project request: %w", err)
	}
	req.Project = strings.TrimSpace(req.Project)
	if req.Project == "" {
		return fmt.Errorf("project is required")
	}
	if req.Project == "." || req.Project == ".." || !httpx.IsSafeRef(req.Project) {
		return fmt.Errorf("invalid project name")
	}

	logger.Logf("restarting compose stack for project %q", req.Project)
	if err := w.host.RestartProjectStackWithLogger(ctx, fmt.Sprintf("job-%d", job.ID), req.Project, logger); err != nil {
		return err
	}
	logger.Logf("compose restart completed for project %q", req.Project)
	return nil
}
