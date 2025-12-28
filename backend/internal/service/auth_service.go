package service

import (
	"context"
	"errors"
	"fmt"

	"go-notes/internal/config"
	"go-notes/internal/models"
	"go-notes/internal/repository"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
)

type AuthService struct {
	cfg         config.Config
	userRepo    repository.UserRepository
	oauthConfig *oauth2.Config
}

func NewAuthService(cfg config.Config, userRepo repository.UserRepository) *AuthService {
	return &AuthService{
		cfg:      cfg,
		userRepo: userRepo,
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			Endpoint:     githuboauth.Endpoint,
			RedirectURL:  cfg.GitHubCallbackURL,
			Scopes:       []string{"read:user"},
		},
	}
}

func (s *AuthService) AuthURL(state, redirectURL string) string {
	cfg := s.oauthConfigForRedirect(redirectURL)
	return cfg.AuthCodeURL(state)
}

func (s *AuthService) Exchange(ctx context.Context, code, redirectURL string) (*models.User, error) {
	cfg := s.oauthConfigForRedirect(redirectURL)
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange token: %w", err)
	}

	client := github.NewClient(cfg.Client(ctx, token))
	ghUser, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("fetch github user: %w", err)
	}

	if ghUser == nil || ghUser.ID == nil || ghUser.Login == nil {
		return nil, errors.New("github user payload incomplete")
	}

	allowed, err := s.isAllowed(ctx, client, ghUser.GetLogin())
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrUnauthorized
	}

	user, err := s.userRepo.UpsertFromGitHub(ghUser.GetID(), ghUser.GetLogin(), ghUser.GetAvatarURL())
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	return user, nil
}

func (s *AuthService) CallbackURL() string {
	return s.oauthConfig.RedirectURL
}

func (s *AuthService) oauthConfigForRedirect(redirectURL string) *oauth2.Config {
	if redirectURL == "" || redirectURL == s.oauthConfig.RedirectURL {
		return s.oauthConfig
	}

	cfg := *s.oauthConfig
	cfg.RedirectURL = redirectURL
	return &cfg
}

func (s *AuthService) isAllowed(ctx context.Context, client *github.Client, login string) (bool, error) {
	allowedUsers := s.cfg.GitHubAllowedUsers
	if len(allowedUsers) == 0 && s.cfg.GitHubAllowedOrg == "" {
		return true, nil
	}

	for _, allowed := range allowedUsers {
		if allowed == login {
			return true, nil
		}
	}

	if s.cfg.GitHubAllowedOrg == "" {
		return false, nil
	}

	ok, _, err := client.Organizations.IsMember(ctx, s.cfg.GitHubAllowedOrg, login)
	if err != nil {
		return false, fmt.Errorf("check org membership: %w", err)
	}

	return ok, nil
}
