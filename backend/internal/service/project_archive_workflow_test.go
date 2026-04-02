package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-notes/internal/config"
	"go-notes/internal/infra/contract"
	"go-notes/internal/integrations/cloudflare"
	"go-notes/internal/jobs"
	"go-notes/internal/models"
	"go-notes/internal/repository"

	"github.com/stretchr/testify/require"
)

func TestNormalizeIngressDeleteTargetsPreservesDuplicates(t *testing.T) {
	t.Parallel()

	targets := normalizeIngressDeleteTargets([]ProjectArchiveIngressDeleteTarget{
		{Hostname: " App.Example.com ", Service: "http://localhost:8080", Source: "local"},
		{Hostname: "app.example.com", Service: "http://localhost:8080", Source: "local"},
		{Hostname: "app.example.com", Service: "http://localhost:9090", Source: "local"},
	})

	require.Len(t, targets, 3)
	require.Equal(t, ProjectArchiveIngressDeleteTarget{Hostname: "app.example.com", Service: "http://localhost:8080", Source: "local"}, targets[0])
	require.Equal(t, ProjectArchiveIngressDeleteTarget{Hostname: "app.example.com", Service: "http://localhost:8080", Source: "local"}, targets[1])
	require.Equal(t, ProjectArchiveIngressDeleteTarget{Hostname: "app.example.com", Service: "http://localhost:9090", Source: "local"}, targets[2])
}

func TestHandleProjectArchiveRemovesLocalIngressRulesWhenOnlyLocalTargetsArePlanned(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	configPath := filepath.Join(t.TempDir(), "config.yml")

	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(
		"ingress:\n"+
			"  - hostname: app.example.com\n"+
			"    service: http://localhost:8080\n"+
			"  - hostname: other.example.com\n"+
			"    service: http://localhost:7070\n"+
			"  - service: http_status:404\n",
	), 0o644))

	projects := &archiveTestProjectRepo{
		projects: []models.Project{{
			Name:   "demo",
			Path:   projectDir,
			Status: "running",
		}},
	}

	req := ProjectArchiveJobRequest{
		Project: "demo",
		Options: ProjectArchiveOptions{
			RemoveContainers: false,
			RemoveVolumes:    false,
			RemoveIngress:    true,
			RemoveDNS:        false,
		},
		Targets: ProjectArchiveTargets{
			Hostnames: []string{"app.example.com"},
			IngressRules: []ProjectArchiveIngressDeleteTarget{
				{Hostname: "app.example.com", Service: "http://localhost:8080", Source: "local"},
			},
		},
	}
	payload, err := json.Marshal(req)
	require.NoError(t, err)

	workflows := &ProjectWorkflows{
		cfg:         config.Config{CloudflaredConfig: configPath},
		projects:    projects,
		infraClient: &archiveTestProjectInfraClient{},
	}

	logger := &archiveTestLogger{}
	err = workflows.handleProjectArchive(context.Background(), models.Job{
		Input: string(payload),
	}, logger)
	require.NoError(t, err)

	rules, err := cloudflare.ListLocalIngressRules(configPath)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	require.Equal(t, "other.example.com", rules[0].Hostname)
	require.Equal(t, "http://localhost:7070", rules[0].Service)
	requireArchiveLogContains(t, logger.lines, "archive step containers: result=skipped")
	requireArchiveLogContains(t, logger.lines, "archive step ingress: start remove_ingress=true")
	requireArchiveLogContains(t, logger.lines, "archive step ingress: result=completed")
	requireArchiveLogContains(t, logger.lines, "archive step dns: result=skipped")
	requireArchiveLogContains(t, logger.lines, "archive step status_audit: result=completed")
	requireArchiveLogContains(t, logger.lines, "archive completion summary: outcome=completed")
	require.Contains(t, logger.lines, "removed 1 local ingress rules")
}

func TestHandleProjectArchiveLogsPartialFailureCompletionSummary(t *testing.T) {
	t.Parallel()

	req := ProjectArchiveJobRequest{
		Project: "demo",
		Options: ProjectArchiveOptions{
			RemoveContainers: true,
			RemoveVolumes:    false,
			RemoveIngress:    false,
			RemoveDNS:        false,
		},
		Targets: ProjectArchiveTargets{
			Containers:         []string{"demo-web"},
			ExposureContainers: []string{"demo-web"},
		},
	}
	payload, err := json.Marshal(req)
	require.NoError(t, err)

	workflows := &ProjectWorkflows{
		projects: &archiveTestProjectRepo{
			projects: []models.Project{{
				Name:   "demo",
				Path:   "/tmp/demo",
				Status: "running",
			}},
		},
	}

	logger := &archiveTestLogger{}
	err = workflows.handleProjectArchive(context.Background(), models.Job{
		Input: string(payload),
	}, logger)
	require.NoError(t, err)

	requireArchiveLogContains(t, logger.lines, "archive step containers: start remove_containers=true")
	requireArchiveLogContains(t, logger.lines, "archive step containers: result=partial_failure")
	requireArchiveLogContains(t, logger.lines, "archive step ingress: result=skipped")
	requireArchiveLogContains(t, logger.lines, "archive step dns: result=skipped")
	requireArchiveLogContains(t, logger.lines, "archive step status_audit: result=completed")
	requireArchiveLogContains(t, logger.lines, "archive completion summary: outcome=partial_failure warnings=1 steps=containers:partial_failure ingress:skipped dns:skipped status_audit:completed")
	requireArchiveLogContains(t, logger.lines, "archive completed with partial failures for project demo (warning_count=1)")
	requireArchiveLogContains(t, logger.lines, "warning: host service unavailable while removing project containers")
}

func TestHandleProjectArchivePersistsCompletionSummaryMetadata(t *testing.T) {
	t.Parallel()

	templatesDir := t.TempDir()
	projectDir := filepath.Join(templatesDir, "demo")
	configPath := filepath.Join(t.TempDir(), "config.yml")

	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(
		"ingress:\n"+
			"  - hostname: app.example.com\n"+
			"    service: http://localhost:8080\n"+
			"  - service: http_status:404\n",
	), 0o644))

	auditRepo := &archiveTestAuditRepo{}
	req := ProjectArchiveJobRequest{
		Project: "demo",
		Options: ProjectArchiveOptions{
			RemoveContainers: false,
			RemoveVolumes:    false,
			RemoveIngress:    true,
			RemoveDNS:        false,
		},
		RequestedBy: ProjectArchiveActor{
			UserID: 7,
			Login:  "tester",
		},
		Targets: ProjectArchiveTargets{
			Hostnames: []string{"app.example.com"},
			IngressRules: []ProjectArchiveIngressDeleteTarget{
				{Hostname: "app.example.com", Service: "http://localhost:8080", Source: "local"},
			},
		},
	}
	payload, err := json.Marshal(req)
	require.NoError(t, err)

	workflows := &ProjectWorkflows{
		cfg: config.Config{CloudflaredConfig: configPath},
		projects: &archiveTestProjectRepo{
			projects: []models.Project{{
				Name:   "demo",
				Path:   projectDir,
				Status: "running",
			}},
		},
		audit:       NewAuditService(auditRepo),
		infraClient: &archiveTestProjectInfraClient{},
	}

	logger := &archiveTestLogger{}
	err = workflows.handleProjectArchive(context.Background(), models.Job{
		Input: string(payload),
	}, logger)
	require.NoError(t, err)
	require.Len(t, auditRepo.entries, 1)
	requireArchiveLogContains(t, logger.lines, "archive completion summary: outcome=completed")

	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(auditRepo.entries[0].Metadata), &metadata))

	completionSummary, ok := metadata["completionSummary"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "completed", completionSummary["outcome"])
	require.EqualValues(t, 0, completionSummary["warningCount"])

	steps, ok := completionSummary["steps"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "skipped", archiveStepMetadataStatus(t, steps, "containers"))
	require.Equal(t, "completed", archiveStepMetadataStatus(t, steps, "ingress"))
	require.Equal(t, "skipped", archiveStepMetadataStatus(t, steps, "dns"))
	require.Equal(t, "completed", archiveStepMetadataStatus(t, steps, "statusAudit"))
}

func TestHandleProjectArchiveCountsAuditWarningInCompletionSummary(t *testing.T) {
	t.Parallel()

	auditRepo := &archiveTestAuditRepo{createErr: fmt.Errorf("audit unavailable")}
	req := ProjectArchiveJobRequest{
		Project: "demo",
		Options: ProjectArchiveOptions{
			RemoveContainers: false,
			RemoveVolumes:    false,
			RemoveIngress:    false,
			RemoveDNS:        false,
		},
		RequestedBy: ProjectArchiveActor{
			UserID: 7,
			Login:  "tester",
		},
	}
	payload, err := json.Marshal(req)
	require.NoError(t, err)

	workflows := &ProjectWorkflows{
		projects: &archiveTestProjectRepo{
			projects: []models.Project{{
				Name:   "demo",
				Path:   "/tmp/demo",
				Status: "running",
			}},
		},
		audit: NewAuditService(auditRepo),
	}

	logger := &archiveTestLogger{}
	err = workflows.handleProjectArchive(context.Background(), models.Job{
		Input: string(payload),
	}, logger)
	require.NoError(t, err)

	requireArchiveLogContains(t, logger.lines, "audit warning: failed to write archive completion event: audit unavailable")
	requireArchiveLogContains(t, logger.lines, "archive step status_audit: result=partial_failure")
	requireArchiveLogContains(t, logger.lines, "archive completion summary: outcome=partial_failure warnings=1 steps=containers:skipped ingress:skipped dns:skipped status_audit:partial_failure")
	requireArchiveLogContains(t, logger.lines, "archive completed with partial failures for project demo (warning_count=1)")
}

type archiveTestLogger struct {
	lines []string
}

var _ jobs.Logger = (*archiveTestLogger)(nil)

func (l *archiveTestLogger) Log(line string) {
	if line == "" {
		return
	}
	l.lines = append(l.lines, line)
}

func (l *archiveTestLogger) Logf(format string, args ...any) {
	l.Log(fmt.Sprintf(format, args...))
}

type archiveTestAuditRepo struct {
	entries   []models.AuditLog
	createErr error
}

func (r *archiveTestAuditRepo) List(ctx context.Context, limit int) ([]models.AuditLog, error) {
	return append([]models.AuditLog(nil), r.entries...), nil
}

func (r *archiveTestAuditRepo) Create(ctx context.Context, entry *models.AuditLog) error {
	if entry == nil {
		return fmt.Errorf("audit entry is nil")
	}
	if r.createErr != nil {
		return r.createErr
	}
	r.entries = append(r.entries, *entry)
	return nil
}

var _ repository.AuditLogRepository = (*archiveTestAuditRepo)(nil)

type archiveTestProjectInfraClient struct{}

var _ infraBridgeClient = (*archiveTestProjectInfraClient)(nil)

func (c *archiveTestProjectInfraClient) HostListenTCPPorts(ctx context.Context, requestID string) (contract.Result, error) {
	return contract.Result{}, fmt.Errorf("not implemented")
}

func (c *archiveTestProjectInfraClient) DockerPublishedPorts(ctx context.Context, requestID string) (contract.Result, error) {
	return contract.Result{}, fmt.Errorf("not implemented")
}

func (c *archiveTestProjectInfraClient) RestartTunnel(ctx context.Context, requestID, configPath string) (contract.Result, error) {
	return contract.Result{
		IntentID: "intent-restart-tunnel",
		Status:   contract.StatusSucceeded,
	}, nil
}

func requireArchiveLogContains(t *testing.T, lines []string, needle string) {
	t.Helper()
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return
		}
	}
	require.Failf(t, "missing archive log line", "expected log containing %q in %#v", needle, lines)
}

func archiveStepMetadataStatus(t *testing.T, steps map[string]any, key string) string {
	t.Helper()
	rawStep, ok := steps[key].(map[string]any)
	require.True(t, ok, "missing step metadata for %s", key)
	status, ok := rawStep["status"].(string)
	require.True(t, ok, "missing status for step %s", key)
	return status
}
