package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gungnr-cli/internal/cli/integrations/cloudflared"
	"gungnr-cli/internal/cli/integrations/filesystem"
)

func RunTunnel() (string, error) {
	paths, err := filesystem.DefaultPaths()
	if err != nil {
		return "", err
	}

	envPath := filepath.Join(paths.DataDir, ".env")
	if _, err := os.Stat(envPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("bootstrap .env not found at %s", envPath)
		}
		return "", fmt.Errorf("unable to access %s: %w", envPath, err)
	}

	if err := cloudflared.CheckInstalled(); err != nil {
		return "", err
	}

	env := readEnvFile(envPath)
	cloudflaredDir := strings.TrimSpace(env["CLOUDFLARED_DIR"])
	if cloudflaredDir == "" {
		cloudflaredDir = paths.CloudflaredDir
	}

	configPath := strings.TrimSpace(env["CLOUDFLARED_CONFIG"])
	if configPath == "" {
		configPath = filepath.Join(cloudflaredDir, "config.yml")
	}

	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("cloudflared config not found at %s", configPath)
		}
		return "", fmt.Errorf("unable to access %s: %w", configPath, err)
	}

	logPath, err := cloudflared.StartTunnel(configPath)
	if err != nil {
		return "", err
	}

	return logPath, nil
}
