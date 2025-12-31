package service

import (
	"context"
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
	BaseDomain            string `json:"baseDomain"`
	GitHubToken           string `json:"githubToken"`
	CloudflareToken       string `json:"cloudflareToken"`
	CloudflareAccountID   string `json:"cloudflareAccountId"`
	CloudflareZoneID      string `json:"cloudflareZoneId"`
	CloudflaredTunnel     string `json:"cloudflaredTunnel"`
	CloudflaredConfigPath string `json:"cloudflaredConfigPath"`
}

type SettingsSources struct {
	BaseDomain            string `json:"baseDomain"`
	GitHubToken           string `json:"githubToken"`
	CloudflareToken       string `json:"cloudflareToken"`
	CloudflareAccountID   string `json:"cloudflareAccountId"`
	CloudflareZoneID      string `json:"cloudflareZoneId"`
	CloudflaredTunnel     string `json:"cloudflaredTunnel"`
	CloudflaredConfigPath string `json:"cloudflaredConfigPath"`
}

type CloudflaredPreview struct {
	Path     string `json:"path"`
	Contents string `json:"contents"`
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

func (s *SettingsService) Update(ctx context.Context, input SettingsPayload) (SettingsPayload, error) {
	stored, err := s.repo.Get(ctx)
	if err != nil && err != repository.ErrNotFound {
		return SettingsPayload{}, err
	}
	if stored == nil {
		stored = &models.Settings{}
	}

	stored.BaseDomain = strings.TrimSpace(input.BaseDomain)
	stored.GitHubToken = strings.TrimSpace(input.GitHubToken)
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
	if settings.GitHubToken != "" {
		cfg.GitHubToken = settings.GitHubToken
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

func (s *SettingsService) ResolveConfigWithSources(ctx context.Context) (config.Config, SettingsSources, error) {
	stored, err := s.repo.Get(ctx)
	if err != nil && err != repository.ErrNotFound {
		return config.Config{}, SettingsSources{}, err
	}

	cfg := s.cfg
	sources := SettingsSources{
		BaseDomain:            sourceFromValue(cfg.Domain, "env"),
		GitHubToken:           sourceFromValue(cfg.GitHubToken, "env"),
		CloudflareToken:       sourceFromValue(cfg.CloudflareAPIToken, "env"),
		CloudflareAccountID:   sourceFromValue(cfg.CloudflareAccountID, "env"),
		CloudflareZoneID:      sourceFromValue(cfg.CloudflareZoneID, "env"),
		CloudflaredTunnel:     sourceFromValue(cfg.CloudflaredTunnel, "env"),
		CloudflaredConfigPath: sourceFromValue(cfg.CloudflaredConfig, "env"),
	}

	if stored != nil {
		if strings.TrimSpace(stored.BaseDomain) != "" {
			cfg.Domain = strings.TrimSpace(stored.BaseDomain)
			sources.BaseDomain = "settings"
		} else if sources.BaseDomain == "" {
			sources.BaseDomain = "unset"
		}
		if strings.TrimSpace(stored.GitHubToken) != "" {
			cfg.GitHubToken = strings.TrimSpace(stored.GitHubToken)
			sources.GitHubToken = "settings"
		} else if sources.GitHubToken == "" {
			sources.GitHubToken = "unset"
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
	githubToken := strings.TrimSpace(s.cfg.GitHubToken)
	cloudflareToken := strings.TrimSpace(s.cfg.CloudflareAPIToken)
	cloudflareAccountID := strings.TrimSpace(s.cfg.CloudflareAccountID)
	cloudflareZoneID := strings.TrimSpace(s.cfg.CloudflareZoneID)
	cloudflaredTunnel := strings.TrimSpace(s.cfg.CloudflaredTunnel)
	cloudflaredConfigPath := strings.TrimSpace(s.cfg.CloudflaredConfig)
	if cloudflaredConfigPath == "" {
		cloudflaredConfigPath = defaultCloudflaredConfigPath
	}

	if stored != nil {
		if strings.TrimSpace(stored.BaseDomain) != "" {
			baseDomain = strings.TrimSpace(stored.BaseDomain)
		}
		if strings.TrimSpace(stored.GitHubToken) != "" {
			githubToken = strings.TrimSpace(stored.GitHubToken)
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
		BaseDomain:            baseDomain,
		GitHubToken:           githubToken,
		CloudflareToken:       cloudflareToken,
		CloudflareAccountID:   cloudflareAccountID,
		CloudflareZoneID:      cloudflareZoneID,
		CloudflaredTunnel:     cloudflaredTunnel,
		CloudflaredConfigPath: expandUserPath(cloudflaredConfigPath),
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
	if input.GitHubToken == "" {
		input.GitHubToken = "unset"
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
	return input
}
