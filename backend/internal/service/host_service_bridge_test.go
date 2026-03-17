package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"

	"github.com/stretchr/testify/require"
)

type stubHostInfraBridgeClient struct {
	stopCalled       bool
	stopRequestID    string
	stopContainer    string
	stopResult       contract.Result
	stopErr          error
	restartCalled    bool
	restartRequestID string
	restartContainer string
	restartResult    contract.Result
	restartErr       error
	removeCalled     bool
	removeRequestID  string
	removeContainer  string
	removeVolumes    bool
	removeResult     contract.Result
	removeErr        error
	composeCalled    bool
	composeRequestID string
	composePayload   contract.ComposeUpStackPayload
	composeResult    contract.Result
	composeErr       error
}

func (s *stubHostInfraBridgeClient) StopContainer(_ context.Context, requestID, container string) (contract.Result, error) {
	s.stopCalled = true
	s.stopRequestID = requestID
	s.stopContainer = container
	return s.stopResult, s.stopErr
}

func (s *stubHostInfraBridgeClient) RestartContainer(_ context.Context, requestID, container string) (contract.Result, error) {
	s.restartCalled = true
	s.restartRequestID = requestID
	s.restartContainer = container
	return s.restartResult, s.restartErr
}

func (s *stubHostInfraBridgeClient) RemoveContainer(_ context.Context, requestID, container string, removeVolumes bool) (contract.Result, error) {
	s.removeCalled = true
	s.removeRequestID = requestID
	s.removeContainer = container
	s.removeVolumes = removeVolumes
	return s.removeResult, s.removeErr
}

func (s *stubHostInfraBridgeClient) ComposeUpStack(_ context.Context, requestID string, payload contract.ComposeUpStackPayload) (contract.Result, error) {
	s.composeCalled = true
	s.composeRequestID = requestID
	s.composePayload = payload
	return s.composeResult, s.composeErr
}

type captureHostLogger struct {
	lines []string
}

func (l *captureHostLogger) Log(line string) {
	l.lines = append(l.lines, line)
}

func (l *captureHostLogger) Logf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func TestHostServiceContainerActionsBridgeSuccess(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		stopResult:    contract.Result{Status: contract.StatusSucceeded},
		restartResult: contract.Result{Status: contract.StatusSucceeded},
		removeResult:  contract.Result{Status: contract.StatusSucceeded},
	}
	svc := &HostService{infraClient: bridge}

	err := svc.StopContainer(context.Background(), "api")
	require.NoError(t, err)
	require.True(t, bridge.stopCalled)
	require.Equal(t, "api", bridge.stopContainer)

	err = svc.RestartContainer(context.Background(), "worker")
	require.NoError(t, err)
	require.True(t, bridge.restartCalled)
	require.Equal(t, "worker", bridge.restartContainer)

	err = svc.RemoveContainer(context.Background(), "db", true)
	require.NoError(t, err)
	require.True(t, bridge.removeCalled)
	require.Equal(t, "db", bridge.removeContainer)
	require.True(t, bridge.removeVolumes)
}

func TestHostServiceContainerActionsBridgeFailureMapping(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		stopErr: fmt.Errorf("bridge unavailable"),
	}
	svc := &HostService{infraClient: bridge}

	err := svc.StopContainer(context.Background(), "api")
	require.Error(t, err)

	typed, ok := errs.From(err)
	require.True(t, ok)
	require.Equal(t, errs.CodeHostDockerFailed, typed.Code)
	require.Equal(t, "failed to stop container", typed.Message)
	details, ok := typed.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, contract.TaskTypeDockerStopContainer, details["task_type"])
	require.Equal(t, "api", details["target"])
}

func TestHostServiceContainerActionsFailedResultMapping(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		removeResult: contract.Result{
			Status:   contract.StatusFailed,
			IntentID: "intent-remove-1",
			LogPath:  "/tmp/infra-remove.log",
			Error: &contract.Error{
				Code:    "DOCKER-500",
				Message: "container remove failed",
			},
		},
	}
	svc := &HostService{infraClient: bridge}

	err := svc.RemoveContainer(context.Background(), "db", true)
	require.Error(t, err)

	typed, ok := errs.From(err)
	require.True(t, ok)
	require.Equal(t, errs.CodeHostDockerFailed, typed.Code)
	require.Equal(t, "failed to remove container", typed.Message)
	details, ok := typed.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, contract.TaskTypeDockerRemoveContainer, details["task_type"])
	require.Equal(t, "db", details["target"])
	require.Equal(t, "intent-remove-1", details["intent_id"])
	require.Equal(t, "DOCKER-500", details["worker_error_code"])
	require.Equal(t, "/tmp/infra-remove.log", details["log_path"])
}

func TestHostServiceRestartProjectStackBridgeSuccess(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		composeResult: contract.Result{
			Status:     contract.StatusSucceeded,
			IntentID:   "intent-compose-1",
			FinishedAt: time.Now().UTC(),
			LogPath:    "/tmp/infra-compose.log",
		},
	}
	logger := &captureHostLogger{}
	svc := &HostService{infraClient: bridge}

	err := svc.RestartProjectStackWithLogger(context.Background(), "job-42", "my-project", logger)
	require.NoError(t, err)
	require.True(t, bridge.composeCalled)
	require.Equal(t, "job-42", bridge.composeRequestID)
	require.Equal(t, "my-project", bridge.composePayload.Project)
	require.True(t, bridge.composePayload.Build)
	require.True(t, bridge.composePayload.ForceRecreate)
	require.NotEmpty(t, logger.lines)
}

func TestHostServiceRestartProjectStackBridgeFailureMapping(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		composeResult: contract.Result{
			Status:   contract.StatusFailed,
			IntentID: "intent-compose-fail",
			LogPath:  "/tmp/infra-compose-fail.log",
			Error: &contract.Error{
				Code:    "COMPOSE-500",
				Message: "compose up failed",
			},
		},
	}
	svc := &HostService{infraClient: bridge}

	err := svc.RestartProjectStack(context.Background(), "my-project")
	require.Error(t, err)

	typed, ok := errs.From(err)
	require.True(t, ok)
	require.Equal(t, errs.CodeHostDockerFailed, typed.Code)
	require.Equal(t, "restart compose stack failed", typed.Message)
	details, ok := typed.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, contract.TaskTypeComposeUpStack, details["task_type"])
	require.Equal(t, "my-project", details["target"])
	require.Equal(t, "intent-compose-fail", details["intent_id"])
	require.Equal(t, "COMPOSE-500", details["worker_error_code"])
}
