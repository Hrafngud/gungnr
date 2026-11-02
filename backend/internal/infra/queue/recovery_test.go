package queue

import (
	"context"
	"github.com/stretchr/testify/require"
	"go-notes/internal/infra/contract"
	"testing"
	"time"
)

func TestRecoverInterruptedPreservesQueuedAndTerminalTasks(t *testing.T) {
	ctx := context.Background()
	q, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)
	for _, id := range []string{"queued", "running", "succeeded"} {
		_, err = q.WriteIntent(ctx, contract.Intent{IntentID: id, TaskType: contract.TaskTypeDockerStopContainer, CreatedAt: time.Now()})
		require.NoError(t, err)
	}
	_, claimed, err := q.ClaimIntent(ctx, "running", "old-worker")
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = q.WriteResult(ctx, contract.Result{IntentID: "running", Status: contract.StatusRunning})
	require.NoError(t, err)
	_, err = q.WriteResult(ctx, contract.Result{IntentID: "succeeded", Status: contract.StatusSucceeded})
	require.NoError(t, err)
	recovered, err := q.RecoverInterrupted(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	result, err := q.ReadResult(ctx, "running")
	require.NoError(t, err)
	require.Equal(t, contract.StatusFailed, result.Status)
	require.False(t, result.Error.Retryable)
	result, err = q.ReadResult(ctx, "succeeded")
	require.NoError(t, err)
	require.Equal(t, contract.StatusSucceeded, result.Status)
	_, err = q.ReadResult(ctx, "queued")
	require.Error(t, err)
	require.NoFileExists(t, q.ClaimPath("running"))
}
