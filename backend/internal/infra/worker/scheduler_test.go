package worker

import (
	"context"
	"github.com/stretchr/testify/require"
	"go-notes/internal/infra/contract"
	"go-notes/internal/infra/queue"
	"io"
	"log"
	"sync/atomic"
	"testing"
	"time"
)

type schedulingExecutor struct {
	started chan string
	release chan struct{}
	reads   atomic.Int32
}

func (e *schedulingExecutor) Run(ctx context.Context, req commandRequest) ([]byte, error) {
	if req.Args[0] == "stop" {
		e.started <- req.Args[1]
		select {
		case <-e.release:
			return []byte("stopped"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	e.reads.Add(1)
	return []byte(""), nil
}

func TestRunReadsBypassMutationsAndMutationsKeepEnqueueOrder(t *testing.T) {
	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)
	now := time.Now().UTC()
	for _, intent := range []contract.Intent{
		{IntentID: "z-oldest", TaskType: contract.TaskTypeDockerStopContainer, CreatedAt: now.Add(-time.Minute), Payload: map[string]any{"container": "first"}},
		{IntentID: "a-next", TaskType: contract.TaskTypeDockerStopContainer, CreatedAt: now, Payload: map[string]any{"container": "second"}},
	} {
		_, err := q.WriteIntent(context.Background(), intent)
		require.NoError(t, err)
	}
	executor := &schedulingExecutor{started: make(chan string, 2), release: make(chan struct{})}
	r := New(q, 5*time.Millisecond, "", log.New(io.Discard, "", 0))
	r.exec = executor
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); r.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("worker did not stop")
		}
	})
	select {
	case name := <-executor.started:
		require.Equal(t, "first", name)
	case <-time.After(time.Second):
		t.Fatal("first mutation did not start")
	}
	_, err = q.WriteIntent(ctx, contract.Intent{IntentID: "read", TaskType: contract.TaskTypeDockerListContainers, CreatedAt: time.Now(), Payload: map[string]any{}})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		res, err := q.ReadResult(ctx, "read")
		return err == nil && res.Status == contract.StatusSucceeded
	}, time.Second, 5*time.Millisecond)
	require.Equal(t, int32(1), executor.reads.Load())
	select {
	case <-executor.started:
		t.Fatal("mutations overlapped")
	default:
	}
	close(executor.release)
	select {
	case name := <-executor.started:
		require.Equal(t, "second", name)
	case <-time.After(time.Second):
		t.Fatal("next mutation did not start")
	}
}

func TestCanceledQueuedReadDoesNotExecuteButMutationCannotBeCanceled(t *testing.T) {
	q, err := queue.NewFilesystem(t.TempDir())
	require.NoError(t, err)
	for _, intent := range []contract.Intent{
		{IntentID: "read", TaskType: contract.TaskTypeDockerListContainers, Payload: map[string]any{}},
		{IntentID: "stop", TaskType: contract.TaskTypeDockerStopContainer, Payload: map[string]any{"container": "api"}},
	} {
		_, err := q.WriteIntent(context.Background(), intent)
		require.NoError(t, err)
		require.NoError(t, q.CancelRead(context.Background(), intent.IntentID))
	}
	require.True(t, q.ReadCanceled("read"))
	require.False(t, q.ReadCanceled("stop"))
	executor := &fakeExecutor{output: []byte("api")}
	r := New(q, time.Millisecond, "", nil)
	r.exec = executor
	require.NoError(t, r.ProcessOnce(context.Background()))
	require.Len(t, executor.calls, 1)
	require.Equal(t, "stop", executor.calls[0].args[0])
	result, err := q.ReadResult(context.Background(), "read")
	require.NoError(t, err)
	require.Equal(t, contract.StatusFailed, result.Status)
	require.Equal(t, "INFRA-408-CANCELED", result.Error.Code)
	require.False(t, q.ReadCanceled("read"))
}
