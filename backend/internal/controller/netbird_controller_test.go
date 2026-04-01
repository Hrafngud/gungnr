package controller

import (
	"context"
	"encoding/json"
	"errors"
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

func TestNetBirdControllerMapsGatewayStyleFailuresToBadGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewNetBirdController(
		service.NewNetBirdService(config.Config{}, nil, &failingProjectRepo{err: errors.New("boom")}, nil),
		nil,
		nil,
		nil,
	)

	testCases := []struct {
		name         string
		path         string
		method       string
		body         string
		withAdmin    bool
		handler      func(*gin.Context)
		expectedCode string
	}{
		{
			name:         "status",
			path:         "/api/v1/netbird/status",
			method:       http.MethodGet,
			handler:      controller.Status,
			expectedCode: "NETBIRD-500-STATUS",
		},
		{
			name:         "graph",
			path:         "/api/v1/netbird/graph",
			method:       http.MethodGet,
			handler:      controller.Graph,
			expectedCode: "NETBIRD-500-ACL-GRAPH",
		},
		{
			name:         "plan",
			path:         "/api/v1/netbird/mode/plan",
			method:       http.MethodPost,
			body:         `{"targetMode":"legacy"}`,
			withAdmin:    true,
			handler:      controller.PlanMode,
			expectedCode: "NETBIRD-500-PLAN",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				ctx.Request.Header.Set("Content-Type", "application/json")
			}
			if tc.withAdmin {
				ctx.Set("session", auth.Session{
					UserID: 1,
					Login:  "admin",
					Role:   models.RoleAdmin,
				})
			}

			tc.handler(ctx)

			if recorder.Code != http.StatusBadGateway {
				t.Fatalf("expected status %d, got %d", http.StatusBadGateway, recorder.Code)
			}

			var payload map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload["code"] != tc.expectedCode {
				t.Fatalf("expected code %q, got %#v", tc.expectedCode, payload["code"])
			}
		})
	}
}

type fakeSettingsRepo struct {
	settings *models.Settings
}

type failingProjectRepo struct {
	err error
}

func (f *failingProjectRepo) List(context.Context) ([]models.Project, error) { return nil, f.err }

func (*failingProjectRepo) Create(context.Context, *models.Project) error { return nil }

func (*failingProjectRepo) GetByName(context.Context, string) (*models.Project, error) {
	return nil, repository.ErrNotFound
}

func (*failingProjectRepo) Update(context.Context, *models.Project) error { return nil }

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
