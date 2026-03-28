package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/auth"
	"go-notes/internal/models"
	"go-notes/internal/service"
)

func TestWorkbenchMutateModuleCompatibilityAddUsesManagedServicePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n  api:\n    image: ghcr.io/example/api:latest\n"), 0o644); err != nil {
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
	ctx.Set("session", auth.Session{
		UserID:    1,
		Login:     "admin",
		Role:      models.RoleAdmin,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/projects/demo/workbench/modules", strings.NewReader(`{"selector":{"serviceName":"redis","moduleType":"redis"},"action":"add"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	controller.WorkbenchMutateModule(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Stack    service.WorkbenchStackSnapshot         `json:"stack"`
		Mutation service.WorkbenchModuleMutationSummary `json:"mutation"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !response.Mutation.Changed {
		t.Fatalf("expected changed mutation summary, got %#v", response.Mutation)
	}
	if response.Mutation.PreviousCount != 0 || response.Mutation.CurrentCount != 1 {
		t.Fatalf("unexpected mutation summary counts: %#v", response.Mutation)
	}
	if got, want := len(response.Stack.ManagedServices), 1; got != want {
		t.Fatalf("expected %d managed service, got %d", want, got)
	}
	if response.Stack.ManagedServices[0] != (service.WorkbenchManagedService{EntryKey: "redis", ServiceName: "redis"}) {
		t.Fatalf("unexpected managed service record: %#v", response.Stack.ManagedServices[0])
	}
}

func TestWorkbenchMutateModuleCompatibilityRemoveMissingIsNoOp(t *testing.T) {
	gin.SetMode(gin.TestMode)

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n  redis:\n    image: redis:7-alpine\n"), 0o644); err != nil {
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
	ctx.Set("session", auth.Session{
		UserID:    1,
		Login:     "admin",
		Role:      models.RoleAdmin,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/projects/demo/workbench/modules", strings.NewReader(`{"selector":{"serviceName":"redis","moduleType":"redis"},"action":"remove"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	controller.WorkbenchMutateModule(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Stack    service.WorkbenchStackSnapshot         `json:"stack"`
		Mutation service.WorkbenchModuleMutationSummary `json:"mutation"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Mutation.Changed {
		t.Fatalf("expected no-op remove summary, got %#v", response.Mutation)
	}
	if response.Mutation.PreviousCount != 0 || response.Mutation.CurrentCount != 0 {
		t.Fatalf("unexpected mutation summary counts: %#v", response.Mutation)
	}
}
