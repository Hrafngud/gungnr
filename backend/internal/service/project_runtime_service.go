package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"
	"go-notes/internal/models"
	"go-notes/internal/repository"
	"go-notes/internal/validate"
)

type ProjectRuntimeService struct {
	templatesDir string
	projects     repository.ProjectRepository
	host         *HostService
}

type ProjectSummary struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	RepoURL   string    `json:"repoUrl"`
	Path      string    `json:"path"`
	ProxyPort int       `json:"proxyPort"`
	DBPort    int       `json:"dbPort"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ProjectDetail struct {
	Project     ProjectDetailProject      `json:"project"`
	Runtime     ProjectDetailRuntime      `json:"runtime"`
	Network     ProjectDetailNetwork      `json:"network"`
	Containers  []DockerContainer         `json:"containers"`
	Diagnostics []ProjectDetailDiagnostic `json:"diagnostics,omitempty"`
}

type ProjectDetailProject struct {
	Name           string          `json:"name"`
	NormalizedName string          `json:"normalizedName"`
	Record         *ProjectSummary `json:"record,omitempty"`
}

type ProjectDetailRuntime struct {
	Path         string   `json:"path"`
	Source       string   `json:"source"`
	ComposeFiles []string `json:"composeFiles"`
	EnvPath      string   `json:"envPath"`
	EnvExists    bool     `json:"envExists"`
}

type ProjectPublishedPort struct {
	Container     string `json:"container"`
	Service       string `json:"service"`
	HostIP        string `json:"hostIp"`
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Proto         string `json:"proto"`
}

type ProjectDetailNetwork struct {
	ProxyPort      int                    `json:"proxyPort"`
	DBPort         int                    `json:"dbPort"`
	PublishedPorts []ProjectPublishedPort `json:"publishedPorts"`
}

type ProjectDetailDiagnostic struct {
	Scope      string `json:"scope"`
	Status     string `json:"status"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	SourceCode string `json:"sourceCode,omitempty"`
	TaskType   string `json:"taskType,omitempty"`
}

const (
	projectDetailDiagnosticStatusDegraded             = "degraded"
	projectDetailDiagnosticScopeContainers            = "containers"
	projectDetailDiagnosticScopePublishedPorts        = "network.publishedPorts"
	projectDetailDiagnosticCodeContainersDegraded     = "PROJECT-RUNTIME-CONTAINERS-DEGRADED"
	projectDetailDiagnosticCodePublishedPortsDegraded = "PROJECT-RUNTIME-PUBLISHED-PORTS-DEGRADED"
)

func NewProjectRuntimeService(
	templatesDir string,
	projects repository.ProjectRepository,
	host *HostService,
) *ProjectRuntimeService {
	return &ProjectRuntimeService{
		templatesDir: strings.TrimSpace(templatesDir),
		projects:     projects,
		host:         host,
	}
}

func (s *ProjectRuntimeService) Resolve(ctx context.Context, projectName string) (projectPathResolution, error) {
	return resolveProjectPath(ctx, s.projects, s.templatesDir, projectName, s.runtimeMetaClient())
}

func (s *ProjectRuntimeService) ListSummaries(ctx context.Context) ([]ProjectSummary, error) {
	projectContainers, runtimeAvailable := s.groupProjectContainers(ctx)
	if runtimeAvailable {
		if _, err := s.syncRuntimeProjects(ctx, projectContainers); err != nil {
			return nil, err
		}
	}

	projects, err := s.projects.List(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]ProjectSummary, 0, len(projects))
	for _, project := range projects {
		key := strings.ToLower(strings.TrimSpace(project.Name))

		status := strings.TrimSpace(project.Status)
		if runtimeAvailable {
			status = deriveProjectRuntimeStatus(projectContainers[key])
		}

		summary := ProjectSummary{
			ID:        project.ID,
			Name:      project.Name,
			RepoURL:   project.RepoURL,
			Path:      project.Path,
			ProxyPort: project.ProxyPort,
			DBPort:    project.DBPort,
			Status:    status,
			CreatedAt: project.CreatedAt,
			UpdatedAt: project.UpdatedAt,
		}

		if runtimeAvailable && strings.TrimSpace(summary.Path) == "" {
			if runtimeDir, _, runtimeErr := resolveDirFromRuntimeCompose(ctx, s.templatesDir, key, s.runtimeMetaClient()); runtimeErr == nil {
				summary.Path = runtimeDir
			}
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

func (s *ProjectRuntimeService) runtimeMetaClient() infraDockerMetadataClient {
	if s.host == nil {
		return nil
	}
	return s.host.infraClient
}

func (s *ProjectRuntimeService) Detail(ctx context.Context, projectName string) (ProjectDetail, error) {
	resolved, err := s.Resolve(ctx, projectName)
	if err != nil {
		return ProjectDetail{}, err
	}

	detail := ProjectDetail{
		Project: ProjectDetailProject{
			Name:           resolved.RequestedName,
			NormalizedName: resolved.NormalizedName,
		},
		Runtime: ProjectDetailRuntime{
			Path:         resolved.ProjectDir,
			Source:       resolved.Source,
			ComposeFiles: resolved.ComposeFiles,
			EnvPath:      resolved.EnvPath,
			EnvExists:    resolved.EnvExists,
		},
		Network: ProjectDetailNetwork{
			PublishedPorts: make([]ProjectPublishedPort, 0),
		},
		Containers: make([]DockerContainer, 0),
	}

	if resolved.ProjectRecord != nil {
		record := resolved.ProjectRecord
		detail.Project.Record = &ProjectSummary{
			ID:        record.ID,
			Name:      record.Name,
			RepoURL:   record.RepoURL,
			Path:      record.Path,
			ProxyPort: record.ProxyPort,
			DBPort:    record.DBPort,
			Status:    record.Status,
			CreatedAt: record.CreatedAt,
			UpdatedAt: record.UpdatedAt,
		}
		detail.Network.ProxyPort = record.ProxyPort
		detail.Network.DBPort = record.DBPort
	}

	filtered, err := s.projectContainers(ctx, resolved.NormalizedName)
	if err != nil {
		detail.Diagnostics = degradedProjectRuntimeDiagnostics(err)
		return detail, nil
	}
	detail.Containers = filtered

	for _, container := range filtered {
		for _, binding := range container.PortBindings {
			if !binding.Published || binding.HostPort <= 0 {
				continue
			}
			detail.Network.PublishedPorts = append(detail.Network.PublishedPorts, ProjectPublishedPort{
				Container:     container.Name,
				Service:       container.Service,
				HostIP:        binding.HostIP,
				HostPort:      binding.HostPort,
				ContainerPort: binding.ContainerPort,
				Proto:         binding.Proto,
			})

			if detail.Network.ProxyPort == 0 && binding.ContainerPort == 80 {
				detail.Network.ProxyPort = binding.HostPort
			}
			if detail.Network.DBPort == 0 && binding.ContainerPort == 5432 {
				detail.Network.DBPort = binding.HostPort
			}
		}
	}

	sort.Slice(detail.Network.PublishedPorts, func(i, j int) bool {
		if detail.Network.PublishedPorts[i].Container == detail.Network.PublishedPorts[j].Container {
			if detail.Network.PublishedPorts[i].ContainerPort == detail.Network.PublishedPorts[j].ContainerPort {
				return detail.Network.PublishedPorts[i].HostPort < detail.Network.PublishedPorts[j].HostPort
			}
			return detail.Network.PublishedPorts[i].ContainerPort < detail.Network.PublishedPorts[j].ContainerPort
		}
		return detail.Network.PublishedPorts[i].Container < detail.Network.PublishedPorts[j].Container
	})

	return detail, nil
}

func (d ProjectDetail) HasDiagnosticScope(scope string) bool {
	normalizedScope := strings.TrimSpace(scope)
	if normalizedScope == "" {
		return false
	}
	for _, diagnostic := range d.Diagnostics {
		if strings.EqualFold(strings.TrimSpace(diagnostic.Scope), normalizedScope) {
			return true
		}
	}
	return false
}

func (s *ProjectRuntimeService) EnsureContainerInProject(
	ctx context.Context,
	projectName string,
	containerName string,
) (string, error) {
	container := strings.TrimSpace(containerName)
	if container == "" {
		return "", errs.New(errs.CodeProjectInvalidContainer, "container is required")
	}

	resolved, err := s.Resolve(ctx, projectName)
	if err != nil {
		return "", err
	}

	containers, err := s.projectContainers(ctx, resolved.NormalizedName)
	if err != nil {
		return "", err
	}
	for _, entry := range containers {
		if entry.Name == container {
			return container, nil
		}
	}

	return "", errs.New(
		errs.CodeProjectContainerNotFound,
		fmt.Sprintf("container %q is not part of project %q", container, resolved.NormalizedName),
	)
}

func (s *ProjectRuntimeService) projectContainers(ctx context.Context, normalizedProject string) ([]DockerContainer, error) {
	if s.host == nil {
		return nil, errs.New(errs.CodeProjectDetailFailed, "host service unavailable")
	}

	containers, err := s.host.ListContainers(ctx, true)
	if err != nil {
		return nil, errs.Wrap(errs.CodeProjectDetailFailed, "failed to list project containers", err)
	}

	filtered := make([]DockerContainer, 0)
	for _, container := range containers {
		if strings.EqualFold(strings.TrimSpace(container.Project), normalizedProject) {
			filtered = append(filtered, container)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	return filtered, nil
}

func (s *ProjectRuntimeService) groupProjectContainers(ctx context.Context) (map[string][]DockerContainer, bool) {
	grouped := make(map[string][]DockerContainer)
	if s.host == nil {
		return grouped, false
	}

	containers, err := s.host.ListContainers(ctx, true)
	if err != nil {
		return grouped, false
	}

	for _, container := range containers {
		project := strings.ToLower(strings.TrimSpace(container.Project))
		if project == "" {
			continue
		}
		grouped[project] = append(grouped[project], container)
	}

	return grouped, true
}

func deriveProjectRuntimeStatus(containers []DockerContainer) string {
	if len(containers) == 0 {
		return "down"
	}

	running := 0
	healthy := 0
	for _, container := range containers {
		if !isRunningContainerStatus(container.Status) {
			continue
		}
		running++
		if isHealthyContainerStatus(container.Status) {
			healthy++
		}
	}

	if running == 0 {
		return "down"
	}
	if running == len(containers) && healthy == len(containers) {
		return "running"
	}
	return "degraded"
}

func isRunningContainerStatus(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	return normalized == "running" || strings.HasPrefix(normalized, "up") || strings.Contains(normalized, " running")
}

func isHealthyContainerStatus(status string) bool {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if !isRunningContainerStatus(normalized) {
		return false
	}

	if strings.Contains(normalized, "unhealthy") {
		return false
	}
	if strings.Contains(normalized, "health: starting") || strings.Contains(normalized, "(starting)") {
		return false
	}
	if strings.Contains(normalized, "restarting") || strings.Contains(normalized, "paused") {
		return false
	}

	return true
}

func deriveProjectRuntimePorts(containers []DockerContainer) (int, int) {
	proxyPort := 0
	dbPort := 0
	for _, container := range containers {
		for _, binding := range container.PortBindings {
			if !binding.Published || binding.HostPort <= 0 {
				continue
			}
			if proxyPort == 0 && binding.ContainerPort == 80 {
				proxyPort = binding.HostPort
			}
			if dbPort == 0 && binding.ContainerPort == 5432 {
				dbPort = binding.HostPort
			}
			if proxyPort != 0 && dbPort != 0 {
				return proxyPort, dbPort
			}
		}
	}
	return proxyPort, dbPort
}

func (s *ProjectRuntimeService) syncRuntimeProjects(
	ctx context.Context,
	projectContainers map[string][]DockerContainer,
) (bool, error) {
	projects, err := s.projects.List(ctx)
	if err != nil {
		return false, err
	}

	projectByName := make(map[string]models.Project, len(projects))
	for _, project := range projects {
		key := strings.ToLower(strings.TrimSpace(project.Name))
		if key == "" {
			continue
		}
		projectByName[key] = project
	}

	changed := false
	for key, containers := range projectContainers {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" {
			continue
		}
		if err := validate.ProjectName(normalized); err != nil {
			continue
		}

		status := deriveProjectRuntimeStatus(containers)
		proxyPort, dbPort := deriveProjectRuntimePorts(containers)
		path := ""
		if runtimeDir, _, runtimeErr := resolveDirFromRuntimeCompose(ctx, s.templatesDir, normalized, s.runtimeMetaClient()); runtimeErr == nil {
			path = runtimeDir
		}

		if project, exists := projectByName[normalized]; exists {
			updated := false

			if strings.TrimSpace(project.Status) != status {
				project.Status = status
				updated = true
			}
			if proxyPort > 0 && project.ProxyPort != proxyPort {
				project.ProxyPort = proxyPort
				updated = true
			}
			if dbPort > 0 && project.DBPort != dbPort {
				project.DBPort = dbPort
				updated = true
			}
			if path != "" && strings.TrimSpace(project.Path) != path {
				project.Path = path
				updated = true
			}

			if updated {
				if err := s.projects.Update(ctx, &project); err != nil {
					return changed, err
				}
				changed = true
			}
			continue
		}

		created := models.Project{
			Name:      normalized,
			Path:      path,
			ProxyPort: proxyPort,
			DBPort:    dbPort,
			Status:    status,
		}
		if err := s.projects.Create(ctx, &created); err != nil {
			return changed, err
		}
		projectByName[normalized] = created
		changed = true
	}

	return changed, nil
}

func envFileInfo(path string) (exists bool, sizeBytes int64, updatedAt *time.Time) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false, 0, nil
	}
	modified := info.ModTime().UTC()
	return true, info.Size(), &modified
}

func degradedProjectRuntimeDiagnostics(err error) []ProjectDetailDiagnostic {
	sourceCode := projectRuntimeDiagnosticSourceCode(err)
	taskType := projectRuntimeDiagnosticTaskType(err)

	return []ProjectDetailDiagnostic{
		{
			Scope:      projectDetailDiagnosticScopeContainers,
			Status:     projectDetailDiagnosticStatusDegraded,
			Code:       projectDetailDiagnosticCodeContainersDegraded,
			Message:    "Docker-backed container inventory is unavailable; showing project metadata only.",
			SourceCode: sourceCode,
			TaskType:   taskType,
		},
		{
			Scope:      projectDetailDiagnosticScopePublishedPorts,
			Status:     projectDetailDiagnosticStatusDegraded,
			Code:       projectDetailDiagnosticCodePublishedPortsDegraded,
			Message:    "Docker-backed published port inventory is unavailable; published port data may be incomplete.",
			SourceCode: sourceCode,
			TaskType:   taskType,
		},
	}
}

func projectRuntimeDiagnosticSourceCode(err error) string {
	for current := errors.Unwrap(err); current != nil; current = errors.Unwrap(current) {
		typed, ok := errs.From(current)
		if !ok {
			continue
		}
		code := strings.TrimSpace(string(typed.Code))
		if code != "" {
			return code
		}
	}

	typed, ok := errs.From(err)
	if !ok {
		return ""
	}
	return strings.TrimSpace(string(typed.Code))
}

func projectRuntimeDiagnosticTaskType(err error) string {
	for current := err; current != nil; current = errors.Unwrap(current) {
		typed, ok := errs.From(current)
		if !ok {
			continue
		}
		details, ok := typed.Details.(map[string]any)
		if !ok {
			continue
		}
		rawTaskType, exists := details["task_type"]
		if !exists {
			continue
		}
		switch value := rawTaskType.(type) {
		case contract.TaskType:
			return strings.TrimSpace(string(value))
		case string:
			return strings.TrimSpace(value)
		}
	}
	return ""
}
