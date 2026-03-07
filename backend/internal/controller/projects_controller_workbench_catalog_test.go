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

func TestWorkbenchCatalogReturnsTransitionAwareCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	compose := `
services:
  cache:
    image: redis:7-alpine
  proxy:
    image: nginx:stable-alpine
`
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	workbench := service.NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	if _, _, err := workbench.ImportComposeSnapshot(context.Background(), "demo", "manual"); err != nil {
		t.Fatalf("ImportComposeSnapshot: %v", err)
	}
	if _, _, err := workbench.MutateStoredSnapshotModule(context.Background(), "demo", service.WorkbenchModuleMutationRequest{
		Selector: service.WorkbenchModuleSelector{
			ServiceName: "proxy",
			ModuleType:  "redis",
		},
		Action: "add",
	}); err != nil {
		t.Fatalf("MutateStoredSnapshotModule: %v", err)
	}

	controller := NewProjectsController(nil, nil, workbench, nil, nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "name", Value: "demo"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/projects/demo/workbench/catalog", nil)

	controller.WorkbenchCatalog(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response struct {
		Catalog service.WorkbenchOptionalServiceCatalog `json:"catalog"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !response.Catalog.SnapshotImported {
		t.Fatal("expected snapshotImported=true")
	}
	if got, want := len(response.Catalog.Entries), 4; got != want {
		t.Fatalf("expected %d entries, got %d", want, got)
	}
	if response.Catalog.LegacyModules.Status != "present" {
		t.Fatalf("expected legacy status present, got %q", response.Catalog.LegacyModules.Status)
	}

	var redisEntry *service.WorkbenchOptionalServiceCatalogEntry
	for idx := range response.Catalog.Entries {
		if response.Catalog.Entries[idx].Key == "redis" {
			redisEntry = &response.Catalog.Entries[idx]
			break
		}
	}
	if redisEntry == nil {
		t.Fatal("expected redis entry in response")
	}
	if redisEntry.Availability.Status != "compose_present_with_legacy_module" {
		t.Fatalf("expected redis transition status, got %q", redisEntry.Availability.Status)
	}
	if got, want := len(redisEntry.Availability.LegacyModules), 1; got != want {
		t.Fatalf("expected %d redis legacy module, got %d", want, got)
	}
}
