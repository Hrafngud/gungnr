package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go-notes/internal/errs"
	ghintegration "go-notes/internal/integrations/github"
	"go-notes/internal/models"
	"go-notes/internal/repository"
	"go-notes/internal/sanitize"
	"go-notes/internal/validate"

	"github.com/google/go-github/v62/github"
)

// Sentinel errors kept temporarily for apierror.mapKnown() compatibility
// during migration. Will be removed in Phase 4.
var ErrLastSuperUser = errors.New("cannot demote last superuser")
var ErrAllowlistUserNotFound = errors.New("github user not found")
var ErrAllowlistLoginRequired = errors.New("github login required")
var ErrCannotRemoveSuperUser = errors.New("cannot remove superuser")
var ErrUserNotFound = errors.New("user not found")

type UserService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

func (s *UserService) List(ctx context.Context) ([]models.User, error) {
	return s.userRepo.ListAll()
}

func (s *UserService) UpdateRole(ctx context.Context, userID uint, rawRole string) (*models.User, error) {
	role := sanitize.Role(rawRole)
	if err := validate.UserID(userID); err != nil {
		return nil, err
	}
	if err := validate.UserRole(role); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errs.New(errs.CodeUserNotFound, "user not found")
		}
		return nil, err
	}

	if user.Role == models.RoleSuperUser && role != models.RoleSuperUser {
		superUsers, err := s.userRepo.CountByRole(models.RoleSuperUser)
		if err != nil {
			return nil, fmt.Errorf("count superusers: %w", err)
		}
		if superUsers <= 1 {
			return nil, errs.New(errs.CodeUserLastSuperUser, "cannot demote last superuser")
		}
	}

	if err := s.userRepo.UpdateRole(user.ID, role); err != nil {
		return nil, fmt.Errorf("update user role: %w", err)
	}

	user.Role = role
	return user, nil
}

func (s *UserService) AddAllowlistUser(ctx context.Context, rawLogin string) (*models.User, error) {
	login := sanitize.UserLogin(rawLogin)
	if err := validate.UserLogin(login); err != nil {
		return nil, err
	}

	client := github.NewClient(nil)
	ghUser, _, err := client.Users.Get(ctx, login)
	if err != nil {
		if isGitHubNotFound(err) {
			return nil, errs.New(errs.CodeUserGitHubNotFound, "github user not found")
		}
		detail := ghintegration.FormatError(err)
		if detail == "" {
			return nil, fmt.Errorf("fetch github user: %w", err)
		}
		return nil, fmt.Errorf("fetch github user: %w; %s", err, detail)
	}
	if ghUser == nil || ghUser.ID == nil || ghUser.Login == nil {
		return nil, errs.New(errs.CodeUserCreateFailed, "github user payload incomplete")
	}

	user, err := s.userRepo.CreateAllowlistUser(ghUser.GetID(), ghUser.GetLogin(), ghUser.GetAvatarURL())
	if err != nil {
		return nil, fmt.Errorf("create allowlist user: %w", err)
	}

	return user, nil
}

func (s *UserService) RemoveAllowlistUser(ctx context.Context, userID uint) error {
	if err := validate.UserID(userID); err != nil {
		return err
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errs.New(errs.CodeUserNotFound, "user not found")
		}
		return err
	}
	if user.Role == models.RoleSuperUser {
		return errs.New(errs.CodeUserRemoveSuperUser, "cannot remove superuser")
	}

	if err := s.userRepo.DeleteByIDs([]uint{userID}); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func isGitHubNotFound(err error) bool {
	var typed *github.ErrorResponse
	if errors.As(err, &typed) && typed.Response != nil {
		return typed.Response.StatusCode == http.StatusNotFound
	}
	return false
}
