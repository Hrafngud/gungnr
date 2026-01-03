package repository

import (
	"context"
	"errors"
	"time"

	"go-notes/internal/models"
	"gorm.io/gorm"
)

type GormJobRepository struct {
	db *gorm.DB
}

func NewGormJobRepository(db *gorm.DB) *GormJobRepository {
	return &GormJobRepository{db: db}
}

func (r *GormJobRepository) List(ctx context.Context) ([]models.Job, error) {
	var jobs []models.Job
	if err := r.db.WithContext(ctx).Order("created_at desc").Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *GormJobRepository) Create(ctx context.Context, job *models.Job) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *GormJobRepository) Get(ctx context.Context, id uint) (*models.Job, error) {
	var job models.Job
	if err := r.db.WithContext(ctx).First(&job, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &job, nil
}

func (r *GormJobRepository) MarkRunning(ctx context.Context, id uint, startedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.Job{}).Where("id = ?", id).Updates(map[string]any{
		"status":     "running",
		"started_at": startedAt,
	}).Error
}

func (r *GormJobRepository) MarkFinished(ctx context.Context, id uint, status string, finishedAt time.Time, errMsg string) error {
	updates := map[string]any{
		"status":      status,
		"finished_at": finishedAt,
	}
	if errMsg != "" {
		updates["error"] = errMsg
	}
	return r.db.WithContext(ctx).Model(&models.Job{}).Where("id = ?", id).Updates(updates).Error
}

func (r *GormJobRepository) AppendLog(ctx context.Context, id uint, line string) error {
	if line == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&models.Job{}).Where("id = ?", id).
		Update("log_lines", gorm.Expr("COALESCE(log_lines, '') || ?", line)).Error
}
