package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func TestWorkbenchParseComposeCoreFromProject(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	compose := `
name: demo
x-meta:
  owner: "${OWNER}"
services:
  db:
    build:
      context: "./db"
    env_file:
      - ".env"
      - "${DB_ENV_FILE}"
    ports:
      - "5432"
  api:
    image: "ghcr.io/demo/api:${API_TAG:-latest}"
    restart: unless-stopped
    depends_on:
      redis:
        condition: service_started
      db:
        condition: service_healthy
    ports:
      - "127.0.0.1:8080:80/tcp"
      - target: 8443
        published: "${API_TLS_PORT:-8443}"
        protocol: tcp
        host_ip: 0.0.0.0
    environment:
      APP_ENV: "${APP_ENV:-dev}"
      API_URL: "http://${API_HOST}:8080"
    command: ["serve"]
`
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchService(templatesDir, nil)
	parsed, err := svc.ParseComposeCore(context.Background(), "demo")
	if err != nil {
		t.Fatalf("ParseComposeCore: %v", err)
	}

	if parsed.SourceFingerprint == "" || !strings.HasPrefix(parsed.SourceFingerprint, "sha256:") {
		t.Fatalf("expected fingerprint, got %q", parsed.SourceFingerprint)
	}
	if parsed.ProjectName != "demo" {
		t.Fatalf("expected project name demo, got %q", parsed.ProjectName)
	}
	if !strings.HasSuffix(parsed.ComposePath, "docker-compose.yml") {
		t.Fatalf("expected compose path suffix docker-compose.yml, got %q", parsed.ComposePath)
	}

	if got, want := len(parsed.Services), 2; got != want {
		t.Fatalf("expected %d services, got %d", want, got)
	}
	if parsed.Services[0].ServiceName != "api" || parsed.Services[1].ServiceName != "db" {
		t.Fatalf("expected deterministic service ordering [api db], got [%s %s]", parsed.Services[0].ServiceName, parsed.Services[1].ServiceName)
	}
	if parsed.Services[0].Image == "" {
		t.Fatal("expected api image to be parsed")
	}
	if parsed.Services[1].BuildSource != "./db" {
		t.Fatalf("expected db build source ./db, got %q", parsed.Services[1].BuildSource)
	}

	if got, want := len(parsed.Dependencies), 2; got != want {
		t.Fatalf("expected %d dependency edges, got %d", want, got)
	}
	if parsed.Dependencies[0] != (WorkbenchComposeDependency{ServiceName: "api", DependsOn: "db"}) {
		t.Fatalf("unexpected first dependency edge: %#v", parsed.Dependencies[0])
	}
	if parsed.Dependencies[1] != (WorkbenchComposeDependency{ServiceName: "api", DependsOn: "redis"}) {
		t.Fatalf("unexpected second dependency edge: %#v", parsed.Dependencies[1])
	}

	if got, want := len(parsed.Ports), 3; got != want {
		t.Fatalf("expected %d ports, got %d", want, got)
	}
	if parsed.Ports[0].ServiceName != "api" || parsed.Ports[0].ContainerPort != 80 {
		t.Fatalf("unexpected first parsed port: %#v", parsed.Ports[0])
	}
	if parsed.Ports[0].HostPort == nil || *parsed.Ports[0].HostPort != 8080 {
		t.Fatalf("expected first parsed port hostPort=8080, got %#v", parsed.Ports[0].HostPort)
	}
	if parsed.Ports[1].ServiceName != "api" || parsed.Ports[1].ContainerPort != 8443 {
		t.Fatalf("unexpected second parsed port: %#v", parsed.Ports[1])
	}
	if parsed.Ports[1].HostPortRaw != "${API_TLS_PORT:-8443}" {
		t.Fatalf("expected env-backed host port raw value, got %q", parsed.Ports[1].HostPortRaw)
	}
	if parsed.Ports[2].ServiceName != "db" || parsed.Ports[2].ContainerPort != 5432 {
		t.Fatalf("unexpected third parsed port: %#v", parsed.Ports[2])
	}

	variables := make(map[string]bool, len(parsed.EnvRefs))
	for _, ref := range parsed.EnvRefs {
		variables[ref.Variable] = true
	}
	for _, variable := range []string{"API_TAG", "API_TLS_PORT", "APP_ENV", "API_HOST", "DB_ENV_FILE"} {
		if !variables[variable] {
			t.Fatalf("expected env ref variable %q to be extracted", variable)
		}
	}

	if len(parsed.Warnings) == 0 {
		t.Fatal("expected parser warnings for unsupported fragments")
	}
	if parsed.Warnings[0].Code == "" || parsed.Warnings[0].Path == "" {
		t.Fatalf("expected warning code/path fields, got %#v", parsed.Warnings[0])
	}
}

func TestParseWorkbenchComposeCoreDeterministicOrdering(t *testing.T) {
	t.Parallel()

	source := `
services:
  worker:
    ports:
      - "127.0.0.1:9100:9000/udp"
      - "9001"
  api:
    depends_on:
      - db
      - redis
    ports:
      - "8080:80"
      - target: 443
        published: "8443"
  db:
    ports:
      - "5432"
`

	first, err := ParseWorkbenchComposeCore(source)
	if err != nil {
		t.Fatalf("ParseWorkbenchComposeCore first parse: %v", err)
	}
	second, err := ParseWorkbenchComposeCore(source)
	if err != nil {
		t.Fatalf("ParseWorkbenchComposeCore second parse: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected deterministic parser output for same input\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestParseWorkbenchComposeCoreAcceptsUnbracedEnvInterpolation(t *testing.T) {
	t.Parallel()

	source := `
services:
  app:
    image: "ghcr.io/demo/app:$IMAGE_TAG"
    ports:
      - "$HOST_PORT:80"
      - target: 443
        published: "$TLS_PORT"
`

	parsed, err := ParseWorkbenchComposeCore(source)
	if err != nil {
		t.Fatalf("ParseWorkbenchComposeCore: %v", err)
	}

	if got, want := len(parsed.Ports), 2; got != want {
		t.Fatalf("expected %d parsed ports, got %d", want, got)
	}
	if parsed.Ports[0].HostPortRaw != "$HOST_PORT" {
		t.Fatalf("expected short-form host port raw $HOST_PORT, got %q", parsed.Ports[0].HostPortRaw)
	}
	if parsed.Ports[1].HostPortRaw != "$TLS_PORT" {
		t.Fatalf("expected long-form published port raw $TLS_PORT, got %q", parsed.Ports[1].HostPortRaw)
	}

	var expressions []string
	variables := make(map[string]bool, len(parsed.EnvRefs))
	for _, ref := range parsed.EnvRefs {
		expressions = append(expressions, ref.Expression)
		variables[ref.Variable] = true
	}
	for _, expected := range []string{"$IMAGE_TAG", "$HOST_PORT", "$TLS_PORT"} {
		found := false
		for _, expression := range expressions {
			if expression == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected env expression %q in parsed env refs", expected)
		}
	}
	for _, expected := range []string{"IMAGE_TAG", "HOST_PORT", "TLS_PORT"} {
		if !variables[expected] {
			t.Fatalf("expected env variable %q in parsed env refs", expected)
		}
	}

	for _, warning := range parsed.Warnings {
		if warning.Code == workbenchWarningInvalidPort {
			t.Fatalf("did not expect invalid port warning for valid $VAR interpolation: %#v", warning)
		}
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
