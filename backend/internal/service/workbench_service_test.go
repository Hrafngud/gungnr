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

	"go-notes/internal/config"
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
networks:
  edge:
    driver: bridge
  backplane: {}
volumes:
  cache: {}
  pgdata:
    external: false
services:
  db:
    build:
      context: "./db"
    deploy:
      resources:
        limits:
          cpus: "0.50"
          memory: "512M"
        reservations:
          cpus: "0.25"
          memory: "256M"
    env_file:
      - ".env"
      - "${DB_ENV_FILE}"
    networks:
      - backplane
    ports:
      - "5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - type: volume
        source: cache
        target: /cache
      - ./local:/tmp/local
  api:
    image: "ghcr.io/demo/api:${API_TAG:-latest}"
    restart: unless-stopped
    deploy:
      resources:
        limits:
          cpus: "${API_CPU_LIMIT:-1.00}"
        reservations:
          memory: "256M"
      placement:
        constraints:
          - node.role==manager
    depends_on:
      redis:
        condition: service_started
      db:
        condition: service_healthy
    networks:
      edge:
        aliases:
          - api-internal
      backplane: {}
    ports:
      - "127.0.0.1:8080:80/tcp"
      - target: 8443
        published: "${API_TLS_PORT:-8443}"
        protocol: tcp
        host_ip: 0.0.0.0
    environment:
      APP_ENV: "${APP_ENV:-dev}"
      API_URL: "http://${API_HOST}:8080"
    volumes:
      - cache:/srv/cache:rw
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

	if got, want := len(parsed.Resources), 2; got != want {
		t.Fatalf("expected %d resources, got %d", want, got)
	}
	if parsed.Resources[0] != (WorkbenchComposeResource{
		ServiceName:       "api",
		LimitCPUs:         "${API_CPU_LIMIT:-1.00}",
		ReservationMemory: "256M",
	}) {
		t.Fatalf("unexpected api resources: %#v", parsed.Resources[0])
	}
	if parsed.Resources[1] != (WorkbenchComposeResource{
		ServiceName:       "db",
		LimitCPUs:         "0.50",
		LimitMemory:       "512M",
		ReservationCPUs:   "0.25",
		ReservationMemory: "256M",
	}) {
		t.Fatalf("unexpected db resources: %#v", parsed.Resources[1])
	}

	if got, want := len(parsed.NetworkRefs), 3; got != want {
		t.Fatalf("expected %d network refs, got %d", want, got)
	}
	if parsed.NetworkRefs[0] != (WorkbenchComposeNetworkRef{ServiceName: "api", NetworkName: "backplane"}) {
		t.Fatalf("unexpected first network ref: %#v", parsed.NetworkRefs[0])
	}
	if parsed.NetworkRefs[1] != (WorkbenchComposeNetworkRef{ServiceName: "api", NetworkName: "edge"}) {
		t.Fatalf("unexpected second network ref: %#v", parsed.NetworkRefs[1])
	}
	if parsed.NetworkRefs[2] != (WorkbenchComposeNetworkRef{ServiceName: "db", NetworkName: "backplane"}) {
		t.Fatalf("unexpected third network ref: %#v", parsed.NetworkRefs[2])
	}

	if got, want := len(parsed.VolumeRefs), 3; got != want {
		t.Fatalf("expected %d volume refs, got %d", want, got)
	}
	if parsed.VolumeRefs[0] != (WorkbenchComposeVolumeRef{ServiceName: "api", VolumeName: "cache"}) {
		t.Fatalf("unexpected first volume ref: %#v", parsed.VolumeRefs[0])
	}
	if parsed.VolumeRefs[1] != (WorkbenchComposeVolumeRef{ServiceName: "db", VolumeName: "cache"}) {
		t.Fatalf("unexpected second volume ref: %#v", parsed.VolumeRefs[1])
	}
	if parsed.VolumeRefs[2] != (WorkbenchComposeVolumeRef{ServiceName: "db", VolumeName: "pgdata"}) {
		t.Fatalf("unexpected third volume ref: %#v", parsed.VolumeRefs[2])
	}

	variables := make(map[string]bool, len(parsed.EnvRefs))
	for _, ref := range parsed.EnvRefs {
		variables[ref.Variable] = true
	}
	for _, variable := range []string{"API_TAG", "API_TLS_PORT", "APP_ENV", "API_HOST", "DB_ENV_FILE", "API_CPU_LIMIT"} {
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
	if !containsWorkbenchWarningCode(parsed.Warnings, workbenchWarningPassThrough) {
		t.Fatalf("expected pass-through warning code %q in warnings: %#v", workbenchWarningPassThrough, parsed.Warnings)
	}
}

func TestParseWorkbenchComposeCoreDeterministicOrdering(t *testing.T) {
	t.Parallel()

	source := `
networks:
  edge:
    driver: bridge
  data: {}
volumes:
  cache:
    labels:
      managed-by: wb
  pgdata: {}
services:
  worker:
    deploy:
      resources:
        limits:
          cpus: "0.75"
          memory: "384M"
        reservations:
          memory: "128M"
    networks:
      - data
    ports:
      - "127.0.0.1:9100:9000/udp"
      - "9001"
    volumes:
      - pgdata:/var/lib/postgresql/data
  api:
    deploy:
      resources:
        limits:
          cpus: "1.00"
      placement:
        constraints:
          - node.role==manager
    depends_on:
      - db
      - redis
    networks:
      edge:
        aliases:
          - api
    ports:
      - "8080:80"
      - target: 443
        published: "8443"
    volumes:
      - cache:/srv/cache
  db:
    networks:
      - data
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

func TestWorkbenchImportComposeSnapshotIdempotentOnUnchangedSource(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	settingsRepo := &fakeSettingsRepo{}
	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, settingsRepo, "test-session-secret")

	first, changed, err := svc.ImportComposeSnapshot(context.Background(), "demo", "manual")
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if !changed {
		t.Fatal("expected first import to report changed=true")
	}
	if first.Revision != 1 {
		t.Fatalf("expected first revision=1, got %d", first.Revision)
	}
	if first.SourceFingerprint == "" {
		t.Fatal("expected fingerprint on first import")
	}

	second, changed, err := svc.ImportComposeSnapshot(context.Background(), "demo", "manual")
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if changed {
		t.Fatal("expected unchanged import to report changed=false")
	}
	if second.Revision != first.Revision {
		t.Fatalf("expected unchanged revision=%d, got %d", first.Revision, second.Revision)
	}
	if second.SourceFingerprint != first.SourceFingerprint {
		t.Fatalf("expected unchanged fingerprint=%q, got %q", first.SourceFingerprint, second.SourceFingerprint)
	}

	if err := os.WriteFile(composePath, []byte("services:\n  app:\n    image: nginx:1.26\n"), 0o644); err != nil {
		t.Fatalf("rewrite compose: %v", err)
	}

	third, changed, err := svc.ImportComposeSnapshot(context.Background(), "demo", "manual")
	if err != nil {
		t.Fatalf("third import after compose change: %v", err)
	}
	if !changed {
		t.Fatal("expected changed import after compose mutation")
	}
	if third.Revision != 2 {
		t.Fatalf("expected revision=2 after source change, got %d", third.Revision)
	}
	if third.SourceFingerprint == first.SourceFingerprint {
		t.Fatalf("expected changed fingerprint, both were %q", third.SourceFingerprint)
	}
}

func TestWorkbenchImportAndNetBirdConfigShareSettingsPayload(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n  app:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	settingsRepo := &fakeSettingsRepo{}
	settingsService := NewSettingsService(config.Config{SessionSecret: "test-session-secret"}, settingsRepo)
	workbenchService := NewWorkbenchServiceWithStorage(templatesDir, nil, settingsRepo, "test-session-secret")

	apiBaseURL := "https://api.netbird.example"
	apiToken := "token-1"
	hostPeerID := "peer-host"
	adminPeerIDs := []string{"peer-admin-1", "peer-admin-2"}
	if _, err := settingsService.UpsertNetBirdModeConfig(context.Background(), NetBirdModeConfigUpdate{
		APIBaseURL:   &apiBaseURL,
		APIToken:     &apiToken,
		HostPeerID:   &hostPeerID,
		AdminPeerIDs: &adminPeerIDs,
	}); err != nil {
		t.Fatalf("seed netbird config: %v", err)
	}

	first, changed, err := workbenchService.ImportComposeSnapshot(context.Background(), "demo", "manual")
	if err != nil {
		t.Fatalf("workbench import: %v", err)
	}
	if !changed {
		t.Fatal("expected first workbench import to report changed=true")
	}

	apiTokenRotated := "token-2"
	if _, err := settingsService.UpsertNetBirdModeConfig(context.Background(), NetBirdModeConfigUpdate{
		APIToken: &apiTokenRotated,
	}); err != nil {
		t.Fatalf("rotate netbird token: %v", err)
	}

	netbirdConfig, err := settingsService.GetNetBirdModeConfig(context.Background())
	if err != nil {
		t.Fatalf("load netbird config: %v", err)
	}
	if !netbirdConfig.APITokenSet {
		t.Fatal("expected netbird token to remain configured")
	}
	if netbirdConfig.APIBaseURL != apiBaseURL {
		t.Fatalf("expected api base url %q, got %q", apiBaseURL, netbirdConfig.APIBaseURL)
	}
	if netbirdConfig.HostPeerID != hostPeerID {
		t.Fatalf("expected host peer id %q, got %q", hostPeerID, netbirdConfig.HostPeerID)
	}

	second, changed, err := workbenchService.ImportComposeSnapshot(context.Background(), "demo", "manual")
	if err != nil {
		t.Fatalf("second workbench import after netbird update: %v", err)
	}
	if changed {
		t.Fatal("expected unchanged workbench import after netbird-only settings update")
	}
	if second.Revision != first.Revision {
		t.Fatalf("expected revision to stay at %d, got %d", first.Revision, second.Revision)
	}
}

func containsWorkbenchWarningCode(warnings []WorkbenchComposeWarning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
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
