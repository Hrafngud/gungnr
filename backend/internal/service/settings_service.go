package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go-notes/internal/config"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

const defaultCloudflaredConfigPath = "~/.cloudflared/config.yml"

type SettingsPayload struct {
	BaseDomain              string                 `json:"baseDomain"`
	GitHubTemplates         []GitHubTemplateSource `json:"githubTemplates,omitempty"`
	GitHubAppID             string                 `json:"githubAppId"`
	GitHubAppClientID       string                 `json:"githubAppClientId"`
	GitHubAppClientSecret   string                 `json:"githubAppClientSecret"`
	GitHubAppInstallationID string                 `json:"githubAppInstallationId"`
	GitHubAppPrivateKey     string                 `json:"githubAppPrivateKey"`
	CloudflareToken         string                 `json:"cloudflareToken"`
	CloudflareAccountID     string                 `json:"cloudflareAccountId"`
	CloudflareZoneID        string                 `json:"cloudflareZoneId"`
	CloudflaredTunnel       string                 `json:"cloudflaredTunnel"`
	CloudflaredConfigPath   string                 `json:"cloudflaredConfigPath"`
}

type SettingsSources struct {
	BaseDomain              string `json:"baseDomain"`
	GitHubAppID             string `json:"githubAppId"`
	GitHubAppClientID       string `json:"githubAppClientId"`
	GitHubAppClientSecret   string `json:"githubAppClientSecret"`
	GitHubAppInstallationID string `json:"githubAppInstallationId"`
	GitHubAppPrivateKey     string `json:"githubAppPrivateKey"`
	TemplatesDir            string `json:"templatesDir"`
	CloudflareToken         string `json:"cloudflareToken"`
	CloudflareAccountID     string `json:"cloudflareAccountId"`
	CloudflareZoneID        string `json:"cloudflareZoneId"`
	CloudflaredTunnel       string `json:"cloudflaredTunnel"`
	CloudflaredConfigPath   string `json:"cloudflaredConfigPath"`
}

type CloudflaredPreview struct {
	Path     string `json:"path"`
	Contents string `json:"contents"`
}

type GitHubTemplateSource struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	Private     bool   `json:"private"`
	Default     bool   `json:"default"`
}

type GitHubAppSettings struct {
	AppID          string
	ClientID       string
	ClientSecret   string
	InstallationID string
	PrivateKey     string
}

type SettingsService struct {
	cfg  config.Config
	repo repository.SettingsRepository
}

func NewSettingsService(cfg config.Config, repo repository.SettingsRepository) *SettingsService {
	return &SettingsService{cfg: cfg, repo: repo}
}

func (s *SettingsService) Get(ctx context.Context) (SettingsPayload, error) {
	stored, err := s.repo.Get(ctx)
	if err != nil && err != repository.ErrNotFound {
		return SettingsPayload{}, err
	}
	return s.resolve(stored), nil
}

func (s *SettingsService) GitHubAppSettings(ctx context.Context) (GitHubAppSettings, bool, error) {
	settings, err := s.Get(ctx)
	if err != nil {
		return GitHubAppSettings{}, false, err
	}

	app := GitHubAppSettings{
		AppID:          strings.TrimSpace(settings.GitHubAppID),
		ClientID:       strings.TrimSpace(settings.GitHubAppClientID),
		ClientSecret:   strings.TrimSpace(settings.GitHubAppClientSecret),
		InstallationID: strings.TrimSpace(settings.GitHubAppInstallationID),
		PrivateKey:     strings.TrimSpace(settings.GitHubAppPrivateKey),
	}
	configured := app.AppID != "" && app.InstallationID != "" && app.PrivateKey != ""
	return app, configured, nil
}

func (s *SettingsService) Update(ctx context.Context, input SettingsPayload) (SettingsPayload, error) {
	stored, err := s.repo.Get(ctx)
	if err != nil && err != repository.ErrNotFound {
		return SettingsPayload{}, err
	}
	if stored == nil {
	stored = &models.Settings{}
}

	stored.BaseDomain = strings.TrimSpace(input.BaseDomain)
	stored.GitHubAppID = strings.TrimSpace(input.GitHubAppID)
	stored.GitHubAppClientID = strings.TrimSpace(input.GitHubAppClientID)
	stored.GitHubAppClientSecret = strings.TrimSpace(input.GitHubAppClientSecret)
	stored.GitHubAppInstallationID = strings.TrimSpace(input.GitHubAppInstallationID)
	stored.GitHubAppPrivateKey = strings.TrimSpace(input.GitHubAppPrivateKey)
	if input.GitHubTemplates != nil {
		normalized := normalizeTemplateSources(input.GitHubTemplates)
		raw, err := json.Marshal(normalized)
		if err != nil {
			return SettingsPayload{}, fmt.Errorf("encode github templates: %w", err)
		}
		stored.GitHubTemplates = string(raw)
	}
	stored.CloudflareToken = strings.TrimSpace(input.CloudflareToken)
	stored.CloudflareAccountID = strings.TrimSpace(input.CloudflareAccountID)
	stored.CloudflareZoneID = strings.TrimSpace(input.CloudflareZoneID)
	stored.CloudflaredTunnel = strings.TrimSpace(input.CloudflaredTunnel)
	stored.CloudflaredConfigPath = strings.TrimSpace(input.CloudflaredConfigPath)

	if err := s.repo.Save(ctx, stored); err != nil {
		return SettingsPayload{}, err
	}
	return s.resolve(stored), nil
}

func (s *SettingsService) ResolveConfig(ctx context.Context) (config.Config, error) {
	settings, err := s.Get(ctx)
	if err != nil {
		return config.Config{}, err
	}

	cfg := s.cfg
	if settings.BaseDomain != "" {
		cfg.Domain = settings.BaseDomain
	}
	if settings.CloudflareToken != "" {
		cfg.CloudflareAPIToken = settings.CloudflareToken
	}
	if settings.CloudflareAccountID != "" {
		cfg.CloudflareAccountID = settings.CloudflareAccountID
	}
	if settings.CloudflareZoneID != "" {
		cfg.CloudflareZoneID = settings.CloudflareZoneID
	}
	if settings.CloudflaredTunnel != "" {
		cfg.CloudflaredTunnel = settings.CloudflaredTunnel
	}
	if settings.CloudflaredConfigPath != "" {
		cfg.CloudflaredConfig = settings.CloudflaredConfigPath
	}
	return cfg, nil
}

func (s *SettingsService) ResolveTemplateCatalog(ctx context.Context) ([]GitHubTemplateSource, *GitHubTemplateSource, bool, error) {
	stored, err := s.repo.Get(ctx)
	if err != nil && err != repository.ErrNotFound {
		return nil, nil, false, err
	}

	templates, fromSettings, err := resolveTemplateSources(s.cfg, stored)
	if err != nil {
		return nil, nil, fromSettings, err
	}
	if len(templates) == 0 {
		return templates, nil, fromSettings, nil
	}
	defaultTemplate := pickDefaultTemplate(templates)
	if fromSettings && defaultTemplate == nil {
		return templates, nil, fromSettings, nil
	}
	if defaultTemplate == nil {
		defaultTemplate = &templates[0]
	}
	return templates, defaultTemplate, fromSettings, nil
}

func (s *SettingsService) ResolveTemplateSelection(ctx context.Context, ref string) (GitHubTemplateSource, error) {
	templates, defaultTemplate, _, err := s.ResolveTemplateCatalog(ctx)
	if err != nil {
		return GitHubTemplateSource{}, err
	}
	if len(templates) == 0 {
		return GitHubTemplateSource{}, fmt.Errorf("template catalog not configured")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		if defaultTemplate != nil {
			return *defaultTemplate, nil
		}
		return templates[0], nil
	}
	owner, repo, err := parseTemplateRef(ref)
	if err != nil {
		return GitHubTemplateSource{}, err
	}
	key := templateKey(owner, repo)
	for _, entry := range templates {
		if templateKey(entry.Owner, entry.Repo) == key {
			return entry, nil
		}
	}
	return GitHubTemplateSource{}, fmt.Errorf("template is not in allowlist")
}

func (s *SettingsService) ResolveConfigWithSources(ctx context.Context) (config.Config, SettingsSources, error) {
	stored, err := s.repo.Get(ctx)
	if err != nil && err != repository.ErrNotFound {
		return config.Config{}, SettingsSources{}, err
	}

	cfg := s.cfg
	sources := SettingsSources{
		BaseDomain:              sourceFromValue(cfg.Domain, "env"),
		GitHubAppID:             sourceFromValue("", "env"),
		GitHubAppClientID:       sourceFromValue("", "env"),
		GitHubAppClientSecret:   sourceFromValue("", "env"),
		GitHubAppInstallationID: sourceFromValue("", "env"),
		GitHubAppPrivateKey:     sourceFromValue("", "env"),
		TemplatesDir:            sourceFromValue(cfg.TemplatesDir, "env"),
		CloudflareToken:         sourceFromValue(cfg.CloudflareAPIToken, "env"),
		CloudflareAccountID:     sourceFromValue(cfg.CloudflareAccountID, "env"),
		CloudflareZoneID:        sourceFromValue(cfg.CloudflareZoneID, "env"),
		CloudflaredTunnel:       sourceFromValue(cfg.CloudflaredTunnel, "env"),
		CloudflaredConfigPath:   sourceFromValue(cfg.CloudflaredConfig, "env"),
	}

	if stored != nil {
		if strings.TrimSpace(stored.BaseDomain) != "" {
			cfg.Domain = strings.TrimSpace(stored.BaseDomain)
			sources.BaseDomain = "settings"
		} else if sources.BaseDomain == "" {
			sources.BaseDomain = "unset"
		}
		if strings.TrimSpace(stored.GitHubAppID) != "" {
			sources.GitHubAppID = "settings"
		} else if sources.GitHubAppID == "" {
			sources.GitHubAppID = "unset"
		}
		if strings.TrimSpace(stored.GitHubAppClientID) != "" {
			sources.GitHubAppClientID = "settings"
		} else if sources.GitHubAppClientID == "" {
			sources.GitHubAppClientID = "unset"
		}
		if strings.TrimSpace(stored.GitHubAppClientSecret) != "" {
			sources.GitHubAppClientSecret = "settings"
		} else if sources.GitHubAppClientSecret == "" {
			sources.GitHubAppClientSecret = "unset"
		}
		if strings.TrimSpace(stored.GitHubAppInstallationID) != "" {
			sources.GitHubAppInstallationID = "settings"
		} else if sources.GitHubAppInstallationID == "" {
			sources.GitHubAppInstallationID = "unset"
		}
		if strings.TrimSpace(stored.GitHubAppPrivateKey) != "" {
			sources.GitHubAppPrivateKey = "settings"
		} else if sources.GitHubAppPrivateKey == "" {
			sources.GitHubAppPrivateKey = "unset"
		}
		if strings.TrimSpace(stored.CloudflareToken) != "" {
			cfg.CloudflareAPIToken = strings.TrimSpace(stored.CloudflareToken)
			sources.CloudflareToken = "settings"
		} else if sources.CloudflareToken == "" {
			sources.CloudflareToken = "unset"
		}
		if strings.TrimSpace(stored.CloudflareAccountID) != "" {
			cfg.CloudflareAccountID = strings.TrimSpace(stored.CloudflareAccountID)
			sources.CloudflareAccountID = "settings"
		} else if sources.CloudflareAccountID == "" {
			sources.CloudflareAccountID = "unset"
		}
		if strings.TrimSpace(stored.CloudflareZoneID) != "" {
			cfg.CloudflareZoneID = strings.TrimSpace(stored.CloudflareZoneID)
			sources.CloudflareZoneID = "settings"
		} else if sources.CloudflareZoneID == "" {
			sources.CloudflareZoneID = "unset"
		}
		if strings.TrimSpace(stored.CloudflaredTunnel) != "" {
			cfg.CloudflaredTunnel = strings.TrimSpace(stored.CloudflaredTunnel)
			sources.CloudflaredTunnel = "settings"
		} else if sources.CloudflaredTunnel == "" {
			sources.CloudflaredTunnel = "unset"
		}
		if strings.TrimSpace(stored.CloudflaredConfigPath) != "" {
			cfg.CloudflaredConfig = strings.TrimSpace(stored.CloudflaredConfigPath)
			sources.CloudflaredConfigPath = "settings"
		} else if sources.CloudflaredConfigPath == "" {
			sources.CloudflaredConfigPath = "unset"
		}
	}
	if strings.TrimSpace(cfg.TemplatesDir) == "" {
		if sources.TemplatesDir == "" {
			sources.TemplatesDir = "unset"
		}
	}

	if strings.TrimSpace(cfg.CloudflaredConfig) == "" {
		cfg.CloudflaredConfig = defaultCloudflaredConfigPath
		if sources.CloudflaredConfigPath == "" {
			sources.CloudflaredConfigPath = "default"
		}
	}
	cfg.CloudflaredConfig = expandUserPath(cfg.CloudflaredConfig)

	sources = normalizeSources(sources)
	return cfg, sources, nil
}

func (s *SettingsService) CloudflaredPreview(ctx context.Context) (CloudflaredPreview, error) {
	cfg, err := s.ResolveConfig(ctx)
	if err != nil {
		return CloudflaredPreview{}, err
	}
	path := strings.TrimSpace(cfg.CloudflaredConfig)
	if path == "" {
		return CloudflaredPreview{}, fmt.Errorf("cloudflared config path is empty")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return CloudflaredPreview{}, fmt.Errorf("read cloudflared config: %w", err)
	}

	return CloudflaredPreview{
		Path:     path,
		Contents: string(raw),
	}, nil
}

func (s *SettingsService) resolve(stored *models.Settings) SettingsPayload {
	baseDomain := strings.TrimSpace(s.cfg.Domain)
	githubAppID := ""
	githubAppClientID := ""
	githubAppClientSecret := ""
	githubAppInstallationID := ""
	githubAppPrivateKey := ""
	cloudflareToken := strings.TrimSpace(s.cfg.CloudflareAPIToken)
	cloudflareAccountID := strings.TrimSpace(s.cfg.CloudflareAccountID)
	cloudflareZoneID := strings.TrimSpace(s.cfg.CloudflareZoneID)
	cloudflaredTunnel := strings.TrimSpace(s.cfg.CloudflaredTunnel)
	cloudflaredConfigPath := strings.TrimSpace(s.cfg.CloudflaredConfig)
	if cloudflaredConfigPath == "" {
		cloudflaredConfigPath = defaultCloudflaredConfigPath
	}
	templates, _, _ := resolveTemplateSources(s.cfg, stored)

	if stored != nil {
		if strings.TrimSpace(stored.BaseDomain) != "" {
			baseDomain = strings.TrimSpace(stored.BaseDomain)
		}
		if strings.TrimSpace(stored.GitHubAppID) != "" {
			githubAppID = strings.TrimSpace(stored.GitHubAppID)
		}
		if strings.TrimSpace(stored.GitHubAppClientID) != "" {
			githubAppClientID = strings.TrimSpace(stored.GitHubAppClientID)
		}
		if strings.TrimSpace(stored.GitHubAppClientSecret) != "" {
			githubAppClientSecret = strings.TrimSpace(stored.GitHubAppClientSecret)
		}
		if strings.TrimSpace(stored.GitHubAppInstallationID) != "" {
			githubAppInstallationID = strings.TrimSpace(stored.GitHubAppInstallationID)
		}
		if strings.TrimSpace(stored.GitHubAppPrivateKey) != "" {
			githubAppPrivateKey = strings.TrimSpace(stored.GitHubAppPrivateKey)
		}
		if strings.TrimSpace(stored.CloudflareToken) != "" {
			cloudflareToken = strings.TrimSpace(stored.CloudflareToken)
		}
		if strings.TrimSpace(stored.CloudflareAccountID) != "" {
			cloudflareAccountID = strings.TrimSpace(stored.CloudflareAccountID)
		}
		if strings.TrimSpace(stored.CloudflareZoneID) != "" {
			cloudflareZoneID = strings.TrimSpace(stored.CloudflareZoneID)
		}
		if strings.TrimSpace(stored.CloudflaredTunnel) != "" {
			cloudflaredTunnel = strings.TrimSpace(stored.CloudflaredTunnel)
		}
		if strings.TrimSpace(stored.CloudflaredConfigPath) != "" {
			cloudflaredConfigPath = strings.TrimSpace(stored.CloudflaredConfigPath)
		}
	}

	return SettingsPayload{
		BaseDomain:              baseDomain,
		GitHubTemplates:         templates,
		GitHubAppID:             githubAppID,
		GitHubAppClientID:       githubAppClientID,
		GitHubAppClientSecret:   githubAppClientSecret,
		GitHubAppInstallationID: githubAppInstallationID,
		GitHubAppPrivateKey:     githubAppPrivateKey,
		CloudflareToken:         cloudflareToken,
		CloudflareAccountID:     cloudflareAccountID,
		CloudflareZoneID:        cloudflareZoneID,
		CloudflaredTunnel:       cloudflaredTunnel,
		CloudflaredConfigPath:   expandUserPath(cloudflaredConfigPath),
	}
}

func expandUserPath(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if trimmed == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return trimmed
		}
		return home
	}
	if strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return trimmed
		}
		return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
	}
	return trimmed
}

func sourceFromValue(value, source string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return source
}

func normalizeSources(input SettingsSources) SettingsSources {
	if input.BaseDomain == "" {
		input.BaseDomain = "unset"
	}
	if input.GitHubAppID == "" {
		input.GitHubAppID = "unset"
	}
	if input.GitHubAppClientID == "" {
		input.GitHubAppClientID = "unset"
	}
	if input.GitHubAppClientSecret == "" {
		input.GitHubAppClientSecret = "unset"
	}
	if input.GitHubAppInstallationID == "" {
		input.GitHubAppInstallationID = "unset"
	}
	if input.GitHubAppPrivateKey == "" {
		input.GitHubAppPrivateKey = "unset"
	}
	if input.CloudflareToken == "" {
		input.CloudflareToken = "unset"
	}
	if input.CloudflareAccountID == "" {
		input.CloudflareAccountID = "unset"
	}
	if input.CloudflareZoneID == "" {
		input.CloudflareZoneID = "unset"
	}
	if input.CloudflaredTunnel == "" {
		input.CloudflaredTunnel = "unset"
	}
	if input.CloudflaredConfigPath == "" {
		input.CloudflaredConfigPath = "unset"
	}
	if input.TemplatesDir == "" {
		input.TemplatesDir = "unset"
	}
	return input
}

func resolveTemplateSources(cfg config.Config, stored *models.Settings) ([]GitHubTemplateSource, bool, error) {
	if stored != nil && strings.TrimSpace(stored.GitHubTemplates) != "" {
		parsed, err := parseTemplateSources(stored.GitHubTemplates)
		if err != nil {
			return nil, true, err
		}
		return normalizeTemplateSources(parsed), true, nil
	}

	owner := strings.TrimSpace(cfg.GitHubTemplateOwner)
	repo := strings.TrimSpace(cfg.GitHubTemplateRepo)
	if owner == "" || repo == "" {
		return []GitHubTemplateSource{}, false, nil
	}
	return []GitHubTemplateSource{
		{
			Owner:   owner,
			Repo:    repo,
			Private: cfg.GitHubRepoPrivate,
			Default: true,
		},
	}, false, nil
}

func parseTemplateSources(raw string) ([]GitHubTemplateSource, error) {
	var parsed []GitHubTemplateSource
	if strings.TrimSpace(raw) == "" {
		return parsed, nil
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("decode github templates: %w", err)
	}
	return parsed, nil
}

func normalizeTemplateSources(input []GitHubTemplateSource) []GitHubTemplateSource {
	seen := make(map[string]bool)
	output := make([]GitHubTemplateSource, 0, len(input))
	defaultIndex := -1
	for _, entry := range input {
		owner := strings.TrimSpace(entry.Owner)
		repo := strings.TrimSpace(entry.Repo)
		if owner == "" || repo == "" {
			continue
		}
		key := templateKey(owner, repo)
		if seen[key] {
			continue
		}
		seen[key] = true
		entry.Owner = owner
		entry.Repo = repo
		entry.DisplayName = strings.TrimSpace(entry.DisplayName)
		entry.Description = strings.TrimSpace(entry.Description)
		if entry.Default {
			if defaultIndex == -1 {
				defaultIndex = len(output)
			} else {
				entry.Default = false
			}
		}
		output = append(output, entry)
	}
	if len(output) > 0 && defaultIndex == -1 {
		output[0].Default = true
	}
	return output
}

func pickDefaultTemplate(templates []GitHubTemplateSource) *GitHubTemplateSource {
	for i := range templates {
		if templates[i].Default {
			return &templates[i]
		}
	}
	return nil
}

func templateKey(owner, repo string) string {
	return strings.ToLower(strings.TrimSpace(owner)) + "/" + strings.ToLower(strings.TrimSpace(repo))
}

func parseTemplateRef(ref string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(ref), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("template must be in owner/repo format")
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("template must be in owner/repo format")
	}
	return owner, repo, nil
}
