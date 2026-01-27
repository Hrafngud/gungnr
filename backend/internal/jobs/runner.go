package jobs

import (
	"context"
	"errors"
	"fmt"
	"go-notes/internal/models"
	"go-notes/internal/repository"
	"log"
	"sync"
	"time"
)

var ErrHandlerMissing = errors.New("job handler not registered")

type Logger interface {
	Log(line string)
	Logf(format string, args ...any)
}

type Handler func(ctx context.Context, job models.Job, logger Logger) error

type Runner struct {
	repo     repository.JobRepository
	handlers map[string]Handler
	mu       sync.RWMutex
}

func NewRunner(repo repository.JobRepository) *Runner {
	return &Runner{
		repo:     repo,
		handlers: make(map[string]Handler),
	}
}

func (r *Runner) Register(jobType string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[jobType] = handler
}

func (r *Runner) Enqueue(_ context.Context, job models.Job) error {
	r.mu.RLock()
	handler := r.handlers[job.Type]
	r.mu.RUnlock()
	if handler == nil {
		return ErrHandlerMissing
	}
	go r.run(job, handler)
	return nil
}

func (r *Runner) run(job models.Job, handler Handler) {
	ctx := context.Background()
	startedAt := time.Now()
	if err := r.repo.MarkRunning(ctx, job.ID, startedAt); err != nil {
		log.Printf("job %d start update failed: %v", job.ID, err)
	}

	logger := &jobLogger{repo: r.repo, jobID: job.ID}
	logger.Logf("job %d (%s) started", job.ID, job.Type)
	var handlerErr error
	defer func() {
		if recovered := recover(); recovered != nil {
			handlerErr = fmt.Errorf("panic: %v", recovered)
		}

		status := "completed"
		errMsg := ""
		if handlerErr != nil {
			status = "failed"
			errMsg = handlerErr.Error()
			logger.Logf("job %d failed: %s", job.ID, errMsg)
		} else {
			logger.Logf("job %d completed", job.ID)
		}

		if err := r.repo.MarkFinished(ctx, job.ID, status, time.Now(), errMsg); err != nil {
			log.Printf("job %d finish update failed: %v", job.ID, err)
		}
	}()
	handlerErr = handler(ctx, job, logger)
}

type jobLogger struct {
	repo  repository.JobRepository
	jobID uint
}

func (l *jobLogger) Log(line string) {
	if line == "" {
		return
	}
	entry := fmt.Sprintf("%s\n", line)
	if err := l.repo.AppendLog(context.Background(), l.jobID, entry); err != nil {
		log.Printf("job %d log append failed: %v", l.jobID, err)
	}
}

func (l *jobLogger) Logf(format string, args ...any) {
	l.Log(fmt.Sprintf(format, args...))
}
