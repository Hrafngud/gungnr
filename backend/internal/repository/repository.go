package repository

import (
	"context"
	"errors"

	"go-notes/internal/models"
)

var ErrNotFound = errors.New("record not found")

type UserRepository interface {
	UpsertFromGitHub(githubID int64, login, avatarURL string) (*models.User, error)
}

type ProjectRepository interface {
	List(ctx context.Context) ([]models.Project, error)
}

type JobRepository interface {
	List(ctx context.Context) ([]models.Job, error)
}
