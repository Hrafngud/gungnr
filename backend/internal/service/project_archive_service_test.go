package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-notes/internal/config"
	"go-notes/internal/infra/contract"
	"go-notes/internal/models"
	"go-notes/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResolveForwardLocalServiceExposureKeepsHostnameCleanupWithoutContainer(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(ForwardLocalRequest{
		Subdomain: "demo-metrics",
		Domain:    "example.com",
	})
	require.NoError(t, err)

	cleanup, ok := resolveForwardLocalServiceExposure(
		newArchiveExposureOwnershipContext("demo", nil),
		"example.com",
		models.Job{
			Model: gorm.Model{ID: 11},
			Input: string(input),
		},
		map[string]struct{}{},
	)
	require.True(t, ok)
	require.Equal(t, uint(11), cleanup.JobID)
	require.Equal(t, JobTypeForwardLocal, cleanup.Type)
	require.Equal(t, "demo-metrics.example.com", cleanup.Hostname)
	require.Equal(t, "", cleanup.Container)
	require.Equal(t, "subdomain.prefix", cleanup.Resolution)
}

func TestResolveForwardLocalServiceExposureAcceptsServiceHostnameUnderKnownProjectHostname(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(ForwardLocalRequest{
		Subdomain: "metrics",
		Domain:    "mock.example.com",
	})
	require.NoError(t, err)

	cleanup, ok := resolveForwardLocalServiceExposure(
		newArchiveExposureOwnershipContext("mock-service", []string{"mock.example.com"}),
		"example.com",
		models.Job{
			Model: gorm.Model{ID: 15},
			Input: string(input),
		},
		map[string]struct{}{},
	)
	require.True(t, ok)
	require.Equal(t, uint(15), cleanup.JobID)
	require.Equal(t, JobTypeForwardLocal, cleanup.Type)
	require.Equal(t, "metrics.mock.example.com", cleanup.Hostname)
	require.Equal(t, "", cleanup.Container)
	require.Equal(t, "hostname.scoped", cleanup.Resolution)
}

func TestResolveForwardLocalServiceExposureAcceptsKnownProjectHostnameExactWithoutAliasInjection(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(ForwardLocalRequest{
		Subdomain: "mock",
		Domain:    "example.com",
	})
	require.NoError(t, err)

	cleanup, ok := resolveForwardLocalServiceExposure(
		newArchiveExposureOwnershipContext("mock-service", []string{"mock.example.com"}),
		"example.com",
		models.Job{
			Model: gorm.Model{ID: 16},
			Input: string(input),
		},
		map[string]struct{}{},
	)
	require.True(t, ok)
	require.Equal(t, uint(16), cleanup.JobID)
	require.Equal(t, JobTypeForwardLocal, cleanup.Type)
	require.Equal(t, "mock.example.com", cleanup.Hostname)
	require.Equal(t, "hostname.exact", cleanup.Resolution)
}

func TestResolveForwardLocalServiceExposureDoesNotTreatNestedServiceHostnameAsProjectAlias(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(ForwardLocalRequest{
		Subdomain: "metrics-api",
		Domain:    "example.com",
	})
	require.NoError(t, err)

	warnings := map[string]struct{}{}
	_, ok := resolveForwardLocalServiceExposure(
		newArchiveExposureOwnershipContext("mock-service", []string{"mock.example.com", "metrics.mock.example.com"}),
		"example.com",
		models.Job{
			Model: gorm.Model{ID: 17},
			Input: string(input),
		},
		warnings,
	)
	require.False(t, ok)
	require.Empty(t, sortedArchiveWarnings(warnings))
}

func TestResolveQuickServiceExposureInternalOnlyKeepsContainerCleanupWithoutHostname(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(QuickServiceRequest{
		Subdomain:    "demo-app",
		ExposureMode: contract.QuickServiceExposureInternal,
	})
	require.NoError(t, err)

	cleanup, ok := resolveQuickServiceExposure(
		newArchiveExposureOwnershipContext("demo", nil),
		"example.com",
		models.Job{
			Model:    gorm.Model{ID: 42},
			Input:    string(input),
			LogLines: "quick-service policy: exposure=internal\nstarting docker container quick-demo-app (demo)\n",
		},
		map[string]struct{}{},
	)
	require.True(t, ok)
	require.Equal(t, uint(42), cleanup.JobID)
	require.Equal(t, JobTypeQuickService, cleanup.Type)
	require.Equal(t, "", cleanup.Hostname)
	require.Equal(t, "quick-demo-app", cleanup.Container)
	require.Equal(t, "subdomain.prefix.exposure.internal", cleanup.Resolution)
}

func TestResolveQuickServiceExposureInternalAcceptsScopedProjectHostnameForContainerCleanup(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(QuickServiceRequest{
		Subdomain:    "api",
		ExposureMode: contract.QuickServiceExposureInternal,
	})
	require.NoError(t, err)

	cleanup, ok := resolveQuickServiceExposure(
		newArchiveExposureOwnershipContext("mock-service", []string{"mock.example.com"}),
		"mock.example.com",
		models.Job{
			Model:    gorm.Model{ID: 44},
			Input:    string(input),
			LogLines: "quick-service policy: exposure=internal\nstarting docker container quick-mock-service-api (mock-service)\n",
		},
		map[string]struct{}{},
	)
	require.True(t, ok)
	require.Equal(t, uint(44), cleanup.JobID)
	require.Equal(t, JobTypeQuickService, cleanup.Type)
	require.Equal(t, "", cleanup.Hostname)
	require.Equal(t, "quick-mock-service-api", cleanup.Container)
	require.Equal(t, "hostname.scoped.exposure.internal", cleanup.Resolution)
}

func TestResolveQuickServiceExposurePublishedKeepsHostnameCleanup(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(QuickServiceRequest{
		Subdomain: "demo-app",
		Domain:    "example.com",
		Port:      18080,
	})
	require.NoError(t, err)

	cleanup, ok := resolveQuickServiceExposure(
		newArchiveExposureOwnershipContext("demo", nil),
		"example.com",
		models.Job{
			Model:    gorm.Model{ID: 7},
			Input:    string(input),
			LogLines: "starting docker container quick-demo-app (demo)\n",
		},
		map[string]struct{}{},
	)
	require.True(t, ok)
	require.Equal(t, "demo-app.example.com", cleanup.Hostname)
	require.Equal(t, "quick-demo-app", cleanup.Container)
	require.Equal(t, "subdomain.prefix.exposure.host_published", cleanup.Resolution)
}

func TestResolveQuickServiceExposurePublishedAcceptsKnownProjectHostnameScope(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(QuickServiceRequest{
		Subdomain: "api",
		Domain:    "mock.example.com",
		Port:      18080,
	})
	require.NoError(t, err)

	cleanup, ok := resolveQuickServiceExposure(
		newArchiveExposureOwnershipContext("mock-service", []string{"mock.example.com"}),
		"example.com",
		models.Job{
			Model:    gorm.Model{ID: 45},
			Input:    string(input),
			LogLines: "starting docker container quick-mock-service-api (mock-service)\n",
		},
		map[string]struct{}{},
	)
	require.True(t, ok)
	require.Equal(t, uint(45), cleanup.JobID)
	require.Equal(t, JobTypeQuickService, cleanup.Type)
	require.Equal(t, "api.mock.example.com", cleanup.Hostname)
	require.Equal(t, "quick-mock-service-api", cleanup.Container)
	require.Equal(t, "hostname.scoped.exposure.host_published", cleanup.Resolution)
}

func TestResolveForwardLocalServiceExposureWarnsForAmbiguousProjectOverlap(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(ForwardLocalRequest{
		Subdomain: "mockservice",
		Domain:    "example.com",
	})
	require.NoError(t, err)

	warnings := map[string]struct{}{}
	_, ok := resolveForwardLocalServiceExposure(
		newArchiveExposureOwnershipContext("mock", nil),
		"example.com",
		models.Job{
			Model: gorm.Model{ID: 19},
			Input: string(input),
		},
		warnings,
	)
	require.False(t, ok)
	require.Contains(t, sortedArchiveWarnings(warnings), "unresolved forward_local ownership for job 19 (subdomain=\"mockservice\"): deterministic project mapping is unavailable")
}

func TestProjectArchiveQueueCapturesExactPlannedIngressRules(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	configPath := filepath.Join(t.TempDir(), "config.yml")

	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(configPath, []byte(
		"ingress:\n"+
			"  - hostname: demo.example.com\n"+
			"    service: http://localhost:8080\n"+
			"  - hostname: demo.example.com\n"+
			"    service: http://localhost:9090\n"+
			"  - hostname: other.example.com\n"+
			"    service: http://localhost:7070\n"+
			"  - service: http_status:404\n",
	), 0o644))

	jobRepo := &archiveTestJobRepo{}
	service := NewProjectArchiveService(
		config.Config{
			TemplatesDir:      templatesDir,
			Domain:            "example.com",
			CloudflaredConfig: configPath,
		},
		&archiveTestProjectRepo{
			projects: []models.Project{{
				Name:   "demo",
				Path:   projectDir,
				Status: "running",
			}},
		},
		nil,
		NewJobService(jobRepo, nil),
		nil,
	)

	job, plan, err := service.Queue(context.Background(), "demo", DefaultProjectArchiveOptions(), ProjectArchiveActor{UserID: 7, Login: "tester"})
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Len(t, plan.Ingress, 2)

	var payload ProjectArchiveJobRequest
	require.NoError(t, json.Unmarshal([]byte(job.Input), &payload))
	require.Equal(t, []string{"demo.example.com"}, payload.Targets.Hostnames)
	require.ElementsMatch(t, []ProjectArchiveIngressDeleteTarget{
		{Hostname: "demo.example.com", Service: "http://localhost:8080", Source: "local"},
		{Hostname: "demo.example.com", Service: "http://localhost:9090", Source: "local"},
	}, payload.Targets.IngressRules)
}

type archiveTestProjectRepo struct {
	projects []models.Project
}

func (r *archiveTestProjectRepo) List(ctx context.Context) ([]models.Project, error) {
	return append([]models.Project(nil), r.projects...), nil
}

func (r *archiveTestProjectRepo) Create(ctx context.Context, project *models.Project) error {
	r.projects = append(r.projects, *project)
	return nil
}

func (r *archiveTestProjectRepo) GetByName(ctx context.Context, name string) (*models.Project, error) {
	for _, project := range r.projects {
		if project.Name == name {
			copied := project
			return &copied, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *archiveTestProjectRepo) Update(ctx context.Context, project *models.Project) error {
	for i := range r.projects {
		if r.projects[i].Name == project.Name {
			r.projects[i] = *project
			return nil
		}
	}
	return repository.ErrNotFound
}

type archiveTestJobRepo struct {
	jobs   []models.Job
	nextID uint
}

func (r *archiveTestJobRepo) List(ctx context.Context) ([]models.Job, error) {
	return append([]models.Job(nil), r.jobs...), nil
}

func (r *archiveTestJobRepo) ListPage(ctx context.Context, offset int, limit int) ([]models.Job, int64, error) {
	jobs := append([]models.Job(nil), r.jobs...)
	if offset >= len(jobs) {
		return []models.Job{}, int64(len(jobs)), nil
	}
	end := offset + limit
	if end > len(jobs) {
		end = len(jobs)
	}
	return jobs[offset:end], int64(len(jobs)), nil
}

func (r *archiveTestJobRepo) GetLatestByType(ctx context.Context, jobType string) (*models.Job, error) {
	for i := len(r.jobs) - 1; i >= 0; i-- {
		if r.jobs[i].Type == jobType {
			job := r.jobs[i]
			return &job, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *archiveTestJobRepo) GetLatestByTypeAndStatus(ctx context.Context, jobType string, status string) (*models.Job, error) {
	for i := len(r.jobs) - 1; i >= 0; i-- {
		if r.jobs[i].Type == jobType && r.jobs[i].Status == status {
			job := r.jobs[i]
			return &job, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *archiveTestJobRepo) Create(ctx context.Context, job *models.Job) error {
	r.nextID++
	job.ID = r.nextID
	r.jobs = append(r.jobs, *job)
	return nil
}

func (r *archiveTestJobRepo) Get(ctx context.Context, id uint) (*models.Job, error) {
	for _, job := range r.jobs {
		if job.ID == id {
			copied := job
			return &copied, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *archiveTestJobRepo) MarkRunning(ctx context.Context, id uint, startedAt time.Time) error {
	for i := range r.jobs {
		if r.jobs[i].ID == id {
			r.jobs[i].Status = "running"
			r.jobs[i].StartedAt = &startedAt
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *archiveTestJobRepo) MarkFinished(ctx context.Context, id uint, status string, finishedAt time.Time, errMsg string) error {
	for i := range r.jobs {
		if r.jobs[i].ID == id {
			r.jobs[i].Status = status
			r.jobs[i].FinishedAt = &finishedAt
			r.jobs[i].Error = errMsg
			return nil
		}
	}
	return repository.ErrNotFound
}

func (r *archiveTestJobRepo) AppendLog(ctx context.Context, id uint, line string) error {
	for i := range r.jobs {
		if r.jobs[i].ID == id {
			r.jobs[i].LogLines += line
			return nil
		}
	}
	return repository.ErrNotFound
}
