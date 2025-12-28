package service

import (
	"errors"
	"fmt"
	"strings"

	"go-notes/internal/models"
	"go-notes/internal/repository"
)

const (
	maxTitleLength   = 255
	maxContentLength = 10000
)

var (
	ErrNoteNotFound = errors.New("note not found")
)

type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Fields)
}

type NoteInput struct {
	Title   string
	Content string
	Tags    string
}

type NoteService struct {
	repo repository.NoteRepository
}

func NewNoteService(repo repository.NoteRepository) *NoteService {
	return &NoteService{repo: repo}
}

func (s *NoteService) CreateNote(input NoteInput) (*models.Note, error) {
	if err := validateNoteInput(input); err != nil {
		return nil, err
	}

	note := &models.Note{
		Title:   strings.TrimSpace(input.Title),
		Content: strings.TrimSpace(input.Content),
		Tags:    strings.TrimSpace(input.Tags),
	}

	if err := s.repo.Create(note); err != nil {
		return nil, err
	}

	return note, nil
}

func (s *NoteService) GetNote(id uint) (*models.Note, error) {
	note, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}
	return note, nil
}

func (s *NoteService) ListNotes() ([]models.Note, error) {
	return s.repo.List()
}

func (s *NoteService) UpdateNote(id uint, input NoteInput) (*models.Note, error) {
	if err := validateNoteInput(input); err != nil {
		return nil, err
	}

	note := &models.Note{
		Title:   strings.TrimSpace(input.Title),
		Content: strings.TrimSpace(input.Content),
		Tags:    strings.TrimSpace(input.Tags),
	}
	note.ID = id

	if err := s.repo.Update(note); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}

	updated, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}

	return updated, nil
}

func (s *NoteService) DeleteNote(id uint) error {
	if err := s.repo.Delete(id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNoteNotFound
		}
		return err
	}
	return nil
}

func validateNoteInput(input NoteInput) error {
	errorsMap := make(map[string]string)

	title := strings.TrimSpace(input.Title)
	if title == "" {
		errorsMap["title"] = "title is required"
	}
	if len(title) > maxTitleLength {
		errorsMap["title"] = fmt.Sprintf("title must be at most %d characters", maxTitleLength)
	}

	content := strings.TrimSpace(input.Content)
	if len(content) > maxContentLength {
		errorsMap["content"] = fmt.Sprintf("content must be at most %d characters", maxContentLength)
	}

	if len(errorsMap) > 0 {
		return ValidationError{Fields: errorsMap}
	}

	return nil
}
