package command

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
