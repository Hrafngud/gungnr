package app

import (
	"context"
)

type Step interface {
	ID() string
	Title() string
	Run(ctx context.Context, state *State, ui UI) error
}

type StepInfo struct {
	ID    string
	Title string
}

type UI interface {
	SetSteps(steps []StepInfo)
	StepStart(id string)
	StepProgress(id, message string)
	StepDone(id, message string)
	StepError(id string, err error)
	Info(message string)
	Warn(message string)
	Prompt(ctx context.Context, prompt Prompt) (string, error)
	FinalSummary(summary Summary)
}

type Runner struct {
	Steps []Step
	State *State
	UI    UI
}

func NewRunner(ui UI, steps []Step) *Runner {
	return &Runner{
		Steps: steps,
		State: &State{},
		UI:    ui,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	stepInfos := make([]StepInfo, 0, len(r.Steps))
	for _, step := range r.Steps {
		stepInfos = append(stepInfos, StepInfo{ID: step.ID(), Title: step.Title()})
	}
	r.UI.SetSteps(stepInfos)

	for _, step := range r.Steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.UI.StepStart(step.ID())
		if err := step.Run(ctx, r.State, r.UI); err != nil {
			r.UI.StepError(step.ID(), err)
			return err
		}
		if err := ctx.Err(); err != nil {
			r.UI.StepError(step.ID(), err)
			return err
		}
		r.UI.StepDone(step.ID(), "completed")
	}

	r.UI.FinalSummary(r.State.Summary())
	return nil
}
