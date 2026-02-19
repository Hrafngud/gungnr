package queue

import (
	"context"
	"os"
	"testing"
	"time"

	"go-notes/internal/infra/contract"

	"github.com/stretchr/testify/require"
)

func TestCleanupStaleRemovesOnlyEligibleArtifacts(t *testing.T) {
	t.Parallel()

	q, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	now := time.Now().UTC().Round(time.Second)
	old := now.Add(-2 * time.Hour)
	fresh := now.Add(-20 * time.Minute)

	writeIntent(t, q, "done-old")
	writeResult(t, q, "done-old", contract.StatusSucceeded, old)

	writeIntent(t, q, "done-fresh")
	writeResult(t, q, "done-fresh", contract.StatusSucceeded, fresh)

	writeIntent(t, q, "running-old")
	writeResult(t, q, "running-old", contract.StatusRunning, old)

	writeIntent(t, q, "queued-claimed")
	claimPath := writeClaim(t, q, "queued-claimed")
	setModTime(t, claimPath, old)

	orphanClaimPath := writeClaim(t, q, "orphan-claim")
	setModTime(t, orphanClaimPath, old)

	writeIntent(t, q, "done-with-claim")
	writeResult(t, q, "done-with-claim", contract.StatusSucceeded, old)
	doneClaimPath := writeClaim(t, q, "done-with-claim")
	setModTime(t, doneClaimPath, old)

	report, err := q.CleanupStale(context.Background(), now, RetentionPolicy{
		IntentMaxAge: time.Hour,
		ResultMaxAge: time.Hour,
		ClaimMaxAge:  time.Hour,
	})
	require.NoError(t, err)

	require.Equal(t, 2, report.RemovedIntents)
	require.Equal(t, 2, report.RemovedResults)
	require.Equal(t, 2, report.RemovedClaims)
	require.Equal(t, 2, report.ProtectedTasks)

	require.NoFileExists(t, q.IntentPath("done-old"))
	require.NoFileExists(t, q.ResultPath("done-old"))
	require.NoFileExists(t, q.IntentPath("done-with-claim"))
	require.NoFileExists(t, q.ResultPath("done-with-claim"))
	require.NoFileExists(t, q.ClaimPath("done-with-claim"))
	require.NoFileExists(t, q.ClaimPath("orphan-claim"))

	require.FileExists(t, q.IntentPath("done-fresh"))
	require.FileExists(t, q.ResultPath("done-fresh"))
	require.FileExists(t, q.IntentPath("running-old"))
	require.FileExists(t, q.ResultPath("running-old"))
	require.FileExists(t, q.IntentPath("queued-claimed"))
	require.FileExists(t, q.ClaimPath("queued-claimed"))
}

func TestCleanupStaleKeepsClaimForActiveTask(t *testing.T) {
	t.Parallel()

	q, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	now := time.Now().UTC().Round(time.Second)
	old := now.Add(-4 * time.Hour)

	writeIntent(t, q, "active-claimed")
	claimPath := writeClaim(t, q, "active-claimed")
	setModTime(t, claimPath, old)

	report, err := q.CleanupStale(context.Background(), now, RetentionPolicy{
		IntentMaxAge: time.Hour,
		ResultMaxAge: time.Hour,
		ClaimMaxAge:  time.Hour,
	})
	require.NoError(t, err)
	require.Equal(t, 0, report.TotalRemoved())
	require.Equal(t, 1, report.ProtectedTasks)
	require.FileExists(t, q.IntentPath("active-claimed"))
	require.FileExists(t, q.ClaimPath("active-claimed"))
}

func writeIntent(t *testing.T, q *Filesystem, intentID string) {
	t.Helper()
	_, err := q.WriteIntent(context.Background(), contract.Intent{
		Version:   contract.VersionV1,
		IntentID:  intentID,
		RequestID: "req-" + intentID,
		TaskType:  contract.TaskTypeRestartTunnel,
		CreatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
}

func writeResult(t *testing.T, q *Filesystem, intentID string, status contract.Status, finishedAt time.Time) {
	t.Helper()
	result := contract.Result{
		Version:   contract.VersionV1,
		IntentID:  intentID,
		RequestID: "req-" + intentID,
		TaskType:  contract.TaskTypeRestartTunnel,
		Status:    status,
		CreatedAt: time.Now().UTC().Add(-3 * time.Hour),
		StartedAt: time.Now().UTC().Add(-2 * time.Hour),
	}
	if contract.IsTerminalStatus(status) {
		result.FinishedAt = finishedAt
	}
	_, err := q.WriteResult(context.Background(), result)
	require.NoError(t, err)
}

func writeClaim(t *testing.T, q *Filesystem, intentID string) string {
	t.Helper()
	_, claimed, err := q.ClaimIntent(context.Background(), intentID, "worker-test")
	require.NoError(t, err)
	require.True(t, claimed)
	return q.ClaimPath(intentID)
}

func setModTime(t *testing.T, path string, timestamp time.Time) {
	t.Helper()
	err := os.Chtimes(path, timestamp, timestamp)
	require.NoError(t, err)
}
