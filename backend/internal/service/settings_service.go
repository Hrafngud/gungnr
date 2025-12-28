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
	if settings.CloudflaredConfigPath != "" {
		cfg.CloudflaredConfig = settings.CloudflaredConfigPath
	}
	return cfg, nil
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
		if strings.TrimSpace(stored.CloudflaredConfigPath) != "" {
			cloudflaredConfigPath = strings.TrimSpace(stored.CloudflaredConfigPath)
		}
	}

	return SettingsPayload{
		BaseDomain:            baseDomain,
		GitHubToken:           githubToken,
		CloudflareToken:       cloudflareToken,
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
