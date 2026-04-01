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
	infraQueueRoot := filepath.Join(paths.DataDir, "templates", ".infra")

	socketGID, err := docker.DockerSocketGID()
	if err != nil {
		return err
	}
	if err := filesystem.UpsertEnvFileEntry(envPath, "INFRA_QUEUE_ROOT", infraQueueRoot); err != nil {
		return fmt.Errorf("unable to update INFRA_QUEUE_ROOT in %s: %w", envPath, err)
	}
	if err := filesystem.UpsertEnvFileEntry(envPath, "DOCKER_SOCKET_GID", socketGID); err != nil {
		return fmt.Errorf("unable to update DOCKER_SOCKET_GID in %s: %w", envPath, err)
	}
	if err := filesystem.UpsertEnvFileEntry(envPath, "DOCKER_NETWORK_GUARDRAILS_MODE", "compat"); err != nil {
		return fmt.Errorf("unable to update DOCKER_NETWORK_GUARDRAILS_MODE in %s: %w", envPath, err)
	}

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
