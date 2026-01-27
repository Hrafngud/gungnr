package service

import (
	"errors"
	"fmt"

	"go-notes/internal/config"
	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

var ErrSuperUserCapExceeded = errors.New("superuser cap exceeded")

type RBACService struct {
	cfg      config.Config
	userRepo repository.UserRepository
}

func NewRBACService(cfg config.Config, userRepo repository.UserRepository) *RBACService {
	return &RBACService{cfg: cfg, userRepo: userRepo}
}

func (s *RBACService) SeedSuperUser() error {
	if s.cfg.SuperUserGitHubName == "" || s.cfg.SuperUserGitHubID == 0 {
		return fmt.Errorf("superuser env missing: SUPERUSER_GH_NAME and SUPER_GH_ID are required to boot")
	}

	if _, err := s.userRepo.UpsertSuperUser(s.cfg.SuperUserGitHubID, s.cfg.SuperUserGitHubName); err != nil {
		return fmt.Errorf("upsert superuser: %w", err)
	}

	superUsers, err := s.userRepo.ListByRole(models.RoleSuperUser)
	if err != nil {
		return fmt.Errorf("list superusers: %w", err)
	}

	if len(superUsers) <= 2 {
		return nil
	}

	var deleteIDs []uint
	for _, user := range superUsers[2:] {
		deleteIDs = append(deleteIDs, user.ID)
	}
	if err := s.userRepo.DeleteByIDs(deleteIDs); err != nil {
		return fmt.Errorf("prune superusers: %w", err)
	}

	detail := fmt.Sprintf("superuser cap exceeded: limit=2 total=%d removed=%d", len(superUsers), len(deleteIDs))
	return errs.Wrap(errs.CodeRBACSuperUserCap, detail, ErrSuperUserCapExceeded)
}
