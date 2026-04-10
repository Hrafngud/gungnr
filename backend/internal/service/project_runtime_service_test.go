package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"
	"go-notes/internal/models"
	"go-notes/internal/repository"

	"github.com/stretchr/testify/require"
)

type stubProjectRepository struct {
	projects       []models.Project
	listCalls      int
	createCalls    int
	updateCalls    int
	getByNameCalls int
}

func (s *stubProjectRepository) List(_ context.Context) ([]models.Project, error) {
	s.listCalls++
	items := make([]models.Project, len(s.projects))
	copy(items, s.projects)
	return items, nil
}

func (s *stubProjectRepository) Create(_ context.Context, project *models.Project) error {
	s.createCalls++
	s.projects = append(s.projects, *project)
	return nil
}

func (s *stubProjectRepository) GetByName(_ context.Context, name string) (*models.Project, error) {
	s.getByNameCalls++
	for _, project := range s.projects {
		if project.Name == name {
			item := project
			return &item, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (s *stubProjectRepository) Update(_ context.Context, project *models.Project) error {
	s.updateCalls++
	for index, existing := range s.projects {
		if existing.Name == project.Name {
			s.projects[index] = *project
			return nil
		}
	}
	return repository.ErrNotFound
}

func TestProjectRuntimeServiceDetailReturnsDegradedRuntimeDiagnosticsWhenDockerInventoryFails(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ".env"), []byte("KEY=value\n"), 0o644))

	repo := &stubProjectRepository{
		projects: []models.Project{
			{
				Name:      "demo",
				Path:      projectDir,
				ProxyPort: 8080,
				DBPort:    5432,
				Status:    "running",
			},
		},
	}
	bridge := &stubHostInfraBridgeClient{
		listContainersErr: errors.New("docker socket unavailable"),
	}
	host := &HostService{infraClient: bridge}
	svc := NewProjectRuntimeService(templatesDir, repo, host)

	detail, err := svc.Detail(context.Background(), "demo")
	require.NoError(t, err)
	require.True(t, bridge.listContainersCalled)

	require.Equal(t, "demo", detail.Project.NormalizedName)
	require.NotNil(t, detail.Project.Record)
	require.Equal(t, projectDir, detail.Runtime.Path)
	require.Equal(t, "db_path", detail.Runtime.Source)
	require.True(t, detail.Runtime.EnvExists)
	require.Equal(t, filepath.Join(projectDir, ".env"), detail.Runtime.EnvPath)

	require.Empty(t, detail.Containers)
	require.Empty(t, detail.Network.PublishedPorts)
	require.Equal(t, 8080, detail.Network.ProxyPort)
	require.Equal(t, 5432, detail.Network.DBPort)
	require.Len(t, detail.Diagnostics, 2)
	require.True(t, detail.HasDiagnosticScope(projectDetailDiagnosticScopeContainers))
	require.True(t, detail.HasDiagnosticScope(projectDetailDiagnosticScopePublishedPorts))

	require.Equal(t, ProjectDetailDiagnostic{
		Scope:      projectDetailDiagnosticScopeContainers,
		Status:     projectDetailDiagnosticStatusDegraded,
		Code:       projectDetailDiagnosticCodeContainersDegraded,
		Message:    "Docker-backed container inventory is unavailable; showing project metadata only.",
		SourceCode: string(errs.CodeHostDockerFailed),
		TaskType:   "docker_list_containers",
	}, detail.Diagnostics[0])
	require.Equal(t, ProjectDetailDiagnostic{
		Scope:      projectDetailDiagnosticScopePublishedPorts,
		Status:     projectDetailDiagnosticStatusDegraded,
		Code:       projectDetailDiagnosticCodePublishedPortsDegraded,
		Message:    "Docker-backed published port inventory is unavailable; published port data may be incomplete.",
		SourceCode: string(errs.CodeHostDockerFailed),
		TaskType:   "docker_list_containers",
	}, detail.Diagnostics[1])
}

func TestProjectRuntimeServiceListStatusesReturnsRuntimeStatusesWithoutPersistenceSync(t *testing.T) {
	t.Parallel()

	repo := &stubProjectRepository{
		projects: []models.Project{
			{Name: "demo", Status: "unknown"},
			{Name: "edge", Status: "unknown"},
		},
	}
	bridge := &stubHostInfraBridgeClient{
		listContainersResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					`{"ID":"1","Names":"demo-api-1","Image":"ghcr.io/acme/demo","Status":"Up 3 minutes (healthy)","Labels":"com.docker.compose.project=demo,com.docker.compose.service=api"}`,
					`{"ID":"2","Names":"demo-db-1","Image":"postgres:16","Status":"Up 3 minutes (healthy)","Labels":"com.docker.compose.project=demo,com.docker.compose.service=db"}`,
					`{"ID":"3","Names":"edge-api-1","Image":"ghcr.io/acme/edge","Status":"Exited (1) 10 seconds ago","Labels":"com.docker.compose.project=edge,com.docker.compose.service=api"}`,
				},
			},
		},
	}
	host := &HostService{infraClient: bridge}
	svc := NewProjectRuntimeService("", repo, host)

	statuses, err := svc.ListStatuses(context.Background())
	require.NoError(t, err)
	require.Equal(t, []ProjectStatus{
		{Name: "demo", Status: "running"},
		{Name: "edge", Status: "down"},
	}, statuses)
	require.True(t, bridge.listContainersCalled)
	require.Equal(t, 1, repo.listCalls)
	require.Zero(t, repo.createCalls)
	require.Zero(t, repo.updateCalls)
}
