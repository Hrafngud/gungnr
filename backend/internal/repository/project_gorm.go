package repository

import (
	"context"
	"errors"

	"go-notes/internal/errs"
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

func (r *GormProjectRepository) Create(ctx context.Context, project *models.Project) error {
	return r.db.WithContext(ctx).Create(project).Error
}

func (r *GormProjectRepository) GetByName(ctx context.Context, name string) (*models.Project, error) {
	var project models.Project
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&project).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.Wrap(errs.CodeNotFound, "record not found", ErrNotFound)
		}
		return nil, err
	}
	return &project, nil
}

func (r *GormProjectRepository) Update(ctx context.Context, project *models.Project) error {
	return r.db.WithContext(ctx).Save(project).Error
}
