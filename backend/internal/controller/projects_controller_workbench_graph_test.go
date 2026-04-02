package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"go-notes/internal/models"
	"go-notes/internal/repository"
	"go-notes/internal/service"
)

type graphTestProjectRepository struct {
	projects []models.Project
}

func (s *graphTestProjectRepository) List(context.Context) ([]models.Project, error) {
	items := make([]models.Project, len(s.projects))
	copy(items, s.projects)
	return items, nil
}

func (s *graphTestProjectRepository) Create(_ context.Context, project *models.Project) error {
	s.projects = append(s.projects, *project)
	return nil
}

func (s *graphTestProjectRepository) GetByName(_ context.Context, name string) (*models.Project, error) {
	for _, project := range s.projects {
		if project.Name == name {
			item := project
			return &item, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (s *graphTestProjectRepository) Update(_ context.Context, project *models.Project) error {
	for index, existing := range s.projects {
		if existing.Name == project.Name {
			s.projects[index] = *project
			return nil
		}
	}
	return repository.ErrNotFound
}

func TestWorkbenchGraphReturnsDedicatedGraphPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	compose := `
services:
  api:
    image: ghcr.io/example/api:latest
    depends_on:
      - db
  db:
    image: postgres:16
`
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	workbench := service.NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	if _, _, err := workbench.ImportComposeSnapshot(context.Background(), "demo", "manual"); err != nil {
		t.Fatalf("ImportComposeSnapshot: %v", err)
	}

	controller := NewProjectsController(nil, nil, workbench, nil, nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "name", Value: "demo"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/projects/demo/workbench/graph", nil)

	controller.WorkbenchGraph(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Graph service.WorkbenchDependencyGraph `json:"graph"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Graph.ProjectName != "demo" {
		t.Fatalf("expected projectName demo, got %q", response.Graph.ProjectName)
	}
	if len(response.Graph.Nodes) != 2 {
		t.Fatalf("expected 2 graph nodes, got %d", len(response.Graph.Nodes))
	}
	if len(response.Graph.Edges) != 1 {
		t.Fatalf("expected 1 graph edge, got %d", len(response.Graph.Edges))
	}
	if got := response.Graph.Edges[0].Key; got != "db->api" {
		t.Fatalf("expected edge key db->api, got %q", got)
	}
	if len(response.Graph.Warnings) == 0 {
		t.Fatal("expected runtime warning when runtime service is unavailable")
	}
}

func TestWorkbenchGraphWarnsWhenRuntimeDetailDegradesContainerInventory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	compose := `
services:
  api:
    image: ghcr.io/example/api:latest
`
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	workbench := service.NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	if _, _, err := workbench.ImportComposeSnapshot(context.Background(), "demo", "manual"); err != nil {
		t.Fatalf("ImportComposeSnapshot: %v", err)
	}

	runtime := service.NewProjectRuntimeService(
		templatesDir,
		&graphTestProjectRepository{
			projects: []models.Project{
				{Name: "demo", Path: projectDir, Status: "running"},
			},
		},
		nil,
	)

	controller := NewProjectsController(nil, nil, workbench, runtime, nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "name", Value: "demo"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/projects/demo/workbench/graph", nil)

	controller.WorkbenchGraph(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Graph service.WorkbenchDependencyGraph `json:"graph"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Graph.Warnings) == 0 {
		t.Fatal("expected runtime warning when runtime detail degrades container inventory")
	}
}
