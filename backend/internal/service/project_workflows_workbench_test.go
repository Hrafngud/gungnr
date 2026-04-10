package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-notes/internal/errs"
)

type captureWorkflowLogger struct {
	lines []string
}

func (l *captureWorkflowLogger) Log(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	l.lines = append(l.lines, line)
}

func (l *captureWorkflowLogger) Logf(format string, args ...any) {
	l.Log(fmt.Sprintf(format, args...))
}

func TestProjectWorkflowsPrepareWorkbenchManagedComposeImportsAndApplies(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	compose := `
services:
  web:
    image: nginx:1.25
    env_file:
      - .env
    ports:
      - "80:80"
  db:
    image: postgres:16
    ports:
      - "${DB_PORT:-5432}:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata: {}
`
	if err := os.WriteFile(composePath, []byte(compose), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	workflows := &ProjectWorkflows{workbench: svc}
	logger := &captureWorkflowLogger{}

	result, err := workflows.prepareWorkbenchManagedCompose(
		context.Background(),
		logger,
		"demo",
		workbenchImportReasonAutoDeploy,
		[]workbenchRequestedPortAssignment{
			{Label: "proxy", ContainerPort: 80, HostPort: 8088, Required: true},
			{Label: "db", ContainerPort: 5432, HostPort: 15432, Required: false},
		},
	)
	if err != nil {
		t.Fatalf("prepare workbench compose: %v", err)
	}

	updated, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	updatedCompose := string(updated)
	for _, want := range []string{
		`8088:80`,
		`15432:5432`,
		"env_file:",
		"pgdata:/var/lib/postgresql/data",
	} {
		if !strings.Contains(updatedCompose, want) {
			t.Fatalf("expected compose to contain %q, got:\n%s", want, updatedCompose)
		}
	}
	if result.ComposeBytes <= 0 {
		t.Fatalf("expected compose bytes > 0")
	}
	if len(logger.lines) == 0 {
		t.Fatalf("expected workflow logs to be written")
	}
	if joined := strings.Join(logger.lines, "\n"); !strings.Contains(joined, "workbench import completed") || !strings.Contains(joined, "workbench compose apply completed") {
		t.Fatalf("expected import/apply logs, got:\n%s", joined)
	}
}

func TestProjectWorkflowsEnsureWorkbenchSnapshotForJobImportsMissingSnapshot(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n  api:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	workflows := &ProjectWorkflows{workbench: svc}
	logger := &captureWorkflowLogger{}

	snapshot, imported, err := workflows.ensureWorkbenchSnapshotForJob(context.Background(), logger, "demo", workbenchImportReasonAutoRedeploy)
	if err != nil {
		t.Fatalf("ensure workbench snapshot: %v", err)
	}
	if !imported {
		t.Fatalf("expected missing snapshot import")
	}
	if snapshot.Revision != 1 {
		t.Fatalf("expected revision 1, got %d", snapshot.Revision)
	}
	if joined := strings.Join(logger.lines, "\n"); !strings.Contains(joined, "workbench snapshot missing") {
		t.Fatalf("expected missing snapshot log, got:\n%s", joined)
	}
}

func TestProjectWorkflowsPrepareWorkbenchManagedComposeMissingRequiredPortReturnsTypedError(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n  api:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	workflows := &ProjectWorkflows{workbench: svc}
	logger := &captureWorkflowLogger{}

	_, err := workflows.prepareWorkbenchManagedCompose(
		context.Background(),
		logger,
		"demo",
		workbenchImportReasonAutoDeploy,
		[]workbenchRequestedPortAssignment{
			{Label: "proxy", ContainerPort: 80, HostPort: 8088, Required: true},
		},
	)
	if err == nil {
		t.Fatal("expected validation error")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}
	if !strings.Contains(err.Error(), string(errs.CodeWorkbenchValidationFailed)) {
		t.Fatalf("expected wrapped error string to include code, got %q", err.Error())
	}
}
