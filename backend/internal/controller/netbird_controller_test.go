package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/auth"
	"go-notes/internal/config"
	"go-notes/internal/models"
	"go-notes/internal/repository"
	"go-notes/internal/service"
)

func TestApplyMode_InvalidModeDoesNotPersistInlineConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	settingsRepo := &fakeSettingsRepo{}
	settingsSvc := service.NewSettingsService(config.Config{SessionSecret: "test-session-secret"}, settingsRepo)

	initialBaseURL := "https://api.netbird.example"
	initialToken := "token-old"
	initialHostPeer := "peer-host-old"
	initialAdmins := []string{"peer-admin-old"}
	if _, err := settingsSvc.UpsertNetBirdModeConfig(context.Background(), service.NetBirdModeConfigUpdate{
		APIBaseURL:   &initialBaseURL,
		APIToken:     &initialToken,
		HostPeerID:   &initialHostPeer,
		AdminPeerIDs: &initialAdmins,
	}); err != nil {
		t.Fatalf("seed netbird mode config: %v", err)
	}

	controller := NewNetBirdController(
		&service.NetBirdService{},
		settingsSvc,
		service.NewJobService(&noopJobRepository{}, nil),
		nil,
	)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/netbird/mode/apply", strings.NewReader(`{
		"targetMode": "invalid_mode",
		"apiToken": "token-new",
		"hostPeerId": "peer-host-new",
		"adminPeerIds": ["peer-admin-new"]
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("session", auth.Session{
		UserID: 1,
		Login:  "admin",
		Role:   models.RoleAdmin,
	})

	controller.ApplyMode(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	resolved, _, err := settingsSvc.ResolveNetBirdModeApplyRequest(context.Background(), service.NetBirdModeApplyRequest{})
	if err != nil {
		t.Fatalf("resolve stored netbird config: %v", err)
	}
	if resolved.APIToken != initialToken {
		t.Fatalf("expected token to remain %q, got %q", initialToken, resolved.APIToken)
	}
	if resolved.HostPeerID != initialHostPeer {
		t.Fatalf("expected host peer to remain %q, got %q", initialHostPeer, resolved.HostPeerID)
	}
	if len(resolved.AdminPeerIDs) != 1 || resolved.AdminPeerIDs[0] != initialAdmins[0] {
		t.Fatalf("expected admin peers to remain %v, got %v", initialAdmins, resolved.AdminPeerIDs)
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

type noopJobRepository struct{}

func (*noopJobRepository) List(context.Context) ([]models.Job, error) { return []models.Job{}, nil }

func (*noopJobRepository) ListPage(context.Context, int, int) ([]models.Job, int64, error) {
	return []models.Job{}, 0, nil
}

func (*noopJobRepository) GetLatestByType(context.Context, string) (*models.Job, error) {
	return nil, repository.ErrNotFound
}

func (*noopJobRepository) GetLatestByTypeAndStatus(context.Context, string, string) (*models.Job, error) {
	return nil, repository.ErrNotFound
}

func (*noopJobRepository) Create(context.Context, *models.Job) error { return nil }

func (*noopJobRepository) Get(context.Context, uint) (*models.Job, error) {
	return nil, repository.ErrNotFound
}

func (*noopJobRepository) MarkRunning(context.Context, uint, time.Time) error { return nil }

func (*noopJobRepository) MarkFinished(context.Context, uint, string, time.Time, string) error {
	return nil
}

func (*noopJobRepository) AppendLog(context.Context, uint, string) error { return nil }
