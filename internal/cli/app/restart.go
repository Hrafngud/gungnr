package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gungnr-cli/internal/cli/integrations/docker"
	"gungnr-cli/internal/cli/integrations/filesystem"
	"gungnr-cli/internal/cli/integrations/health"
)

func Restart() error {
	paths, err := filesystem.DefaultPaths()
	if err != nil {
		return err
	}

	envPath := filepath.Join(paths.DataDir, ".env")
	if _, err := os.Stat(envPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("bootstrap .env not found at %s", envPath)
		}
		return fmt.Errorf("unable to access %s: %w", envPath, err)
	}

	composeFile, err := docker.FindComposeFile()
	if err != nil {
		return err
	}

	stateDir := filepath.Join(paths.DataDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("unable to create state directory: %w", err)
	}
	logPath := filepath.Join(stateDir, "docker-compose.log")

	if err := docker.StopCompose(composeFile, envPath); err != nil {
		return err
	}
	if err := docker.StartCompose(composeFile, envPath, logPath); err != nil {
		return err
	}
	if err := health.WaitForHTTPHealth("http://localhost/healthz", 3*time.Minute); err != nil {
		return err
	}

	return nil
}
