package service

import (
	"context"
	"testing"

	"go-notes/internal/config"
	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"

	"github.com/stretchr/testify/require"
)

func TestDockerHealthReportsEnforcedNetworkGuardrailsByDefault(t *testing.T) {
	svc := NewHealthService(nil, nil, config.Config{
		DBHostPublishMode:     "disabled",
		DockerNetworkMode:     "enforced",
		DockerDaemonIsolation: "disabled",
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
	require.Equal(t, "disabled", health.DaemonIsolation.Mode)
	require.Equal(t, "error", health.DaemonIsolation.PreflightStatus)
	require.Equal(t, "/var/run/docker.sock", health.DaemonIsolation.SocketPath)
	require.Equal(t, "disabled", health.DaemonIsolation.RollbackMode)
	require.NotEmpty(t, health.DaemonIsolation.RollbackSteps)
	require.Empty(t, health.Diagnostics)
}

func TestDockerHealthReportsCompatFallbackGuardrailMode(t *testing.T) {
	svc := NewHealthService(nil, nil, config.Config{
		DBHostPublishMode:     "loopback",
		DBHostPublishHost:     "127.0.0.1",
		DBHostPublishPort:     15432,
		DockerNetworkMode:     "compat",
		DockerDaemonIsolation: "rootless",
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
	require.Equal(t, "rootless", health.DaemonIsolation.Mode)
	require.Equal(t, "error", health.DaemonIsolation.PreflightStatus)
	require.Equal(t, "/var/run/docker.sock", health.DaemonIsolation.SocketPath)
	require.Empty(t, health.Diagnostics)
}

func TestDockerHealthReportsUsernsDaemonIsolationReadyWhenRuntimeMatches(t *testing.T) {
	bridge := &stubHostInfraBridgeClient{
		dockerRuntimeResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"server_version":   "27.5.1",
				"docker_root_dir":  "/var/lib/docker/231072.231072",
				"security_options": []string{"name=seccomp,profile=default", "name=userns"},
				"userns_remap":     true,
			},
		},
		listContainersResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					`{"ID":"a1","Names":"api"}`,
					`{"ID":"a2","Names":"db"}`,
				},
			},
		},
	}
	svc := NewHealthService(&HostService{infraClient: bridge}, nil, config.Config{
		DBHostPublishMode:     "disabled",
		DockerNetworkMode:     "enforced",
		DockerDaemonIsolation: "userns-remap",
	})

	health := svc.Docker(context.Background())
	require.Equal(t, "ok", health.Status)
	require.Equal(t, 2, health.Containers)
	require.Equal(t, "userns-remap", health.DaemonIsolation.Mode)
	require.Equal(t, "userns-remap", health.DaemonIsolation.ActiveMode)
	require.True(t, health.DaemonIsolation.Active)
	require.True(t, health.DaemonIsolation.Supported)
	require.Equal(t, "ready", health.DaemonIsolation.PreflightStatus)
	require.Equal(t, "27.5.1", health.DaemonIsolation.ServerVersion)
	require.Equal(t, "/var/lib/docker/231072.231072", health.DaemonIsolation.DockerRootDir)
	require.True(t, health.DaemonIsolation.UsernsRemap)
	require.False(t, health.DaemonIsolation.Rootless)
	require.Empty(t, health.DaemonIsolation.Blockers)
	require.Empty(t, health.Diagnostics)
}

func TestDockerHealthBlocksRootlessSelectionWhenDaemonDoesNotReportIt(t *testing.T) {
	bridge := &stubHostInfraBridgeClient{
		dockerRuntimeResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"server_version":   "27.5.1",
				"docker_root_dir":  "/var/lib/docker",
				"security_options": []string{"name=seccomp,profile=default"},
				"rootless":         false,
				"userns_remap":     false,
			},
		},
		listContainersResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					`{"ID":"a1","Names":"api"}`,
				},
			},
		},
	}
	svc := NewHealthService(&HostService{infraClient: bridge}, nil, config.Config{
		DBHostPublishMode:     "disabled",
		DockerNetworkMode:     "enforced",
		DockerDaemonIsolation: "rootless",
	})

	health := svc.Docker(context.Background())
	require.Equal(t, "ok", health.Status)
	require.Equal(t, "rootless", health.DaemonIsolation.Mode)
	require.Equal(t, "disabled", health.DaemonIsolation.ActiveMode)
	require.False(t, health.DaemonIsolation.Active)
	require.False(t, health.DaemonIsolation.Supported)
	require.Equal(t, "blocked", health.DaemonIsolation.PreflightStatus)
	require.NotEmpty(t, health.DaemonIsolation.Blockers)
	require.Contains(t, health.DaemonIsolation.Blockers[1], "/var/run/docker.sock")
	require.Empty(t, health.Diagnostics)
}

func TestDockerHealthDegradesWhenDockerRuntimeCheckFails(t *testing.T) {
	bridge := &stubHostInfraBridgeClient{
		dockerRuntimeErr: context.DeadlineExceeded,
	}
	svc := NewHealthService(&HostService{infraClient: bridge}, nil, config.Config{
		DBHostPublishMode:     "loopback",
		DBHostPublishHost:     "127.0.0.1",
		DBHostPublishPort:     15432,
		DockerNetworkMode:     "compat",
		DockerDaemonIsolation: "disabled",
	})

	health := svc.Docker(context.Background())
	require.Equal(t, "warning", health.Status)
	require.Equal(t, "Docker runtime visibility is unavailable; showing configured Docker posture only.", health.Detail)
	require.Equal(t, "loopback", health.DBHostPublish.Mode)
	require.Equal(t, "compat", health.NetworkGuardrails.Mode)
	require.Equal(t, "error", health.DaemonIsolation.PreflightStatus)
	require.Equal(t, []string{"live Docker runtime confirmation is unavailable"}, health.DaemonIsolation.Blockers)
	require.Contains(t, health.DaemonIsolation.Warnings, "failed to inspect docker runtime")
	require.Len(t, health.Diagnostics, 1)
	require.Equal(t, dockerReadDiagnosticScopeRuntime, health.Diagnostics[0].Scope)
	require.Equal(t, dockerReadDiagnosticStatusDegraded, health.Diagnostics[0].Status)
	require.Equal(t, dockerReadDiagnosticCodeRuntimeDegraded, health.Diagnostics[0].Code)
	require.Equal(t, string(errs.CodeHostDockerFailed), health.Diagnostics[0].SourceCode)
	require.Equal(t, string(contract.TaskTypeDockerRuntimeCheck), health.Diagnostics[0].TaskType)
}

func TestDockerHealthDegradesWhenContainerCountFails(t *testing.T) {
	bridge := &stubHostInfraBridgeClient{
		dockerRuntimeResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"server_version":  "27.5.1",
				"docker_root_dir": "/var/lib/docker",
			},
		},
		listContainersErr: context.DeadlineExceeded,
	}
	svc := NewHealthService(&HostService{infraClient: bridge}, nil, config.Config{
		DBHostPublishMode:     "disabled",
		DockerNetworkMode:     "enforced",
		DockerDaemonIsolation: "disabled",
	})

	health := svc.Docker(context.Background())
	require.Equal(t, "warning", health.Status)
	require.Equal(t, "Docker container inventory is unavailable; showing configured Docker posture only.", health.Detail)
	require.Equal(t, "disabled", health.DaemonIsolation.Mode)
	require.Equal(t, "ready", health.DaemonIsolation.PreflightStatus)
	require.Len(t, health.Diagnostics, 1)
	require.Equal(t, dockerReadDiagnosticScopeContainers, health.Diagnostics[0].Scope)
	require.Equal(t, dockerReadDiagnosticStatusDegraded, health.Diagnostics[0].Status)
	require.Equal(t, dockerReadDiagnosticCodeContainersDegraded, health.Diagnostics[0].Code)
	require.Equal(t, string(errs.CodeHostDockerFailed), health.Diagnostics[0].SourceCode)
	require.Equal(t, string(contract.TaskTypeDockerListContainers), health.Diagnostics[0].TaskType)
}
