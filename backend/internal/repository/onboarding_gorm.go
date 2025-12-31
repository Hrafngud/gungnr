package repository

import (
	"context"
	"errors"

	"go-notes/internal/models"
	"gorm.io/gorm"
)

type GormOnboardingRepository struct {
	db *gorm.DB
}

func NewGormOnboardingRepository(db *gorm.DB) *GormOnboardingRepository {
	return &GormOnboardingRepository{db: db}
}

func (r *GormOnboardingRepository) GetByUser(ctx context.Context, userID uint) (*models.OnboardingState, error) {
	var state models.OnboardingState
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &state, nil
}

func (r *GormOnboardingRepository) Save(ctx context.Context, state *models.OnboardingState) error {
	return r.db.WithContext(ctx).Save(state).Error
}
