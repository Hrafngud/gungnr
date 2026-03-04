package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-notes/internal/errs"
	"go-notes/internal/models"
)

func TestWorkbenchSourceFingerprintEquivalentInput(t *testing.T) {
	t.Parallel()

	a := "services:\r\n  web:\r\n    image: nginx:stable\r\n\r\n---\r\n"
	b := "services:\n  web:\n    image: nginx:stable\n"

	normalizedA, fingerprintA := WorkbenchSourceFingerprint([]byte(a))
	normalizedB, fingerprintB := WorkbenchSourceFingerprint([]byte(b))

	if normalizedA != normalizedB {
		t.Fatalf("expected normalized compose source to match:\nA=%q\nB=%q", normalizedA, normalizedB)
	}
	if fingerprintA != fingerprintB {
		t.Fatalf("expected equivalent sources to share fingerprint: A=%s B=%s", fingerprintA, fingerprintB)
	}
}

func TestWorkbenchSourceFingerprintDiffersOnContentChange(t *testing.T) {
	t.Parallel()

	_, fingerprintA := WorkbenchSourceFingerprint([]byte("services:\n  web:\n    image: nginx:stable\n"))
	_, fingerprintB := WorkbenchSourceFingerprint([]byte("services:\n  web:\n    image: nginx:1.25\n"))

	if fingerprintA == fingerprintB {
		t.Fatalf("expected fingerprint mismatch on content change: %s", fingerprintA)
	}
}

func TestWorkbenchLockConflictTimeout(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchService("/tmp", nil)
	svc.lockWaitTimeout = 40 * time.Millisecond

	release, err := svc.AcquireProjectLock(context.Background(), "demo")
	if err != nil {
		t.Fatalf("acquire initial lock: %v", err)
	}
	defer release()

	_, err = svc.AcquireProjectLock(context.Background(), "demo")
	if err == nil {
		t.Fatal("expected lock conflict error")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed errs.Error, got: %T", err)
	}
	if typed.Code != errs.CodeWorkbenchLocked {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchLocked, typed.Code)
	}
}

func TestWorkbenchResolveComposeSourceSuccess(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectDir, "compose.yml"), []byte("services:\n  app:\n    image: busybox\n"), 0o644); err != nil {
		t.Fatalf("write compose.yml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n  app:\n    image: nginx\n"), 0o644); err != nil {
		t.Fatalf("write docker-compose.yml: %v", err)
	}

	svc := NewWorkbenchService(templatesDir, nil)
	source, err := svc.ResolveComposeSource(context.Background(), "demo")
	if err != nil {
		t.Fatalf("ResolveComposeSource: %v", err)
	}

	if !strings.HasSuffix(source.ComposePath, "docker-compose.yml") {
		t.Fatalf("expected canonical compose candidate selection, got %q", source.ComposePath)
	}
	if source.Fingerprint == "" || !strings.HasPrefix(source.Fingerprint, "sha256:") {
		t.Fatalf("expected sha256 fingerprint, got %q", source.Fingerprint)
	}
	if source.ProjectName != "demo" {
		t.Fatalf("expected normalized project name demo, got %q", source.ProjectName)
	}
}

func TestWorkbenchResolveComposeSourceNotFound(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	svc := NewWorkbenchService(templatesDir, nil)
	_, err := svc.ResolveComposeSource(context.Background(), "demo")
	if err == nil {
		t.Fatal("expected not-found source error")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed errs.Error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchSourceNotFound {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchSourceNotFound, typed.Code)
	}
}

func TestWorkbenchResolveComposeSourceInvalidSymlink(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	outsidePath := filepath.Join(templatesDir, "outside-compose.yml")
	if err := os.WriteFile(outsidePath, []byte("services:\n  app:\n    image: busybox\n"), 0o644); err != nil {
		t.Fatalf("write outside compose: %v", err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(projectDir, "docker-compose.yml")); err != nil {
		t.Skipf("symlink not supported on this platform: %v", err)
	}

	svc := NewWorkbenchService(templatesDir, nil)
	_, err := svc.ResolveComposeSource(context.Background(), "demo")
	if err == nil {
		t.Fatal("expected invalid source error")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed errs.Error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchSourceInvalid {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchSourceInvalid, typed.Code)
	}
}

func TestWorkbenchAcquireProjectLockRejectsInvalidName(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchService("/tmp", fakeWorkbenchProjectRepo{})
	_, err := svc.AcquireProjectLock(context.Background(), "INVALID NAME")
	if err == nil {
		t.Fatal("expected invalid project name error")
	}

	var typed *errs.Error
	if !errors.As(err, &typed) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeProjectInvalidName {
		t.Fatalf("expected %q, got %q", errs.CodeProjectInvalidName, typed.Code)
	}
}

type fakeWorkbenchProjectRepo struct{}

func (fakeWorkbenchProjectRepo) List(context.Context) ([]models.Project, error) {
	return []models.Project{}, nil
}

func (fakeWorkbenchProjectRepo) Create(context.Context, *models.Project) error { return nil }

func (fakeWorkbenchProjectRepo) GetByName(context.Context, string) (*models.Project, error) {
	return nil, nil
}

func (fakeWorkbenchProjectRepo) Update(context.Context, *models.Project) error { return nil }
