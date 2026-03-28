package worker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"go-notes/internal/infra/contract"
	"go-notes/internal/infra/queue"

	"github.com/stretchr/testify/require"
)

func decodeDataLines(t *testing.T, data map[string]any) []string {
	t.Helper()
	raw, err := json.Marshal(data["lines"])
	require.NoError(t, err)
	var lines []string
	require.NoError(t, json.Unmarshal(raw, &lines))
	return lines
}

type fakeExecCall struct {
	dir  string
	name string
	args []string
}

type fakeExecutor struct {
	calls   []fakeExecCall
	output  []byte
	err     error
	outputs [][]byte
	errs    []error
}

func (f *fakeExecutor) Run(_ context.Context, dir string, name string, args ...string) ([]byte, error) {
	copiedArgs := make([]string, len(args))
	copy(copiedArgs, args)
	callIndex := len(f.calls)
	f.calls = append(f.calls, fakeExecCall{
		dir:  dir,
		name: name,
		args: copiedArgs,
	})
	output := f.output
	err := f.err
	if callIndex < len(f.outputs) {
		output = f.outputs[callIndex]
	}
	if callIndex < len(f.errs) {
		err = f.errs[callIndex]
	}
	return output, err
}

type fakeTunnelLifecycle struct {
	called     bool
	configPath string
	logPath    string
	logTail    []string
	err        error
}

func (f *fakeTunnelLifecycle) Restart(_ context.Context, configPath string) (string, []string, error) {
	f.called = true
	f.configPath = configPath
	return f.logPath, f.logTail, f.err
}

func TestProcessOnceHandlesDockerStop(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-stop",
		RequestID: "req-stop",
		TaskType:  contract.TaskTypeDockerStopContainer,
		Payload: map[string]any{
			"container": "api",
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{output: []byte("api\n")}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 1)
	require.Equal(t, "docker", exec.calls[0].name)
	require.Equal(t, []string{"stop", "api"}, exec.calls[0].args)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, []string{"api"}, result.LogTail)
	require.NoFileExists(t, q.ClaimPath(intent.IntentID))
}

func TestProcessOnceHandlesDockerListContainers(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-list-containers",
		RequestID: "req-list-containers",
		TaskType:  contract.TaskTypeDockerListContainers,
		Payload: map[string]any{
			"include_all": true,
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{output: []byte("{\"ID\":\"abc\"}\n{\"ID\":\"def\"}\n")}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 1)
	require.Equal(t, "docker", exec.calls[0].name)
	require.Equal(t, []string{"ps", "-a", "--format", "{{json .}}"}, exec.calls[0].args)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, []string{"{\"ID\":\"abc\"}", "{\"ID\":\"def\"}"}, decodeDataLines(t, result.Data))
}

func TestProcessOnceHandlesDockerSystemDF(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-system-df",
		RequestID: "req-system-df",
		TaskType:  contract.TaskTypeDockerSystemDF,
		Payload:   map[string]any{},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{output: []byte("{\"Type\":\"Images\"}\n")}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 1)
	require.Equal(t, "docker", exec.calls[0].name)
	require.Equal(t, []string{"system", "df", "--format", "{{json .}}"}, exec.calls[0].args)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, []string{"{\"Type\":\"Images\"}"}, decodeDataLines(t, result.Data))
}

func TestProcessOnceHandlesDockerContainerLogs(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-container-logs",
		RequestID: "req-container-logs",
		TaskType:  contract.TaskTypeDockerContainerLogs,
		Payload: map[string]any{
			"container":  "demo-api",
			"tail":       300,
			"follow":     true,
			"timestamps": true,
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{output: []byte("  line one  \nline two\t\n")}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 1)
	require.Equal(t, "docker", exec.calls[0].name)
	require.Equal(t, []string{"logs", "-f", "--timestamps", "--tail", "300", "demo-api"}, exec.calls[0].args)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, []string{"  line one  ", "line two\t"}, decodeDataLines(t, result.Data))
}

func TestProcessOnceHandlesDockerContainerLogsSinceWithoutTail(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-container-logs-since",
		RequestID: "req-container-logs-since",
		TaskType:  contract.TaskTypeDockerContainerLogs,
		Payload: map[string]any{
			"container": "demo-api",
			"since":     "2026-03-27T16:00:00Z",
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{output: []byte("line one\n")}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 1)
	require.Equal(t, "docker", exec.calls[0].name)
	require.Equal(t, []string{"logs", "--since", "2026-03-27T16:00:00Z", "demo-api"}, exec.calls[0].args)
}

func TestProcessOnceHandlesDockerRuntimeCheck(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-runtime-check",
		RequestID: "req-runtime-check",
		TaskType:  contract.TaskTypeDockerRuntimeCheck,
		Payload:   map[string]any{},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{output: []byte("27.5.1\n")}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 1)
	require.Equal(t, "docker", exec.calls[0].name)
	require.Equal(t, []string{"version", "--format", "{{.Server.Version}}"}, exec.calls[0].args)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, []string{"27.5.1"}, decodeDataLines(t, result.Data))
}

func TestProcessOnceHandlesDockerRunQuickService(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-quick-service",
		RequestID: "req-quick-service",
		TaskType:  contract.TaskTypeDockerRunQuickService,
		Payload: map[string]any{
			"image":          "excalidraw/excalidraw:latest",
			"host_port":      19000,
			"container_port": 80,
			"container_name": "quick-excalidraw",
			"exposure_mode":  contract.QuickServiceExposureHostPublished,
			"publish_host":   contract.QuickServicePublishLoopbackHost,
			"network_name":   "warp-panel_quick_internal",
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{
		outputs: [][]byte{
			[]byte(`[{"Name":"warp-panel_quick_internal"}]` + "\n"),
			[]byte("container-id-1\n"),
		},
	}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 2)
	require.Equal(t, "docker", exec.calls[0].name)
	require.Equal(t, []string{"network", "inspect", "warp-panel_quick_internal"}, exec.calls[0].args)
	require.Equal(t, "docker", exec.calls[1].name)
	require.Equal(t, []string{
		"run",
		"-d",
		"--restart",
		"unless-stopped",
		"--network",
		"warp-panel_quick_internal",
		"--security-opt",
		"no-new-privileges:true",
		"--cap-drop",
		"ALL",
		"--cap-add",
		"NET_BIND_SERVICE",
		"--pids-limit",
		strconv.Itoa(contract.QuickServiceDefaultPIDsLimit),
		"--memory",
		contract.QuickServiceDefaultMemory,
		"--memory-swap",
		contract.QuickServiceDefaultMemory,
		"--cpus",
		contract.QuickServiceDefaultCPUs,
		"--label",
		contract.QuickServiceManagedLabelKey + "=" + contract.QuickServiceManagedLabelValue,
		"--label",
		contract.QuickServiceExposureLabelKey + "=" + contract.QuickServiceExposureHostPublished,
		"-p",
		"127.0.0.1:19000:80",
		"--name",
		"quick-excalidraw",
		"excalidraw/excalidraw:latest",
	}, exec.calls[1].args)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, []string{"container-id-1"}, decodeDataLines(t, result.Data))
}

func TestProcessOnceHandlesDockerRunQuickServiceInternalOnlyCreatesManagedNetwork(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-quick-service-internal",
		RequestID: "req-quick-service-internal",
		TaskType:  contract.TaskTypeDockerRunQuickService,
		Payload: map[string]any{
			"image":          "excalidraw/excalidraw:latest",
			"host_port":      18080,
			"container_port": 80,
			"container_name": "quick-excalidraw-internal",
			"exposure_mode":  contract.QuickServiceExposureInternal,
			"publish_host":   contract.QuickServicePublishLoopbackHost,
			"network_name":   "warp-panel_quick_internal",
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{
		outputs: [][]byte{
			[]byte(""),
			[]byte("warp-panel_quick_internal\n"),
			[]byte("container-id-2\n"),
		},
		errs: []error{
			errors.New("Error response from daemon: network warp-panel_quick_internal not found"),
			nil,
			nil,
		},
	}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 3)
	require.Equal(t, []string{"network", "inspect", "warp-panel_quick_internal"}, exec.calls[0].args)
	require.Equal(t, []string{
		"network",
		"create",
		"--driver",
		"bridge",
		"--internal",
		"--label",
		contract.QuickServiceManagedLabelKey + "=" + contract.QuickServiceManagedLabelValue,
		"--label",
		contract.QuickServiceNetworkLabelKey + "=" + contract.QuickServiceNetworkLabelValue,
		"warp-panel_quick_internal",
	}, exec.calls[1].args)
	require.Equal(t, []string{
		"run",
		"-d",
		"--restart",
		"unless-stopped",
		"--network",
		"warp-panel_quick_internal",
		"--security-opt",
		"no-new-privileges:true",
		"--cap-drop",
		"ALL",
		"--cap-add",
		"NET_BIND_SERVICE",
		"--pids-limit",
		strconv.Itoa(contract.QuickServiceDefaultPIDsLimit),
		"--memory",
		contract.QuickServiceDefaultMemory,
		"--memory-swap",
		contract.QuickServiceDefaultMemory,
		"--cpus",
		contract.QuickServiceDefaultCPUs,
		"--label",
		contract.QuickServiceManagedLabelKey + "=" + contract.QuickServiceManagedLabelValue,
		"--label",
		contract.QuickServiceExposureLabelKey + "=" + contract.QuickServiceExposureInternal,
		"-p",
		"127.0.0.1:18080:80",
		"--name",
		"quick-excalidraw-internal",
		"excalidraw/excalidraw:latest",
	}, exec.calls[2].args)
}

func TestProcessOnceHandlesDockerRunQuickServiceIgnoresConcurrentNetworkCreate(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-quick-service-network-race",
		RequestID: "req-quick-service-network-race",
		TaskType:  contract.TaskTypeDockerRunQuickService,
		Payload: map[string]any{
			"image":          "excalidraw/excalidraw:latest",
			"host_port":      18081,
			"container_port": 80,
			"container_name": "quick-excalidraw-race",
			"exposure_mode":  contract.QuickServiceExposureInternal,
			"publish_host":   contract.QuickServicePublishLoopbackHost,
			"network_name":   "warp-panel_quick_internal",
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{
		outputs: [][]byte{
			[]byte(""),
			[]byte("Error response from daemon: network with name warp-panel_quick_internal already exists\n"),
			[]byte("container-id-3\n"),
		},
		errs: []error{
			errors.New("Error response from daemon: network warp-panel_quick_internal not found"),
			errors.New("Error response from daemon: network with name warp-panel_quick_internal already exists"),
			nil,
		},
	}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 3)
	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, []string{"container-id-3"}, decodeDataLines(t, result.Data))
}

func TestProcessOnceHandlesHostListenTCPPorts(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-host-listen",
		RequestID: "req-host-listen",
		TaskType:  contract.TaskTypeHostListenTCPPorts,
		Payload:   map[string]any{},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{output: []byte("LISTEN 0 128 0.0.0.0:80\nLISTEN 0 128 [::]:443\n")}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 1)
	require.Equal(t, "ss", exec.calls[0].name)
	require.Equal(t, []string{"-ltnH"}, exec.calls[0].args)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, []string{"LISTEN 0 128 0.0.0.0:80", "LISTEN 0 128 [::]:443"}, decodeDataLines(t, result.Data))
}

func TestProcessOnceHandlesDockerPublishedPorts(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-docker-published",
		RequestID: "req-docker-published",
		TaskType:  contract.TaskTypeDockerPublishedPorts,
		Payload:   map[string]any{},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{output: []byte("0.0.0.0:8080->80/tcp\n")}
	r := New(q, 10*time.Millisecond, "", nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 1)
	require.Equal(t, "docker", exec.calls[0].name)
	require.Equal(t, []string{"ps", "--format", "{{.Ports}}"}, exec.calls[0].args)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, []string{"0.0.0.0:8080->80/tcp"}, decodeDataLines(t, result.Data))
}

func TestProcessOnceHandlesRestartTunnel(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-restart-tunnel",
		RequestID: "req-restart-tunnel",
		TaskType:  contract.TaskTypeRestartTunnel,
		Payload: map[string]any{
			"config_path": "/tmp/cloudflared.yml",
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	tunnel := &fakeTunnelLifecycle{
		logPath: "/tmp/restart-worker.log",
		logTail: []string{"restart ok"},
	}
	r := New(q, 10*time.Millisecond, "", nil)
	r.tunnel = tunnel

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)
	require.True(t, tunnel.called)
	require.Equal(t, "/tmp/cloudflared.yml", tunnel.configPath)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, "/tmp/restart-worker.log", result.LogPath)
	require.Equal(t, []string{"restart ok"}, result.LogTail)
}

func TestProcessOnceHandlesComposeFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	q, err := queue.NewFilesystem(root)
	require.NoError(t, err)

	templatesDir := t.TempDir()
	projectDir := templatesDir + "/demo"
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-compose",
		RequestID: "req-compose",
		TaskType:  contract.TaskTypeComposeUpStack,
		Payload: map[string]any{
			"project":        "demo",
			"build":          true,
			"force_recreate": true,
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	exec := &fakeExecutor{
		output: []byte("compose failed"),
		err:    errors.New("exit status 1"),
	}
	r := New(q, 10*time.Millisecond, templatesDir, nil)
	r.exec = exec

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	require.Len(t, exec.calls, 1)
	require.Equal(t, "docker", exec.calls[0].name)
	require.Equal(t, projectDir, exec.calls[0].dir)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusFailed, result.Status)
	require.NotNil(t, result.Error)
	require.Equal(t, "INFRA-500-EXEC", result.Error.Code)
	require.Contains(t, result.Error.Message, "docker compose up --build --force-recreate -d failed")
}

func TestProcessOnceSkipsUnsupportedTask(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-restart-tunnel",
		RequestID: "req-restart-tunnel",
		TaskType:  contract.TaskTypeTunnelRestart,
		Payload: map[string]any{
			"config_path": "/tmp/cloudflared.yml",
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	r := New(q, 10*time.Millisecond, "", nil)
	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	_, err = q.ReadResult(context.Background(), intent.IntentID)
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist))
	require.NoFileExists(t, q.ClaimPath(intent.IntentID))
}

func TestProcessOnceHandlesProjectFileWriteAtomic(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	baseDir := t.TempDir()
	targetPath := filepath.Join(baseDir, ".env")
	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-project-file-write",
		RequestID: "req-project-file-write",
		TaskType:  contract.TaskTypeProjectFileWriteAtomic,
		Payload: map[string]any{
			"base_path":      baseDir,
			"path":           targetPath,
			"content":        "APP_ENV=prod\n",
			"mode":           0o600,
			"create_parents": true,
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	r := New(q, 10*time.Millisecond, "", nil)
	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, targetPath, result.Data["path"])

	raw, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	require.Equal(t, "APP_ENV=prod\n", string(raw))
}

func TestProcessOnceRejectsProjectFileWriteOutsideBase(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	baseDir := t.TempDir()
	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-project-file-write-outside",
		RequestID: "req-project-file-write-outside",
		TaskType:  contract.TaskTypeProjectFileWriteAtomic,
		Payload: map[string]any{
			"base_path": baseDir,
			"path":      filepath.Join(baseDir, "..", "outside.env"),
			"content":   "X=1\n",
			"mode":      0o600,
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)

	r := New(q, 10*time.Millisecond, "", nil)
	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	result, err := q.ReadResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusFailed, result.Status)
	require.NotNil(t, result.Error)
	require.Contains(t, result.Error.Message, "outside base path")
}

func TestProcessOnceHandlesProjectFileCopyAndRemove(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	baseDir := t.TempDir()
	sourcePath := filepath.Join(baseDir, ".env")
	copyPath := filepath.Join(baseDir, ".env.backup")
	require.NoError(t, os.WriteFile(sourcePath, []byte("TOKEN=abc\n"), 0o600))

	copyIntent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-project-file-copy",
		RequestID: "req-project-file-copy",
		TaskType:  contract.TaskTypeProjectFileCopy,
		Payload: map[string]any{
			"base_path":        baseDir,
			"source_path":      sourcePath,
			"destination_path": copyPath,
			"mode":             0o600,
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), copyIntent)
	require.NoError(t, err)

	r := New(q, 10*time.Millisecond, "", nil)
	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	copyResult, err := q.ReadResult(context.Background(), copyIntent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, copyResult.Status)
	raw, err := os.ReadFile(copyPath)
	require.NoError(t, err)
	require.Equal(t, "TOKEN=abc\n", string(raw))

	removeIntent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-project-file-remove",
		RequestID: "req-project-file-remove",
		TaskType:  contract.TaskTypeProjectFileRemove,
		Payload: map[string]any{
			"base_path":        baseDir,
			"path":             copyPath,
			"ignore_not_exist": true,
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	_, err = q.WriteIntent(context.Background(), removeIntent)
	require.NoError(t, err)

	err = r.ProcessOnce(context.Background())
	require.NoError(t, err)

	removeResult, err := q.ReadResult(context.Background(), removeIntent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, removeResult.Status)
	_, err = os.Stat(copyPath)
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestValidateTaskCoverageIncludesRestartTunnel(t *testing.T) {
	t.Parallel()

	r := New(nil, 10*time.Millisecond, "", nil)
	err := r.ValidateTaskCoverage([]contract.TaskType{
		contract.TaskTypeRestartTunnel,
		contract.TaskTypeDockerStopContainer,
		contract.TaskTypeDockerRestartContainer,
		contract.TaskTypeDockerRemoveContainer,
		contract.TaskTypeDockerListContainers,
		contract.TaskTypeDockerSystemDF,
		contract.TaskTypeDockerListVolumes,
		contract.TaskTypeDockerContainerLogs,
		contract.TaskTypeDockerRuntimeCheck,
		contract.TaskTypeDockerRunQuickService,
		contract.TaskTypeHostListenTCPPorts,
		contract.TaskTypeDockerPublishedPorts,
		contract.TaskTypeComposeUpStack,
		contract.TaskTypeHostRuntimeStats,
		contract.TaskTypeProjectFileWriteAtomic,
		contract.TaskTypeProjectFileCopy,
		contract.TaskTypeProjectFileRemove,
	})
	require.NoError(t, err)
}

func TestResolveTunnelRunIdentitySkipsWhenNotRoot(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("tunnel: test\n"), 0o644))

	identity, err := resolveTunnelRunIdentity(configPath, 1000)
	require.NoError(t, err)
	require.Nil(t, identity)
}

func TestResolveTunnelRunIdentityUsesConfigOwner(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("tunnel: test\n"), 0o644))

	info, err := os.Stat(configPath)
	require.NoError(t, err)
	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)

	identity, err := resolveTunnelRunIdentity(configPath, 0)
	require.NoError(t, err)
	if stat.Uid == 0 {
		require.Nil(t, identity)
		return
	}
	require.NotNil(t, identity)
	require.Equal(t, uint32(stat.Uid), identity.uid)
	require.Equal(t, uint32(stat.Gid), identity.gid)
}

func TestWithTunnelRunIdentityEnv(t *testing.T) {
	t.Parallel()

	identity := &tunnelRunIdentity{
		uid:      1000,
		gid:      1000,
		username: "joaod",
		homeDir:  "/home/joaod",
	}
	env := withTunnelRunIdentityEnv([]string{
		"PATH=/usr/bin",
		"USER=root",
		"LOGNAME=root",
		"HOME=/root",
	}, identity)

	joined := strings.Join(env, "\n")
	require.Contains(t, joined, "USER=joaod")
	require.Contains(t, joined, "LOGNAME=joaod")
	require.Contains(t, joined, "HOME=/home/joaod")
	require.Contains(t, joined, "PATH=/usr/bin")
}

func TestResolveTunnelRunIdentityMissingConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "missing.yml")
	identity, err := resolveTunnelRunIdentity(configPath, 0)
	require.NoError(t, err)
	require.Nil(t, identity)
}

func TestResolveTunnelRunIdentityLookupFallback(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("tunnel: test\n"), 0o644))

	info, err := os.Stat(configPath)
	require.NoError(t, err)
	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)
	if stat.Uid == 0 {
		t.Skip("config owner is root in this environment")
	}

	identity, err := resolveTunnelRunIdentity(configPath, 0)
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.Equal(t, strconv.FormatUint(uint64(identity.uid), 10), strconv.FormatUint(uint64(stat.Uid), 10))
}
