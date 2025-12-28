package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-notes/internal/jobs"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

type JobService struct {
	repo   repository.JobRepository
	runner *jobs.Runner
}

func NewJobService(repo repository.JobRepository, runner *jobs.Runner) *JobService {
	return &JobService{repo: repo, runner: runner}
}

func (s *JobService) List(ctx context.Context) ([]models.Job, error) {
	return s.repo.List(ctx)
}

func (s *JobService) Get(ctx context.Context, id uint) (*models.Job, error) {
	return s.repo.Get(ctx, id)
}

func (s *JobService) Create(ctx context.Context, jobType string, payload any) (*models.Job, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal job payload: %w", err)
	}

	job := models.Job{
		Type:   jobType,
		Status: "pending",
		Input:  string(body),
	}

	if err := s.repo.Create(ctx, &job); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}

	if s.runner != nil {
		if err := s.runner.Enqueue(ctx, job); err != nil {
			_ = s.repo.MarkFinished(ctx, job.ID, "failed", time.Now(), err.Error())
			return nil, fmt.Errorf("enqueue job: %w", err)
		}
	}

	return &job, nil
}

func (s *JobService) LogLines(job *models.Job) []string {
	if job == nil || job.LogLines == "" {
		return nil
	}
	raw := strings.TrimRight(job.LogLines, "\n")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}
