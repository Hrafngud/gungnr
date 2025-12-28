package repository

import (
	"context"

	"go-notes/internal/models"
	"gorm.io/gorm"
)

type GormProjectRepository struct {
	db *gorm.DB
}

func NewGormProjectRepository(db *gorm.DB) *GormProjectRepository {
	return &GormProjectRepository{db: db}
}

func (r *GormProjectRepository) List(ctx context.Context) ([]models.Project, error) {
	var projects []models.Project
	if err := r.db.WithContext(ctx).Order("created_at desc").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}
