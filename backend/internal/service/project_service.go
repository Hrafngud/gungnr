package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-notes/internal/config"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

type ProjectService struct {
	cfg  config.Config
	repo repository.ProjectRepository
	jobs *JobService
}

func NewProjectService(cfg config.Config, repo repository.ProjectRepository, jobs *JobService) *ProjectService {
	return &ProjectService{cfg: cfg, repo: repo, jobs: jobs}
}

func (s *ProjectService) List(ctx context.Context) ([]models.Project, error) {
	return s.repo.List(ctx)
}

type LocalProject struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type CreateTemplateRequest struct {
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	ProxyPort int    `json:"proxyPort"`
	DBPort    int    `json:"dbPort"`
}

type DeployExistingRequest struct {
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	Port      int    `json:"port"`
}

type QuickServiceRequest struct {
	Subdomain string `json:"subdomain"`
	Port      int    `json:"port"`
}

func (s *ProjectService) ListLocal(ctx context.Context) ([]LocalProject, error) {
	if strings.TrimSpace(s.cfg.TemplatesDir) == "" {
		return nil, fmt.Errorf("TEMPLATES_DIR not configured")
	}

	entries, err := os.ReadDir(s.cfg.TemplatesDir)
	if err != nil {
		return nil, fmt.Errorf("read templates dir: %w", err)
	}

	var projects []LocalProject
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		projects = append(projects, LocalProject{
			Name: entry.Name(),
			Path: filepath.Join(s.cfg.TemplatesDir, entry.Name()),
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects, nil
}

func (s *ProjectService) CreateFromTemplate(ctx context.Context, req CreateTemplateRequest) (*models.Job, error) {
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if req.Subdomain == "" {
		req.Subdomain = req.Name
	}
	if err := ValidateProjectName(req.Name); err != nil {
		return nil, err
	}
	if err := ValidateSubdomain(req.Subdomain); err != nil {
		return nil, err
	}
	if req.ProxyPort != 0 {
		if err := ValidatePort(req.ProxyPort); err != nil {
			return nil, err
		}
	}
	if req.DBPort != 0 {
		if err := ValidatePort(req.DBPort); err != nil {
			return nil, err
		}
	}
	return s.jobs.Create(ctx, JobTypeCreateTemplate, req)
}

func (s *ProjectService) DeployExisting(ctx context.Context, req DeployExistingRequest) (*models.Job, error) {
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if err := ValidateProjectName(req.Name); err != nil {
		return nil, err
	}
	if err := ValidateSubdomain(req.Subdomain); err != nil {
		return nil, err
	}
	if req.Port == 0 {
		req.Port = 80
	}
	if err := ValidatePort(req.Port); err != nil {
		return nil, err
	}
	return s.jobs.Create(ctx, JobTypeDeployExisting, req)
}

func (s *ProjectService) QuickService(ctx context.Context, req QuickServiceRequest) (*models.Job, error) {
	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if err := ValidateSubdomain(req.Subdomain); err != nil {
		return nil, err
	}
	if err := ValidatePort(req.Port); err != nil {
		return nil, err
	}
	return s.jobs.Create(ctx, JobTypeQuickService, req)
}
