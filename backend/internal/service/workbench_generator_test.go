package service

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go-notes/internal/errs"
)

func TestWorkbenchGenerateComposeFromStoredSnapshotDeterministic(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	composeSource := `
services:
  db:
    image: postgres:16
    ports:
      - "5432:5432"
    networks:
      - backplane
  api:
    image: "ghcr.io/demo/api:${API_TAG:-latest}"
    restart: unless-stopped
    depends_on:
      - db
    ports:
      - "127.0.0.1:${API_PORT}:8080/tcp"
      - "9443:9443"
    deploy:
      resources:
        limits:
          cpus: "${API_CPU_LIMIT:-1.00}"
          memory: "512M"
        reservations:
          memory: "256M"
    networks:
      - edge
      - backplane
networks:
  edge: {}
  backplane: {}
`
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte(composeSource), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	settingsRepo := &fakeSettingsRepo{}
	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, settingsRepo, "test-session-secret")
	if _, _, err := svc.ImportComposeSnapshot(context.Background(), "demo", "manual"); err != nil {
		t.Fatalf("import snapshot: %v", err)
	}

	_, first, err := svc.GenerateComposeFromStoredSnapshot(context.Background(), "demo")
	if err != nil {
		t.Fatalf("first generation: %v", err)
	}
	_, second, err := svc.GenerateComposeFromStoredSnapshot(context.Background(), "demo")
	if err != nil {
		t.Fatalf("second generation: %v", err)
	}

	if first != second {
		t.Fatalf("expected byte-stable generation output for identical snapshot\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	for _, want := range []string{
		"${API_TAG:-latest}",
		"127.0.0.1:${API_PORT}:8080",
		"${API_CPU_LIMIT:-1.00}",
	} {
		if !strings.Contains(first, want) {
			t.Fatalf("expected generated compose to preserve interpolation %q", want)
		}
	}

	apiIndex := strings.Index(first, "\n  api:\n")
	dbIndex := strings.Index(first, "\n  db:\n")
	if apiIndex < 0 || dbIndex < 0 || apiIndex > dbIndex {
		t.Fatalf("expected deterministic service ordering [api, db], output:\n%s", first)
	}

	apiSection := first[apiIndex:dbIndex]
	imageIndex := strings.Index(apiSection, "\n    image:")
	restartIndex := strings.Index(apiSection, "\n    restart:")
	dependsIndex := strings.Index(apiSection, "\n    depends_on:")
	portsIndex := strings.Index(apiSection, "\n    ports:")
	deployIndex := strings.Index(apiSection, "\n    deploy:")
	networksIndex := strings.Index(apiSection, "\n    networks:")
	if !(imageIndex >= 0 && restartIndex > imageIndex && dependsIndex > restartIndex && portsIndex > dependsIndex && deployIndex > portsIndex && networksIndex > deployIndex) {
		t.Fatalf("expected deterministic service field ordering in api section, got:\n%s", apiSection)
	}
}

func TestWorkbenchGenerateComposeFromStoredSnapshotNotFound(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	_, _, err := svc.GenerateComposeFromStoredSnapshot(context.Background(), "demo")
	if err == nil {
		t.Fatal("expected missing snapshot error")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchSourceNotFound {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchSourceNotFound, typed.Code)
	}
}

func TestGenerateWorkbenchComposeInvalidModelValidationError(t *testing.T) {
	t.Parallel()

	snapshot := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
		},
		Dependencies: []WorkbenchComposeDependency{
			{ServiceName: "api", DependsOn: "db"},
		},
		Ports: []WorkbenchComposePort{
			{ServiceName: "missing", ContainerPort: 8080},
		},
		VolumeRefs: []WorkbenchComposeVolumeRef{
			{ServiceName: "api", VolumeName: "cache"},
		},
	}

	_, err := generateWorkbenchCompose(snapshot)
	if err == nil {
		t.Fatal("expected invalid snapshot validation error")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}

	details, ok := typed.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected details map, got %T", typed.Details)
	}
	issues := extractWorkbenchValidationIssues(t, details)
	if len(issues) < 3 {
		t.Fatalf("expected multiple validation issues, got %v", issues)
	}
	hasSchema := false
	hasDependency := false
	for _, issue := range issues {
		if issue.Class == workbenchValidationClassSchema {
			hasSchema = true
		}
		if issue.Class == workbenchValidationClassDependency {
			hasDependency = true
		}
	}
	if !hasSchema || !hasDependency {
		t.Fatalf("expected schema and dependency issue classes, got %#v", issues)
	}
}

func TestGenerateWorkbenchComposeDetectsHostPortConflictsAcrossServices(t *testing.T) {
	t.Parallel()

	hostPort := 8080
	snapshot := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "web", Image: "nginx:stable"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:   "api",
				ContainerPort: 8080,
				HostPort:      &hostPort,
				Protocol:      "tcp",
				HostIP:        "",
			},
			{
				ServiceName:   "web",
				ContainerPort: 9090,
				HostPort:      &hostPort,
				Protocol:      "tcp",
				HostIP:        "127.0.0.1",
			},
		},
	}

	_, err := generateWorkbenchCompose(snapshot)
	if err == nil {
		t.Fatal("expected host port conflict validation error")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}

	details, ok := typed.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected details map, got %T", typed.Details)
	}
	issues := extractWorkbenchValidationIssues(t, details)

	foundConflict := false
	for _, issue := range issues {
		if issue.Class == workbenchValidationClassPortConflict && issue.Code == "WB-VAL-PORT-HOST-CONFLICT" {
			foundConflict = true
			break
		}
	}
	if !foundConflict {
		t.Fatalf("expected host port conflict issue, got %#v", issues)
	}
}

func TestGenerateWorkbenchComposeValidationDiagnosticsDeterministic(t *testing.T) {
	t.Parallel()

	hostPort := 8080
	invalid := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "web", Image: "nginx:stable"},
			{ServiceName: "", BuildSource: ""},
		},
		Dependencies: []WorkbenchComposeDependency{
			{ServiceName: "api", DependsOn: "missing"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:   "api",
				ContainerPort: 8080,
				HostPort:      &hostPort,
				Protocol:      "tcp",
			},
			{
				ServiceName:   "web",
				ContainerPort: 8081,
				HostPort:      &hostPort,
				Protocol:      "tcp",
				HostIP:        "127.0.0.1",
			},
			{
				ServiceName:   "api",
				ContainerPort: 70000,
				Protocol:      "tcp",
			},
		},
	}

	_, firstErr := generateWorkbenchCompose(invalid)
	if firstErr == nil {
		t.Fatal("expected first validation error")
	}
	firstTyped, ok := errs.From(firstErr)
	if !ok {
		t.Fatalf("expected typed first error, got %T", firstErr)
	}
	firstDetails, ok := firstTyped.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected first details map, got %T", firstTyped.Details)
	}
	firstIssues := extractWorkbenchValidationIssues(t, firstDetails)

	_, secondErr := generateWorkbenchCompose(invalid)
	if secondErr == nil {
		t.Fatal("expected second validation error")
	}
	secondTyped, ok := errs.From(secondErr)
	if !ok {
		t.Fatalf("expected typed second error, got %T", secondErr)
	}
	secondDetails, ok := secondTyped.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected second details map, got %T", secondTyped.Details)
	}
	secondIssues := extractWorkbenchValidationIssues(t, secondDetails)

	if !reflect.DeepEqual(firstIssues, secondIssues) {
		t.Fatalf("expected deterministic issue ordering\nfirst=%#v\nsecond=%#v", firstIssues, secondIssues)
	}

	hasSchema := false
	hasDependency := false
	hasPortConflict := false
	for _, issue := range firstIssues {
		switch issue.Class {
		case workbenchValidationClassSchema:
			hasSchema = true
		case workbenchValidationClassDependency:
			hasDependency = true
		case workbenchValidationClassPortConflict:
			hasPortConflict = true
		}
	}
	if !hasSchema || !hasDependency || !hasPortConflict {
		t.Fatalf(
			"expected schema+dependency+port_conflict classes, got schema=%t dependency=%t port_conflict=%t issues=%#v",
			hasSchema,
			hasDependency,
			hasPortConflict,
			firstIssues,
		)
	}
}

func TestWorkbenchValidateStoredSnapshotForCompose(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n  api:\n    image: nginx:stable\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	settingsRepo := &fakeSettingsRepo{}
	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, settingsRepo, "test-session-secret")
	if _, _, err := svc.ImportComposeSnapshot(context.Background(), "demo", "manual"); err != nil {
		t.Fatalf("import snapshot: %v", err)
	}

	if _, err := svc.ValidateStoredSnapshotForCompose(context.Background(), "demo"); err != nil {
		t.Fatalf("expected valid stored snapshot, got error: %v", err)
	}
}

func TestWorkbenchValidateStoredSnapshotForComposeValidationError(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	invalid := WorkbenchStackSnapshot{
		ProjectName: "demo",
		ComposePath: "/tmp/demo/docker-compose.yml",
		Services: []WorkbenchComposeService{
			{ServiceName: "api"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", invalid); err != nil {
		t.Fatalf("save invalid snapshot: %v", err)
	}

	_, err := svc.ValidateStoredSnapshotForCompose(context.Background(), "demo")
	if err == nil {
		t.Fatal("expected validation error for invalid stored snapshot")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}
	details, ok := typed.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected details map, got %T", typed.Details)
	}
	issues := extractWorkbenchValidationIssues(t, details)
	if len(issues) == 0 {
		t.Fatalf("expected validation issues, got %#v", issues)
	}
}

func extractWorkbenchValidationIssues(t *testing.T, details map[string]any) []WorkbenchValidationIssue {
	t.Helper()

	issuesAny, ok := details["issues"]
	if !ok {
		t.Fatalf("expected issues in details: %#v", details)
	}
	issues, ok := issuesAny.([]WorkbenchValidationIssue)
	if !ok {
		t.Fatalf("expected issues as []WorkbenchValidationIssue, got %T", issuesAny)
	}
	return issues
}
