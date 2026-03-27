package service

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"go-notes/internal/errs"
	"go-notes/internal/infra/contract"

	"github.com/stretchr/testify/require"
)

type stubHostInfraBridgeClient struct {
	stopCalled               bool
	stopRequestID            string
	stopContainer            string
	stopResult               contract.Result
	stopErr                  error
	restartCalled            bool
	restartRequestID         string
	restartContainer         string
	restartResult            contract.Result
	restartErr               error
	removeCalled             bool
	removeRequestID          string
	removeContainer          string
	removeVolumes            bool
	removeResult             contract.Result
	removeErr                error
	listContainersCalled     bool
	listContainersRequestID  string
	listContainersIncludeAll bool
	listContainersResult     contract.Result
	listContainersErr        error
	systemDFCalled           bool
	systemDFRequestID        string
	systemDFResult           contract.Result
	systemDFErr              error
	listVolumesCalled        bool
	listVolumesRequestID     string
	listVolumesResult        contract.Result
	listVolumesErr           error
	containerLogsCalled      bool
	containerLogsCalls       int
	containerLogsRequestID   string
	containerLogsPayload     contract.DockerContainerLogsPayload
	containerLogsResult      contract.Result
	containerLogsErr         error
	runtimeCalled            bool
	runtimeRequestID         string
	runtimeResult            contract.Result
	runtimeErr               error
	composeCalled            bool
	composeRequestID         string
	composePayload           contract.ComposeUpStackPayload
	composeResult            contract.Result
	composeErr               error
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

func (s *stubHostInfraBridgeClient) DockerListContainers(_ context.Context, requestID string, includeAll bool) (contract.Result, error) {
	s.listContainersCalled = true
	s.listContainersRequestID = requestID
	s.listContainersIncludeAll = includeAll
	return s.listContainersResult, s.listContainersErr
}

func (s *stubHostInfraBridgeClient) DockerSystemDF(_ context.Context, requestID string) (contract.Result, error) {
	s.systemDFCalled = true
	s.systemDFRequestID = requestID
	return s.systemDFResult, s.systemDFErr
}

func (s *stubHostInfraBridgeClient) DockerListVolumes(_ context.Context, requestID string) (contract.Result, error) {
	s.listVolumesCalled = true
	s.listVolumesRequestID = requestID
	return s.listVolumesResult, s.listVolumesErr
}

func (s *stubHostInfraBridgeClient) DockerContainerLogs(_ context.Context, requestID string, payload contract.DockerContainerLogsPayload) (contract.Result, error) {
	s.containerLogsCalled = true
	s.containerLogsCalls++
	s.containerLogsRequestID = requestID
	s.containerLogsPayload = payload
	return s.containerLogsResult, s.containerLogsErr
}

func (s *stubHostInfraBridgeClient) HostRuntimeStats(_ context.Context, requestID string) (contract.Result, error) {
	s.runtimeCalled = true
	s.runtimeRequestID = requestID
	return s.runtimeResult, s.runtimeErr
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

func TestHostServiceListContainersBridgeSuccess(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		listContainersResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					`{"ID":"abc123","Image":"nginx:alpine","Names":"demo-web","Status":"Up 2 minutes","Ports":"0.0.0.0:8080->80/tcp, 443/tcp","CreatedAt":"2026-03-21 10:00:00 +0000 UTC","RunningFor":"2 minutes","Labels":"com.docker.compose.project=demo,com.docker.compose.service=web"}`,
				},
			},
		},
	}
	svc := &HostService{infraClient: bridge}

	containers, err := svc.ListContainers(context.Background(), true)
	require.NoError(t, err)
	require.True(t, bridge.listContainersCalled)
	require.True(t, bridge.listContainersIncludeAll)
	require.Len(t, containers, 1)
	require.Equal(t, "demo-web", containers[0].Name)
	require.Equal(t, "demo", containers[0].Project)
	require.Equal(t, "web", containers[0].Service)
	require.Len(t, containers[0].PortBindings, 2)
}

func TestHostServiceCountRunningContainersBridgeSuccess(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		listContainersResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					`{"ID":"a1","Names":"one"}`,
					`{"ID":"a2","Names":"two"}`,
					`{"ID":"a3","Names":"three"}`,
				},
			},
		},
	}
	svc := &HostService{infraClient: bridge}

	count, err := svc.CountRunningContainers(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.True(t, bridge.listContainersCalled)
	require.False(t, bridge.listContainersIncludeAll)
}

func TestReadComposeProjectMetaBridgeSuccess(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		listContainersResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					`{"ID":"abc123","Names":"demo-api","Labels":"com.docker.compose.project=demo,com.docker.compose.project.working_dir=/templates/demo,com.docker.compose.project.config_files=/templates/demo/docker-compose.yml,com.docker.compose.service=api"}`,
				},
			},
		},
	}

	meta, err := readComposeProjectMeta(context.Background(), bridge, "demo")
	require.NoError(t, err)
	require.True(t, bridge.listContainersCalled)
	require.True(t, bridge.listContainersIncludeAll)
	require.Equal(t, "/templates/demo", meta.WorkingDir)
	require.Equal(t, []string{"/templates/demo/docker-compose.yml"}, meta.ConfigFiles)
}

func TestReadComposeProjectMetaPreservesMultiConfigFilesLabel(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		listContainersResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					`{"ID":"abc123","Names":"demo-api","Labels":"com.docker.compose.project=demo,com.docker.compose.project.config_files=/templates/demo/base.yml,/templates/demo/override.yml,com.example.flag=true"}`,
				},
			},
		},
	}

	meta, err := readComposeProjectMeta(context.Background(), bridge, "demo")
	require.NoError(t, err)
	require.Equal(t, []string{"/templates/demo/base.yml", "/templates/demo/override.yml"}, meta.ConfigFiles)
}

func TestReadComposeProjectMetaRequiresBridgeClient(t *testing.T) {
	t.Parallel()

	_, err := readComposeProjectMeta(context.Background(), nil, "demo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "infra bridge client unavailable")
}

func TestHostServiceStartContainerLogsBridgeSuccess(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		containerLogsResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					"2026-03-27T14:00:00Z service started",
					"2026-03-27T14:00:01Z request ok",
				},
			},
		},
	}
	svc := &HostService{infraClient: bridge}

	waiter, reader, err := svc.StartContainerLogs(context.Background(), "demo-api", ContainerLogsOptions{
		Tail:       250,
		Follow:     false,
		Timestamps: true,
	})
	require.NoError(t, err)
	payload, readErr := io.ReadAll(reader)
	require.NoError(t, readErr)
	require.NoError(t, waiter.Wait())
	require.True(t, bridge.containerLogsCalled)
	require.Equal(t, "demo-api", bridge.containerLogsPayload.Container)
	require.Equal(t, 250, bridge.containerLogsPayload.Tail)
	require.False(t, bridge.containerLogsPayload.Follow)
	require.True(t, bridge.containerLogsPayload.Timestamps)
	require.Equal(t, "2026-03-27T14:00:00Z service started\n2026-03-27T14:00:01Z request ok\n", string(payload))
}

func TestHostServiceStartContainerLogsBridgeFailure(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		containerLogsErr: fmt.Errorf("bridge unavailable"),
	}
	svc := &HostService{infraClient: bridge}

	waiter, reader, err := svc.StartContainerLogs(context.Background(), "demo-api", ContainerLogsOptions{Tail: 100})
	require.NoError(t, err)
	_, readErr := io.ReadAll(reader)
	require.Error(t, readErr)
	waitErr := waiter.Wait()
	require.Error(t, waitErr)
	require.Contains(t, waitErr.Error(), "fetch docker logs via infra bridge")
}

func TestHostServiceStartContainerLogsFollowStreamUsesPolling(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		containerLogsResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					"line one",
				},
			},
		},
	}
	svc := &HostService{infraClient: bridge}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	waiter, reader, err := svc.StartContainerLogs(ctx, "demo-api", ContainerLogsOptions{
		Tail:       10,
		Follow:     true,
		Timestamps: false,
	})
	require.NoError(t, err)
	payload, readErr := io.ReadAll(reader)
	require.NoError(t, readErr)
	require.NoError(t, waiter.Wait())
	require.True(t, bridge.containerLogsCalled)
	require.GreaterOrEqual(t, bridge.containerLogsCalls, 1)
	require.False(t, bridge.containerLogsPayload.Follow)
	require.Equal(t, "line one\n", string(payload))
}

func TestHostServiceDockerUsageBridgeSuccess(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		systemDFResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					`{"Type":"Images","TotalCount":"8","Active":"2","Size":"3.2GB","Reclaimable":"1.1GB (34%)"}`,
					`{"Type":"Containers","TotalCount":"6","Active":"3","Size":"512MB","Reclaimable":"0B (0%)"}`,
					`{"Type":"Local Volumes","TotalCount":"5","Active":"4","Size":"1.5GB","Reclaimable":"0B (0%)"}`,
					`{"Type":"Build Cache","TotalCount":"2","Active":"0","Size":"120MB","Reclaimable":"120MB (100%)"}`,
				},
			},
		},
		listContainersResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					`{"ID":"c1","Image":"nginx:alpine","Names":"demo-web","Status":"Up","Ports":"","CreatedAt":"","RunningFor":"","Labels":"com.docker.compose.project=demo,com.docker.compose.service=web"}`,
					`{"ID":"c2","Image":"redis:7","Names":"demo-redis","Status":"Up","Ports":"","CreatedAt":"","RunningFor":"","Labels":"com.docker.compose.project=demo,com.docker.compose.service=redis"}`,
					`{"ID":"c3","Image":"nginx:alpine","Names":"other-web","Status":"Up","Ports":"","CreatedAt":"","RunningFor":"","Labels":"com.docker.compose.project=other,com.docker.compose.service=web"}`,
				},
			},
		},
		listVolumesResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"lines": []string{
					`{"Name":"demo_data","Driver":"local","Labels":"com.docker.compose.project=demo"}`,
					`{"Name":"other_data","Driver":"local","Labels":"com.docker.compose.project=other"}`,
				},
			},
		},
	}
	svc := &HostService{infraClient: bridge}

	summary, err := svc.DockerUsage(context.Background(), "demo")
	require.NoError(t, err)
	require.True(t, bridge.systemDFCalled)
	require.True(t, bridge.listContainersCalled)
	require.True(t, bridge.listVolumesCalled)
	require.Equal(t, "demo", summary.Project)
	require.NotNil(t, summary.ProjectCounts)
	require.Equal(t, 2, summary.ProjectCounts.Containers)
	require.Equal(t, 2, summary.ProjectCounts.Images)
	require.Equal(t, 1, summary.ProjectCounts.Volumes)
	require.Equal(t, 8, summary.Images.Count)
	require.Equal(t, 6, summary.Containers.Count)
	require.Equal(t, 5, summary.Volumes.Count)
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

func TestHostServiceListContainersBridgeFailureMapping(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		listContainersErr: fmt.Errorf("bridge unavailable"),
	}
	svc := &HostService{infraClient: bridge}

	_, err := svc.ListContainers(context.Background(), true)
	require.Error(t, err)

	typed, ok := errs.From(err)
	require.True(t, ok)
	require.Equal(t, errs.CodeHostDockerFailed, typed.Code)
	require.Equal(t, "failed to list docker containers", typed.Message)
	details, ok := typed.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, contract.TaskTypeDockerListContainers, details["task_type"])
	require.Equal(t, "docker", details["target"])
}

func TestHostServiceDockerUsageBridgeFailureMapping(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		systemDFResult: contract.Result{
			Status:   contract.StatusFailed,
			IntentID: "intent-df-fail",
			LogPath:  "/tmp/infra-df.log",
			Error: &contract.Error{
				Code:    "DOCKER-500",
				Message: "docker system df failed",
			},
		},
	}
	svc := &HostService{infraClient: bridge}

	_, err := svc.DockerUsage(context.Background(), "")
	require.Error(t, err)

	typed, ok := errs.From(err)
	require.True(t, ok)
	require.Equal(t, errs.CodeHostUsageFailed, typed.Code)
	require.Equal(t, "failed to load docker usage", typed.Message)
	details, ok := typed.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, contract.TaskTypeDockerSystemDF, details["task_type"])
	require.Equal(t, "docker", details["target"])
	require.Equal(t, "intent-df-fail", details["intent_id"])
	require.Equal(t, "DOCKER-500", details["worker_error_code"])
	require.Equal(t, "/tmp/infra-df.log", details["log_path"])
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

func TestHostServiceRuntimeStatsBridgeSuccess(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		runtimeResult: contract.Result{
			Status: contract.StatusSucceeded,
			Data: map[string]any{
				"collectedAt": "2026-03-18T18:00:00Z",
				"hostname":    "runner-01",
				"systemImage": "Ubuntu 24.04 LTS",
				"cpu": map[string]any{
					"model":    "AMD Ryzen",
					"cores":    16,
					"threads":  32,
					"speedMHz": 4250,
				},
			},
		},
	}
	svc := &HostService{infraClient: bridge}

	stats, err := svc.RuntimeStats(context.Background())
	require.NoError(t, err)
	require.True(t, bridge.runtimeCalled)
	require.Equal(t, "runner-01", stats.Hostname)
	require.Equal(t, "Ubuntu 24.04 LTS", stats.SystemImage)
	require.Equal(t, "AMD Ryzen", stats.CPU.Model)
	require.Equal(t, 16, stats.CPU.Cores)
	require.Equal(t, 32, stats.CPU.Threads)
	require.Equal(t, 4250.0, stats.CPU.SpeedMHz)
}

func TestHostServiceRuntimeStatsBridgeFailureMapping(t *testing.T) {
	t.Parallel()

	bridge := &stubHostInfraBridgeClient{
		runtimeResult: contract.Result{
			Status:   contract.StatusFailed,
			IntentID: "intent-runtime-fail",
			LogPath:  "/tmp/infra-runtime.log",
			Error: &contract.Error{
				Code:    "HOST-500-WORKER",
				Message: "runtime probe failed",
			},
		},
	}
	svc := &HostService{infraClient: bridge}

	_, err := svc.RuntimeStats(context.Background())
	require.Error(t, err)

	typed, ok := errs.From(err)
	require.True(t, ok)
	require.Equal(t, errs.CodeHostStatsFailed, typed.Code)
	require.Equal(t, "failed to load host runtime stats", typed.Message)
	details, ok := typed.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, contract.TaskTypeHostRuntimeStats, details["task_type"])
	require.Equal(t, "host", details["target"])
	require.Equal(t, "intent-runtime-fail", details["intent_id"])
}
