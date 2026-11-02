package worker

import (
	"context"
	"errors"
	"go-notes/internal/infra/contract"
	"os"
	"sort"
	"sync"
	"time"
)

type terminalRecord struct {
	task     contract.TaskType
	finished time.Time
}

func taskLane(task contract.TaskType) string {
	if task == contract.TaskTypeHostRuntimeStream {
		return "telemetry"
	}
	if task.ReadOnly() {
		return "read"
	}
	return "mutation"
}

// One dispatcher owns discovery. Atomic filesystem claims remain authoritative.
func (r *Runner) dispatch(ctx context.Context, lanes map[string]chan struct{}, active *sync.WaitGroup) error {
	ids, err := r.queue.ListIntentIDs(ctx)
	if err != nil {
		return err
	}
	pending := make([]contract.Intent, 0)
	for _, id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, done := r.terminal.Load(id); done {
			continue
		}
		result, err := r.queue.ReadResult(ctx, id)
		if err == nil {
			if result.Terminal() {
				r.terminal.Store(id, terminalRecord{result.TaskType, result.FinishedAt})
				continue
			}
			if result.Status == contract.StatusRunning {
				continue
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			continue
		}
		intent, err := r.queue.ReadIntent(ctx, id)
		if err == nil && r.supportsTask(intent.TaskType) {
			pending = append(pending, intent)
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].CreatedAt.Equal(pending[j].CreatedAt) {
			return pending[i].IntentID < pending[j].IntentID
		}
		return pending[i].CreatedAt.Before(pending[j].CreatedAt)
	})
	for _, intent := range pending {
		lane := lanes[taskLane(intent.TaskType)]
		select {
		case lane <- struct{}{}:
		default:
			continue
		}
		_, claimed, err := r.queue.ClaimIntent(ctx, intent.IntentID, r.owner)
		if err != nil || !claimed {
			<-lane
			continue
		}
		active.Add(1)
		go func(intent contract.Intent) {
			defer active.Done()
			defer func() { <-lane }()
			if err := r.handleIntent(ctx, intent); err != nil {
				r.logger.Printf("warn: infra task %s failed: %v", intent.IntentID, err)
				return
			}
			r.terminal.Store(intent.IntentID, terminalRecord{intent.TaskType, time.Now().UTC()})
		}(intent)
	}
	return nil
}

// Read results are transient data, not lifecycle audit records. Retaining ten
// minutes bounds the history from both dashboards and permanently open log views.
func (r *Runner) pruneReadHistory(ctx context.Context) {
	before := time.Now().Add(-10 * time.Minute)
	r.terminal.Range(func(key, value any) bool {
		id := key.(string)
		record := value.(terminalRecord)
		if record.task.ReadOnly() {
			if !record.finished.IsZero() && record.finished.Before(before) {
				if err := r.queue.PruneReadResult(ctx, id, before); err == nil {
					r.terminal.Delete(id)
				}
			}
		} else if _, err := os.Stat(r.queue.IntentPath(id)); errors.Is(err, os.ErrNotExist) {
			r.terminal.Delete(id)
		}
		return ctx.Err() == nil
	})
}
