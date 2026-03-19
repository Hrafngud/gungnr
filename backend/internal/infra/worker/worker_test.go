package worker

import (
	"context"
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

type fakeExecCall struct {
	dir  string
	name string
	args []string
}

type fakeExecutor struct {
	calls  []fakeExecCall
	output []byte
	err    error
}

func (f *fakeExecutor) Run(_ context.Context, dir string, name string, args ...string) ([]byte, error) {
	copiedArgs := make([]string, len(args))
	copy(copiedArgs, args)
	f.calls = append(f.calls, fakeExecCall{
		dir:  dir,
		name: name,
		args: copiedArgs,
	})
	return f.output, f.err
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
		TaskType:  contract.TaskTypeDockerRunQuickService,
		Payload: map[string]any{
			"image":          "excalidraw/excalidraw:latest",
			"host_port":      9000,
			"container_port": 80,
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

func TestValidateTaskCoverageIncludesRestartTunnel(t *testing.T) {
	t.Parallel()

	r := New(nil, 10*time.Millisecond, "", nil)
	err := r.ValidateTaskCoverage([]contract.TaskType{
		contract.TaskTypeRestartTunnel,
		contract.TaskTypeDockerStopContainer,
		contract.TaskTypeDockerRestartContainer,
		contract.TaskTypeDockerRemoveContainer,
		contract.TaskTypeComposeUpStack,
		contract.TaskTypeHostRuntimeStats,
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
