package hostworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	versionV1           = "v1"
	taskProjectFileRead = "project_file_read"
	statusRunning       = "running"
	statusSucceeded     = "succeeded"
	statusFailed        = "failed"
	defaultPollInterval = 500 * time.Millisecond
	errorCodeExec       = "INFRA-500-EXEC"
)

var safeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type Intent struct {
	Version   string         `json:"version"`
	IntentID  string         `json:"intent_id"`
	RequestID string         `json:"request_id"`
	TaskType  string         `json:"task_type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type Claim struct {
	Version   string    `json:"version"`
	IntentID  string    `json:"intent_id"`
	Owner     string    `json:"owner"`
	ClaimedAt time.Time `json:"claimed_at"`
}

type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type Result struct {
	Version    string         `json:"version"`
	IntentID   string         `json:"intent_id"`
	RequestID  string         `json:"request_id"`
	TaskType   string         `json:"task_type"`
	Status     string         `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
	LogPath    string         `json:"log_path"`
	LogTail    []string       `json:"log_tail,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	Error      *Error         `json:"error,omitempty"`
}

type ProjectFileReadPayload struct {
	BasePath string `json:"base_path"`
	Path     string `json:"path"`
}

type Runner struct {
	queue        *filesystemQueue
	pollInterval time.Duration
	owner        string
	logger       *log.Logger
}

func New(queueRoot string, pollInterval time.Duration, logger *log.Logger) (*Runner, error) {
	queue, err := newFilesystemQueue(queueRoot)
	if err != nil {
		return nil, err
	}
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	if logger == nil {
		logger = log.Default()
	}
	hostname, _ := os.Hostname()
	return &Runner{
		queue:        queue,
		pollInterval: pollInterval,
		owner:        fmt.Sprintf("host-worker:%s:%d", hostname, os.Getpid()),
		logger:       logger,
	}, nil
}

func (r *Runner) Run(ctx context.Context) error {
	if r == nil || r.queue == nil {
		return nil
	}
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if err := r.ProcessOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.logger.Printf("warn: host worker cycle failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Runner) ProcessOnce(ctx context.Context) error {
	ids, err := r.queue.ListIntentIDs(ctx)
	if err != nil {
		return err
	}
	for _, intentID := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}

		intent, err := r.queue.ReadIntent(ctx, intentID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			r.logger.Printf("warn: host worker read intent %s failed: %v", intentID, err)
			continue
		}
		if intent.TaskType != taskProjectFileRead {
			continue
		}

		result, err := r.queue.ReadResult(ctx, intentID)
		if err == nil && result.Terminal() {
			continue
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			r.logger.Printf("warn: host worker read result %s failed: %v", intentID, err)
			continue
		}

		_, claimed, err := r.queue.ClaimIntent(ctx, intentID, r.owner)
		if err != nil {
			r.logger.Printf("warn: host worker claim %s failed: %v", intentID, err)
			continue
		}
		if !claimed {
			continue
		}

		if err := r.handleIntent(ctx, intent); err != nil {
			r.logger.Printf("warn: host worker handle intent %s failed: %v", intentID, err)
		}
	}
	return nil
}

func (r *Runner) handleIntent(ctx context.Context, intent Intent) error {
	startedAt := time.Now().UTC()
	if _, err := r.queue.WriteResult(ctx, Result{
		Version:   versionV1,
		IntentID:  intent.IntentID,
		RequestID: intent.RequestID,
		TaskType:  intent.TaskType,
		Status:    statusRunning,
		CreatedAt: intent.CreatedAt,
		StartedAt: startedAt,
	}); err != nil {
		return fmt.Errorf("write running result for %s: %w", intent.IntentID, err)
	}

	data, outcomeErr := handleProjectFileRead(intent.Payload)
	final := Result{
		Version:    versionV1,
		IntentID:   intent.IntentID,
		RequestID:  intent.RequestID,
		TaskType:   intent.TaskType,
		Status:     statusSucceeded,
		CreatedAt:  intent.CreatedAt,
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
		Data:       data,
	}
	if outcomeErr != nil {
		final.Status = statusFailed
		final.Error = &Error{
			Code:    errorCodeExec,
			Message: outcomeErr.Error(),
		}
	}

	if _, err := r.queue.WriteResult(ctx, final); err != nil {
		return fmt.Errorf("write final result for %s: %w", intent.IntentID, err)
	}
	if err := os.Remove(r.queue.ClaimPath(intent.IntentID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		r.logger.Printf("warn: remove claim for %s failed: %v", intent.IntentID, err)
	}
	return nil
}

func handleProjectFileRead(payloadMap map[string]any) (map[string]any, error) {
	var payload ProjectFileReadPayload
	if err := decodePayload(payloadMap, &payload); err != nil {
		return nil, err
	}

	basePath, err := resolveProjectBasePath(payload.BasePath)
	if err != nil {
		return nil, err
	}
	requestedPath, err := resolveProjectRequestedPath(basePath, payload.Path)
	if err != nil {
		return nil, err
	}
	if info, err := os.Lstat(requestedPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing to read symlinked file")
		}
		if info.IsDir() {
			return nil, fmt.Errorf("target path points to a directory")
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat target path: %w", err)
	}

	targetPath, err := resolveProjectPath(basePath, payload.Path)
	if err != nil {
		return nil, err
	}

	info, err := os.Lstat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("stat target path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlinked file")
	}
	if info.IsDir() {
		return nil, fmt.Errorf("target path points to a directory")
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read target file: %w", err)
	}

	return map[string]any{
		"path":       targetPath,
		"content":    string(raw),
		"size_bytes": info.Size(),
		"updated_at": info.ModTime().UTC().Format(time.RFC3339Nano),
	}, nil
}

func decodePayload(payload map[string]any, target any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return nil
}

func resolveProjectBasePath(rawBasePath string) (string, error) {
	basePath := filepath.Clean(strings.TrimSpace(rawBasePath))
	if basePath == "" || basePath == "." {
		return "", fmt.Errorf("base_path is required")
	}
	if !filepath.IsAbs(basePath) {
		return "", fmt.Errorf("base_path must be absolute")
	}

	if resolved, err := filepath.EvalSymlinks(basePath); err == nil {
		basePath = filepath.Clean(resolved)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("resolve base path: %w", err)
	}

	info, err := os.Stat(basePath)
	if err != nil {
		return "", fmt.Errorf("base path missing: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("base path is not a directory")
	}
	return basePath, nil
}

func resolveProjectPath(basePath, rawPath string) (string, error) {
	path, err := resolveProjectRequestedPath(basePath, rawPath)
	if err != nil {
		return "", err
	}
	if !pathWithinBase(basePath, path) {
		return "", fmt.Errorf("path resolves outside base path")
	}

	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = filepath.Clean(resolved)
		if !pathWithinBase(basePath, path) {
			return "", fmt.Errorf("path resolves outside base path")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	return path, nil
}

func resolveProjectRequestedPath(basePath, rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(basePath, path)
	}
	return filepath.Clean(path), nil
}

func pathWithinBase(basePath, targetPath string) bool {
	base := filepath.Clean(basePath)
	target := filepath.Clean(targetPath)
	if base == target {
		return true
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

type filesystemQueue struct {
	rootDir    string
	intentsDir string
	claimsDir  string
	resultsDir string
}

func newFilesystemQueue(root string) (*filesystemQueue, error) {
	normalized := expandUserPath(strings.TrimSpace(root))
	if normalized == "" {
		return nil, fmt.Errorf("host infra queue root is empty")
	}
	q := &filesystemQueue{
		rootDir:    normalized,
		intentsDir: filepath.Join(normalized, "intents"),
		claimsDir:  filepath.Join(normalized, "claims"),
		resultsDir: filepath.Join(normalized, "results"),
	}
	if err := q.EnsureDirs(); err != nil {
		return nil, err
	}
	return q, nil
}

func (q *filesystemQueue) EnsureDirs() error {
	for _, dir := range []string{q.rootDir, q.intentsDir, q.claimsDir, q.resultsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create host infra queue directory %s: %w", dir, err)
		}
	}
	return nil
}

func (q *filesystemQueue) IntentPath(intentID string) string {
	return filepath.Join(q.intentsDir, intentID+".json")
}

func (q *filesystemQueue) ClaimPath(intentID string) string {
	return filepath.Join(q.claimsDir, intentID+".lock")
}

func (q *filesystemQueue) ResultPath(intentID string) string {
	return filepath.Join(q.resultsDir, intentID+".json")
}

func (q *filesystemQueue) ListIntentIDs(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(q.intentsDir)
	if err != nil {
		return nil, fmt.Errorf("read intents directory: %w", err)
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		if err := validateIdentifier(id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (q *filesystemQueue) ReadIntent(ctx context.Context, intentID string) (Intent, error) {
	if err := ctx.Err(); err != nil {
		return Intent{}, err
	}
	if err := validateIdentifier(intentID); err != nil {
		return Intent{}, err
	}
	payload, err := os.ReadFile(q.IntentPath(intentID))
	if err != nil {
		return Intent{}, err
	}
	var intent Intent
	if err := json.Unmarshal(payload, &intent); err != nil {
		return Intent{}, fmt.Errorf("decode intent %s: %w", intentID, err)
	}
	return intent, nil
}

func (q *filesystemQueue) ReadResult(ctx context.Context, intentID string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := validateIdentifier(intentID); err != nil {
		return Result{}, err
	}
	payload, err := os.ReadFile(q.ResultPath(intentID))
	if err != nil {
		return Result{}, err
	}
	var result Result
	if err := json.Unmarshal(payload, &result); err != nil {
		return Result{}, fmt.Errorf("decode result %s: %w", intentID, err)
	}
	return result, nil
}

func (q *filesystemQueue) ClaimIntent(ctx context.Context, intentID, owner string) (Claim, bool, error) {
	if err := ctx.Err(); err != nil {
		return Claim{}, false, err
	}
	if err := validateIdentifier(intentID); err != nil {
		return Claim{}, false, err
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		hostname, _ := os.Hostname()
		owner = fmt.Sprintf("%s:%d", hostname, os.Getpid())
	}
	claim := Claim{
		Version:   versionV1,
		IntentID:  intentID,
		Owner:     owner,
		ClaimedAt: time.Now().UTC(),
	}
	path := q.ClaimPath(intentID)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return Claim{}, false, nil
		}
		return Claim{}, false, fmt.Errorf("claim intent %s: %w", intentID, err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(claim); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return Claim{}, false, fmt.Errorf("encode claim %s: %w", intentID, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return Claim{}, false, fmt.Errorf("sync claim %s: %w", intentID, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return Claim{}, false, fmt.Errorf("close claim %s: %w", intentID, err)
	}
	return claim, true, nil
}

func (q *filesystemQueue) WriteResult(ctx context.Context, result Result) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateIdentifier(result.IntentID); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Version) == "" {
		result.Version = versionV1
	}
	path := q.ResultPath(result.IntentID)
	if err := writeJSONAtomic(path, result, 0o644, true); err != nil {
		return "", err
	}
	return path, nil
}

func (r Result) Terminal() bool {
	return r.Status == statusSucceeded || r.Status == statusFailed
}

func writeJSONAtomic(path string, payload any, mode os.FileMode, allowReplace bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if !allowReplace {
		if _, err := os.Stat(path); err == nil {
			return os.ErrExist
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", path, err)
		}
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpPath := tmpFile.Name()

	encoder := json.NewEncoder(tmpFile)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := tmpFile.Chmod(mode); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file for %s: %w", path, err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync temp file for %s: %w", path, err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file for %s: %w", path, err)
	}
	return nil
}

func validateIdentifier(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("identifier is required")
	}
	if !safeIDPattern.MatchString(trimmed) {
		return fmt.Errorf("identifier %q contains unsupported characters", id)
	}
	return nil
}

func expandUserPath(raw string) string {
	if raw == "" || raw[0] != '~' {
		return raw
	}
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return raw
	}
	if raw == "~" {
		return homeDir
	}
	if len(raw) > 1 && os.IsPathSeparator(raw[1]) {
		return filepath.Join(homeDir, raw[2:])
	}
	return raw
}
