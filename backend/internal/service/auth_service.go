package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"go-notes/internal/config"
	ghintegration "go-notes/internal/integrations/github"
	"go-notes/internal/models"
	"go-notes/internal/repository"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
	githuboauth "golang.org/x/oauth2/github"
)

var (
	ErrUnauthorized      = errors.New("unauthorized")
	ErrAdminAuthDisabled = errors.New("admin auth disabled")
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

func (s *AuthService) AuthenticateAdmin(ctx context.Context, login, password string) (*models.User, error) {
	adminLogin := strings.TrimSpace(s.cfg.AdminLogin)
	adminPassword := strings.TrimSpace(s.cfg.AdminPassword)
	if adminLogin == "" || adminPassword == "" {
		return nil, ErrAdminAuthDisabled
	}

	inputLogin := strings.TrimSpace(login)
	inputPassword := strings.TrimSpace(password)
	loginMatch := subtle.ConstantTimeCompare([]byte(inputLogin), []byte(adminLogin)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(inputPassword), []byte(adminPassword)) == 1
	if !loginMatch || !passwordMatch {
		return nil, ErrUnauthorized
	}

	user, err := s.userRepo.UpsertFromGitHub(-1, adminLogin, "")
	if err != nil {
		return nil, fmt.Errorf("upsert admin user: %w", err)
	}

	return user, nil
}

func (s *AuthService) Exchange(ctx context.Context, code, redirectURL string) (*models.User, error) {
	cfg := s.oauthConfigForRedirect(redirectURL)
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange token: %w%s", err, formatOAuthError(err))
	}

	httpClient := ghintegration.WrapHTTPClient(cfg.Client(ctx, token))
	client := github.NewClient(httpClient)
	ghUser, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, formatGitHubClientError("fetch github user", err)
	}

	if ghUser == nil || ghUser.ID == nil || ghUser.Login == nil {
		return nil, errors.New("github user payload incomplete")
	}

	if _, err := s.userRepo.GetByGitHubID(ghUser.GetID()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("check allowlist: %w", err)
	}
	if ghUser.GetLogin() == "" {
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

func formatGitHubClientError(action string, err error) error {
	detail := ghintegration.FormatError(err)
	if detail == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w; %s", action, err, detail)
}

func formatOAuthError(err error) string {
	var retrieveErr *oauth2.RetrieveError
	if !errors.As(err, &retrieveErr) {
		return ""
	}
	status := ""
	if retrieveErr.Response != nil {
		status = retrieveErr.Response.Status
		if status == "" && retrieveErr.Response.StatusCode != 0 {
			status = fmt.Sprintf("%d", retrieveErr.Response.StatusCode)
		}
	}
	body := strings.TrimSpace(string(retrieveErr.Body))
	if len(body) > 600 {
		body = body[:600] + "..."
	}
	meta := ""
	if status != "" {
		meta = fmt.Sprintf(" status=%s", status)
	}
	if body != "" {
		if meta != "" {
			meta = meta + " "
		}
		meta = fmt.Sprintf("%sresponse=%s", meta, body)
	}
	if meta == "" {
		return ""
	}
	return fmt.Sprintf(" (%s)", strings.TrimSpace(meta))
}
