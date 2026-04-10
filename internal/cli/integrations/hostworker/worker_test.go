package hostworker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessOnceHandlesProjectFileRead(t *testing.T) {
	t.Parallel()

	queueRoot := t.TempDir()
	runner, err := New(queueRoot, 10*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	baseDir := t.TempDir()
	targetPath := filepath.Join(baseDir, "docker-compose.yml")
	if err := os.WriteFile(targetPath, []byte("services:\n  api:\n    image: nginx:stable\n"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	intent := Intent{
		Version:   versionV1,
		IntentID:  "intent-project-file-read",
		RequestID: "req-project-file-read",
		TaskType:  taskProjectFileRead,
		Payload: map[string]any{
			"base_path": baseDir,
			"path":      targetPath,
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	writeIntentFile(t, runner.queue.IntentPath(intent.IntentID), intent)

	if err := runner.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("ProcessOnce() error = %v", err)
	}

	result := readResultFile(t, runner.queue.ResultPath(intent.IntentID))
	if result.Status != statusSucceeded {
		t.Fatalf("expected status %q, got %q", statusSucceeded, result.Status)
	}
	if got := result.Data["path"]; got != targetPath {
		t.Fatalf("expected path %q, got %#v", targetPath, got)
	}
	if got := result.Data["content"]; got != "services:\n  api:\n    image: nginx:stable\n" {
		t.Fatalf("unexpected content %#v", got)
	}
}

func TestProcessOnceRejectsPathOutsideBase(t *testing.T) {
	t.Parallel()

	queueRoot := t.TempDir()
	runner, err := New(queueRoot, 10*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	baseDir := t.TempDir()
	intent := Intent{
		Version:   versionV1,
		IntentID:  "intent-project-file-read-outside",
		RequestID: "req-project-file-read-outside",
		TaskType:  taskProjectFileRead,
		Payload: map[string]any{
			"base_path": baseDir,
			"path":      filepath.Join(baseDir, "..", "docker-compose.yml"),
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	writeIntentFile(t, runner.queue.IntentPath(intent.IntentID), intent)

	if err := runner.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("ProcessOnce() error = %v", err)
	}

	result := readResultFile(t, runner.queue.ResultPath(intent.IntentID))
	if result.Status != statusFailed {
		t.Fatalf("expected status %q, got %q", statusFailed, result.Status)
	}
	if result.Error == nil || result.Error.Message == "" {
		t.Fatalf("expected failure details, got %#v", result.Error)
	}
}

func TestProcessOnceRefusesSymlinkedFile(t *testing.T) {
	t.Parallel()

	queueRoot := t.TempDir()
	runner, err := New(queueRoot, 10*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	baseDir := t.TempDir()
	outsidePath := filepath.Join(t.TempDir(), "docker-compose.yml")
	if err := os.WriteFile(outsidePath, []byte("services:\n"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	targetPath := filepath.Join(baseDir, "docker-compose.yml")
	if err := os.Symlink(outsidePath, targetPath); err != nil {
		t.Skipf("symlink not supported on this platform: %v", err)
	}

	intent := Intent{
		Version:   versionV1,
		IntentID:  "intent-project-file-read-symlink",
		RequestID: "req-project-file-read-symlink",
		TaskType:  taskProjectFileRead,
		Payload: map[string]any{
			"base_path": baseDir,
			"path":      targetPath,
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	writeIntentFile(t, runner.queue.IntentPath(intent.IntentID), intent)

	if err := runner.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("ProcessOnce() error = %v", err)
	}

	result := readResultFile(t, runner.queue.ResultPath(intent.IntentID))
	if result.Status != statusFailed {
		t.Fatalf("expected status %q, got %q", statusFailed, result.Status)
	}
	if result.Error == nil || result.Error.Message != "refusing to read symlinked file" {
		t.Fatalf("expected symlink refusal, got %#v", result.Error)
	}
}

func TestResolveProjectBasePathRequiresAbsolutePath(t *testing.T) {
	t.Parallel()

	_, err := resolveProjectBasePath("relative/path")
	if err == nil || err.Error() != "base_path must be absolute" {
		t.Fatalf("expected absolute-path error, got %v", err)
	}
}

func TestReadMissingResultReturnsNotExist(t *testing.T) {
	t.Parallel()

	queueRoot := t.TempDir()
	runner, err := New(queueRoot, 10*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = runner.queue.ReadResult(context.Background(), "missing")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func writeIntentFile(t *testing.T, path string, intent Intent) {
	t.Helper()
	raw, err := json.Marshal(intent)
	if err != nil {
		t.Fatalf("marshal intent: %v", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("write intent: %v", err)
	}
}

func readResultFile(t *testing.T, path string) Result {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var result Result
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return result
}
