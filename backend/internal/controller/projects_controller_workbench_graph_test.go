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

	"go-notes/internal/service"
)

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
