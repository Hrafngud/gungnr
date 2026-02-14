package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-notes/internal/config"
	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

type ProjectService struct {
	cfg      config.Config
	repo     repository.ProjectRepository
	jobs     *JobService
	settings *SettingsService
}

func NewProjectService(cfg config.Config, repo repository.ProjectRepository, jobs *JobService, settings *SettingsService) *ProjectService {
	return &ProjectService{cfg: cfg, repo: repo, jobs: jobs, settings: settings}
}

func (s *ProjectService) List(ctx context.Context) ([]models.Project, error) {
	return s.repo.List(ctx)
}

type LocalProject struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type CreateTemplateRequest struct {
	Template  string `json:"template,omitempty"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	Domain    string `json:"domain,omitempty"`
	ProxyPort int    `json:"proxyPort"`
	DBPort    int    `json:"dbPort"`
}

type DeployExistingRequest struct {
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	Domain    string `json:"domain,omitempty"`
	Port      int    `json:"port"`
}

type ForwardLocalRequest struct {
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	Domain    string `json:"domain,omitempty"`
	Port      int    `json:"port"`
}

type QuickServiceRequest struct {
	Subdomain     string `json:"subdomain"`
	Domain        string `json:"domain,omitempty"`
	Port          int    `json:"port"`
	Image         string `json:"image,omitempty"`
	ContainerPort int    `json:"containerPort,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
}

func (s *ProjectService) ListLocal(ctx context.Context) ([]LocalProject, error) {
	if strings.TrimSpace(s.cfg.TemplatesDir) == "" {
		return nil, errs.New(errs.CodeProjectLocalListFailed, "TEMPLATES_DIR not configured")
	}

	entries, err := os.ReadDir(s.cfg.TemplatesDir)
	if err != nil {
		return nil, errs.Wrap(errs.CodeProjectLocalListFailed, "read templates dir failed", err)
	}

	projects := make([]LocalProject, 0, len(entries))
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
	domain, err := s.resolveDomain(ctx, req.Domain)
	if err != nil {
		return nil, err
	}
	req.Domain = domain
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
	normalizedTemplate, err := s.resolveTemplateSelection(ctx, req.Template)
	if err != nil {
		return nil, err
	}
	req.Template = normalizedTemplate
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
	domain, err := s.resolveDomain(ctx, req.Domain)
	if err != nil {
		return nil, err
	}
	req.Domain = domain
	if req.Port == 0 {
		req.Port = 80
	}
	if err := ValidatePort(req.Port); err != nil {
		return nil, err
	}
	return s.jobs.Create(ctx, JobTypeDeployExisting, req)
}

func (s *ProjectService) ForwardLocal(ctx context.Context, req ForwardLocalRequest) (*models.Job, error) {
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if err := ValidateServiceName(req.Name); err != nil {
		return nil, err
	}
	if err := ValidateSubdomain(req.Subdomain); err != nil {
		return nil, err
	}
	domain, err := s.resolveDomain(ctx, req.Domain)
	if err != nil {
		return nil, err
	}
	req.Domain = domain
	if req.Port == 0 {
		req.Port = 80
	}
	if err := ValidatePort(req.Port); err != nil {
		return nil, err
	}
	return s.jobs.Create(ctx, JobTypeForwardLocal, req)
}

func (s *ProjectService) QuickService(ctx context.Context, req QuickServiceRequest) (*models.Job, int, error) {
	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if err := ValidateSubdomain(req.Subdomain); err != nil {
		return nil, 0, err
	}
	domain, err := s.resolveDomain(ctx, req.Domain)
	if err != nil {
		return nil, 0, err
	}
	req.Domain = domain
	if err := ValidatePort(req.Port); err != nil {
		return nil, 0, err
	}
	req.Image = strings.TrimSpace(req.Image)
	req.ContainerName = strings.TrimSpace(req.ContainerName)
	if req.Image == "" {
		req.Image = defaultQuickServiceImage
	}
	if req.ContainerPort == 0 {
		req.ContainerPort = defaultQuickServiceContainerPort
	}
	if err := ValidatePort(req.ContainerPort); err != nil {
		return nil, 0, err
	}
	if req.ContainerName != "" {
		if err := validateContainerName(req.ContainerName); err != nil {
			return nil, 0, err
		}
	}
	chosenPort, err := ensureAvailableHostPort(ctx, req.Port)
	if err != nil {
		return nil, 0, err
	}
	req.Port = chosenPort
	job, err := s.jobs.Create(ctx, JobTypeQuickService, req)
	if err != nil {
		return nil, 0, err
	}
	return job, chosenPort, nil
}

func (s *ProjectService) resolveDomain(ctx context.Context, requested string) (string, error) {
	if s.settings != nil {
		selection, err := s.settings.ResolveDomainSelection(ctx, requested)
		if err != nil {
			return "", err
		}
		return selection.Domain, nil
	}
	base := normalizeDomain(s.cfg.Domain)
	return selectDomain(requested, base, nil)
}

func (s *ProjectService) resolveTemplateSelection(ctx context.Context, templateRef string) (string, error) {
	templateRef = strings.TrimSpace(templateRef)
	if s.settings != nil {
		selection, err := s.settings.ResolveTemplateSelection(ctx, templateRef)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s/%s", selection.Owner, selection.Repo), nil
	}
	owner := strings.TrimSpace(s.cfg.GitHubTemplateOwner)
	repo := strings.TrimSpace(s.cfg.GitHubTemplateRepo)
	if owner == "" || repo == "" {
		return "", errs.New(errs.CodeProjectTemplateSource, "template source not configured")
	}
	if templateRef == "" {
		return fmt.Sprintf("%s/%s", owner, repo), nil
	}
	selectedOwner, selectedRepo, err := parseTemplateRef(templateRef)
	if err != nil {
		return "", errs.Wrap(errs.CodeProjectTemplateSource, "template reference is invalid", err)
	}
	if templateKey(selectedOwner, selectedRepo) != templateKey(owner, repo) {
		return "", errs.New(errs.CodeProjectTemplateSource, "template is not in allowlist")
	}
	return fmt.Sprintf("%s/%s", owner, repo), nil
}
