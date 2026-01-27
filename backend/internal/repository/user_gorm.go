package repository

import (
	"errors"
	"time"

	"go-notes/internal/errs"
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
				Role:        models.RoleUser,
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
	if user.Role == "" {
		user.Role = models.RoleUser
	}
	user.LastLoginAt = now

	if err := r.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *GormUserRepository) CreateAllowlistUser(githubID int64, login, avatarURL string) (*models.User, error) {
	var user models.User
	err := r.db.Where("git_hub_id = ?", githubID).First(&user).Error
	if err == nil {
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	user = models.User{
		GitHubID:    githubID,
		Login:       login,
		AvatarURL:   avatarURL,
		Role:        models.RoleUser,
		LastLoginAt: time.Time{},
	}
	if err := r.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) GetByGitHubID(githubID int64) (*models.User, error) {
	var user models.User
	if err := r.db.Where("git_hub_id = ?", githubID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.Wrap(errs.CodeNotFound, "record not found", ErrNotFound)
		}
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) GetByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.Wrap(errs.CodeNotFound, "record not found", ErrNotFound)
		}
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) UpsertSuperUser(githubID int64, login string) (*models.User, error) {
	var user models.User
	err := r.db.Where(&models.User{GitHubID: githubID}).First(&user).Error
	now := time.Now()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = models.User{
				GitHubID:    githubID,
				Login:       login,
				Role:        models.RoleSuperUser,
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
	user.Role = models.RoleSuperUser
	if user.LastLoginAt.IsZero() {
		user.LastLoginAt = now
	}

	if err := r.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *GormUserRepository) ListByRole(role string) ([]models.User, error) {
	var users []models.User
	if err := r.db.Where("role = ?", role).Order("id asc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *GormUserRepository) ListAll() ([]models.User, error) {
	var users []models.User
	if err := r.db.Order("id asc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *GormUserRepository) CountByRole(role string) (int64, error) {
	var count int64
	if err := r.db.Model(&models.User{}).Where("role = ?", role).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormUserRepository) UpdateRole(id uint, role string) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Update("role", role).Error
}

func (r *GormUserRepository) DeleteByIDs(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("id IN ?", ids).Delete(&models.User{}).Error
}
