package service

import (
	"context"
	"testing"
	"time"

	"go-notes/internal/models"
	"go-notes/internal/repository"

	"github.com/stretchr/testify/require"
)

type recoveryJobRepo struct {
	repository.JobRepository
	jobs     []models.Job
	finished []uint
	logs     []uint
}

func (r *recoveryJobRepo) List(context.Context) ([]models.Job, error) { return r.jobs, nil }
func (r *recoveryJobRepo) MarkFinished(_ context.Context, id uint, status string, at time.Time, message string) error {
	if status != "failed" || at.IsZero() || message == "" {
		panic("invalid interrupted-job outcome")
	}
	r.finished = append(r.finished, id)
	return nil
}
func (r *recoveryJobRepo) AppendLog(_ context.Context, id uint, _ string) error {
	r.logs = append(r.logs, id)
	return nil
}

func TestRecoverInterruptedJobsPreservesTerminalJobs(t *testing.T) {
	repo := &recoveryJobRepo{jobs: []models.Job{
		{Status: "running"}, {Status: "pending"}, {Status: "completed"}, {Status: "failed"},
	}}
	for i := range repo.jobs {
		repo.jobs[i].ID = uint(i + 1)
	}
	count, err := NewJobService(repo, nil).RecoverInterruptedJobs(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, []uint{1, 2}, repo.finished)
	require.Equal(t, []uint{1, 2}, repo.logs)
}
