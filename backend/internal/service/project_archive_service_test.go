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
