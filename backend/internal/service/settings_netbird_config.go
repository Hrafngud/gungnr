package service

import (
	"context"
	"strings"
)

type NetBirdModeConfig struct {
	APIBaseURL      string   `json:"apiBaseUrl,omitempty"`
	APITokenSet     bool     `json:"apiTokenSet"`
	HostPeerID      string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs    []string `json:"adminPeerIds"`
	ModeBProjectIDs []uint   `json:"modeBProjectIds"`
}

type NetBirdModeConfigUpdate struct {
	APIBaseURL      *string   `json:"apiBaseUrl,omitempty"`
	APIToken        *string   `json:"apiToken,omitempty"`
	HostPeerID      *string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs    *[]string `json:"adminPeerIds,omitempty"`
	ModeBProjectIDs *[]uint   `json:"modeBProjectIds,omitempty"`
}

type netBirdStoredConfig struct {
	APIBaseURL      string   `json:"apiBaseUrl,omitempty"`
	APIToken        string   `json:"apiToken,omitempty"`
	HostPeerID      string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs    []string `json:"adminPeerIds,omitempty"`
	ModeBProjectIDs []uint   `json:"modeBProjectIds,omitempty"`
}

func (s *SettingsService) GetNetBirdModeConfig(ctx context.Context) (NetBirdModeConfig, error) {
	config, _, err := s.loadNetBirdStoredConfig(ctx)
	if err != nil {
		return NetBirdModeConfig{}, err
	}
	return netBirdModeConfigView(config), nil
}

func (s *SettingsService) UpsertNetBirdModeConfig(ctx context.Context, input NetBirdModeConfigUpdate) (NetBirdModeConfig, error) {
	blob, stored, err := s.loadSecureSettingsBlob(ctx)
	if err != nil {
		return NetBirdModeConfig{}, err
	}

	current := netBirdStoredConfig{}
	if blob.NetBird != nil {
		current = *blob.NetBird
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
	if input.ModeBProjectIDs != nil {
		next.ModeBProjectIDs = normalizeUintList(*input.ModeBProjectIDs)
	}
	if input.APIToken != nil {
		next.APIToken = strings.TrimSpace(*input.APIToken)
	}

	next = normalizeStoredNetBirdConfig(next)
	if isEmptyNetBirdStoredConfig(next) {
		blob.NetBird = nil
	} else {
		copyConfig := next
		blob.NetBird = &copyConfig
	}

	if err := s.saveSecureSettingsBlob(ctx, stored, blob); err != nil {
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
		TargetMode:      input.TargetMode,
		AllowLocalhost:  input.AllowLocalhost,
		APIBaseURL:      input.APIBaseURL,
		APIToken:        input.APIToken,
		HostPeerID:      input.HostPeerID,
		AdminPeerIDs:    input.AdminPeerIDs,
		ModeBProjectIDs: input.ModeBProjectIDs,
	})
	if err != nil {
		return NetBirdModeApplyRequest{}, false, err
	}
	return resolved, usedStored, nil
}

func (s *SettingsService) loadNetBirdStoredConfig(ctx context.Context) (netBirdStoredConfig, bool, error) {
	blob, _, err := s.loadSecureSettingsBlob(ctx)
	if err != nil {
		return netBirdStoredConfig{}, false, err
	}
	if blob.NetBird == nil {
		return netBirdStoredConfig{}, false, nil
	}
	return normalizeStoredNetBirdConfig(*blob.NetBird), true, nil
}

func netBirdModeConfigView(stored netBirdStoredConfig) NetBirdModeConfig {
	normalized := normalizeStoredNetBirdConfig(stored)
	return NetBirdModeConfig{
		APIBaseURL:      normalized.APIBaseURL,
		APITokenSet:     normalized.APIToken != "",
		HostPeerID:      normalized.HostPeerID,
		AdminPeerIDs:    append([]string(nil), normalized.AdminPeerIDs...),
		ModeBProjectIDs: normalizeUintList(normalized.ModeBProjectIDs),
	}
}

func normalizeStoredNetBirdConfig(input netBirdStoredConfig) netBirdStoredConfig {
	return netBirdStoredConfig{
		APIBaseURL:      strings.TrimSpace(input.APIBaseURL),
		APIToken:        strings.TrimSpace(input.APIToken),
		HostPeerID:      strings.TrimSpace(input.HostPeerID),
		AdminPeerIDs:    normalizeStringList(input.AdminPeerIDs),
		ModeBProjectIDs: normalizeUintList(input.ModeBProjectIDs),
	}
}

func decodeStoredNetBirdConfig(secret, encrypted string) (netBirdStoredConfig, error) {
	blob, err := decodeSettingsSecureBlob(secret, encrypted)
	if err != nil {
		return netBirdStoredConfig{}, err
	}
	if blob.NetBird == nil {
		return netBirdStoredConfig{}, nil
	}
	return normalizeStoredNetBirdConfig(*blob.NetBird), nil
}

func encodeStoredNetBirdConfig(secret string, config netBirdStoredConfig) (string, error) {
	normalized := normalizeStoredNetBirdConfig(config)
	blob := settingsSecureBlob{}
	if !isEmptyNetBirdStoredConfig(normalized) {
		copyConfig := normalized
		blob.NetBird = &copyConfig
	}
	return encodeSettingsSecureBlob(secret, blob)
}
