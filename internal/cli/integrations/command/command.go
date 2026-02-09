package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return trimmed, fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
		}
		return trimmed, fmt.Errorf("%s %s failed: %s", name, strings.Join(args, " "), trimmed)
	}
	return trimmed, nil
}

func RunWithTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	if timeout <= 0 {
		return Run(name, args...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return trimmed, fmt.Errorf("%s %s timed out after %s", name, strings.Join(args, " "), timeout)
		}
		if trimmed == "" {
			return trimmed, fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
		}
		return trimmed, fmt.Errorf("%s %s failed: %s", name, strings.Join(args, " "), trimmed)
	}
	return trimmed, nil
}

func RunInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func RunInteractiveInDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func RunLoggedInDir(dir, name, logPath string, args ...string) error {
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", logPath, err)
	}
	defer logFile.Close()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
