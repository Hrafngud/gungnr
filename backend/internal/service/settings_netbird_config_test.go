package service

import (
	"context"
	"testing"

	"go-notes/internal/config"
	"go-notes/internal/models"
	"go-notes/internal/repository"
)

func TestUpsertNetBirdModeConfig_PartialUpdatePreservesExistingFields(t *testing.T) {
	ctx := context.Background()
	repo := &fakeSettingsRepo{}
	svc := NewSettingsService(config.Config{SessionSecret: "test-session-secret"}, repo)

	apiBaseURL := "https://api.netbird.example"
	apiToken := "token-old"
	hostPeerID := "peer-host"
	adminPeerIDs := []string{"peer-admin-1", "peer-admin-2"}
	if _, err := svc.UpsertNetBirdModeConfig(ctx, NetBirdModeConfigUpdate{
		APIBaseURL:   &apiBaseURL,
		APIToken:     &apiToken,
		HostPeerID:   &hostPeerID,
		AdminPeerIDs: &adminPeerIDs,
	}); err != nil {
		t.Fatalf("seed upsert failed: %v", err)
	}

	rotatedToken := "token-rotated"
	updated, err := svc.UpsertNetBirdModeConfig(ctx, NetBirdModeConfigUpdate{
		APIToken: &rotatedToken,
	})
	if err != nil {
		t.Fatalf("partial upsert failed: %v", err)
	}

	if updated.APIBaseURL != apiBaseURL {
		t.Fatalf("expected apiBaseURL %q, got %q", apiBaseURL, updated.APIBaseURL)
	}
	if updated.HostPeerID != hostPeerID {
		t.Fatalf("expected hostPeerId %q, got %q", hostPeerID, updated.HostPeerID)
	}
	if len(updated.AdminPeerIDs) != 2 {
		t.Fatalf("expected 2 admin peer ids, got %d", len(updated.AdminPeerIDs))
	}
	if !updated.APITokenSet {
		t.Fatal("expected api token to remain set")
	}

	stored, _, err := svc.loadNetBirdStoredConfig(ctx)
	if err != nil {
		t.Fatalf("load stored config failed: %v", err)
	}
	if stored.APIToken != rotatedToken {
		t.Fatalf("expected rotated token %q, got %q", rotatedToken, stored.APIToken)
	}
	if stored.HostPeerID != hostPeerID {
		t.Fatalf("expected stored hostPeerId %q, got %q", hostPeerID, stored.HostPeerID)
	}
	if len(stored.AdminPeerIDs) != 2 {
		t.Fatalf("expected stored admin peers to be preserved, got %d", len(stored.AdminPeerIDs))
	}
}

type fakeSettingsRepo struct {
	settings *models.Settings
}

func (f *fakeSettingsRepo) Get(context.Context) (*models.Settings, error) {
	if f.settings == nil {
		return nil, repository.ErrNotFound
	}
	copy := *f.settings
	return &copy, nil
}

func (f *fakeSettingsRepo) Save(_ context.Context, settings *models.Settings) error {
	if settings == nil {
		f.settings = nil
		return nil
	}
	copy := *settings
	f.settings = &copy
	return nil
}
