package service

import (
	"context"
	"time"

	"go-notes/internal/infra/contract"

	"github.com/stretchr/testify/require"
	"testing"
)

type stubDockerRunnerInfra struct {
	runtimeCalled bool
	composeCalled bool

	composeRequestID string
	composePayload   contract.ComposeUpStackPayload
	composeDeadline  time.Time
	composeHasDL     bool
}

func (s *stubDockerRunnerInfra) HostListenTCPPorts(_ context.Context, _ string) (contract.Result, error) {
	return contract.Result{Status: contract.StatusSucceeded}, nil
}

func (s *stubDockerRunnerInfra) DockerPublishedPorts(_ context.Context, _ string) (contract.Result, error) {
	return contract.Result{Status: contract.StatusSucceeded}, nil
}

func (s *stubDockerRunnerInfra) DockerRuntimeCheck(_ context.Context, _ string) (contract.Result, error) {
	s.runtimeCalled = true
	return contract.Result{Status: contract.StatusSucceeded}, nil
}

func (s *stubDockerRunnerInfra) DockerListContainers(_ context.Context, _ string, _ bool) (contract.Result, error) {
	return contract.Result{Status: contract.StatusSucceeded}, nil
}

func (s *stubDockerRunnerInfra) DockerRunQuickService(_ context.Context, _ string, _ contract.DockerRunQuickServicePayload) (contract.Result, error) {
	return contract.Result{Status: contract.StatusSucceeded}, nil
}

func (s *stubDockerRunnerInfra) ComposeUpStack(ctx context.Context, requestID string, payload contract.ComposeUpStackPayload) (contract.Result, error) {
	s.composeCalled = true
	s.composeRequestID = requestID
	s.composePayload = payload
	s.composeDeadline, s.composeHasDL = ctx.Deadline()
	return contract.Result{Status: contract.StatusSucceeded}, nil
}

type noopDockerRunnerLogger struct{}

func (noopDockerRunnerLogger) Log(_ string) {}

func (noopDockerRunnerLogger) Logf(_ string, _ ...any) {}

func TestDockerRunnerComposeUpUsesExtendedTimeoutWhenCallerHasNoDeadline(t *testing.T) {
	t.Parallel()

	infra := &stubDockerRunnerInfra{}
	runner := NewDockerRunner(infra)
	logger := noopDockerRunnerLogger{}

	err := runner.ComposeUp(context.Background(), logger, DockerComposeRequest{ProjectDir: "/tmp/demo"})
	require.NoError(t, err)

	require.True(t, infra.runtimeCalled)
	require.True(t, infra.composeCalled)
	require.True(t, infra.composeHasDL)
	require.Equal(t, "", infra.composeRequestID)
	require.Equal(t, "demo", infra.composePayload.Project)
	require.Equal(t, "/tmp/demo", infra.composePayload.ProjectDir)
	require.True(t, infra.composePayload.Build)

	remaining := time.Until(infra.composeDeadline)
	require.Greater(t, remaining, 29*time.Minute)
	require.LessOrEqual(t, remaining, defaultComposeUpWaitTimeout+time.Second)
}

func TestDockerRunnerComposeUpPreservesCallerDeadline(t *testing.T) {
	t.Parallel()

	infra := &stubDockerRunnerInfra{}
	runner := NewDockerRunner(infra)
	logger := noopDockerRunnerLogger{}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	err := runner.ComposeUp(ctx, logger, DockerComposeRequest{ProjectDir: "/tmp/demo"})
	require.NoError(t, err)

	require.True(t, infra.runtimeCalled)
	require.True(t, infra.composeCalled)
	require.True(t, infra.composeHasDL)

	remaining := time.Until(infra.composeDeadline)
	require.Greater(t, remaining, time.Duration(0))
	require.LessOrEqual(t, remaining, 1500*time.Millisecond)
}
