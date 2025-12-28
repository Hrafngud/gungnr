package repository

import (
	"context"
	"errors"
	"time"

	"go-notes/internal/models"
)

var ErrNotFound = errors.New("record not found")

type UserRepository interface {
	UpsertFromGitHub(githubID int64, login, avatarURL string) (*models.User, error)
}

type ProjectRepository interface {
	List(ctx context.Context) ([]models.Project, error)
	Create(ctx context.Context, project *models.Project) error
	GetByName(ctx context.Context, name string) (*models.Project, error)
	Update(ctx context.Context, project *models.Project) error
}

type JobRepository interface {
	List(ctx context.Context) ([]models.Job, error)
	Create(ctx context.Context, job *models.Job) error
	Get(ctx context.Context, id uint) (*models.Job, error)
	MarkRunning(ctx context.Context, id uint, startedAt time.Time) error
	MarkFinished(ctx context.Context, id uint, status string, finishedAt time.Time, errMsg string) error
	AppendLog(ctx context.Context, id uint, line string) error
}

type SettingsRepository interface {
	Get(ctx context.Context) (*models.Settings, error)
	Save(ctx context.Context, settings *models.Settings) error
}

type AuditLogRepository interface {
	List(ctx context.Context, limit int) ([]models.AuditLog, error)
	Create(ctx context.Context, entry *models.AuditLog) error
}
