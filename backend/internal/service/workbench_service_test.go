package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go-notes/internal/config"
	"go-notes/internal/errs"
	"go-notes/internal/models"
)

func TestWorkbenchServiceGetSnapshotEmptyState(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "alpha")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}
	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	settingsRepo := &fakeSettingsRepo{}
	settingsService := NewSettingsService(config.Config{SessionSecret: "test-secret"}, settingsRepo)
	workbenchService := NewWorkbenchService(config.Config{TemplatesDir: templatesDir}, nil, settingsService)

	snapshot, err := workbenchService.GetSnapshot(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("GetSnapshot returned error: %v", err)
	}

	if snapshot.Project.Name != "alpha" {
		t.Fatalf("expected project name alpha, got %q", snapshot.Project.Name)
	}
	if snapshot.Project.NormalizedName != "alpha" {
		t.Fatalf("expected normalized project name alpha, got %q", snapshot.Project.NormalizedName)
	}
	if snapshot.Project.Path != projectDir {
		t.Fatalf("expected project path %q, got %q", projectDir, snapshot.Project.Path)
	}
	if snapshot.Project.ComposePath != composePath {
		t.Fatalf("expected compose path %q, got %q", composePath, snapshot.Project.ComposePath)
	}
	if snapshot.ModelVersion != 1 {
		t.Fatalf("expected modelVersion 1, got %d", snapshot.ModelVersion)
	}
	if snapshot.Revision != 0 {
		t.Fatalf("expected revision 0, got %d", snapshot.Revision)
	}
	if snapshot.SourceFingerprint != nil {
		t.Fatalf("expected nil source fingerprint, got %q", *snapshot.SourceFingerprint)
	}
	if len(snapshot.Services) != 0 || len(snapshot.Ports) != 0 || len(snapshot.Resources) != 0 || len(snapshot.Modules) != 0 || len(snapshot.Warnings) != 0 {
		t.Fatalf("expected all workbench collections to be empty, got services=%d ports=%d resources=%d modules=%d warnings=%d",
			len(snapshot.Services),
			len(snapshot.Ports),
			len(snapshot.Resources),
			len(snapshot.Modules),
			len(snapshot.Warnings),
		)
	}
}

func TestWorkbenchServiceGetSnapshotNormalizesOrdering(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "alpha")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	blob := settingsSecureBlob{
		WorkbenchSnapshots: map[string]workbenchStoredSnapshot{
			"ALPHA": {
				ComposePath:       "/custom/compose.yaml",
				ModelVersion:      3,
				Revision:          9,
				SourceFingerprint: " sha256:seeded ",
				Services: []WorkbenchSnapshotService{
					{ServiceName: "web"},
					{ServiceName: "api"},
				},
				Ports: []WorkbenchSnapshotPort{
					{ServiceName: "web", ContainerPort: 443, Protocol: "tcp", HostIP: "", HostPort: 8443},
					{ServiceName: "api", ContainerPort: 80, Protocol: "udp", HostIP: "", HostPort: 81},
					{ServiceName: "api", ContainerPort: 80, Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 82},
					{ServiceName: "api", ContainerPort: 80, Protocol: "tcp", HostIP: "", HostPort: 83},
				},
				Resources: []WorkbenchSnapshotResource{
					{ServiceName: "web"},
					{ServiceName: "api"},
				},
				Modules: []WorkbenchSnapshotModule{
					{ModuleType: "redis", ServiceName: "cache"},
					{ModuleType: "addon", ServiceName: "web"},
					{ModuleType: "addon", ServiceName: "api"},
				},
				Warnings: []WorkbenchSnapshotWarning{
					{Code: "WB-2", Path: "/z"},
					{Code: "WB-1", Path: "/b"},
					{Code: "WB-1", Path: "/a"},
				},
			},
		},
	}
	encoded, err := encodeSettingsSecureBlob("test-secret", blob)
	if err != nil {
		t.Fatalf("encode secure blob: %v", err)
	}

	settingsRepo := &fakeSettingsRepo{
		settings: &models.Settings{NetBirdConfigEncrypted: encoded},
	}
	settingsService := NewSettingsService(config.Config{SessionSecret: "test-secret"}, settingsRepo)
	workbenchService := NewWorkbenchService(config.Config{TemplatesDir: templatesDir}, nil, settingsService)

	snapshot, err := workbenchService.GetSnapshot(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("GetSnapshot returned error: %v", err)
	}

	if snapshot.ModelVersion != 3 {
		t.Fatalf("expected modelVersion 3, got %d", snapshot.ModelVersion)
	}
	if snapshot.Revision != 9 {
		t.Fatalf("expected revision 9, got %d", snapshot.Revision)
	}
	if snapshot.SourceFingerprint == nil || *snapshot.SourceFingerprint != "sha256:seeded" {
		t.Fatalf("expected source fingerprint sha256:seeded, got %#v", snapshot.SourceFingerprint)
	}
	if snapshot.Project.ComposePath != filepath.Join(projectDir, "docker-compose.yml") {
		t.Fatalf("expected compose path to prefer resolved project compose file, got %q", snapshot.Project.ComposePath)
	}

	expectedServices := []string{"api", "web"}
	for i, expected := range expectedServices {
		if snapshot.Services[i].ServiceName != expected {
			t.Fatalf("expected service[%d]=%s, got %s", i, expected, snapshot.Services[i].ServiceName)
		}
	}

	expectedPorts := []WorkbenchSnapshotPort{
		{ServiceName: "api", ContainerPort: 80, Protocol: "tcp", HostIP: "", HostPort: 83},
		{ServiceName: "api", ContainerPort: 80, Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 82},
		{ServiceName: "api", ContainerPort: 80, Protocol: "udp", HostIP: "", HostPort: 81},
		{ServiceName: "web", ContainerPort: 443, Protocol: "tcp", HostIP: "", HostPort: 8443},
	}
	for i, expected := range expectedPorts {
		got := snapshot.Ports[i]
		if got != expected {
			t.Fatalf("expected ports[%d]=%+v, got %+v", i, expected, got)
		}
	}

	expectedResources := []string{"api", "web"}
	for i, expected := range expectedResources {
		if snapshot.Resources[i].ServiceName != expected {
			t.Fatalf("expected resource[%d]=%s, got %s", i, expected, snapshot.Resources[i].ServiceName)
		}
	}

	expectedModules := []WorkbenchSnapshotModule{
		{ModuleType: "addon", ServiceName: "api", Source: ""},
		{ModuleType: "addon", ServiceName: "web", Source: ""},
		{ModuleType: "redis", ServiceName: "cache", Source: ""},
	}
	for i, expected := range expectedModules {
		got := snapshot.Modules[i]
		if got != expected {
			t.Fatalf("expected modules[%d]=%+v, got %+v", i, expected, got)
		}
	}

	expectedWarnings := []WorkbenchSnapshotWarning{
		{Code: "WB-1", Path: "/a", Message: ""},
		{Code: "WB-1", Path: "/b", Message: ""},
		{Code: "WB-2", Path: "/z", Message: ""},
	}
	for i, expected := range expectedWarnings {
		got := snapshot.Warnings[i]
		if got != expected {
			t.Fatalf("expected warnings[%d]=%+v, got %+v", i, expected, got)
		}
	}
}

func TestWorkbenchServiceGetSnapshotStorageFailureIsTyped(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "alpha")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	settingsRepo := &fakeSettingsRepo{
		settings: &models.Settings{NetBirdConfigEncrypted: "not-encrypted"},
	}
	settingsService := NewSettingsService(config.Config{SessionSecret: "test-secret"}, settingsRepo)
	workbenchService := NewWorkbenchService(config.Config{TemplatesDir: templatesDir}, nil, settingsService)

	_, err := workbenchService.GetSnapshot(context.Background(), "alpha")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed errs.Error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchStorageFailed {
		t.Fatalf("expected code %s, got %s", errs.CodeWorkbenchStorageFailed, typed.Code)
	}
}
