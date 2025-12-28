package repository

import (
	"errors"

	"go-notes/internal/models"
)

var ErrNotFound = errors.New("record not found")

type UserRepository interface {
	UpsertFromGitHub(githubID int64, login, avatarURL string) (*models.User, error)
}
