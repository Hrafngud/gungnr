package repository

import (
	"context"

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
