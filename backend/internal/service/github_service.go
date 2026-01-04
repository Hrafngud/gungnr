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

type GitHubAppStatus struct {
	Configured               bool `json:"configured"`
	AppIDConfigured          bool `json:"appIdConfigured"`
	InstallationIDConfigured bool `json:"installationIdConfigured"`
	PrivateKeyConfigured     bool `json:"privateKeyConfigured"`
}

type GitHubCatalog struct {
	TokenConfigured bool                   `json:"tokenConfigured"`
	Template        GitHubTemplateCatalog  `json:"template"`
	Templates       []GitHubTemplateSource `json:"templates,omitempty"`
	Allowlist       GitHubAllowlist        `json:"allowlist"`
	App             GitHubAppStatus        `json:"app"`
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
	var templates []GitHubTemplateSource
	var defaultTemplate *GitHubTemplateSource
	var templatesFromSettings bool
	appStatus := GitHubAppStatus{}
	if s.settings != nil {
		resolved, err := s.settings.ResolveConfig(ctx)
		if err != nil {
			return GitHubCatalog{}, err
		}
		cfg = resolved

		catalog, selected, fromSettings, err := s.settings.ResolveTemplateCatalog(ctx)
		if err != nil {
			return GitHubCatalog{}, err
		}
		templates = catalog
		defaultTemplate = selected
		templatesFromSettings = fromSettings

		appSettings, configured, err := s.settings.GitHubAppSettings(ctx)
		if err != nil {
			return GitHubCatalog{}, err
		}
		appStatus = GitHubAppStatus{
			Configured:               configured,
			AppIDConfigured:          appSettings.AppID != "",
			InstallationIDConfigured: appSettings.InstallationID != "",
			PrivateKeyConfigured:     appSettings.PrivateKey != "",
		}
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

	template := GitHubTemplateCatalog{
		Configured:  templateConfigured,
		Owner:       templateOwner,
		Repo:        templateRepo,
		TargetOwner: targetOwner,
		Private:     cfg.GitHubRepoPrivate,
	}

	if defaultTemplate != nil {
		template = GitHubTemplateCatalog{
			Configured:  true,
			Owner:       defaultTemplate.Owner,
			Repo:        defaultTemplate.Repo,
			TargetOwner: targetOwner,
			Private:     defaultTemplate.Private,
		}
	} else if templatesFromSettings {
		template = GitHubTemplateCatalog{}
	}

	tokenConfigured := appStatus.Configured

	return GitHubCatalog{
		TokenConfigured: tokenConfigured,
		Template:        template,
		Templates:       templates,
		Allowlist: GitHubAllowlist{
			Mode:  allowlistMode,
			Users: users,
			Org:   org,
		},
		App: appStatus,
	}, nil
}
