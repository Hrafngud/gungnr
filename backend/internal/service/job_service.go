package service

import (
	"context"

	"go-notes/internal/models"
	"go-notes/internal/repository"
)

type JobService struct {
	repo repository.JobRepository
}

func NewJobService(repo repository.JobRepository) *JobService {
	return &JobService{repo: repo}
}

func (s *JobService) List(ctx context.Context) ([]models.Job, error) {
	return s.repo.List(ctx)
}
