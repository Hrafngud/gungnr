package client

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go-notes/internal/infra/contract"
	"go-notes/internal/infra/queue"

	"github.com/stretchr/testify/require"
)

func TestSubmitAndWaitResultSuccess(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	c := New(q, 10*time.Millisecond, 2*time.Second)
	intent, err := c.SubmitIntent(context.Background(), "req-123", contract.TaskTypeRestartTunnel, map[string]any{
		"config_path": "/tmp/cloudflared/config.yml",
	})
	require.NoError(t, err)
	require.NotEmpty(t, intent.IntentID)

	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = q.WriteResult(context.Background(), contract.Result{
			Version:    contract.VersionV1,
			IntentID:   intent.IntentID,
			RequestID:  intent.RequestID,
			TaskType:   contract.TaskTypeRestartTunnel,
			Status:     contract.StatusSucceeded,
			CreatedAt:  intent.CreatedAt,
			StartedAt:  time.Now().UTC().Add(-10 * time.Millisecond),
			FinishedAt: time.Now().UTC(),
			LogPath:    "/tmp/worker.log",
		})
	}()

	result, err := c.WaitResult(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	require.Equal(t, intent.IntentID, result.IntentID)
}

func TestWaitResultTimeout(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	c := New(q, 10*time.Millisecond, 80*time.Millisecond)
	_, err = c.WaitResult(context.Background(), "intent-timeout")
	require.Error(t, err)

	var timeoutErr *TimeoutError
	require.True(t, errors.As(err, &timeoutErr))
	require.Equal(t, "intent-timeout", timeoutErr.IntentID)
}

func TestWaitResultHonorsCallerDeadlineOverDefaultTimeout(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	c := New(q, 10*time.Millisecond, 50*time.Millisecond)
	intent, err := c.SubmitIntent(context.Background(), "req-long", contract.TaskTypeRestartTunnel, map[string]any{
		"config_path": "/tmp/cloudflared/config.yml",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()

	go func() {
		time.Sleep(120 * time.Millisecond)
		_, _ = q.WriteResult(context.Background(), contract.Result{
			Version:    contract.VersionV1,
			IntentID:   intent.IntentID,
			RequestID:  intent.RequestID,
			TaskType:   contract.TaskTypeRestartTunnel,
			Status:     contract.StatusSucceeded,
			CreatedAt:  intent.CreatedAt,
			StartedAt:  time.Now().UTC().Add(-10 * time.Millisecond),
			FinishedAt: time.Now().UTC(),
		})
	}()

	result, err := c.WaitResult(ctx, intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
}

func TestWaitResultMalformedFile(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	c := New(q, 10*time.Millisecond, 500*time.Millisecond)
	err = os.WriteFile(q.ResultPath("intent-bad"), []byte("{not-json"), 0o644)
	require.NoError(t, err)

	_, err = c.WaitResult(context.Background(), "intent-bad")
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode result")
}

func TestLoadResultMissing(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	c := New(q, 10*time.Millisecond, 500*time.Millisecond)
	_, err = c.LoadResult(context.Background(), "intent-missing")
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestRestartTunnelOmitsLegacyHealthURL(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	c := New(q, 10*time.Millisecond, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			ids, listErr := q.ListIntentIDs(ctx)
			if listErr != nil || len(ids) == 0 {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			intent, readErr := q.ReadIntent(ctx, ids[0])
			if readErr != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			_, _ = q.WriteResult(ctx, contract.Result{
				Version:    contract.VersionV1,
				IntentID:   intent.IntentID,
				RequestID:  intent.RequestID,
				TaskType:   intent.TaskType,
				Status:     contract.StatusSucceeded,
				CreatedAt:  intent.CreatedAt,
				StartedAt:  time.Now().UTC().Add(-10 * time.Millisecond),
				FinishedAt: time.Now().UTC(),
			})
			return
		}
	}()

	_, err = c.RestartTunnel(ctx, "req-rt", "/tmp/cloudflared/config.yml")
	require.NoError(t, err)
	<-done

	ids, err := q.ListIntentIDs(context.Background())
	require.NoError(t, err)
	require.Len(t, ids, 1)

	intent, err := q.ReadIntent(context.Background(), ids[0])
	require.NoError(t, err)
	require.Equal(t, "/tmp/cloudflared/config.yml", intent.Payload["config_path"])
	_, exists := intent.Payload["health_url"]
	require.False(t, exists)
}

func TestDockerContainerLogsPayload(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	c := New(q, 10*time.Millisecond, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			ids, listErr := q.ListIntentIDs(ctx)
			if listErr != nil || len(ids) == 0 {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			intent, readErr := q.ReadIntent(ctx, ids[0])
			if readErr != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			_, _ = q.WriteResult(ctx, contract.Result{
				Version:    contract.VersionV1,
				IntentID:   intent.IntentID,
				RequestID:  intent.RequestID,
				TaskType:   intent.TaskType,
				Status:     contract.StatusSucceeded,
				CreatedAt:  intent.CreatedAt,
				StartedAt:  time.Now().UTC().Add(-10 * time.Millisecond),
				FinishedAt: time.Now().UTC(),
				Data: map[string]any{
					"lines": []string{"ok"},
				},
			})
			return
		}
	}()

	_, err = c.DockerContainerLogs(ctx, "req-logs", contract.DockerContainerLogsPayload{
		Container:  "demo-api",
		Tail:       300,
		Follow:     true,
		Timestamps: true,
		Since:      "2026-03-27T16:00:00Z",
	})
	require.NoError(t, err)
	<-done

	ids, err := q.ListIntentIDs(context.Background())
	require.NoError(t, err)
	require.Len(t, ids, 1)

	intent, err := q.ReadIntent(context.Background(), ids[0])
	require.NoError(t, err)
	require.Equal(t, contract.TaskTypeDockerContainerLogs, intent.TaskType)
	require.Equal(t, "demo-api", intent.Payload["container"])
	require.Equal(t, float64(300), intent.Payload["tail"])
	require.Equal(t, true, intent.Payload["follow"])
	require.Equal(t, true, intent.Payload["timestamps"])
	require.Equal(t, "2026-03-27T16:00:00Z", intent.Payload["since"])
}

func TestProbeTaskPayloads(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	c := New(q, 10*time.Millisecond, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ack := make(chan struct{}, 2)
	go func() {
		for {
			ids, listErr := q.ListIntentIDs(ctx)
			if listErr != nil || len(ids) == 0 {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			for _, id := range ids {
				resultPath := q.ResultPath(id)
				if _, statErr := os.Stat(resultPath); statErr == nil {
					continue
				}
				intent, readErr := q.ReadIntent(ctx, id)
				if readErr != nil {
					continue
				}
				_, _ = q.WriteResult(ctx, contract.Result{
					Version:    contract.VersionV1,
					IntentID:   intent.IntentID,
					RequestID:  intent.RequestID,
					TaskType:   intent.TaskType,
					Status:     contract.StatusSucceeded,
					CreatedAt:  intent.CreatedAt,
					StartedAt:  time.Now().UTC().Add(-10 * time.Millisecond),
					FinishedAt: time.Now().UTC(),
					Data: map[string]any{
						"lines": []string{"ok"},
					},
				})
				ack <- struct{}{}
			}
			if len(ack) >= 2 {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	_, err = c.HostListenTCPPorts(ctx, "req-host-listen")
	require.NoError(t, err)
	_, err = c.DockerPublishedPorts(ctx, "req-docker-published")
	require.NoError(t, err)

	<-ack
	<-ack

	ids, err := q.ListIntentIDs(context.Background())
	require.NoError(t, err)
	require.Len(t, ids, 2)

	hostIntentFound := false
	dockerIntentFound := false
	for _, id := range ids {
		intent, readErr := q.ReadIntent(context.Background(), id)
		require.NoError(t, readErr)
		switch intent.TaskType {
		case contract.TaskTypeHostListenTCPPorts:
			hostIntentFound = true
			require.Equal(t, "req-host-listen", intent.RequestID)
			require.Empty(t, intent.Payload)
		case contract.TaskTypeDockerPublishedPorts:
			dockerIntentFound = true
			require.Equal(t, "req-docker-published", intent.RequestID)
			require.Empty(t, intent.Payload)
		}
	}
	require.True(t, hostIntentFound)
	require.True(t, dockerIntentFound)
}

func TestDockerRunnerTaskPayloads(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	c := New(q, 10*time.Millisecond, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ack := make(chan struct{}, 2)
	go func() {
		for {
			ids, listErr := q.ListIntentIDs(ctx)
			if listErr != nil || len(ids) == 0 {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			for _, id := range ids {
				resultPath := q.ResultPath(id)
				if _, statErr := os.Stat(resultPath); statErr == nil {
					continue
				}
				intent, readErr := q.ReadIntent(ctx, id)
				if readErr != nil {
					continue
				}
				_, _ = q.WriteResult(ctx, contract.Result{
					Version:    contract.VersionV1,
					IntentID:   intent.IntentID,
					RequestID:  intent.RequestID,
					TaskType:   intent.TaskType,
					Status:     contract.StatusSucceeded,
					CreatedAt:  intent.CreatedAt,
					StartedAt:  time.Now().UTC().Add(-10 * time.Millisecond),
					FinishedAt: time.Now().UTC(),
					Data: map[string]any{
						"lines": []string{"ok"},
					},
				})
				ack <- struct{}{}
			}
			if len(ack) >= 2 {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	_, err = c.DockerRuntimeCheck(ctx, "req-docker-runtime-check")
	require.NoError(t, err)
	_, err = c.DockerRunQuickService(ctx, "req-docker-run-quick-service", contract.DockerRunQuickServicePayload{
		Image:         "excalidraw/excalidraw:latest",
		HostPort:      19000,
		ContainerPort: 80,
		ContainerName: "quick-excalidraw",
	})
	require.NoError(t, err)

	<-ack
	<-ack

	ids, err := q.ListIntentIDs(context.Background())
	require.NoError(t, err)
	require.Len(t, ids, 2)

	runtimeIntentFound := false
	quickRunIntentFound := false
	for _, id := range ids {
		intent, readErr := q.ReadIntent(context.Background(), id)
		require.NoError(t, readErr)
		switch intent.TaskType {
		case contract.TaskTypeDockerRuntimeCheck:
			runtimeIntentFound = true
			require.Equal(t, "req-docker-runtime-check", intent.RequestID)
			require.Empty(t, intent.Payload)
		case contract.TaskTypeDockerRunQuickService:
			quickRunIntentFound = true
			require.Equal(t, "req-docker-run-quick-service", intent.RequestID)
			require.Equal(t, "excalidraw/excalidraw:latest", intent.Payload["image"])
			require.Equal(t, float64(19000), intent.Payload["host_port"])
			require.Equal(t, float64(80), intent.Payload["container_port"])
			require.Equal(t, "quick-excalidraw", intent.Payload["container_name"])
		}
	}
	require.True(t, runtimeIntentFound)
	require.True(t, quickRunIntentFound)
}

func TestProjectFileTaskPayloads(t *testing.T) {
	t.Parallel()

	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)

	c := New(q, 10*time.Millisecond, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ack := make(chan struct{}, 3)
	go func() {
		for {
			ids, listErr := q.ListIntentIDs(ctx)
			if listErr != nil || len(ids) == 0 {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			for _, id := range ids {
				resultPath := q.ResultPath(id)
				if _, statErr := os.Stat(resultPath); statErr == nil {
					continue
				}
				intent, readErr := q.ReadIntent(ctx, id)
				if readErr != nil {
					continue
				}
				_, _ = q.WriteResult(ctx, contract.Result{
					Version:    contract.VersionV1,
					IntentID:   intent.IntentID,
					RequestID:  intent.RequestID,
					TaskType:   intent.TaskType,
					Status:     contract.StatusSucceeded,
					CreatedAt:  intent.CreatedAt,
					StartedAt:  time.Now().UTC().Add(-10 * time.Millisecond),
					FinishedAt: time.Now().UTC(),
				})
				ack <- struct{}{}
			}
			if len(ack) >= 3 {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	_, err = c.ProjectFileWriteAtomic(ctx, "req-file-write", contract.ProjectFileWriteAtomicPayload{
		BasePath:      "/templates/demo",
		Path:          "/templates/demo/.env",
		Content:       "A=1\n",
		Mode:          0o600,
		PreserveMode:  false,
		CreateParents: true,
	})
	require.NoError(t, err)

	_, err = c.ProjectFileCopy(ctx, "req-file-copy", contract.ProjectFileCopyPayload{
		BasePath:        "/templates/demo",
		SourcePath:      "/templates/demo/.env",
		DestinationPath: "/templates/demo/.env.backup",
		Mode:            0o600,
		CreateParents:   true,
	})
	require.NoError(t, err)

	_, err = c.ProjectFileRemove(ctx, "req-file-remove", contract.ProjectFileRemovePayload{
		BasePath:       "/templates/demo",
		Path:           "/templates/demo/.env.backup",
		IgnoreNotExist: true,
	})
	require.NoError(t, err)

	<-ack
	<-ack
	<-ack

	ids, err := q.ListIntentIDs(context.Background())
	require.NoError(t, err)
	require.Len(t, ids, 3)

	writeFound := false
	copyFound := false
	removeFound := false
	for _, id := range ids {
		intent, readErr := q.ReadIntent(context.Background(), id)
		require.NoError(t, readErr)
		switch intent.TaskType {
		case contract.TaskTypeProjectFileWriteAtomic:
			writeFound = true
			require.Equal(t, "req-file-write", intent.RequestID)
			require.Equal(t, "/templates/demo", intent.Payload["base_path"])
			require.Equal(t, "/templates/demo/.env", intent.Payload["path"])
			require.Equal(t, "A=1\n", intent.Payload["content"])
			require.Equal(t, float64(0o600), intent.Payload["mode"])
			require.Equal(t, true, intent.Payload["create_parents"])
		case contract.TaskTypeProjectFileCopy:
			copyFound = true
			require.Equal(t, "req-file-copy", intent.RequestID)
			require.Equal(t, "/templates/demo", intent.Payload["base_path"])
			require.Equal(t, "/templates/demo/.env", intent.Payload["source_path"])
			require.Equal(t, "/templates/demo/.env.backup", intent.Payload["destination_path"])
			require.Equal(t, float64(0o600), intent.Payload["mode"])
			require.Equal(t, true, intent.Payload["create_parents"])
		case contract.TaskTypeProjectFileRemove:
			removeFound = true
			require.Equal(t, "req-file-remove", intent.RequestID)
			require.Equal(t, "/templates/demo", intent.Payload["base_path"])
			require.Equal(t, "/templates/demo/.env.backup", intent.Payload["path"])
			require.Equal(t, true, intent.Payload["ignore_not_exist"])
		}
	}
	require.True(t, writeFound)
	require.True(t, copyFound)
	require.True(t, removeFound)
}
