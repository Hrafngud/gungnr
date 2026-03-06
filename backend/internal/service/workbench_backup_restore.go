package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go-notes/internal/errs"
)

const (
	workbenchComposeBackupIndexVersion = 1
	workbenchComposeBackupDirRelative  = ".gungnr/workbench/compose-backups"
	workbenchComposeBackupIndexName    = "index.json"
)

type WorkbenchComposeBackupMetadata struct {
	BackupID          string    `json:"backupId"`
	Sequence          int       `json:"sequence"`
	Revision          int       `json:"revision"`
	SourceFingerprint string    `json:"sourceFingerprint,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	ComposeBytes      int       `json:"composeBytes"`
}

type WorkbenchComposeBackupRetentionInfo struct {
	RetainedCount int `json:"retainedCount"`
	PrunedCount   int `json:"prunedCount"`
}

type WorkbenchComposeRestoreRequest struct {
	BackupID string `json:"backupId"`
}

type WorkbenchComposeRestoreMetadata struct {
	Revision            int    `json:"revision"`
	SourceFingerprint   string `json:"sourceFingerprint,omitempty"`
	RestoredFingerprint string `json:"restoredFingerprint,omitempty"`
	ComposePath         string `json:"composePath"`
	RequiresImport      bool   `json:"requiresImport"`
}

type WorkbenchComposeRestoreResult struct {
	Metadata     WorkbenchComposeRestoreMetadata `json:"metadata"`
	Backup       WorkbenchComposeBackupMetadata  `json:"backup"`
	ComposeBytes int                             `json:"composeBytes"`
}

type workbenchComposeBackupIndex struct {
	Version int                            `json:"version"`
	Backups []workbenchStoredComposeBackup `json:"backups"`
}

type workbenchStoredComposeBackup struct {
	BackupID          string    `json:"backupId"`
	Sequence          int       `json:"sequence"`
	Revision          int       `json:"revision"`
	SourceFingerprint string    `json:"sourceFingerprint"`
	ArtifactPath      string    `json:"artifactPath"`
	CreatedAt         time.Time `json:"createdAt"`
	ComposeBytes      int       `json:"composeBytes"`
}

func (s *WorkbenchService) RestoreComposeFromBackup(
	ctx context.Context,
	projectName string,
	input WorkbenchComposeRestoreRequest,
) (WorkbenchComposeRestoreResult, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchComposeRestoreResult{}, err
	}

	normalizedInput, err := normalizeWorkbenchComposeRestoreRequest(input)
	if err != nil {
		return WorkbenchComposeRestoreResult{}, err
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchComposeRestoreResult{}, err
	}
	defer release()

	snapshot, err := s.loadStoredSnapshotForComposeLocked(ctx, normalizedProject)
	if err != nil {
		return WorkbenchComposeRestoreResult{}, err
	}

	currentSource, err := s.ResolveComposeSource(ctx, normalizedProject)
	if err != nil {
		return WorkbenchComposeRestoreResult{}, err
	}

	backups, err := loadWorkbenchComposeBackupIndex(currentSource.ProjectDir)
	if err != nil {
		return WorkbenchComposeRestoreResult{}, workbenchComposeBackupIntegrityError(snapshot, currentSource, normalizedInput.BackupID, "failed to load workbench compose backup history", err)
	}

	target, ok := findWorkbenchComposeBackup(backups, normalizedInput.BackupID)
	if !ok {
		return WorkbenchComposeRestoreResult{}, workbenchComposeBackupNotFoundError(snapshot, currentSource, normalizedInput.BackupID, "workbench compose backup target not found")
	}

	artifactPath, err := resolveWorkbenchComposeBackupArtifactPath(currentSource.ProjectDir, target.ArtifactPath)
	if err != nil {
		return WorkbenchComposeRestoreResult{}, workbenchComposeBackupIntegrityError(snapshot, currentSource, normalizedInput.BackupID, "workbench compose backup target has an invalid artifact path", err)
	}

	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return WorkbenchComposeRestoreResult{}, workbenchComposeBackupNotFoundError(snapshot, currentSource, normalizedInput.BackupID, "workbench compose backup artifact not found")
		}
		return WorkbenchComposeRestoreResult{}, workbenchComposeBackupIntegrityError(snapshot, currentSource, normalizedInput.BackupID, "failed to read workbench compose backup artifact", err)
	}

	_, restoredFingerprint := WorkbenchSourceFingerprint(raw)
	if restoredFingerprint != target.SourceFingerprint {
		return WorkbenchComposeRestoreResult{}, workbenchComposeBackupIntegrityError(
			snapshot,
			currentSource,
			normalizedInput.BackupID,
			"workbench compose backup fingerprint does not match stored metadata",
			nil,
		)
	}
	if target.ComposeBytes > 0 && len(raw) != target.ComposeBytes {
		return WorkbenchComposeRestoreResult{}, workbenchComposeBackupIntegrityError(
			snapshot,
			currentSource,
			normalizedInput.BackupID,
			"workbench compose backup size does not match stored metadata",
			nil,
		)
	}

	if err := replaceWorkbenchComposeAtomically(currentSource.ComposePath, raw); err != nil {
		return WorkbenchComposeRestoreResult{}, workbenchComposeRestoreError(snapshot, currentSource, target, "failed to restore workbench compose backup", err)
	}

	return WorkbenchComposeRestoreResult{
		Metadata: WorkbenchComposeRestoreMetadata{
			Revision:            snapshot.Revision,
			SourceFingerprint:   strings.TrimSpace(snapshot.SourceFingerprint),
			RestoredFingerprint: restoredFingerprint,
			ComposePath:         currentSource.ComposePath,
			RequiresImport:      strings.TrimSpace(snapshot.SourceFingerprint) != restoredFingerprint,
		},
		Backup:       workbenchComposeBackupMetadataFromStored(target),
		ComposeBytes: len(raw),
	}, nil
}

func normalizeWorkbenchComposeRestoreRequest(input WorkbenchComposeRestoreRequest) (WorkbenchComposeRestoreRequest, error) {
	backupID := strings.ToLower(strings.TrimSpace(input.BackupID))
	if backupID == "" {
		return WorkbenchComposeRestoreRequest{}, errs.New(errs.CodeProjectInvalidBody, "backupId is required")
	}
	return WorkbenchComposeRestoreRequest{BackupID: backupID}, nil
}

func (s *WorkbenchService) createComposeBackup(
	ctx context.Context,
	normalizedProject string,
	snapshot WorkbenchStackSnapshot,
	source WorkbenchComposeSource,
) (WorkbenchComposeBackupMetadata, WorkbenchComposeBackupRetentionInfo, error) {
	backups, err := loadWorkbenchComposeBackupIndex(source.ProjectDir)
	if err != nil {
		return WorkbenchComposeBackupMetadata{}, WorkbenchComposeBackupRetentionInfo{}, workbenchComposeBackupIntegrityError(snapshot, source, "", "failed to load workbench compose backup history", err)
	}

	now := time.Now().UTC()
	if s.nowFn != nil {
		now = s.nowFn().UTC()
	}

	nextSequence := 1
	for _, backup := range backups {
		if backup.Sequence >= nextSequence {
			nextSequence = backup.Sequence + 1
		}
	}

	backupID := workbenchComposeBackupID(nextSequence)
	artifactRelative := workbenchComposeBackupArtifactRelativePath(backupID)
	artifactPath, err := resolveWorkbenchComposeBackupArtifactPath(source.ProjectDir, artifactRelative)
	if err != nil {
		return WorkbenchComposeBackupMetadata{}, WorkbenchComposeBackupRetentionInfo{}, workbenchComposeBackupWriteError(snapshot, source, backupID, "failed to resolve workbench compose backup artifact path", err, nil)
	}

	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		return WorkbenchComposeBackupMetadata{}, WorkbenchComposeBackupRetentionInfo{}, workbenchComposeBackupWriteError(snapshot, source, backupID, "failed to create workbench compose backup directory", err, nil)
	}
	if err := writeWorkbenchFileAtomically(artifactPath, source.Raw, 0o600); err != nil {
		return WorkbenchComposeBackupMetadata{}, WorkbenchComposeBackupRetentionInfo{}, workbenchComposeBackupWriteError(snapshot, source, backupID, "failed to write workbench compose backup artifact", err, nil)
	}

	created := normalizeWorkbenchStoredComposeBackup(workbenchStoredComposeBackup{
		BackupID:          backupID,
		Sequence:          nextSequence,
		Revision:          snapshot.Revision,
		SourceFingerprint: strings.TrimSpace(source.Fingerprint),
		ArtifactPath:      artifactRelative,
		CreatedAt:         now,
		ComposeBytes:      len(source.Raw),
	})

	retained, pruned := pruneWorkbenchComposeBackups(append(backups, created), now, s.backupMaxCount, s.backupMaxAge)
	if err := writeWorkbenchComposeBackupIndex(source.ProjectDir, retained); err != nil {
		cleanupErr := os.Remove(artifactPath)
		return WorkbenchComposeBackupMetadata{}, WorkbenchComposeBackupRetentionInfo{}, workbenchComposeBackupWriteError(snapshot, source, backupID, "failed to persist workbench compose backup history", err, cleanupErr)
	}

	if err := removeWorkbenchComposeBackupArtifacts(source.ProjectDir, pruned); err != nil {
		return WorkbenchComposeBackupMetadata{}, WorkbenchComposeBackupRetentionInfo{}, workbenchComposeBackupRetentionError(snapshot, source, backupID, "failed to prune retained workbench compose backup artifacts", err, len(pruned))
	}

	return workbenchComposeBackupMetadataFromStored(created), WorkbenchComposeBackupRetentionInfo{
		RetainedCount: len(retained),
		PrunedCount:   len(pruned),
	}, nil
}

func loadWorkbenchComposeBackupIndex(projectDir string) ([]workbenchStoredComposeBackup, error) {
	indexPath, err := resolveWorkbenchComposeBackupIndexPath(projectDir)
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(indexPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []workbenchStoredComposeBackup{}, nil
		}
		return nil, err
	}

	var index workbenchComposeBackupIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		return nil, err
	}

	return normalizeWorkbenchComposeBackupIndex(index).Backups, nil
}

func writeWorkbenchComposeBackupIndex(projectDir string, backups []workbenchStoredComposeBackup) error {
	indexPath, err := resolveWorkbenchComposeBackupIndexPath(projectDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return err
	}

	index := normalizeWorkbenchComposeBackupIndex(workbenchComposeBackupIndex{
		Version: workbenchComposeBackupIndexVersion,
		Backups: backups,
	})
	encoded, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return writeWorkbenchFileAtomically(indexPath, encoded, 0o600)
}

func normalizeWorkbenchComposeBackupIndex(index workbenchComposeBackupIndex) workbenchComposeBackupIndex {
	normalized := workbenchComposeBackupIndex{
		Version: workbenchComposeBackupIndexVersion,
		Backups: []workbenchStoredComposeBackup{},
	}
	if len(index.Backups) == 0 {
		return normalized
	}

	seen := make(map[string]struct{}, len(index.Backups))
	for _, backup := range index.Backups {
		next := normalizeWorkbenchStoredComposeBackup(backup)
		if next.BackupID == "" || next.Sequence <= 0 || next.ArtifactPath == "" || next.SourceFingerprint == "" {
			continue
		}
		if _, exists := seen[next.BackupID]; exists {
			continue
		}
		seen[next.BackupID] = struct{}{}
		normalized.Backups = append(normalized.Backups, next)
	}

	sort.SliceStable(normalized.Backups, func(i, j int) bool {
		if normalized.Backups[i].Sequence != normalized.Backups[j].Sequence {
			return normalized.Backups[i].Sequence < normalized.Backups[j].Sequence
		}
		return normalized.Backups[i].BackupID < normalized.Backups[j].BackupID
	})
	return normalized
}

func normalizeWorkbenchStoredComposeBackup(backup workbenchStoredComposeBackup) workbenchStoredComposeBackup {
	normalized := backup
	normalized.BackupID = strings.ToLower(strings.TrimSpace(normalized.BackupID))
	normalized.SourceFingerprint = strings.TrimSpace(normalized.SourceFingerprint)
	normalized.ArtifactPath = filepath.ToSlash(filepath.Clean(strings.TrimSpace(normalized.ArtifactPath)))
	normalized.CreatedAt = normalized.CreatedAt.UTC()
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = time.Unix(0, 0).UTC()
	}
	return normalized
}

func pruneWorkbenchComposeBackups(
	backups []workbenchStoredComposeBackup,
	now time.Time,
	maxCount int,
	maxAge time.Duration,
) ([]workbenchStoredComposeBackup, []workbenchStoredComposeBackup) {
	normalized := normalizeWorkbenchComposeBackupIndex(workbenchComposeBackupIndex{Backups: backups}).Backups
	if len(normalized) == 0 {
		return []workbenchStoredComposeBackup{}, []workbenchStoredComposeBackup{}
	}

	kept := make([]workbenchStoredComposeBackup, 0, len(normalized))
	pruned := make([]workbenchStoredComposeBackup, 0, len(normalized))
	cutoff := time.Time{}
	if maxAge > 0 {
		cutoff = now.Add(-maxAge)
	}

	for _, backup := range normalized {
		if !cutoff.IsZero() && backup.CreatedAt.Before(cutoff) {
			pruned = append(pruned, backup)
			continue
		}
		kept = append(kept, backup)
	}

	if maxCount > 0 && len(kept) > maxCount {
		overflow := len(kept) - maxCount
		pruned = append(pruned, kept[:overflow]...)
		kept = append([]workbenchStoredComposeBackup(nil), kept[overflow:]...)
	}

	sort.SliceStable(pruned, func(i, j int) bool {
		if pruned[i].Sequence != pruned[j].Sequence {
			return pruned[i].Sequence < pruned[j].Sequence
		}
		return pruned[i].BackupID < pruned[j].BackupID
	})
	return kept, pruned
}

func removeWorkbenchComposeBackupArtifacts(projectDir string, backups []workbenchStoredComposeBackup) error {
	for _, backup := range backups {
		artifactPath, err := resolveWorkbenchComposeBackupArtifactPath(projectDir, backup.ArtifactPath)
		if err != nil {
			return err
		}
		if err := os.Remove(artifactPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func findWorkbenchComposeBackup(backups []workbenchStoredComposeBackup, backupID string) (workbenchStoredComposeBackup, bool) {
	normalizedID := strings.ToLower(strings.TrimSpace(backupID))
	for _, backup := range backups {
		if backup.BackupID == normalizedID {
			return backup, true
		}
	}
	return workbenchStoredComposeBackup{}, false
}

func resolveWorkbenchComposeBackupIndexPath(projectDir string) (string, error) {
	return resolveWorkbenchComposeBackupArtifactPath(projectDir, filepath.ToSlash(filepath.Join(workbenchComposeBackupDirRelative, workbenchComposeBackupIndexName)))
}

func resolveWorkbenchComposeBackupArtifactPath(projectDir, relativePath string) (string, error) {
	projectRoot := filepath.Clean(strings.TrimSpace(projectDir))
	if projectRoot == "" {
		return "", fmt.Errorf("project root is empty")
	}
	candidate := filepath.Join(projectRoot, filepath.FromSlash(strings.TrimSpace(relativePath)))
	candidate = filepath.Clean(candidate)
	if !isPathWithinBase(projectRoot, candidate) {
		return "", fmt.Errorf("backup artifact path resolves outside project root")
	}
	return candidate, nil
}

func writeWorkbenchFileAtomically(path string, content []byte, mode os.FileMode) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("path is empty")
	}

	dir := filepath.Dir(trimmedPath)
	tempFile, err := os.CreateTemp(dir, ".workbench-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if err := tempFile.Chmod(mode); err != nil {
		_ = tempFile.Close()
		return err
	}
	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, trimmedPath); err != nil {
		return err
	}
	cleanupTemp = false
	return nil
}

func workbenchComposeBackupID(sequence int) string {
	return fmt.Sprintf("wbk-%06d", sequence)
}

func workbenchComposeBackupArtifactRelativePath(backupID string) string {
	return filepath.ToSlash(filepath.Join(workbenchComposeBackupDirRelative, backupID+".compose.yml"))
}

func workbenchComposeBackupMetadataFromStored(backup workbenchStoredComposeBackup) WorkbenchComposeBackupMetadata {
	return WorkbenchComposeBackupMetadata{
		BackupID:          backup.BackupID,
		Sequence:          backup.Sequence,
		Revision:          backup.Revision,
		SourceFingerprint: backup.SourceFingerprint,
		CreatedAt:         backup.CreatedAt,
		ComposeBytes:      backup.ComposeBytes,
	}
}

func workbenchComposeBackupNotFoundError(
	snapshot WorkbenchStackSnapshot,
	source WorkbenchComposeSource,
	backupID string,
	message string,
) error {
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchBackupNotFound, message),
		map[string]any{
			"project":     strings.TrimSpace(snapshot.ProjectName),
			"projectPath": strings.TrimSpace(source.ProjectDir),
			"composePath": strings.TrimSpace(source.ComposePath),
			"backupId":    strings.TrimSpace(strings.ToLower(backupID)),
			"revision":    snapshot.Revision,
		},
	)
}

func workbenchComposeBackupIntegrityError(
	snapshot WorkbenchStackSnapshot,
	source WorkbenchComposeSource,
	backupID string,
	message string,
	cause error,
) error {
	details := map[string]any{
		"project":           strings.TrimSpace(snapshot.ProjectName),
		"projectPath":       strings.TrimSpace(source.ProjectDir),
		"composePath":       strings.TrimSpace(source.ComposePath),
		"backupId":          strings.TrimSpace(strings.ToLower(backupID)),
		"revision":          snapshot.Revision,
		"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
	}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	return errs.WithDetails(errs.Wrap(errs.CodeWorkbenchBackupIntegrity, message, cause), details)
}

func workbenchComposeBackupWriteError(
	snapshot WorkbenchStackSnapshot,
	source WorkbenchComposeSource,
	backupID string,
	message string,
	cause error,
	cleanupErr error,
) error {
	details := map[string]any{
		"project":           strings.TrimSpace(snapshot.ProjectName),
		"projectPath":       strings.TrimSpace(source.ProjectDir),
		"composePath":       strings.TrimSpace(source.ComposePath),
		"backupId":          strings.TrimSpace(strings.ToLower(backupID)),
		"revision":          snapshot.Revision,
		"sourceFingerprint": strings.TrimSpace(source.Fingerprint),
	}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	if cleanupErr != nil {
		details["cleanupError"] = cleanupErr.Error()
	}
	return errs.WithDetails(errs.Wrap(errs.CodeWorkbenchBackupWriteFailed, message, cause), details)
}

func workbenchComposeBackupRetentionError(
	snapshot WorkbenchStackSnapshot,
	source WorkbenchComposeSource,
	backupID string,
	message string,
	cause error,
	prunedCount int,
) error {
	details := map[string]any{
		"project":           strings.TrimSpace(snapshot.ProjectName),
		"projectPath":       strings.TrimSpace(source.ProjectDir),
		"composePath":       strings.TrimSpace(source.ComposePath),
		"backupId":          strings.TrimSpace(strings.ToLower(backupID)),
		"revision":          snapshot.Revision,
		"sourceFingerprint": strings.TrimSpace(source.Fingerprint),
		"prunedCount":       prunedCount,
	}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	return errs.WithDetails(errs.Wrap(errs.CodeWorkbenchBackupRetentionFailed, message, cause), details)
}

func workbenchComposeRestoreError(
	snapshot WorkbenchStackSnapshot,
	source WorkbenchComposeSource,
	backup workbenchStoredComposeBackup,
	message string,
	cause error,
) error {
	details := map[string]any{
		"project":            strings.TrimSpace(snapshot.ProjectName),
		"projectPath":        strings.TrimSpace(source.ProjectDir),
		"composePath":        strings.TrimSpace(source.ComposePath),
		"backupId":           strings.TrimSpace(backup.BackupID),
		"backupFingerprint":  strings.TrimSpace(backup.SourceFingerprint),
		"sourceFingerprint":  strings.TrimSpace(snapshot.SourceFingerprint),
		"revision":           snapshot.Revision,
		"backupComposeBytes": backup.ComposeBytes,
		"backupCreatedAt":    backup.CreatedAt,
	}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	return errs.WithDetails(errs.Wrap(errs.CodeWorkbenchRestoreFailed, message, cause), details)
}
