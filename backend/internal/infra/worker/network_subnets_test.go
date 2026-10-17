package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go-notes/internal/infra/contract"
	"go-notes/internal/infra/queue"
)

func TestDockerNetworkSubnetsBridgeReturnsInventoryAndFailsClosed(t *testing.T) {
	for _, fail := range []bool{false, true} {
		q, err := queue.NewFilesystem(t.TempDir())
		require.NoError(t, err)
		intent := contract.Intent{Version: contract.VersionV1, IntentID: "networks", TaskType: contract.TaskTypeDockerNetworkSubnets, Payload: map[string]any{}, CreatedAt: time.Now()}
		_, err = q.WriteIntent(context.Background(), intent)
		require.NoError(t, err)
		exec := &fakeExecutor{outputs: [][]byte{[]byte("network-a\nnetwork-b\n"), []byte("172.30.90.0/24\n\n10.0.0.0/8\n")}}
		if fail {
			exec.errs = []error{nil, errors.New("inspect failed")}
		}
		runner := New(q, time.Millisecond, "", nil)
		runner.exec = exec
		require.NoError(t, runner.ValidateTaskCoverage([]contract.TaskType{contract.TaskTypeDockerNetworkSubnets}))
		require.NoError(t, runner.ProcessOnce(context.Background()))
		result, err := q.ReadResult(context.Background(), intent.IntentID)
		require.NoError(t, err)
		require.Equal(t, []string{"network", "ls", "-q"}, exec.calls[0].args)
		require.Equal(t, []string{"network-a", "network-b"}, exec.calls[1].args[4:])
		if fail {
			require.Equal(t, contract.StatusFailed, result.Status)
		} else {
			require.Equal(t, contract.StatusSucceeded, result.Status)
			require.Equal(t, []string{"172.30.90.0/24", "10.0.0.0/8"}, decodeDataLines(t, result.Data))
		}
	}
}
