package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkbenchOptionalServiceCatalogEmptySnapshot(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n  app:\n    image: nginx:stable\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")

	catalog, err := svc.GetOptionalServiceCatalog(context.Background(), "demo")
	if err != nil {
		t.Fatalf("GetOptionalServiceCatalog: %v", err)
	}

	if catalog.ProjectName != "demo" {
		t.Fatalf("expected project demo, got %q", catalog.ProjectName)
	}
	if catalog.SnapshotImported {
		t.Fatal("expected empty snapshot to report snapshotImported=false")
	}
	if catalog.SnapshotRevision != 0 {
		t.Fatalf("expected revision 0, got %d", catalog.SnapshotRevision)
	}
	if got, want := len(catalog.Entries), 4; got != want {
		t.Fatalf("expected %d catalog entries, got %d", want, got)
	}
	for _, entry := range catalog.Entries {
		if entry.Availability.Status != workbenchOptionalServiceStatusAvailable {
			t.Fatalf("expected entry %q to be available, got %q", entry.Key, entry.Availability.Status)
		}
		if len(entry.Availability.ComposeServices) != 0 {
			t.Fatalf("expected entry %q compose matches to be empty, got %#v", entry.Key, entry.Availability.ComposeServices)
		}
		if len(entry.Availability.ManagedServices) != 0 {
			t.Fatalf("expected entry %q managed services to be empty, got %#v", entry.Key, entry.Availability.ManagedServices)
		}
		if !entry.Transition.MutationReady {
			t.Fatalf("expected entry %q mutationReady=true", entry.Key)
		}
		if !entry.Transition.ComposeGenerationReady {
			t.Fatalf("expected entry %q composeGenerationReady=true", entry.Key)
		}
	}
	if catalog.LegacyModules.Status != workbenchOptionalServiceLegacyModulesStatusEmpty {
		t.Fatalf("expected empty legacy status, got %q", catalog.LegacyModules.Status)
	}
	if len(catalog.LegacyModules.Records) != 0 {
		t.Fatalf("expected no legacy records, got %#v", catalog.LegacyModules.Records)
	}
}

func TestWorkbenchOptionalServiceCatalogTracksComposeAndLegacyState(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	compose := `
services:
  cache:
    image: redis:7-alpine
  gateway:
    image: docker.io/library/nginx:stable-alpine
  metrics:
    image: prom/prometheus:v2.54.1
`
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	if _, changed, err := svc.ImportComposeSnapshot(context.Background(), "demo", workbenchImportReasonManual); err != nil {
		t.Fatalf("ImportComposeSnapshot: %v", err)
	} else if !changed {
		t.Fatal("expected initial import to change snapshot")
	}
	if _, _, err := svc.MutateStoredSnapshotModule(context.Background(), "demo", WorkbenchModuleMutationRequest{
		Selector: WorkbenchModuleSelector{
			ServiceName: "gateway",
			ModuleType:  "redis",
		},
		Action: workbenchModuleMutationActionAdd,
	}); err != nil {
		t.Fatalf("MutateStoredSnapshotModule: %v", err)
	}

	catalog, err := svc.GetOptionalServiceCatalog(context.Background(), "demo")
	if err != nil {
		t.Fatalf("GetOptionalServiceCatalog: %v", err)
	}

	if !catalog.SnapshotImported {
		t.Fatal("expected imported snapshot to report snapshotImported=true")
	}
	if catalog.SnapshotRevision != 2 {
		t.Fatalf("expected revision 2 after legacy module mutation, got %d", catalog.SnapshotRevision)
	}
	if catalog.LegacyModules.Status != workbenchOptionalServiceLegacyModulesStatusPresent {
		t.Fatalf("expected legacy modules status present, got %q", catalog.LegacyModules.Status)
	}
	if got, want := len(catalog.LegacyModules.Records), 1; got != want {
		t.Fatalf("expected %d legacy record, got %d", want, got)
	}
	if got, want := catalog.LegacyModules.Records[0].ServiceName, "gateway"; got != want {
		t.Fatalf("expected legacy service %q, got %q", want, got)
	}

	entries := make(map[string]WorkbenchOptionalServiceCatalogEntry, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		entries[entry.Key] = entry
	}

	redisEntry, ok := entries["redis"]
	if !ok {
		t.Fatal("expected redis catalog entry")
	}
	if redisEntry.Availability.Status != workbenchOptionalServiceStatusComposePresentWithLegacy {
		t.Fatalf("expected redis status %q, got %q", workbenchOptionalServiceStatusComposePresentWithLegacy, redisEntry.Availability.Status)
	}
	if got, want := len(redisEntry.Availability.ComposeServices), 1; got != want {
		t.Fatalf("expected %d redis compose match, got %d", want, got)
	}
	if got, want := redisEntry.Availability.ComposeServices[0].ServiceName, "cache"; got != want {
		t.Fatalf("expected redis compose service %q, got %q", want, got)
	}
	if got, want := redisEntry.Availability.ComposeServices[0].MatchReason, workbenchOptionalServiceMatchReasonImageRepository; got != want {
		t.Fatalf("expected redis match reason %q, got %q", want, got)
	}
	if got, want := len(redisEntry.Availability.LegacyModules), 1; got != want {
		t.Fatalf("expected %d redis legacy module, got %d", want, got)
	}
	if redisEntry.Transition.CurrentState != workbenchOptionalServiceCurrentStateLegacyModules {
		t.Fatalf("expected redis current state %q, got %q", workbenchOptionalServiceCurrentStateLegacyModules, redisEntry.Transition.CurrentState)
	}
	if redisEntry.Transition.LegacyMutationPath == "" {
		t.Fatal("expected redis legacy mutation path to be present")
	}
	if !redisEntry.Transition.MutationReady {
		t.Fatal("expected redis mutationReady=true")
	}
	if !redisEntry.Transition.ComposeGenerationReady {
		t.Fatal("expected redis composeGenerationReady=true")
	}

	nginxEntry, ok := entries["nginx"]
	if !ok {
		t.Fatal("expected nginx catalog entry")
	}
	if nginxEntry.Availability.Status != workbenchOptionalServiceStatusComposePresent {
		t.Fatalf("expected nginx status %q, got %q", workbenchOptionalServiceStatusComposePresent, nginxEntry.Availability.Status)
	}
	if got, want := nginxEntry.Availability.ComposeServices[0].ServiceName, "gateway"; got != want {
		t.Fatalf("expected nginx compose service %q, got %q", want, got)
	}

	prometheusEntry, ok := entries["prometheus"]
	if !ok {
		t.Fatal("expected prometheus catalog entry")
	}
	if prometheusEntry.Availability.Status != workbenchOptionalServiceStatusComposePresent {
		t.Fatalf("expected prometheus status %q, got %q", workbenchOptionalServiceStatusComposePresent, prometheusEntry.Availability.Status)
	}

	minioEntry, ok := entries["minio"]
	if !ok {
		t.Fatal("expected minio catalog entry")
	}
	if minioEntry.Availability.Status != workbenchOptionalServiceStatusAvailable {
		t.Fatalf("expected minio status %q, got %q", workbenchOptionalServiceStatusAvailable, minioEntry.Availability.Status)
	}
}

func TestWorkbenchOptionalServiceCatalogTracksManagedServicesSeparately(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n  api:\n    image: ghcr.io/example/api:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    5,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "ghcr.io/example/api:latest"},
		},
		ManagedServices: []WorkbenchManagedService{
			{EntryKey: "minio", ServiceName: "minio"},
		},
		Modules: []WorkbenchStackModule{
			{ModuleType: "redis", ServiceName: "api"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	catalog, err := svc.GetOptionalServiceCatalog(context.Background(), "demo")
	if err != nil {
		t.Fatalf("GetOptionalServiceCatalog: %v", err)
	}

	entries := make(map[string]WorkbenchOptionalServiceCatalogEntry, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		entries[entry.Key] = entry
	}

	minioEntry, ok := entries["minio"]
	if !ok {
		t.Fatal("expected minio catalog entry")
	}
	if minioEntry.Availability.Status != workbenchOptionalServiceStatusCatalogManaged {
		t.Fatalf("expected minio status %q, got %q", workbenchOptionalServiceStatusCatalogManaged, minioEntry.Availability.Status)
	}
	if got, want := len(minioEntry.Availability.ManagedServices), 1; got != want {
		t.Fatalf("expected %d managed minio service, got %d", want, got)
	}
	if got, want := minioEntry.Availability.ManagedServices[0].ServiceName, "minio"; got != want {
		t.Fatalf("expected managed minio service name %q, got %q", want, got)
	}
	if minioEntry.Transition.CurrentState != workbenchOptionalServiceCurrentStateCatalogManaged {
		t.Fatalf("expected minio current state %q, got %q", workbenchOptionalServiceCurrentStateCatalogManaged, minioEntry.Transition.CurrentState)
	}

	redisEntry, ok := entries["redis"]
	if !ok {
		t.Fatal("expected redis catalog entry")
	}
	if got, want := len(redisEntry.Availability.ManagedServices), 0; got != want {
		t.Fatalf("expected %d redis managed services, got %d", want, got)
	}
	if redisEntry.Availability.Status != workbenchOptionalServiceStatusLegacyModuleOnly {
		t.Fatalf("expected redis legacy-only status %q, got %q", workbenchOptionalServiceStatusLegacyModuleOnly, redisEntry.Availability.Status)
	}
}
