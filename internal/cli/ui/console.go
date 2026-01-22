package cliui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"gungnr-cli/internal/cli/app"
	"gungnr-cli/internal/cli/prompts"
)

type ConsoleUI struct {
	out    io.Writer
	err    io.Writer
	in     *bufio.Reader
	inFile *os.File
	steps  map[string]string
}

func NewConsoleUI(out, errOut io.Writer, in io.Reader) *ConsoleUI {
	var inFile *os.File
	if file, ok := in.(*os.File); ok {
		inFile = file
	}
	reader := bufio.NewReader(in)
	return &ConsoleUI{out: out, err: errOut, in: reader, inFile: inFile, steps: map[string]string{}}
}

func (c *ConsoleUI) SetSteps(steps []app.StepInfo) {
	for _, step := range steps {
		c.steps[step.ID] = step.Title
	}
}

func (c *ConsoleUI) StepStart(id string) {
	title := c.steps[id]
	if title == "" {
		title = id
	}
	fmt.Fprintf(c.out, "\n==> %s\n", title)
}

func (c *ConsoleUI) StepProgress(id, message string) {
	fmt.Fprintf(c.out, "- %s\n", message)
}

func (c *ConsoleUI) StepDone(id, message string) {
	if message == "" {
		message = "done"
	}
	fmt.Fprintf(c.out, "- %s\n", message)
}

func (c *ConsoleUI) StepError(id string, err error) {
	fmt.Fprintf(c.err, "Error: %v\n", err)
}

func (c *ConsoleUI) Info(message string) {
	fmt.Fprintln(c.out, message)
}

func (c *ConsoleUI) Warn(message string) {
	fmt.Fprintln(c.out, message)
}

func (c *ConsoleUI) Prompt(ctx context.Context, prompt prompts.Prompt) (string, error) {
	_ = ctx
	for {
		label := prompt.Label
		if prompt.Default != "" {
			label = fmt.Sprintf("%s [%s]", prompt.Label, prompt.Default)
		}
		if len(prompt.Help) > 0 {
			for _, line := range prompt.Help {
				fmt.Fprintln(c.out, line)
			}
		}
		fmt.Fprintf(c.out, "%s: ", label)

		var value string
		var err error
		if prompt.Secret {
			value, err = c.readSecret()
			fmt.Fprintln(c.out)
		} else {
			value, err = c.in.ReadString('\n')
		}
		if err != nil {
			return "", fmt.Errorf("unable to read input: %w", err)
		}
		value = strings.TrimSpace(value)

		resolved, err := prompts.Apply(prompt, value)
		if err != nil {
			fmt.Fprintf(c.out, "%s\n", err.Error())
			continue
		}
		return resolved, nil
	}
}

func (c *ConsoleUI) readSecret() (string, error) {
	if c.inFile != nil && term.IsTerminal(int(c.inFile.Fd())) {
		bytes, err := term.ReadPassword(int(c.inFile.Fd()))
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
	return c.in.ReadString('\n')
}

func (c *ConsoleUI) FinalSummary(summary app.Summary) {
	fmt.Fprintln(c.out, "\nBootstrap configuration written.")
	fmt.Fprintf(c.out, "- Data directory: %s\n", summary.DataDir)
	fmt.Fprintf(c.out, "- Templates directory: %s\n", summary.TemplatesDir)
	fmt.Fprintf(c.out, "- State directory: %s\n", summary.StateDir)
	fmt.Fprintf(c.out, "- .env path: %s\n", summary.EnvPath)
	fmt.Fprintf(c.out, "- Panel hostname: %s\n", summary.PanelURL)
	fmt.Fprintf(c.out, "- Cloudflared config: %s\n", summary.CloudflaredConfig)
	fmt.Fprintf(c.out, "- Cloudflared log: %s\n", summary.CloudflaredLog)
	fmt.Fprintf(c.out, "- Docker build log: %s\n", summary.ComposeLog)
	fmt.Fprintf(c.out, "- Cloudflare tunnel: %s (%s)\n", summary.CloudflaredTunnel, summary.CloudflaredTunnelID)
}
