package service

import (
	"context"
	"testing"

	"go-notes/internal/config"

	"github.com/stretchr/testify/require"
)

func TestDockerHealthReportsEnforcedNetworkGuardrailsByDefault(t *testing.T) {
	svc := NewHealthService(nil, nil, config.Config{
		DBHostPublishMode: "disabled",
		DockerNetworkMode: "enforced",
	})

	health := svc.Docker(context.Background())
	require.Equal(t, "error", health.Status)
	require.Equal(t, "host service unavailable", health.Detail)
	require.Equal(t, "disabled", health.DBHostPublish.Mode)
	require.False(t, health.DBHostPublish.Enabled)
	require.Equal(t, "enforced", health.NetworkGuardrails.Mode)
	require.True(t, health.NetworkGuardrails.ICCEnforced)
	require.Equal(t, "edge", health.NetworkGuardrails.EdgeNetwork)
	require.Equal(t, "core", health.NetworkGuardrails.CoreNetwork)
	require.False(t, health.NetworkGuardrails.Fallback)
	require.Empty(t, health.NetworkGuardrails.FallbackNotes)
}

func TestDockerHealthReportsCompatFallbackGuardrailMode(t *testing.T) {
	svc := NewHealthService(nil, nil, config.Config{
		DBHostPublishMode: "loopback",
		DBHostPublishHost: "127.0.0.1",
		DBHostPublishPort: 15432,
		DockerNetworkMode: "compat",
	})

	health := svc.Docker(context.Background())
	require.Equal(t, "error", health.Status)
	require.Equal(t, "loopback", health.DBHostPublish.Mode)
	require.True(t, health.DBHostPublish.Enabled)
	require.Equal(t, "127.0.0.1", health.DBHostPublish.Host)
	require.Equal(t, 15432, health.DBHostPublish.Port)
	require.Equal(t, "compat", health.NetworkGuardrails.Mode)
	require.False(t, health.NetworkGuardrails.ICCEnforced)
	require.Equal(t, "edge", health.NetworkGuardrails.EdgeNetwork)
	require.Equal(t, "core", health.NetworkGuardrails.CoreNetwork)
	require.True(t, health.NetworkGuardrails.Fallback)
	require.NotEmpty(t, health.NetworkGuardrails.FallbackNotes)
}
