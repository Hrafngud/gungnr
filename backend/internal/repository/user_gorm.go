package repository

import (
	"errors"
	"time"

	"go-notes/internal/models"
	"gorm.io/gorm"
)

type GormUserRepository struct {
	db *gorm.DB
}

func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) UpsertFromGitHub(githubID int64, login, avatarURL string) (*models.User, error) {
	var user models.User
	err := r.db.Where(&models.User{GitHubID: githubID}).First(&user).Error
	now := time.Now()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = models.User{
				GitHubID:    githubID,
				Login:       login,
				AvatarURL:   avatarURL,
				LastLoginAt: now,
			}
			if err := r.db.Create(&user).Error; err != nil {
				return nil, err
			}
			return &user, nil
		}
		return nil, err
	}

	user.Login = login
	user.AvatarURL = avatarURL
	user.LastLoginAt = now

	if err := r.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
