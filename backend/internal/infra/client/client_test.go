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
