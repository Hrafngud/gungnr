package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"go-notes/internal/config"
	"go-notes/internal/models"
	"go-notes/internal/repository"
	"gorm.io/gorm"
)

func TestFetchNetBirdLiveStatus_PartialDecodeFailureKeepsConnectivity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/peers":
			_, _ = w.Write([]byte(`[{"id":"peer-host","name":"host","ip":"100.64.0.10","connected":true}]`))
		case "/api/groups":
			// Valid JSON but incompatible schema for Group decode to trigger warning path.
			_, _ = w.Write([]byte(`[{"id":"group-1","name":"gungnr-panel","peers":123}]`))
		case "/api/policies":
			// Valid JSON but incompatible schema for Policy decode to trigger warning path.
			_, _ = w.Write([]byte(`[{"id":"policy-1","name":"gungnr-test","enabled":true,"rules":"bad-type"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	status, err := fetchNetBirdLiveStatus(context.Background(), server.URL, "token", "peer-host")
	if err != nil {
		t.Fatalf("fetchNetBirdLiveStatus returned error: %v", err)
	}

	if !status.ClientInstalled {
		t.Fatal("expected clientInstalled to be true")
	}
	if !status.DaemonRunning {
		t.Fatal("expected daemonRunning to be true")
	}
	if !status.Connected {
		t.Fatal("expected connected to be true")
	}
	if status.PeerID != "peer-host" {
		t.Fatalf("expected peer id peer-host, got %q", status.PeerID)
	}
	if status.GroupsKnown {
		t.Fatal("expected groupsKnown to be false due decode warning")
	}
	if status.PoliciesKnown {
		t.Fatal("expected policiesKnown to be false due decode warning")
	}
	if len(status.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(status.Warnings))
	}
}

func TestFetchNetBirdLiveStatus_DisconnectedRecentHeartbeatInfersDaemonRunning(t *testing.T) {
	now := time.Now().UTC()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/peers":
			_, _ = w.Write([]byte(`[{"id":"peer-host","name":"host","ip":"100.64.0.10","connected":false,"last_seen":"` + now.Format(time.RFC3339Nano) + `"}]`))
		case "/api/groups":
			_, _ = w.Write([]byte(`[]`))
		case "/api/policies":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	status, err := fetchNetBirdLiveStatus(context.Background(), server.URL, "token", "peer-host")
	if err != nil {
		t.Fatalf("fetchNetBirdLiveStatus returned error: %v", err)
	}
	if status.Connected {
		t.Fatal("expected connected to be false")
	}
	if !status.DaemonRunning {
		t.Fatal("expected daemonRunning to be inferred true from recent heartbeat")
	}
	if !containsWarning(status.Warnings, "heartbeat is recent") {
		t.Fatalf("expected heartbeat warning, got %v", status.Warnings)
	}
}

func TestFetchNetBirdLiveStatus_DisconnectedStaleHeartbeatMarksDaemonOffline(t *testing.T) {
	stale := time.Now().UTC().Add(-15 * time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/peers":
			_, _ = w.Write([]byte(`[{"id":"peer-host","name":"host","ip":"100.64.0.10","connected":false,"last_seen":"` + stale.Format(time.RFC3339Nano) + `"}]`))
		case "/api/groups":
			_, _ = w.Write([]byte(`[]`))
		case "/api/policies":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	status, err := fetchNetBirdLiveStatus(context.Background(), server.URL, "token", "peer-host")
	if err != nil {
		t.Fatalf("fetchNetBirdLiveStatus returned error: %v", err)
	}
	if status.DaemonRunning {
		t.Fatal("expected daemonRunning to be false for stale heartbeat")
	}
	if !containsWarning(status.Warnings, "heartbeat is stale") {
		t.Fatalf("expected stale heartbeat warning, got %v", status.Warnings)
	}
}

func TestStatus_AuthFailureUsesLastKnownSuccessfulConnectivity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"invalid token","code":404}`))
	}))
	defer server.Close()

	now := time.Now().UTC()
	latestFailed := models.Job{
		Model:  gorm.Model{ID: 2, CreatedAt: now},
		Type:   JobTypeNetBirdModeApply,
		Status: "failed",
		Input:  `{"targetMode":"mode_a","allowLocalhost":false,"apiBaseUrl":"` + server.URL + `","apiToken":"bad-token","hostPeerId":"peer-failed","adminPeerIds":["peer-failed"]}`,
	}
	previousSuccess := models.Job{
		Model:  gorm.Model{ID: 1, CreatedAt: now.Add(-time.Minute)},
		Type:   JobTypeNetBirdModeApply,
		Status: "completed",
		Input:  `{"targetMode":"mode_a","allowLocalhost":false,"apiBaseUrl":"` + server.URL + `","apiToken":"good-token","hostPeerId":"peer-last-good","adminPeerIds":["peer-last-good"]}`,
	}

	svc := &NetBirdService{
		cfg: config.Config{
			Port:        "8080",
			NetBirdMode: string(NetBirdModeA),
		},
		projects: fakeNetBirdProjectRepo{},
		jobs: &fakeNetBirdJobRepo{
			jobs: []models.Job{latestFailed, previousSuccess},
		},
	}

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.LastPolicySyncStatus != netBirdSyncStatusFailed {
		t.Fatalf("expected last sync status %q, got %q", netBirdSyncStatusFailed, status.LastPolicySyncStatus)
	}
	if !status.ClientInstalled {
		t.Fatal("expected clientInstalled to remain true")
	}
	if !status.DaemonRunning {
		t.Fatal("expected daemonRunning to use last known successful connectivity")
	}
	if !status.Connected {
		t.Fatal("expected connected to use last known successful connectivity")
	}
	if status.PeerID != "peer-last-good" {
		t.Fatalf("expected fallback peer id peer-last-good, got %q", status.PeerID)
	}
	if !strings.Contains(strings.ToLower(status.APIReachability.Message), "invalid token") {
		t.Fatalf("expected API reachability message to include invalid token details, got %q", status.APIReachability.Message)
	}
	if !containsWarning(status.Warnings, "authentication failed") {
		t.Fatalf("expected warnings to mention auth fallback, got %v", status.Warnings)
	}
}

func TestStatus_UsesLastSuccessfulApplyAsRuntimeMode(t *testing.T) {
	now := time.Now().UTC()
	success := models.Job{
		Model:  gorm.Model{ID: 7, CreatedAt: now},
		Type:   JobTypeNetBirdModeApply,
		Status: "completed",
		Input:  `{"targetMode":"mode_a","allowLocalhost":false,"apiToken":""}`,
	}

	svc := &NetBirdService{
		cfg: config.Config{
			Port:                  "8080",
			NetBirdMode:           string(NetBirdModeLegacy),
			NetBirdAllowLocalhost: true,
		},
		projects: fakeNetBirdProjectRepo{},
		jobs: &fakeNetBirdJobRepo{
			jobs: []models.Job{success},
		},
	}

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.CurrentMode != NetBirdModeA {
		t.Fatalf("expected runtime currentMode %q, got %q", NetBirdModeA, status.CurrentMode)
	}
	if status.ConfiguredMode != NetBirdModeLegacy {
		t.Fatalf("expected configuredMode %q, got %q", NetBirdModeLegacy, status.ConfiguredMode)
	}
	if status.ModeSource != netBirdModeSourceLastSuccessfulSync {
		t.Fatalf("expected mode source %q, got %q", netBirdModeSourceLastSuccessfulSync, status.ModeSource)
	}
	if !status.ModeDrift {
		t.Fatal("expected mode drift to be true")
	}
	if !containsWarning(status.Warnings, "differs from the latest successful apply") {
		t.Fatalf("expected drift warning, got %v", status.Warnings)
	}
}

func TestACLGraph_UsesLastSuccessfulApplyAsRuntimeMode(t *testing.T) {
	now := time.Now().UTC()
	success := models.Job{
		Model:  gorm.Model{ID: 8, CreatedAt: now},
		Type:   JobTypeNetBirdModeApply,
		Status: "completed",
		Input:  `{"targetMode":"mode_a","allowLocalhost":false,"apiToken":""}`,
	}

	svc := &NetBirdService{
		cfg: config.Config{
			Port:        "8080",
			NetBirdMode: string(NetBirdModeLegacy),
		},
		projects: fakeNetBirdProjectRepo{},
		jobs: &fakeNetBirdJobRepo{
			jobs: []models.Job{success},
		},
	}

	graph, err := svc.ACLGraph(context.Background())
	if err != nil {
		t.Fatalf("ACLGraph returned error: %v", err)
	}
	if graph.CurrentMode != NetBirdModeA {
		t.Fatalf("expected runtime currentMode %q, got %q", NetBirdModeA, graph.CurrentMode)
	}
	if graph.ConfiguredMode != NetBirdModeLegacy {
		t.Fatalf("expected configuredMode %q, got %q", NetBirdModeLegacy, graph.ConfiguredMode)
	}
	if graph.ModeSource != netBirdModeSourceLastSuccessfulSync {
		t.Fatalf("expected mode source %q, got %q", netBirdModeSourceLastSuccessfulSync, graph.ModeSource)
	}
	if !graph.ModeDrift {
		t.Fatal("expected mode drift to be true")
	}
}

type fakeNetBirdProjectRepo struct{}

func (fakeNetBirdProjectRepo) List(context.Context) ([]models.Project, error) {
	return []models.Project{}, nil
}
func (fakeNetBirdProjectRepo) Create(context.Context, *models.Project) error { return nil }
func (fakeNetBirdProjectRepo) GetByName(context.Context, string) (*models.Project, error) {
	return nil, repository.ErrNotFound
}
func (fakeNetBirdProjectRepo) Update(context.Context, *models.Project) error { return nil }

type fakeNetBirdJobRepo struct {
	jobs                     []models.Job
	listErr                  error
	latestByTypeAndStatusErr error
	listCalls                int
	latestByTypeStatusCalls  int
}

func (f *fakeNetBirdJobRepo) List(context.Context) ([]models.Job, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	jobs := append([]models.Job(nil), f.jobs...)
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].CreatedAt.Equal(jobs[j].CreatedAt) {
			return jobs[i].ID > jobs[j].ID
		}
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return jobs, nil
}

func (f *fakeNetBirdJobRepo) ListPage(context.Context, int, int) ([]models.Job, int64, error) {
	jobs, err := f.List(context.Background())
	return jobs, int64(len(jobs)), err
}

func (f *fakeNetBirdJobRepo) GetLatestByType(_ context.Context, jobType string) (*models.Job, error) {
	jobs, _ := f.List(context.Background())
	for _, job := range jobs {
		if strings.TrimSpace(job.Type) != strings.TrimSpace(jobType) {
			continue
		}
		copy := job
		return &copy, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeNetBirdJobRepo) GetLatestByTypeAndStatus(_ context.Context, jobType string, status string) (*models.Job, error) {
	f.latestByTypeStatusCalls++
	if f.latestByTypeAndStatusErr != nil {
		return nil, f.latestByTypeAndStatusErr
	}
	jobs := append([]models.Job(nil), f.jobs...)
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].CreatedAt.Equal(jobs[j].CreatedAt) {
			return jobs[i].ID > jobs[j].ID
		}
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	for _, job := range jobs {
		if strings.TrimSpace(job.Type) != strings.TrimSpace(jobType) {
			continue
		}
		if strings.TrimSpace(job.Status) != strings.TrimSpace(status) {
			continue
		}
		copy := job
		return &copy, nil
	}
	return nil, repository.ErrNotFound
}

func (*fakeNetBirdJobRepo) Create(context.Context, *models.Job) error { return nil }
func (f *fakeNetBirdJobRepo) Get(_ context.Context, id uint) (*models.Job, error) {
	for _, job := range f.jobs {
		if job.ID != id {
			continue
		}
		copy := job
		return &copy, nil
	}
	return nil, repository.ErrNotFound
}
func (*fakeNetBirdJobRepo) MarkRunning(context.Context, uint, time.Time) error { return nil }
func (*fakeNetBirdJobRepo) MarkFinished(context.Context, uint, string, time.Time, string) error {
	return nil
}
func (*fakeNetBirdJobRepo) AppendLog(context.Context, uint, string) error { return nil }

func containsWarning(warnings []string, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	for _, warning := range warnings {
		if strings.Contains(strings.ToLower(warning), needle) {
			return true
		}
	}
	return false
}

func TestLatestSuccessfulModeApplySnapshot_UsesFilteredLookup(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeNetBirdJobRepo{
		jobs: []models.Job{
			{
				Model:  gorm.Model{ID: 10, CreatedAt: now.Add(-time.Minute)},
				Type:   JobTypeNetBirdModeApply,
				Status: "completed",
				Input:  `{"targetMode":"mode_a","allowLocalhost":false}`,
			},
			{
				Model:  gorm.Model{ID: 11, CreatedAt: now},
				Type:   JobTypeNetBirdModeApply,
				Status: "failed",
				Input:  `{"targetMode":"mode_b","allowLocalhost":false}`,
			},
		},
		listErr: errors.New("full list should not be called"),
	}

	svc := &NetBirdService{
		cfg:      config.Config{},
		projects: fakeNetBirdProjectRepo{},
		jobs:     repo,
	}

	snapshot, err := svc.latestSuccessfulModeApplySnapshot(context.Background())
	if err != nil {
		t.Fatalf("latestSuccessfulModeApplySnapshot returned error: %v", err)
	}
	if !snapshot.Found {
		t.Fatal("expected successful snapshot to be found")
	}
	if snapshot.Job.ID != 10 {
		t.Fatalf("expected latest successful job id 10, got %d", snapshot.Job.ID)
	}
	if repo.latestByTypeStatusCalls != 1 {
		t.Fatalf("expected exactly one filtered lookup call, got %d", repo.latestByTypeStatusCalls)
	}
	if repo.listCalls != 0 {
		t.Fatalf("expected no full list calls, got %d", repo.listCalls)
	}
}
