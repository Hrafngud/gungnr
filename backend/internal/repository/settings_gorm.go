package repository

import (
	"context"
	"errors"

	"go-notes/internal/models"
	"gorm.io/gorm"
)

type GormSettingsRepository struct {
	db *gorm.DB
}

func NewGormSettingsRepository(db *gorm.DB) *GormSettingsRepository {
	return &GormSettingsRepository{db: db}
}

func (r *GormSettingsRepository) Get(ctx context.Context) (*models.Settings, error) {
	var settings models.Settings
	if err := r.db.WithContext(ctx).Order("id asc").First(&settings).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &settings, nil
}

func (r *GormSettingsRepository) Save(ctx context.Context, settings *models.Settings) error {
	return r.db.WithContext(ctx).Save(settings).Error
}
