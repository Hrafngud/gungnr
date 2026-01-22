package docker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gungnr-cli/internal/cli/integrations/command"
)

func CheckDockerAccess() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("docker not found in PATH. Install Docker and retry")
	}

	if _, err := command.Run("docker", "info"); err != nil {
		return fmt.Errorf("docker access failed: %w", err)
	}

	return nil
}

func CheckCompose() error {
	if _, err := command.Run("docker", "compose", "version"); err == nil {
		return nil
	}

	if _, err := exec.LookPath("docker-compose"); err == nil {
		if _, runErr := command.Run("docker-compose", "version"); runErr == nil {
			return nil
		}
	}

	return errors.New("docker compose not available. Install Docker Compose v2 (docker compose) or docker-compose")
}

func ResolveComposeCommand() (string, []string, error) {
	if _, err := command.Run("docker", "compose", "version"); err == nil {
		return "docker", []string{"compose"}, nil
	}

	if _, err := exec.LookPath("docker-compose"); err == nil {
		if _, runErr := command.Run("docker-compose", "version"); runErr == nil {
			return "docker-compose", nil, nil
		}
	}

	return "", nil, errors.New("docker compose not available")
}

func FindComposeFile() (string, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("unable to resolve working directory: %w", err)
	}

	dir := startDir
	for {
		composePath := filepath.Join(dir, "docker-compose.yml")
		if info, err := os.Stat(composePath); err == nil && !info.IsDir() {
			return composePath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("docker-compose.yml not found from %s upward; run bootstrap from the repo root", startDir)
}

func StartCompose(composeFile, envFile, logPath string) error {
	commandName, baseArgs, err := ResolveComposeCommand()
	if err != nil {
		return err
	}

	composeDir := filepath.Dir(composeFile)
	args := append([]string{}, baseArgs...)
	args = append(args, "--env-file", envFile, "-f", composeFile, "up", "-d", "--build")
	return command.RunLoggedInDir(composeDir, commandName, logPath, args...)
}

func StopCompose(composeFile, envFile string) error {
	commandName, baseArgs, err := ResolveComposeCommand()
	if err != nil {
		return err
	}

	composeDir := filepath.Dir(composeFile)
	args := append([]string{}, baseArgs...)
	args = append(args, "--env-file", envFile, "-f", composeFile, "down")
	return command.RunInteractiveInDir(composeDir, commandName, args...)
}
