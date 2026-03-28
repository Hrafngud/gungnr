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
	runCalled     bool

	composeRequestID string
	composePayload   contract.ComposeUpStackPayload
	composeDeadline  time.Time
	composeHasDL     bool
	runPayload       contract.DockerRunQuickServicePayload
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

func (s *stubDockerRunnerInfra) DockerRunQuickService(_ context.Context, _ string, payload contract.DockerRunQuickServicePayload) (contract.Result, error) {
	s.runCalled = true
	s.runPayload = payload
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

func TestDockerRunnerRunContainerUsesInternalOnlyManagedPolicyByDefault(t *testing.T) {
	t.Parallel()

	infra := &stubDockerRunnerInfra{}
	runner := NewDockerRunner(infra)
	logger := noopDockerRunnerLogger{}

	err := runner.RunContainer(context.Background(), logger, DockerRunRequest{
		Image:         "ghcr.io/acme/demo:1.0.0",
		HostPort:      19000,
		ContainerPort: 8080,
		ExposureMode:  contract.QuickServiceExposureInternal,
	})
	require.NoError(t, err)

	require.True(t, infra.runtimeCalled)
	require.True(t, infra.runCalled)
	require.Equal(t, "ghcr.io/acme/demo:1.0.0", infra.runPayload.Image)
	require.Equal(t, 19000, infra.runPayload.HostPort)
	require.Equal(t, 8080, infra.runPayload.ContainerPort)
	require.Equal(t, "demo", infra.runPayload.ContainerName)
	require.Equal(t, contract.QuickServiceExposureInternal, infra.runPayload.ExposureMode)
	require.Equal(t, contract.QuickServiceDefaultNetwork, infra.runPayload.NetworkName)
	require.Equal(t, contract.QuickServicePublishLoopbackHost, infra.runPayload.PublishHost)
}

func TestDockerRunnerRunContainerUsesLoopbackForExplicitHostPublish(t *testing.T) {
	t.Parallel()

	infra := &stubDockerRunnerInfra{}
	runner := NewDockerRunner(infra)
	logger := noopDockerRunnerLogger{}

	const hostPort = 19000

	err := runner.RunContainer(context.Background(), logger, DockerRunRequest{
		Image:         "ghcr.io/acme/demo:1.0.0",
		HostPort:      hostPort,
		ContainerPort: 8080,
		ExposureMode:  contract.QuickServiceExposureHostPublished,
	})
	require.NoError(t, err)

	require.True(t, infra.runCalled)
	require.Equal(t, hostPort, infra.runPayload.HostPort)
	require.Equal(t, contract.QuickServiceExposureHostPublished, infra.runPayload.ExposureMode)
	require.Equal(t, contract.QuickServicePublishLoopbackHost, infra.runPayload.PublishHost)
}
