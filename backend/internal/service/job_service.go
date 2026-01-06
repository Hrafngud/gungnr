package service

import (
	"context"
	"encoding/json"
	"errors"
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

var (
	ErrJobAlreadyFinished = errors.New("job already finished")
	ErrJobRunning         = errors.New("job already running")
	ErrJobNotStoppable    = errors.New("job is not stoppable")
	ErrJobNotRetryable    = errors.New("job is not retryable")
)

func NewJobService(repo repository.JobRepository, runner *jobs.Runner) *JobService {
	return &JobService{repo: repo, runner: runner}
}

func (s *JobService) List(ctx context.Context) ([]models.Job, error) {
	return s.repo.List(ctx)
}

func (s *JobService) ListPage(ctx context.Context, page int, pageSize int) ([]models.Job, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 1
	}
	offset := (page - 1) * pageSize
	return s.repo.ListPage(ctx, offset, pageSize)
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

func (s *JobService) Stop(ctx context.Context, id uint, errMsg string) (*models.Job, error) {
	job, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	switch job.Status {
	case "completed", "failed":
		return nil, ErrJobAlreadyFinished
	case "running":
		return nil, ErrJobRunning
	case "pending":
		// ok
	default:
		return nil, ErrJobNotStoppable
	}

	message := strings.TrimSpace(errMsg)
	if message == "" {
		message = "manually stopped"
	}

	finishedAt := time.Now()
	if err := s.repo.MarkFinished(ctx, job.ID, "failed", finishedAt, message); err != nil {
		return nil, err
	}
	_ = s.repo.AppendLog(ctx, job.ID, fmt.Sprintf("job marked failed: %s\n", message))

	job.Status = "failed"
	job.FinishedAt = &finishedAt
	job.Error = message

	return job, nil
}

func (s *JobService) Retry(ctx context.Context, id uint) (*models.Job, error) {
	job, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	switch job.Status {
	case "failed":
		// ok
	case "running":
		return nil, ErrJobRunning
	case "completed":
		return nil, ErrJobAlreadyFinished
	case "pending":
		return nil, ErrJobNotRetryable
	default:
		return nil, ErrJobNotRetryable
	}

	retry := models.Job{
		Type:   job.Type,
		Status: "pending",
		Input:  job.Input,
	}

	if err := s.repo.Create(ctx, &retry); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}

	if s.runner != nil {
		if err := s.runner.Enqueue(ctx, retry); err != nil {
			_ = s.repo.MarkFinished(ctx, retry.ID, "failed", time.Now(), err.Error())
			return nil, fmt.Errorf("enqueue job: %w", err)
		}
	}

	return &retry, nil
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
