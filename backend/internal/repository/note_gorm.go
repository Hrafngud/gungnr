package repository

import (
	"errors"

	"go-notes/internal/models"
	"gorm.io/gorm"
)

type GormNoteRepository struct {
	db *gorm.DB
}

func NewGormNoteRepository(db *gorm.DB) *GormNoteRepository {
	return &GormNoteRepository{db: db}
}

func (r *GormNoteRepository) Create(note *models.Note) error {
	return r.db.Create(note).Error
}

func (r *GormNoteRepository) GetByID(id uint) (*models.Note, error) {
	var note models.Note
	if err := r.db.First(&note, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &note, nil
}

func (r *GormNoteRepository) List() ([]models.Note, error) {
	var notes []models.Note
	if err := r.db.Order("created_at DESC").Find(&notes).Error; err != nil {
		return nil, err
	}
	return notes, nil
}

func (r *GormNoteRepository) Update(note *models.Note) error {
	res := r.db.Model(&models.Note{}).
		Where("id = ?", note.ID).
		Updates(map[string]interface{}{
			"title":      note.Title,
			"content":    note.Content,
			"tags":       note.Tags,
			"updated_at": gorm.Expr("NOW()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormNoteRepository) Delete(id uint) error {
	res := r.db.Where("id = ?", id).Delete(&models.Note{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
