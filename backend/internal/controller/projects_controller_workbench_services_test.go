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

func TestWorkbenchAddServiceReturnsManagedMutationSummary(t *testing.T) {
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
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/projects/demo/workbench/services", strings.NewReader(`{"entryKey":"minio"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	controller.WorkbenchAddService(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Stack    service.WorkbenchStackSnapshot                  `json:"stack"`
		Mutation service.WorkbenchOptionalServiceMutationSummary `json:"mutation"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Mutation.EntryKey != "minio" || response.Mutation.ServiceName != "minio" {
		t.Fatalf("unexpected mutation summary: %#v", response.Mutation)
	}
	if got, want := len(response.Stack.ManagedServices), 1; got != want {
		t.Fatalf("expected %d managed service, got %d", want, got)
	}
}

func TestWorkbenchRemoveServiceReturnsManagedMutationSummary(t *testing.T) {
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
	if _, _, err := workbench.AddOptionalService(context.Background(), "demo", service.WorkbenchOptionalServiceAddRequest{
		EntryKey: "redis",
	}); err != nil {
		t.Fatalf("AddOptionalService: %v", err)
	}

	controller := NewProjectsController(nil, nil, workbench, nil, nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "name", Value: "demo"}, {Key: "serviceName", Value: "redis"}}
	ctx.Set("session", auth.Session{
		UserID:    1,
		Login:     "admin",
		Role:      models.RoleAdmin,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/projects/demo/workbench/services/redis", nil)

	controller.WorkbenchRemoveService(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Stack    service.WorkbenchStackSnapshot                  `json:"stack"`
		Mutation service.WorkbenchOptionalServiceMutationSummary `json:"mutation"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Mutation.EntryKey != "redis" || response.Mutation.ServiceName != "redis" {
		t.Fatalf("unexpected mutation summary: %#v", response.Mutation)
	}
	if got, want := len(response.Stack.ManagedServices), 0; got != want {
		t.Fatalf("expected %d managed services after removal, got %d", want, got)
	}
}
