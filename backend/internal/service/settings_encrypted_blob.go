package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"go-notes/internal/utils/cryptox"
)

type settingsEncryptedPayload struct {
	NetBird   *netBirdStoredConfig               `json:"netbird,omitempty"`
	Workbench map[string]workbenchStoredSnapshot `json:"workbench,omitempty"`
}

func loadSettingsEncryptedPayload(secret, encrypted string) (settingsEncryptedPayload, error) {
	trimmed := strings.TrimSpace(encrypted)
	if trimmed == "" {
		return settingsEncryptedPayload{}, nil
	}

	raw, err := cryptox.DecryptWithSecret(secret, trimmed)
	if err != nil {
		return settingsEncryptedPayload{}, fmt.Errorf("decrypt settings payload: %w", err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return settingsEncryptedPayload{}, nil
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return settingsEncryptedPayload{}, fmt.Errorf("decode settings payload: %w", err)
	}

	if _, hasNetBird := probe["netbird"]; hasNetBird {
		var payload settingsEncryptedPayload
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return settingsEncryptedPayload{}, fmt.Errorf("decode settings envelope payload: %w", err)
		}
		return normalizeSettingsEncryptedPayload(payload), nil
	}
	if _, hasWorkbench := probe["workbench"]; hasWorkbench {
		var payload settingsEncryptedPayload
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return settingsEncryptedPayload{}, fmt.Errorf("decode settings envelope payload: %w", err)
		}
		return normalizeSettingsEncryptedPayload(payload), nil
	}

	// Backward-compatible path for legacy netbird-only payloads.
	var legacy netBirdStoredConfig
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return settingsEncryptedPayload{}, fmt.Errorf("decode legacy netbird payload: %w", err)
	}
	payload := settingsEncryptedPayload{
		NetBird: &legacy,
	}
	return normalizeSettingsEncryptedPayload(payload), nil
}

func encodeSettingsEncryptedPayload(secret string, payload settingsEncryptedPayload) (string, error) {
	normalized := normalizeSettingsEncryptedPayload(payload)
	if normalized.NetBird == nil && len(normalized.Workbench) == 0 {
		return "", nil
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("encode settings payload: %w", err)
	}
	encrypted, err := cryptox.EncryptWithSecret(secret, string(raw))
	if err != nil {
		return "", fmt.Errorf("encrypt settings payload: %w", err)
	}
	return encrypted, nil
}

func normalizeSettingsEncryptedPayload(payload settingsEncryptedPayload) settingsEncryptedPayload {
	normalized := settingsEncryptedPayload{}

	if payload.NetBird != nil {
		next := normalizeStoredNetBirdConfig(*payload.NetBird)
		if !isEmptyNetBirdStoredConfig(next) {
			normalized.NetBird = &next
		}
	}

	if len(payload.Workbench) > 0 {
		snapshots := make(map[string]workbenchStoredSnapshot, len(payload.Workbench))
		for projectName, snapshot := range payload.Workbench {
			key := strings.ToLower(strings.TrimSpace(projectName))
			if key == "" {
				continue
			}
			normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
			normalizedSnapshot.ProjectName = key
			snapshots[key] = normalizedSnapshot
		}
		if len(snapshots) > 0 {
			normalized.Workbench = snapshots
		}
	}

	return normalized
}

func isEmptyNetBirdStoredConfig(config netBirdStoredConfig) bool {
	return strings.TrimSpace(config.APIBaseURL) == "" &&
		strings.TrimSpace(config.APIToken) == "" &&
		strings.TrimSpace(config.HostPeerID) == "" &&
		len(config.AdminPeerIDs) == 0 &&
		len(config.ModeBProjectIDs) == 0
}
