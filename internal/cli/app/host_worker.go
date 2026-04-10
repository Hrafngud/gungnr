package app

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"gungnr-cli/internal/cli/integrations/filesystem"
	"gungnr-cli/internal/cli/integrations/hostworker"
	"gungnr-cli/internal/cli/integrations/supervisor"
)

func RunHostWorker(ctx context.Context, queueRoot string) error {
	resolvedQueueRoot, err := resolveHostWorkerQueueRoot(queueRoot)
	if err != nil {
		return err
	}

	runner, err := hostworker.New(resolvedQueueRoot, 0, log.Default())
	if err != nil {
		return err
	}
	return runner.Run(ctx)
}

func SetupHostWorker() (supervisor.HostWorkerSetupResult, error) {
	return supervisor.SetupHostWorker()
}

func TeardownHostWorker() (supervisor.HostWorkerTeardownResult, error) {
	return supervisor.TeardownHostWorker()
}

func resolveHostWorkerQueueRoot(queueRoot string) (string, error) {
	queueRoot = strings.TrimSpace(queueRoot)
	if queueRoot != "" {
		return queueRoot, nil
	}

	paths, err := filesystem.DefaultPaths()
	if err != nil {
		return "", err
	}
	envPath := filepath.Join(paths.DataDir, ".env")
	env := readEnvFile(envPath)
	if configured := strings.TrimSpace(env["HOST_INFRA_QUEUE_ROOT"]); configured != "" {
		return configured, nil
	}

	runtimeEnv, err := ResolvePanelRuntimeEnv(paths.DataDir)
	if err != nil {
		return "", fmt.Errorf("resolve host worker queue root: %w", err)
	}
	return runtimeEnv.HostInfraQueueRoot, nil
}
