package prompts

import (
	"context"
	"strings"
)

type Prompt struct {
	Label     string
	Help      []string
	Default   string
	Secret    bool
	Validate  func(string) error
	Normalize func(string) string
}

type Prompter interface {
	Prompt(ctx context.Context, prompt Prompt) (string, error)
}

func Apply(prompt Prompt, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" && prompt.Default != "" {
		trimmed = prompt.Default
	}
	if prompt.Normalize != nil {
		trimmed = prompt.Normalize(trimmed)
	}
	if prompt.Validate != nil {
		if err := prompt.Validate(trimmed); err != nil {
			return "", err
		}
	}
	return trimmed, nil
}
