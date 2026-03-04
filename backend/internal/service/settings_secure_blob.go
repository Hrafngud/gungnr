package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go-notes/internal/models"
	"go-notes/internal/repository"
	"go-notes/internal/utils/cryptox"
)

type settingsSecureBlob struct {
	NetBird            *netBirdStoredConfig               `json:"netbird,omitempty"`
	WorkbenchSnapshots map[string]workbenchStoredSnapshot `json:"workbenchSnapshots,omitempty"`
}

func (s *SettingsService) loadSecureSettingsBlob(ctx context.Context) (settingsSecureBlob, *models.Settings, error) {
	stored, err := s.repo.Get(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return settingsSecureBlob{}, &models.Settings{}, nil
		}
		return settingsSecureBlob{}, nil, err
	}
	if stored == nil {
		return settingsSecureBlob{}, &models.Settings{}, nil
	}

	blob, err := decodeSettingsSecureBlob(s.cfg.SessionSecret, stored.NetBirdConfigEncrypted)
	if err != nil {
		return settingsSecureBlob{}, nil, err
	}
	return blob, stored, nil
}

func (s *SettingsService) saveSecureSettingsBlob(ctx context.Context, stored *models.Settings, blob settingsSecureBlob) error {
	if stored == nil {
		stored = &models.Settings{}
	}

	encoded, err := encodeSettingsSecureBlob(s.cfg.SessionSecret, blob)
	if err != nil {
		return err
	}
	stored.NetBirdConfigEncrypted = encoded
	return s.repo.Save(ctx, stored)
}

func (s *SettingsService) loadWorkbenchStoredSnapshot(ctx context.Context, projectName string) (workbenchStoredSnapshot, bool, error) {
	key := normalizeWorkbenchProjectKey(projectName)
	if key == "" {
		return workbenchStoredSnapshot{}, false, nil
	}

	blob, _, err := s.loadSecureSettingsBlob(ctx)
	if err != nil {
		return workbenchStoredSnapshot{}, false, err
	}
	if len(blob.WorkbenchSnapshots) == 0 {
		return workbenchStoredSnapshot{}, false, nil
	}
	snapshot, ok := blob.WorkbenchSnapshots[key]
	if !ok {
		return workbenchStoredSnapshot{}, false, nil
	}
	return normalizeWorkbenchStoredSnapshot(snapshot), true, nil
}

func (s *SettingsService) upsertWorkbenchStoredSnapshot(ctx context.Context, projectName string, snapshot workbenchStoredSnapshot) error {
	key := normalizeWorkbenchProjectKey(projectName)
	if key == "" {
		return fmt.Errorf("project name is required")
	}

	blob, stored, err := s.loadSecureSettingsBlob(ctx)
	if err != nil {
		return err
	}
	if blob.WorkbenchSnapshots == nil {
		blob.WorkbenchSnapshots = map[string]workbenchStoredSnapshot{}
	}
	blob.WorkbenchSnapshots[key] = normalizeWorkbenchStoredSnapshot(snapshot)
	return s.saveSecureSettingsBlob(ctx, stored, blob)
}

func decodeSettingsSecureBlob(secret, encrypted string) (settingsSecureBlob, error) {
	trimmed := strings.TrimSpace(encrypted)
	if trimmed == "" {
		return settingsSecureBlob{}, nil
	}

	raw, err := cryptox.DecryptWithSecret(secret, trimmed)
	if err != nil {
		return settingsSecureBlob{}, err
	}
	if strings.TrimSpace(raw) == "" {
		return settingsSecureBlob{}, nil
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return settingsSecureBlob{}, fmt.Errorf("unmarshal secure settings blob keys: %w", err)
	}

	if _, hasNetBird := keys["netbird"]; hasNetBird {
		return decodeStructuredSettingsBlob(raw)
	}
	if _, hasWorkbench := keys["workbenchSnapshots"]; hasWorkbench {
		return decodeStructuredSettingsBlob(raw)
	}

	// Backward compatibility for legacy payloads that only stored NetBird config at the root.
	var legacy netBirdStoredConfig
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return settingsSecureBlob{}, fmt.Errorf("unmarshal legacy netbird settings blob: %w", err)
	}

	normalizedLegacy := normalizeStoredNetBirdConfig(legacy)
	blob := settingsSecureBlob{}
	if !isEmptyNetBirdStoredConfig(normalizedLegacy) {
		blob.NetBird = &normalizedLegacy
	}
	return normalizeSettingsSecureBlob(blob), nil
}

func decodeStructuredSettingsBlob(raw string) (settingsSecureBlob, error) {
	var parsed settingsSecureBlob
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return settingsSecureBlob{}, fmt.Errorf("unmarshal secure settings blob: %w", err)
	}
	return normalizeSettingsSecureBlob(parsed), nil
}

func encodeSettingsSecureBlob(secret string, blob settingsSecureBlob) (string, error) {
	normalized := normalizeSettingsSecureBlob(blob)
	if isEmptySettingsSecureBlob(normalized) {
		return "", nil
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal secure settings blob: %w", err)
	}
	encrypted, err := cryptox.EncryptWithSecret(secret, string(raw))
	if err != nil {
		return "", fmt.Errorf("encrypt secure settings blob: %w", err)
	}
	return encrypted, nil
}

func normalizeSettingsSecureBlob(input settingsSecureBlob) settingsSecureBlob {
	normalized := settingsSecureBlob{
		WorkbenchSnapshots: map[string]workbenchStoredSnapshot{},
	}

	if input.NetBird != nil {
		cleaned := normalizeStoredNetBirdConfig(*input.NetBird)
		if !isEmptyNetBirdStoredConfig(cleaned) {
			copyConfig := cleaned
			normalized.NetBird = &copyConfig
		}
	}

	for projectName, snapshot := range input.WorkbenchSnapshots {
		key := normalizeWorkbenchProjectKey(projectName)
		if key == "" {
			continue
		}
		normalized.WorkbenchSnapshots[key] = normalizeWorkbenchStoredSnapshot(snapshot)
	}
	if len(normalized.WorkbenchSnapshots) == 0 {
		normalized.WorkbenchSnapshots = nil
	}

	return normalized
}

func isEmptySettingsSecureBlob(blob settingsSecureBlob) bool {
	if blob.NetBird != nil && !isEmptyNetBirdStoredConfig(*blob.NetBird) {
		return false
	}
	return len(blob.WorkbenchSnapshots) == 0
}

func isEmptyNetBirdStoredConfig(config netBirdStoredConfig) bool {
	return config.APIBaseURL == "" &&
		config.APIToken == "" &&
		config.HostPeerID == "" &&
		len(config.AdminPeerIDs) == 0 &&
		len(config.ModeBProjectIDs) == 0
}
