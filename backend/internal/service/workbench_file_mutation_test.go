package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"
)

type selectiveFailProjectFileMutationClient struct {
	stubProjectFileMutationClient
	failWritePath string
	failWriteErr  error
}

func (s *selectiveFailProjectFileMutationClient) ProjectFileWriteAtomic(ctx context.Context, requestID string, payload contract.ProjectFileWriteAtomicPayload) (contract.Result, error) {
	if s.failWriteErr != nil && strings.EqualFold(filepath.Clean(payload.Path), filepath.Clean(s.failWritePath)) {
		return contract.Result{}, s.failWriteErr
	}
	return s.stubProjectFileMutationClient.ProjectFileWriteAtomic(ctx, requestID, payload)
}

func TestWorkbenchApplyComposeUsesBridgeFileMutationClient(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("services:\n  api:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	fileClient := &stubProjectFileMutationClient{}
	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	svc.SetFileMutationClient(fileClient)

	imported, _, err := svc.ImportComposeSnapshot(context.Background(), "demo", "manual")
	if err != nil {
		t.Fatalf("import snapshot: %v", err)
	}
	imported.Services[0].Image = "nginx:1.26"
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", imported); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	expectedRevision := imported.Revision
	_, err = svc.ApplyComposeFromStoredSnapshot(context.Background(), "demo", WorkbenchComposeApplyRequest{
		ExpectedRevision:          &expectedRevision,
		ExpectedSourceFingerprint: imported.SourceFingerprint,
	})
	if err != nil {
		t.Fatalf("apply compose: %v", err)
	}

	if len(fileClient.writeCalls) < 3 {
		t.Fatalf("expected at least 3 file-write bridge calls (backup artifact, index, compose), got %d", len(fileClient.writeCalls))
	}
	foundComposeWrite := false
	for _, call := range fileClient.writeCalls {
		if filepath.Clean(call.Path) == filepath.Clean(composePath) {
			foundComposeWrite = true
			break
		}
	}
	if !foundComposeWrite {
		t.Fatalf("expected compose write intent for %q", composePath)
	}
}

func TestWorkbenchApplyComposeBridgeWriteFailureReturnsTypedError(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("services:\n  api:\n    image: nginx:1.25\n"), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	fileClient := &selectiveFailProjectFileMutationClient{
		failWritePath: composePath,
		failWriteErr:  errors.New("bridge write failed"),
	}
	svc := NewWorkbenchServiceWithStorage(templatesDir, nil, &fakeSettingsRepo{}, "test-session-secret")
	svc.SetFileMutationClient(fileClient)

	imported, _, err := svc.ImportComposeSnapshot(context.Background(), "demo", "manual")
	if err != nil {
		t.Fatalf("import snapshot: %v", err)
	}
	imported.Services[0].Image = "nginx:1.26"
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", imported); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	expectedRevision := imported.Revision
	_, err = svc.ApplyComposeFromStoredSnapshot(context.Background(), "demo", WorkbenchComposeApplyRequest{
		ExpectedRevision:          &expectedRevision,
		ExpectedSourceFingerprint: imported.SourceFingerprint,
	})
	if err == nil {
		t.Fatal("expected apply failure")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchSourceInvalid {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchSourceInvalid, typed.Code)
	}
	if typed.Message != "failed to replace compose source" {
		t.Fatalf("expected message %q, got %q", "failed to replace compose source", typed.Message)
	}
}
