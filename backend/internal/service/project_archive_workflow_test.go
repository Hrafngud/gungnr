package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go-notes/internal/config"
	"go-notes/internal/integrations/cloudflare"
	"go-notes/internal/jobs"
	"go-notes/internal/models"

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
		cfg:      config.Config{CloudflaredConfig: configPath},
		projects: projects,
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
	require.Contains(t, logger.lines, "removed 1 local ingress rules")
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
