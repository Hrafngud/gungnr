package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-notes/internal/errs"
)

func TestWorkbenchApplyComposeFromStoredSnapshotCreatesBackupsAndPrunesRetention(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("services:\n  api:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	svc.backupMaxCount = 2
	svc.backupMaxAge = 72 * time.Hour

	imported, _, err := svc.ImportComposeSnapshot(ctx, "demo", "manual")
	if err != nil {
		t.Fatalf("import snapshot: %v", err)
	}

	applyTimes := []time.Time{
		time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 3, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 4, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 5, 10, 0, 0, 0, time.UTC),
	}
	images := []string{"nginx:1.26", "nginx:1.27", "nginx:1.28", "nginx:1.29"}

	var lastResult WorkbenchComposeApplyResult
	for idx, image := range images {
		snapshot := loadWorkbenchSnapshotForTest(t, svc, ctx, "demo")
		snapshot.Services[0].Image = image
		if err := svc.saveWorkbenchSnapshot(ctx, "demo", snapshot); err != nil {
			t.Fatalf("save mutated snapshot %d: %v", idx, err)
		}

		now := applyTimes[idx]
		svc.nowFn = func() time.Time { return now }

		expectedRevision := snapshot.Revision
		result, applyErr := svc.ApplyComposeFromStoredSnapshot(ctx, "demo", WorkbenchComposeApplyRequest{
			ExpectedRevision:          &expectedRevision,
			ExpectedSourceFingerprint: snapshot.SourceFingerprint,
		})
		if applyErr != nil {
			t.Fatalf("apply %d: %v", idx, applyErr)
		}
		lastResult = result
	}

	if lastResult.Backup.BackupID != "wbk-000004" {
		t.Fatalf("expected last backup id wbk-000004, got %q", lastResult.Backup.BackupID)
	}
	if lastResult.Retention.RetainedCount != 2 {
		t.Fatalf("expected retained count 2, got %d", lastResult.Retention.RetainedCount)
	}
	if lastResult.Retention.PrunedCount != 1 {
		t.Fatalf("expected final-step pruned count 1, got %d", lastResult.Retention.PrunedCount)
	}

	backups, err := loadWorkbenchComposeBackupIndex(projectDir)
	if err != nil {
		t.Fatalf("load backups: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected 2 retained backups, got %d", len(backups))
	}
	if backups[0].BackupID != "wbk-000003" || backups[1].BackupID != "wbk-000004" {
		t.Fatalf("expected retained backups [wbk-000003 wbk-000004], got %#v", backups)
	}

	for _, backupID := range []string{"wbk-000003", "wbk-000004"} {
		artifactPath, pathErr := resolveWorkbenchComposeBackupArtifactPath(projectDir, workbenchComposeBackupArtifactRelativePath(backupID))
		if pathErr != nil {
			t.Fatalf("resolve retained artifact %s: %v", backupID, pathErr)
		}
		if _, statErr := os.Stat(artifactPath); statErr != nil {
			t.Fatalf("expected retained artifact %s: %v", backupID, statErr)
		}
	}
	for _, backupID := range []string{"wbk-000001", "wbk-000002"} {
		artifactPath, pathErr := resolveWorkbenchComposeBackupArtifactPath(projectDir, workbenchComposeBackupArtifactRelativePath(backupID))
		if pathErr != nil {
			t.Fatalf("resolve pruned artifact %s: %v", backupID, pathErr)
		}
		if _, statErr := os.Stat(artifactPath); !os.IsNotExist(statErr) {
			t.Fatalf("expected pruned artifact %s to be removed, stat err=%v", backupID, statErr)
		}
	}

	stored := loadWorkbenchSnapshotForTest(t, svc, ctx, "demo")
	if stored.SourceFingerprint != lastResult.Metadata.SourceFingerprint {
		t.Fatalf("expected stored fingerprint %q, got %q", lastResult.Metadata.SourceFingerprint, stored.SourceFingerprint)
	}
	if stored.Revision != imported.Revision {
		t.Fatalf("expected snapshot revision to remain %d, got %d", imported.Revision, stored.Revision)
	}
}

func TestWorkbenchRestoreComposeFromBackupSuccessAndRequiresImport(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("services:\n  api:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	imported, _, err := svc.ImportComposeSnapshot(ctx, "demo", "manual")
	if err != nil {
		t.Fatalf("import snapshot: %v", err)
	}

	for idx, image := range []string{"nginx:1.26", "nginx:1.27"} {
		snapshot := loadWorkbenchSnapshotForTest(t, svc, ctx, "demo")
		snapshot.Services[0].Image = image
		if err := svc.saveWorkbenchSnapshot(ctx, "demo", snapshot); err != nil {
			t.Fatalf("save mutated snapshot %d: %v", idx, err)
		}

		expectedRevision := snapshot.Revision
		if _, err := svc.ApplyComposeFromStoredSnapshot(ctx, "demo", WorkbenchComposeApplyRequest{
			ExpectedRevision:          &expectedRevision,
			ExpectedSourceFingerprint: snapshot.SourceFingerprint,
		}); err != nil {
			t.Fatalf("apply %d: %v", idx, err)
		}
	}

	restore, err := svc.RestoreComposeFromBackup(ctx, "demo", WorkbenchComposeRestoreRequest{
		BackupID: "wbk-000001",
	})
	if err != nil {
		t.Fatalf("restore from backup: %v", err)
	}
	if restore.Backup.BackupID != "wbk-000001" {
		t.Fatalf("expected backup id wbk-000001, got %q", restore.Backup.BackupID)
	}
	if !restore.Metadata.RequiresImport {
		t.Fatal("expected restore to require re-import for drift safety")
	}
	if restore.Metadata.Revision != imported.Revision {
		t.Fatalf("expected revision %d, got %d", imported.Revision, restore.Metadata.Revision)
	}

	currentSource, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read restored compose: %v", err)
	}
	if !strings.Contains(string(currentSource), "nginx:1.25") {
		t.Fatalf("expected restored compose image nginx:1.25, got:\n%s", string(currentSource))
	}

	stored := loadWorkbenchSnapshotForTest(t, svc, ctx, "demo")
	expectedRevision := stored.Revision
	_, applyErr := svc.ApplyComposeFromStoredSnapshot(ctx, "demo", WorkbenchComposeApplyRequest{
		ExpectedRevision:          &expectedRevision,
		ExpectedSourceFingerprint: stored.SourceFingerprint,
	})
	if applyErr == nil {
		t.Fatal("expected apply after restore to be blocked by drift")
	}

	typed, ok := errs.From(applyErr)
	if !ok {
		t.Fatalf("expected typed drift error, got %T", applyErr)
	}
	if typed.Code != errs.CodeWorkbenchDriftDetected {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchDriftDetected, typed.Code)
	}
}

func TestWorkbenchRestoreComposeFromBackupMissingTarget(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("services:\n  api:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	if _, _, err := svc.ImportComposeSnapshot(ctx, "demo", "manual"); err != nil {
		t.Fatalf("import snapshot: %v", err)
	}

	_, err := svc.RestoreComposeFromBackup(ctx, "demo", WorkbenchComposeRestoreRequest{
		BackupID: "wbk-999999",
	})
	if err == nil {
		t.Fatal("expected missing backup error")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchBackupNotFound {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchBackupNotFound, typed.Code)
	}
}

func TestWorkbenchRestoreComposeFromBackupRejectsCorruptArtifact(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("services:\n  api:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	if _, _, err := svc.ImportComposeSnapshot(ctx, "demo", "manual"); err != nil {
		t.Fatalf("import snapshot: %v", err)
	}

	snapshot := loadWorkbenchSnapshotForTest(t, svc, ctx, "demo")
	snapshot.Services[0].Image = "nginx:1.26"
	if err := svc.saveWorkbenchSnapshot(ctx, "demo", snapshot); err != nil {
		t.Fatalf("save mutated snapshot: %v", err)
	}

	expectedRevision := snapshot.Revision
	if _, err := svc.ApplyComposeFromStoredSnapshot(ctx, "demo", WorkbenchComposeApplyRequest{
		ExpectedRevision:          &expectedRevision,
		ExpectedSourceFingerprint: snapshot.SourceFingerprint,
	}); err != nil {
		t.Fatalf("apply compose: %v", err)
	}

	artifactPath, err := resolveWorkbenchComposeBackupArtifactPath(projectDir, workbenchComposeBackupArtifactRelativePath("wbk-000001"))
	if err != nil {
		t.Fatalf("resolve artifact path: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("services:\n  api:\n    image: corrupt\n"), 0o600); err != nil {
		t.Fatalf("corrupt artifact: %v", err)
	}

	_, err = svc.RestoreComposeFromBackup(ctx, "demo", WorkbenchComposeRestoreRequest{
		BackupID: "wbk-000001",
	})
	if err == nil {
		t.Fatal("expected corrupt backup error")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchBackupIntegrity {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchBackupIntegrity, typed.Code)
	}
}

func loadWorkbenchSnapshotForTest(t *testing.T, svc *WorkbenchService, ctx context.Context, project string) WorkbenchStackSnapshot {
	t.Helper()

	snapshot, exists, err := svc.loadStoredWorkbenchSnapshot(ctx, project)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if !exists {
		t.Fatalf("expected snapshot for project %q", project)
	}
	return snapshot
}
