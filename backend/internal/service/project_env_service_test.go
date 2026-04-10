package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"
)

type stubProjectFileMutationClient struct {
	readCalls         []contract.ProjectFileReadPayload
	writeCalls        []contract.ProjectFileWriteAtomicPayload
	copyCalls         []contract.ProjectFileCopyPayload
	removeCalls       []contract.ProjectFileRemovePayload
	readContentByPath map[string]string
	readErr           error
	writeErr          error
	copyErr           error
	removeErr         error
}

func (s *stubProjectFileMutationClient) ProjectFileRead(_ context.Context, _ string, payload contract.ProjectFileReadPayload) (contract.Result, error) {
	s.readCalls = append(s.readCalls, payload)
	if s.readErr != nil {
		return contract.Result{}, s.readErr
	}
	if s.readContentByPath != nil {
		if content, ok := s.readContentByPath[payload.Path]; ok {
			return contract.Result{
				Status: contract.StatusSucceeded,
				Data: map[string]any{
					"path":       payload.Path,
					"content":    content,
					"size_bytes": len(content),
				},
			}, nil
		}
	}
	raw, err := os.ReadFile(payload.Path)
	if err != nil {
		return contract.Result{}, err
	}
	return contract.Result{
		Status: contract.StatusSucceeded,
		Data: map[string]any{
			"path":       payload.Path,
			"content":    string(raw),
			"size_bytes": len(raw),
		},
	}, nil
}

func (s *stubProjectFileMutationClient) ProjectFileWriteAtomic(_ context.Context, _ string, payload contract.ProjectFileWriteAtomicPayload) (contract.Result, error) {
	s.writeCalls = append(s.writeCalls, payload)
	if s.writeErr != nil {
		return contract.Result{}, s.writeErr
	}
	if payload.CreateParents {
		if err := os.MkdirAll(filepath.Dir(payload.Path), 0o755); err != nil {
			return contract.Result{}, err
		}
	}
	mode := os.FileMode(payload.Mode)
	if mode == 0 {
		mode = 0o600
	}
	if err := os.WriteFile(payload.Path, []byte(payload.Content), mode); err != nil {
		return contract.Result{}, err
	}
	return contract.Result{Status: contract.StatusSucceeded}, nil
}

func (s *stubProjectFileMutationClient) ProjectFileCopy(_ context.Context, _ string, payload contract.ProjectFileCopyPayload) (contract.Result, error) {
	s.copyCalls = append(s.copyCalls, payload)
	if s.copyErr != nil {
		return contract.Result{}, s.copyErr
	}
	if payload.CreateParents {
		if err := os.MkdirAll(filepath.Dir(payload.DestinationPath), 0o755); err != nil {
			return contract.Result{}, err
		}
	}
	raw, err := os.ReadFile(payload.SourcePath)
	if err != nil {
		return contract.Result{}, err
	}
	mode := os.FileMode(payload.Mode)
	if mode == 0 {
		mode = 0o600
	}
	if err := os.WriteFile(payload.DestinationPath, raw, mode); err != nil {
		return contract.Result{}, err
	}
	return contract.Result{Status: contract.StatusSucceeded}, nil
}

func (s *stubProjectFileMutationClient) ProjectFileRemove(_ context.Context, _ string, payload contract.ProjectFileRemovePayload) (contract.Result, error) {
	s.removeCalls = append(s.removeCalls, payload)
	if s.removeErr != nil {
		return contract.Result{}, s.removeErr
	}
	err := os.Remove(payload.Path)
	if err != nil {
		if payload.IgnoreNotExist && errors.Is(err, os.ErrNotExist) {
			return contract.Result{Status: contract.StatusSucceeded}, nil
		}
		return contract.Result{}, err
	}
	return contract.Result{Status: contract.StatusSucceeded}, nil
}

func TestProjectEnvServiceSaveUsesBridgeFileMutations(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	envPath := filepath.Join(projectDir, ".env")
	if err := os.WriteFile(envPath, []byte("A=1\n"), 0o600); err != nil {
		t.Fatalf("seed env file: %v", err)
	}

	stub := &stubProjectFileMutationClient{}
	svc := NewProjectEnvService(templatesDir, nil)
	svc.SetFileMutationClient(stub)

	result, err := svc.Save(context.Background(), "demo", "A=2\nB=3\n", true)
	if err != nil {
		t.Fatalf("save env: %v", err)
	}
	if result.Path != envPath {
		t.Fatalf("expected path %q, got %q", envPath, result.Path)
	}
	if result.SizeBytes != int64(len("A=2\nB=3\n")) {
		t.Fatalf("expected size %d, got %d", len("A=2\nB=3\n"), result.SizeBytes)
	}
	if result.UpdatedAt.Before(time.Now().UTC().Add(-time.Minute)) {
		t.Fatalf("unexpected updatedAt timestamp: %s", result.UpdatedAt)
	}
	if result.BackupPath == "" {
		t.Fatal("expected backup path")
	}
	if len(stub.copyCalls) != 1 {
		t.Fatalf("expected one copy call, got %d", len(stub.copyCalls))
	}
	if len(stub.writeCalls) != 1 {
		t.Fatalf("expected one write call, got %d", len(stub.writeCalls))
	}

	raw, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if string(raw) != "A=2\nB=3\n" {
		t.Fatalf("expected updated env content, got %q", string(raw))
	}

	backupRaw, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatalf("read backup env: %v", err)
	}
	if string(backupRaw) != "A=1\n" {
		t.Fatalf("expected backup content %q, got %q", "A=1\n", string(backupRaw))
	}
}

func TestProjectEnvServiceSaveBridgeFailureReturnsTypedError(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	envPath := filepath.Join(projectDir, ".env")
	if err := os.WriteFile(envPath, []byte("A=1\n"), 0o600); err != nil {
		t.Fatalf("seed env file: %v", err)
	}

	svc := NewProjectEnvService(templatesDir, nil)
	svc.SetFileMutationClient(&stubProjectFileMutationClient{writeErr: errors.New("bridge unavailable")})

	_, err := svc.Save(context.Background(), "demo", "A=2\n", false)
	if err == nil {
		t.Fatal("expected bridge write failure")
	}

	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeProjectEnvWriteFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeProjectEnvWriteFailed, typed.Code)
	}
	if typed.Message != "failed to replace .env file" {
		t.Fatalf("expected message %q, got %q", "failed to replace .env file", typed.Message)
	}
}
