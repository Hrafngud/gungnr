package service

import (
	"context"
	"strings"

	"go-notes/internal/config"
)

type GitHubTemplateCatalog struct {
	Configured  bool   `json:"configured"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	TargetOwner string `json:"targetOwner"`
	Private     bool   `json:"private"`
}

type GitHubAllowlist struct {
	Mode  string   `json:"mode"`
	Users []string `json:"users"`
	Org   string   `json:"org"`
}

type GitHubCatalog struct {
	TokenConfigured bool                  `json:"tokenConfigured"`
	Template        GitHubTemplateCatalog `json:"template"`
	Allowlist       GitHubAllowlist       `json:"allowlist"`
}

type GitHubService struct {
	cfg      config.Config
	settings *SettingsService
}

func NewGitHubService(cfg config.Config, settings *SettingsService) *GitHubService {
	return &GitHubService{cfg: cfg, settings: settings}
}

func (s *GitHubService) Catalog(ctx context.Context) (GitHubCatalog, error) {
	cfg := s.cfg
	if s.settings != nil {
		resolved, err := s.settings.ResolveConfig(ctx)
		if err != nil {
			return GitHubCatalog{}, err
		}
		cfg = resolved
	}

	templateOwner := strings.TrimSpace(cfg.GitHubTemplateOwner)
	templateRepo := strings.TrimSpace(cfg.GitHubTemplateRepo)
	templateConfigured := templateOwner != "" && templateRepo != ""

	targetOwner := strings.TrimSpace(cfg.GitHubRepoOwner)
	if targetOwner == "" {
		targetOwner = templateOwner
	}

	users := make([]string, 0, len(cfg.GitHubAllowedUsers))
	for _, user := range cfg.GitHubAllowedUsers {
		trimmed := strings.TrimSpace(user)
		if trimmed != "" {
			users = append(users, trimmed)
		}
	}

	org := strings.TrimSpace(cfg.GitHubAllowedOrg)
	allowlistMode := "none"
	if len(users) > 0 {
		allowlistMode = "users"
	} else if org != "" {
		allowlistMode = "org"
	}

	return GitHubCatalog{
		TokenConfigured: strings.TrimSpace(cfg.GitHubToken) != "",
		Template: GitHubTemplateCatalog{
			Configured:  templateConfigured,
			Owner:       templateOwner,
			Repo:        templateRepo,
			TargetOwner: targetOwner,
			Private:     cfg.GitHubRepoPrivate,
		},
		Allowlist: GitHubAllowlist{
			Mode:  allowlistMode,
			Users: users,
			Org:   org,
		},
	}, nil
}
