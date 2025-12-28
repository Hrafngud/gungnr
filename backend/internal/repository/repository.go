package repository

import (
	"errors"

	"go-notes/internal/models"
)

var ErrNotFound = errors.New("record not found")

type NoteRepository interface {
	Create(note *models.Note) error
	GetByID(id uint) (*models.Note, error)
	List() ([]models.Note, error)
	Update(note *models.Note) error
	Delete(id uint) error
}
