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

type NetBirdModeConfig struct {
	APIBaseURL   string   `json:"apiBaseUrl,omitempty"`
	APITokenSet  bool     `json:"apiTokenSet"`
	HostPeerID   string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs []string `json:"adminPeerIds"`
}

type NetBirdModeConfigUpdate struct {
	APIBaseURL   *string   `json:"apiBaseUrl,omitempty"`
	APIToken     *string   `json:"apiToken,omitempty"`
	HostPeerID   *string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs *[]string `json:"adminPeerIds,omitempty"`
}

type netBirdStoredConfig struct {
	APIBaseURL   string   `json:"apiBaseUrl,omitempty"`
	APIToken     string   `json:"apiToken,omitempty"`
	HostPeerID   string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs []string `json:"adminPeerIds,omitempty"`
}

func (s *SettingsService) GetNetBirdModeConfig(ctx context.Context) (NetBirdModeConfig, error) {
	config, _, err := s.loadNetBirdStoredConfig(ctx)
	if err != nil {
		return NetBirdModeConfig{}, err
	}
	return netBirdModeConfigView(config), nil
}

func (s *SettingsService) UpsertNetBirdModeConfig(ctx context.Context, input NetBirdModeConfigUpdate) (NetBirdModeConfig, error) {
	stored, err := s.repo.Get(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			stored = &models.Settings{}
		} else {
			return NetBirdModeConfig{}, err
		}
	}
	if stored == nil {
		stored = &models.Settings{}
	}

	current, err := decodeStoredNetBirdConfig(s.cfg.SessionSecret, stored.NetBirdConfigEncrypted)
	if err != nil {
		return NetBirdModeConfig{}, fmt.Errorf("decode stored netbird config: %w", err)
	}

	next := current
	if input.APIBaseURL != nil {
		next.APIBaseURL = strings.TrimSpace(*input.APIBaseURL)
	}
	if input.HostPeerID != nil {
		next.HostPeerID = strings.TrimSpace(*input.HostPeerID)
	}
	if input.AdminPeerIDs != nil {
		next.AdminPeerIDs = normalizeStringList(*input.AdminPeerIDs)
	}
	if input.APIToken != nil {
		next.APIToken = strings.TrimSpace(*input.APIToken)
	}

	encoded, err := encodeStoredNetBirdConfig(s.cfg.SessionSecret, next)
	if err != nil {
		return NetBirdModeConfig{}, err
	}
	stored.NetBirdConfigEncrypted = encoded

	if err := s.repo.Save(ctx, stored); err != nil {
		return NetBirdModeConfig{}, err
	}
	return netBirdModeConfigView(next), nil
}

func (s *SettingsService) ResolveNetBirdModeApplyRequest(ctx context.Context, input NetBirdModeApplyRequest) (NetBirdModeApplyRequest, bool, error) {
	request := NormalizeNetBirdModeApplyRequest(input)
	stored, _, err := s.loadNetBirdStoredConfig(ctx)
	if err != nil {
		return NetBirdModeApplyRequest{}, false, err
	}

	usedStored := false
	if request.APIBaseURL == "" && stored.APIBaseURL != "" {
		request.APIBaseURL = stored.APIBaseURL
		usedStored = true
	}
	if request.APIToken == "" && stored.APIToken != "" {
		request.APIToken = stored.APIToken
		usedStored = true
	}
	if request.HostPeerID == "" && stored.HostPeerID != "" {
		request.HostPeerID = stored.HostPeerID
		usedStored = true
	}
	if len(request.AdminPeerIDs) == 0 && len(stored.AdminPeerIDs) > 0 {
		request.AdminPeerIDs = append([]string(nil), stored.AdminPeerIDs...)
		usedStored = true
	}

	return NormalizeNetBirdModeApplyRequest(request), usedStored, nil
}

func (s *SettingsService) ResolveNetBirdModeApplyJobRequest(ctx context.Context, input NetBirdModeApplyJobRequest) (NetBirdModeApplyRequest, bool, error) {
	resolved, usedStored, err := s.ResolveNetBirdModeApplyRequest(ctx, NetBirdModeApplyRequest{
		TargetMode:     input.TargetMode,
		AllowLocalhost: input.AllowLocalhost,
		APIBaseURL:     input.APIBaseURL,
		APIToken:       input.APIToken,
		HostPeerID:     input.HostPeerID,
		AdminPeerIDs:   input.AdminPeerIDs,
	})
	if err != nil {
		return NetBirdModeApplyRequest{}, false, err
	}
	return resolved, usedStored, nil
}

func (s *SettingsService) loadNetBirdStoredConfig(ctx context.Context) (netBirdStoredConfig, bool, error) {
	stored, err := s.repo.Get(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return netBirdStoredConfig{}, false, nil
		}
		return netBirdStoredConfig{}, false, err
	}
	if stored == nil {
		return netBirdStoredConfig{}, false, nil
	}
	config, err := decodeStoredNetBirdConfig(s.cfg.SessionSecret, stored.NetBirdConfigEncrypted)
	if err != nil {
		return netBirdStoredConfig{}, false, fmt.Errorf("decode stored netbird config: %w", err)
	}
	return config, true, nil
}

func netBirdModeConfigView(stored netBirdStoredConfig) NetBirdModeConfig {
	normalized := normalizeStoredNetBirdConfig(stored)
	return NetBirdModeConfig{
		APIBaseURL:   normalized.APIBaseURL,
		APITokenSet:  normalized.APIToken != "",
		HostPeerID:   normalized.HostPeerID,
		AdminPeerIDs: append([]string(nil), normalized.AdminPeerIDs...),
	}
}

func normalizeStoredNetBirdConfig(input netBirdStoredConfig) netBirdStoredConfig {
	return netBirdStoredConfig{
		APIBaseURL:   strings.TrimSpace(input.APIBaseURL),
		APIToken:     strings.TrimSpace(input.APIToken),
		HostPeerID:   strings.TrimSpace(input.HostPeerID),
		AdminPeerIDs: normalizeStringList(input.AdminPeerIDs),
	}
}

func decodeStoredNetBirdConfig(secret, encrypted string) (netBirdStoredConfig, error) {
	trimmed := strings.TrimSpace(encrypted)
	if trimmed == "" {
		return netBirdStoredConfig{}, nil
	}

	raw, err := cryptox.DecryptWithSecret(secret, trimmed)
	if err != nil {
		return netBirdStoredConfig{}, err
	}
	if strings.TrimSpace(raw) == "" {
		return netBirdStoredConfig{}, nil
	}

	var parsed netBirdStoredConfig
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return netBirdStoredConfig{}, fmt.Errorf("unmarshal netbird config: %w", err)
	}
	return normalizeStoredNetBirdConfig(parsed), nil
}

func encodeStoredNetBirdConfig(secret string, config netBirdStoredConfig) (string, error) {
	normalized := normalizeStoredNetBirdConfig(config)
	if normalized.APIBaseURL == "" && normalized.APIToken == "" && normalized.HostPeerID == "" && len(normalized.AdminPeerIDs) == 0 {
		return "", nil
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal netbird config: %w", err)
	}
	encrypted, err := cryptox.EncryptWithSecret(secret, string(raw))
	if err != nil {
		return "", fmt.Errorf("encrypt netbird config: %w", err)
	}
	return encrypted, nil
}
