package queue

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go-notes/internal/infra/contract"

	"github.com/stretchr/testify/require"
)

func TestWriteAndReadIntent(t *testing.T) {
	t.Parallel()

	q, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	intent := contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-1",
		RequestID: "req-1",
		TaskType:  contract.TaskTypeRestartTunnel,
		Payload: map[string]any{
			"config_path": "/home/user/.cloudflared/config.yml",
		},
		CreatedAt: time.Now().UTC().Round(time.Second),
	}

	path, err := q.WriteIntent(context.Background(), intent)
	require.NoError(t, err)
	require.FileExists(t, path)

	loaded, err := q.ReadIntent(context.Background(), intent.IntentID)
	require.NoError(t, err)
	require.Equal(t, intent.IntentID, loaded.IntentID)
	require.Equal(t, intent.RequestID, loaded.RequestID)
	require.Equal(t, intent.TaskType, loaded.TaskType)
	require.Equal(t, intent.Payload["config_path"], loaded.Payload["config_path"])

	tmpMatches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".tmp-*.json"))
	require.NoError(t, err)
	require.Empty(t, tmpMatches)
}

func TestClaimIntentExclusive(t *testing.T) {
	t.Parallel()

	q, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	const workers = 8
	type claimResult struct {
		claimed bool
		err     error
	}
	results := make([]claimResult, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, claimed, err := q.ClaimIntent(context.Background(), "intent-exclusive", fmt.Sprintf("worker-%d", idx))
			results[idx] = claimResult{claimed: claimed, err: err}
		}(i)
	}
	wg.Wait()

	claimedCount := 0
	for _, result := range results {
		require.NoError(t, result.err)
		if result.claimed {
			claimedCount++
		}
	}
	require.Equal(t, 1, claimedCount)
	require.FileExists(t, q.ClaimPath("intent-exclusive"))
}

func TestWriteAndReadResult(t *testing.T) {
	t.Parallel()

	q, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	running := contract.Result{
		Version:    contract.VersionV1,
		IntentID:   "intent-result",
		RequestID:  "req-result",
		TaskType:   contract.TaskTypeRestartTunnel,
		Status:     contract.StatusRunning,
		CreatedAt:  time.Now().UTC(),
		StartedAt:  time.Now().UTC(),
		FinishedAt: time.Time{},
		LogPath:    "/tmp/worker.log",
	}
	_, err = q.WriteResult(context.Background(), running)
	require.NoError(t, err)

	loadedRunning, err := q.ReadResult(context.Background(), "intent-result")
	require.NoError(t, err)
	require.Equal(t, contract.StatusRunning, loadedRunning.Status)

	succeeded := loadedRunning
	succeeded.Status = contract.StatusSucceeded
	succeeded.FinishedAt = time.Now().UTC()
	_, err = q.WriteResult(context.Background(), succeeded)
	require.NoError(t, err)

	loadedSucceeded, err := q.ReadResult(context.Background(), "intent-result")
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, loadedSucceeded.Status)
	require.False(t, loadedSucceeded.FinishedAt.IsZero())
}

func TestListIntentIDs(t *testing.T) {
	t.Parallel()

	q, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	_, err = q.WriteIntent(context.Background(), contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-b",
		RequestID: "req-b",
		TaskType:  contract.TaskTypeRestartTunnel,
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	_, err = q.WriteIntent(context.Background(), contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  "intent-a",
		RequestID: "req-a",
		TaskType:  contract.TaskTypeRestartTunnel,
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	ids, err := q.ListIntentIDs(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"intent-a", "intent-b"}, ids)
}
