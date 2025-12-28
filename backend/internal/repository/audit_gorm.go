package repository

import (
	"context"

	"go-notes/internal/models"
	"gorm.io/gorm"
)

type GormAuditLogRepository struct {
	db *gorm.DB
}

func NewGormAuditLogRepository(db *gorm.DB) *GormAuditLogRepository {
	return &GormAuditLogRepository{db: db}
}

func (r *GormAuditLogRepository) List(ctx context.Context, limit int) ([]models.AuditLog, error) {
	var entries []models.AuditLog
	query := r.db.WithContext(ctx).Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *GormAuditLogRepository) Create(ctx context.Context, entry *models.AuditLog) error {
	return r.db.WithContext(ctx).Create(entry).Error
}
