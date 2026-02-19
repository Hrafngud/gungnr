package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-notes/internal/config"
	"go-notes/internal/infra/contract"
	"go-notes/internal/integrations/cloudflare"

	"github.com/stretchr/testify/require"
)

type testWorkflowLogger struct{}

func (l *testWorkflowLogger) Log(_ string) {}

func (l *testWorkflowLogger) Logf(_ string, _ ...any) {}

type stubCloudflareWorkflowClient struct {
	dnsErr     error
	ingressErr error
}

func (s *stubCloudflareWorkflowClient) EnsureDNSForZone(_ context.Context, _ string, _ string) error {
	return s.dnsErr
}

func (s *stubCloudflareWorkflowClient) UpdateIngress(_ context.Context, _ string, _ int) error {
	return s.ingressErr
}

type stubInfraBridgeClient struct {
	result     contract.Result
	err        error
	called     bool
	requestID  string
	configPath string
	fn         func(ctx context.Context, requestID, configPath string) (contract.Result, error)
}

func (s *stubInfraBridgeClient) RestartTunnel(ctx context.Context, requestID, configPath string) (contract.Result, error) {
	s.called = true
	s.requestID = requestID
	s.configPath = configPath
	if s.fn != nil {
		return s.fn(ctx, requestID, configPath)
	}
	return s.result, s.err
}

func TestCloudflareSetupLocalTunnelBridgeSuccess(t *testing.T) {
	t.Parallel()

	configPath := writeCloudflaredConfigFixture(t)
	bridge := &stubInfraBridgeClient{
		result: contract.Result{
			Version:    contract.VersionV1,
			IntentID:   "intent-ok",
			RequestID:  "job-99",
			TaskType:   contract.TaskTypeRestartTunnel,
			Status:     contract.StatusSucceeded,
			CreatedAt:  time.Now().UTC().Add(-2 * time.Second),
			StartedAt:  time.Now().UTC().Add(-1 * time.Second),
			FinishedAt: time.Now().UTC(),
			LogPath:    filepath.Join(t.TempDir(), "worker.log"),
		},
	}
	workflows := &ProjectWorkflows{infraClient: bridge}
	cloudfl := &stubCloudflareWorkflowClient{ingressErr: cloudflare.ErrTunnelNotRemote}
	logger := &testWorkflowLogger{}

	err := workflows.cloudflareSetup(
		context.Background(),
		logger,
		config.Config{CloudflaredConfig: configPath},
		cloudfl,
		"job-99",
		"app.example.com",
		"example.com",
		"zone-1",
		8080,
	)
	require.NoError(t, err)
	require.True(t, bridge.called)
	require.Equal(t, "job-99", bridge.requestID)
	require.Equal(t, configPath, bridge.configPath)

	updated, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	require.Contains(t, string(updated), "app.example.com")
	require.Contains(t, string(updated), "http://localhost:8080")
}

func TestCloudflareSetupLocalTunnelBridgeFailureMapping(t *testing.T) {
	t.Parallel()

	configPath := writeCloudflaredConfigFixture(t)
	bridge := &stubInfraBridgeClient{
		result: contract.Result{
			Version:    contract.VersionV1,
			IntentID:   "intent-failed",
			RequestID:  "job-12",
			TaskType:   contract.TaskTypeRestartTunnel,
			Status:     contract.StatusFailed,
			CreatedAt:  time.Now().UTC().Add(-2 * time.Second),
			StartedAt:  time.Now().UTC().Add(-1 * time.Second),
			FinishedAt: time.Now().UTC(),
			LogPath:    filepath.Join(t.TempDir(), "worker.log"),
			Error: &contract.Error{
				Code:    "TUNNEL-500",
				Message: "restart failed on host worker",
			},
		},
	}
	workflows := &ProjectWorkflows{infraClient: bridge}
	cloudfl := &stubCloudflareWorkflowClient{ingressErr: cloudflare.ErrTunnelNotRemote}
	logger := &testWorkflowLogger{}

	err := workflows.cloudflareSetup(
		context.Background(),
		logger,
		config.Config{CloudflaredConfig: configPath},
		cloudfl,
		"job-12",
		"app.example.com",
		"example.com",
		"zone-1",
		8080,
	)
	require.Error(t, err)
	require.True(t, bridge.called)
	require.True(t, strings.Contains(err.Error(), "TUNNEL-500"))
	require.True(t, strings.Contains(err.Error(), "restart failed on host worker"))
}

func TestCloudflareSetupLocalTunnelBridgeTimeoutFails(t *testing.T) {
	t.Parallel()

	configPath := writeCloudflaredConfigFixture(t)
	bridge := &stubInfraBridgeClient{
		err: context.DeadlineExceeded,
	}
	workflows := &ProjectWorkflows{infraClient: bridge}
	cloudfl := &stubCloudflareWorkflowClient{ingressErr: cloudflare.ErrTunnelNotRemote}
	logger := &testWorkflowLogger{}

	err := workflows.cloudflareSetup(
		context.Background(),
		logger,
		config.Config{CloudflaredConfig: configPath},
		cloudfl,
		"job-77",
		"app.example.com",
		"example.com",
		"zone-1",
		8080,
	)
	require.Error(t, err)
	require.True(t, bridge.called)
	require.Contains(t, err.Error(), "cloudflared restart")
}

func TestCloudflareSetupLocalTunnelBridgeUsesCallerDeadline(t *testing.T) {
	t.Parallel()

	configPath := writeCloudflaredConfigFixture(t)
	bridge := &stubInfraBridgeClient{
		fn: func(ctx context.Context, _ string, _ string) (contract.Result, error) {
			deadline, ok := ctx.Deadline()
			require.True(t, ok)

			remaining := time.Until(deadline)
			require.Greater(t, remaining, time.Duration(0))
			require.Greater(t, remaining, time.Second)
			require.LessOrEqual(t, remaining, 2500*time.Millisecond)

			return contract.Result{}, context.DeadlineExceeded
		},
	}
	workflows := &ProjectWorkflows{infraClient: bridge}
	cloudfl := &stubCloudflareWorkflowClient{ingressErr: cloudflare.ErrTunnelNotRemote}
	logger := &testWorkflowLogger{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := workflows.cloudflareSetup(
		ctx,
		logger,
		config.Config{CloudflaredConfig: configPath},
		cloudfl,
		"job-88",
		"app.example.com",
		"example.com",
		"zone-1",
		8080,
	)
	require.Error(t, err)
	require.True(t, bridge.called)
}

func TestRestartTunnelLikelyIPv6LoopbackIssue(t *testing.T) {
	t.Parallel()

	err := errors.New("dial tcp [::1]:90: connect: connection refused")
	result := contract.Result{
		LogTail: []string{
			"ERR Unable to reach the origin service: dial tcp [::1]:90: connect: connection refused",
		},
	}

	require.True(t, restartTunnelLikelyIPv6LoopbackIssue(err, result))
}

func TestOwnershipCandidatesExpandsCloudflaredTildePath(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	require.NotEmpty(t, home)

	candidates := ownershipCandidates("/templates", "~/.cloudflared/config.yml")
	require.Contains(t, candidates, filepath.Clean("/templates"))
	require.Contains(t, candidates, filepath.Join(home, ".cloudflared"))
}

func writeCloudflaredConfigFixture(t *testing.T) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yml")
	content := "ingress:\n  - service: http_status:404\n"
	err := os.WriteFile(configPath, []byte(content), 0o644)
	require.NoError(t, err)
	return configPath
}
