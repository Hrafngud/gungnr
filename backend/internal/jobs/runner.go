package jobs

import (
	"context"
	"errors"

	"go-notes/internal/models"
)

var ErrNotImplemented = errors.New("job runner not implemented")

type Handler func(ctx context.Context, job models.Job) error

type Runner struct {
	handlers map[string]Handler
}

func NewRunner() *Runner {
	return &Runner{
		handlers: make(map[string]Handler),
	}
}

func (r *Runner) Register(jobType string, handler Handler) {
	r.handlers[jobType] = handler
}

func (r *Runner) Enqueue(_ context.Context, _ models.Job) error {
	return ErrNotImplemented
}
