package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go-notes/internal/infra/contract"
)

var safeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type Filesystem struct {
	rootDir    string
	intentsDir string
	claimsDir  string
	resultsDir string
}

type RetentionPolicy struct {
	IntentMaxAge time.Duration
	ResultMaxAge time.Duration
	ClaimMaxAge  time.Duration
}

type CleanupReport struct {
	RemovedIntents int
	RemovedResults int
	RemovedClaims  int
	ProtectedTasks int
}

func (r CleanupReport) TotalRemoved() int {
	return r.RemovedIntents + r.RemovedResults + r.RemovedClaims
}

type artifactState struct {
	intentPath string
	claimPath  string
	resultPath string
	claimMod   time.Time
	resultMod  time.Time

	resultLoaded    bool
	resultTerminal  bool
	resultTimestamp time.Time
}

var defaultRetentionPolicy = RetentionPolicy{
	IntentMaxAge: 7 * 24 * time.Hour,
	ResultMaxAge: 7 * 24 * time.Hour,
	ClaimMaxAge:  60 * time.Minute,
}

func NewFilesystem(root string) (*Filesystem, error) {
	normalized := expandUserPath(strings.TrimSpace(root))
	if normalized == "" {
		return nil, fmt.Errorf("infra root path is empty")
	}
	q := &Filesystem{
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

func (q *Filesystem) RootDir() string {
	return q.rootDir
}

func (q *Filesystem) CleanupStale(ctx context.Context, now time.Time, policy RetentionPolicy) (CleanupReport, error) {
	if err := ctx.Err(); err != nil {
		return CleanupReport{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	policy = normalizeRetentionPolicy(policy)

	states, err := q.loadArtifactStates(ctx)
	if err != nil {
		return CleanupReport{}, err
	}

	ids := make([]string, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	report := CleanupReport{}
	protected := make(map[string]struct{})
	var cleanupErr error

	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		state := states[id]

		if state.claimPath != "" {
			if shouldRemoveClaim(state, now, policy) {
				if err := os.Remove(state.claimPath); err != nil && !errors.Is(err, os.ErrNotExist) {
					cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove stale claim %s: %w", state.claimPath, err))
				} else {
					state.claimPath = ""
					report.RemovedClaims++
				}
			} else if state.intentPath != "" && !state.resultTerminal {
				protected[id] = struct{}{}
			}
		}
	}

	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		state := states[id]
		if state.resultPath == "" {
			continue
		}

		if shouldRemoveResult(state, now, policy) {
			if err := os.Remove(state.resultPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove stale result %s: %w", state.resultPath, err))
			} else {
				state.resultPath = ""
				report.RemovedResults++
			}
			continue
		}
		if !state.resultTerminal {
			protected[id] = struct{}{}
		}
	}

	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		state := states[id]
		if state.intentPath == "" {
			continue
		}

		if shouldRemoveIntent(state, now, policy) {
			if err := os.Remove(state.intentPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove stale intent %s: %w", state.intentPath, err))
			} else {
				state.intentPath = ""
				report.RemovedIntents++
			}
			continue
		}
		if state.claimPath != "" || !state.resultTerminal {
			protected[id] = struct{}{}
		}
	}

	report.ProtectedTasks = len(protected)
	return report, cleanupErr
}

func (q *Filesystem) EnsureDirs() error {
	dirs := []string{q.rootDir, q.intentsDir, q.claimsDir, q.resultsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create infra queue directory %s: %w", dir, err)
		}
	}
	return nil
}

func (q *Filesystem) WriteIntent(ctx context.Context, intent contract.Intent) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateIdentifier(intent.IntentID); err != nil {
		return "", err
	}
	if intent.Version == "" {
		intent.Version = contract.VersionV1
	}
	if intent.CreatedAt.IsZero() {
		intent.CreatedAt = time.Now().UTC()
	}
	path := q.IntentPath(intent.IntentID)
	if err := writeJSONAtomic(path, intent, 0o644, false); err != nil {
		return "", err
	}
	return path, nil
}

func (q *Filesystem) ReadIntent(ctx context.Context, intentID string) (contract.Intent, error) {
	if err := ctx.Err(); err != nil {
		return contract.Intent{}, err
	}
	if err := validateIdentifier(intentID); err != nil {
		return contract.Intent{}, err
	}
	path := q.IntentPath(intentID)
	payload, err := os.ReadFile(path)
	if err != nil {
		return contract.Intent{}, err
	}
	var intent contract.Intent
	if err := json.Unmarshal(payload, &intent); err != nil {
		return contract.Intent{}, fmt.Errorf("decode intent %s: %w", intentID, err)
	}
	return intent, nil
}

func (q *Filesystem) ClaimIntent(ctx context.Context, intentID, owner string) (contract.Claim, bool, error) {
	if err := ctx.Err(); err != nil {
		return contract.Claim{}, false, err
	}
	if err := validateIdentifier(intentID); err != nil {
		return contract.Claim{}, false, err
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		hostname, _ := os.Hostname()
		owner = fmt.Sprintf("%s:%d", hostname, os.Getpid())
	}
	claim := contract.Claim{
		Version:   contract.VersionV1,
		IntentID:  intentID,
		Owner:     owner,
		ClaimedAt: time.Now().UTC(),
	}
	path := q.ClaimPath(intentID)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return contract.Claim{}, false, nil
		}
		return contract.Claim{}, false, fmt.Errorf("claim intent %s: %w", intentID, err)
	}

	enc := json.NewEncoder(file)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(claim); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return contract.Claim{}, false, fmt.Errorf("encode claim %s: %w", intentID, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return contract.Claim{}, false, fmt.Errorf("sync claim %s: %w", intentID, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return contract.Claim{}, false, fmt.Errorf("close claim %s: %w", intentID, err)
	}
	return claim, true, nil
}

func (q *Filesystem) WriteResult(ctx context.Context, result contract.Result) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateIdentifier(result.IntentID); err != nil {
		return "", err
	}
	if result.Version == "" {
		result.Version = contract.VersionV1
	}
	path := q.ResultPath(result.IntentID)
	if err := writeJSONAtomic(path, result, 0o644, true); err != nil {
		return "", err
	}
	return path, nil
}

func (q *Filesystem) ReadResult(ctx context.Context, intentID string) (contract.Result, error) {
	if err := ctx.Err(); err != nil {
		return contract.Result{}, err
	}
	if err := validateIdentifier(intentID); err != nil {
		return contract.Result{}, err
	}
	path := q.ResultPath(intentID)
	payload, err := os.ReadFile(path)
	if err != nil {
		return contract.Result{}, err
	}
	var result contract.Result
	if err := json.Unmarshal(payload, &result); err != nil {
		return contract.Result{}, fmt.Errorf("decode result %s: %w", intentID, err)
	}
	return result, nil
}

func (q *Filesystem) IntentPath(intentID string) string {
	return filepath.Join(q.intentsDir, intentID+".json")
}

func (q *Filesystem) ClaimPath(intentID string) string {
	return filepath.Join(q.claimsDir, intentID+".lock")
}

func (q *Filesystem) ResultPath(intentID string) string {
	return filepath.Join(q.resultsDir, intentID+".json")
}

func (q *Filesystem) ListIntentIDs(ctx context.Context) ([]string, error) {
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
		id, ok := artifactID(entry.Name(), ".json")
		if !ok {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func (q *Filesystem) loadArtifactStates(ctx context.Context) (map[string]*artifactState, error) {
	states := make(map[string]*artifactState)

	intentEntries, err := os.ReadDir(q.intentsDir)
	if err != nil {
		return nil, fmt.Errorf("read intents directory: %w", err)
	}
	for _, entry := range intentEntries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, ok := artifactID(entry.Name(), ".json")
		if !ok {
			continue
		}
		state := ensureArtifactState(states, id)
		state.intentPath = filepath.Join(q.intentsDir, entry.Name())
	}

	claimEntries, err := os.ReadDir(q.claimsDir)
	if err != nil {
		return nil, fmt.Errorf("read claims directory: %w", err)
	}
	for _, entry := range claimEntries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, ok := artifactID(entry.Name(), ".lock")
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("read claim info %s: %w", entry.Name(), err)
		}
		state := ensureArtifactState(states, id)
		state.claimPath = filepath.Join(q.claimsDir, entry.Name())
		state.claimMod = info.ModTime().UTC()
	}

	resultEntries, err := os.ReadDir(q.resultsDir)
	if err != nil {
		return nil, fmt.Errorf("read results directory: %w", err)
	}
	for _, entry := range resultEntries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, ok := artifactID(entry.Name(), ".json")
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("read result info %s: %w", entry.Name(), err)
		}

		state := ensureArtifactState(states, id)
		state.resultPath = filepath.Join(q.resultsDir, entry.Name())
		state.resultMod = info.ModTime().UTC()

		payload, err := os.ReadFile(state.resultPath)
		if err != nil {
			return nil, fmt.Errorf("read result payload %s: %w", state.resultPath, err)
		}
		var result contract.Result
		if err := json.Unmarshal(payload, &result); err != nil {
			continue
		}
		state.resultLoaded = true
		state.resultTerminal = contract.IsTerminalStatus(result.Status)
		if !result.FinishedAt.IsZero() {
			state.resultTimestamp = result.FinishedAt.UTC()
		} else {
			state.resultTimestamp = state.resultMod
		}
	}

	return states, nil
}

func ensureArtifactState(states map[string]*artifactState, id string) *artifactState {
	state := states[id]
	if state == nil {
		state = &artifactState{}
		states[id] = state
	}
	return state
}

func artifactID(filename, suffix string) (string, bool) {
	if !strings.HasSuffix(filename, suffix) {
		return "", false
	}
	id := strings.TrimSuffix(filename, suffix)
	if err := validateIdentifier(id); err != nil {
		return "", false
	}
	return id, true
}

func shouldRemoveClaim(state *artifactState, now time.Time, policy RetentionPolicy) bool {
	if state == nil || state.claimPath == "" {
		return false
	}
	if !state.claimMod.IsZero() && now.Sub(state.claimMod) < policy.ClaimMaxAge {
		return false
	}
	if state.resultTerminal {
		return true
	}
	return state.intentPath == ""
}

func shouldRemoveResult(state *artifactState, now time.Time, policy RetentionPolicy) bool {
	if state == nil || state.resultPath == "" {
		return false
	}
	if !state.resultTerminal || state.claimPath != "" {
		return false
	}
	if state.resultTimestamp.IsZero() {
		return false
	}
	return now.Sub(state.resultTimestamp) >= policy.ResultMaxAge
}

func shouldRemoveIntent(state *artifactState, now time.Time, policy RetentionPolicy) bool {
	if state == nil || state.intentPath == "" {
		return false
	}
	if state.claimPath != "" || !state.resultTerminal {
		return false
	}
	if state.resultTimestamp.IsZero() {
		return false
	}
	return now.Sub(state.resultTimestamp) >= policy.IntentMaxAge
}

func normalizeRetentionPolicy(policy RetentionPolicy) RetentionPolicy {
	normalized := policy
	if normalized.IntentMaxAge <= 0 {
		normalized.IntentMaxAge = defaultRetentionPolicy.IntentMaxAge
	}
	if normalized.ResultMaxAge <= 0 {
		normalized.ResultMaxAge = defaultRetentionPolicy.ResultMaxAge
	}
	if normalized.ClaimMaxAge <= 0 {
		normalized.ClaimMaxAge = defaultRetentionPolicy.ClaimMaxAge
	}
	return normalized
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

	enc := json.NewEncoder(tmpFile)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("encode json for %s: %w", path, err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync json for %s: %w", path, err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp json for %s: %w", path, err)
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod temp json for %s: %w", path, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic rename for %s: %w", path, err)
	}
	return nil
}

func validateIdentifier(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("identifier is empty")
	}
	if !safeIDPattern.MatchString(trimmed) {
		return fmt.Errorf("identifier %q is invalid", id)
	}
	return nil
}

func expandUserPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed[0] != '~' {
		return trimmed
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return strings.TrimPrefix(trimmed, "~")
	}
	switch trimmed {
	case "~":
		return home
	default:
		return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
	}
}
