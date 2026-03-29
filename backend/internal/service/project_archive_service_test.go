package service

import (
	"encoding/json"
	"testing"

	"go-notes/internal/infra/contract"
	"go-notes/internal/models"

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
		"demo",
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

func TestResolveQuickServiceExposureInternalOnlyKeepsContainerCleanupWithoutHostname(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(QuickServiceRequest{
		Subdomain:    "demo-app",
		ExposureMode: contract.QuickServiceExposureInternal,
	})
	require.NoError(t, err)

	cleanup, ok := resolveQuickServiceExposure(
		"demo",
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

func TestResolveQuickServiceExposurePublishedKeepsHostnameCleanup(t *testing.T) {
	t.Parallel()

	input, err := json.Marshal(QuickServiceRequest{
		Subdomain: "demo-app",
		Domain:    "example.com",
		Port:      18080,
	})
	require.NoError(t, err)

	cleanup, ok := resolveQuickServiceExposure(
		"demo",
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
