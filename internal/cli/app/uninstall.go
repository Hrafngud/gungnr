package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gungnr-cli/internal/cli/integrations/filesystem"
)

type UninstallPlan struct {
	Targets []string
}

func BuildUninstallPlan() (UninstallPlan, error) {
	paths, err := filesystem.DefaultPaths()
	if err != nil {
		return UninstallPlan{}, err
	}

	envPath := filepath.Join(paths.DataDir, ".env")
	env := readEnvFile(envPath)

	cloudflaredDir := env["CLOUDFLARED_DIR"]
	if strings.TrimSpace(cloudflaredDir) == "" {
		cloudflaredDir = paths.CloudflaredDir
	}

	cloudflaredConfig := env["CLOUDFLARED_CONFIG"]
	if strings.TrimSpace(cloudflaredConfig) == "" {
		cloudflaredConfig = filepath.Join(cloudflaredDir, "config.yml")
	}
	cloudflaredLog := filepath.Join(filepath.Dir(cloudflaredConfig), "cloudflared.log")

	var targets []string
	targets = appendIfExists(targets, paths.DataDir)
	targets = appendIfExists(targets, cloudflaredConfig)
	targets = appendIfExists(targets, cloudflaredLog)
	targets = appendIfExists(targets, filepath.Join(cloudflaredDir, "cert.pem"))

	tunnelID := strings.TrimSpace(env["CLOUDFLARE_TUNNEL_ID"])
	if tunnelID != "" {
		targets = appendIfExists(targets, filepath.Join(cloudflaredDir, tunnelID+".json"))
	}

	return UninstallPlan{Targets: targets}, nil
}

func ExecuteUninstall(plan UninstallPlan) error {
	for _, target := range plan.Targets {
		info, err := os.Stat(target)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("unable to access %s: %w", target, err)
		}

		if info.IsDir() {
			if err := os.RemoveAll(target); err != nil {
				return fmt.Errorf("remove %s: %w", target, err)
			}
			continue
		}

		if err := os.Remove(target); err != nil {
			return fmt.Errorf("remove %s: %w", target, err)
		}
	}

	for _, dir := range []string{planTargetsConfigDir(plan), planTargetsCloudflaredDir(plan)} {
		if dir == "" || dir == "." {
			continue
		}
		_ = removeDirIfEmpty(dir)
	}

	return nil
}

func appendIfExists(targets []string, path string) []string {
	if strings.TrimSpace(path) == "" {
		return targets
	}
	if _, err := os.Stat(path); err == nil {
		return append(targets, path)
	}
	return targets
}

func removeDirIfEmpty(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return os.Remove(path)
	}
	return nil
}

func planTargetsConfigDir(plan UninstallPlan) string {
	for _, target := range plan.Targets {
		if strings.HasSuffix(target, "config.yml") {
			return filepath.Dir(target)
		}
	}
	return ""
}

func planTargetsCloudflaredDir(plan UninstallPlan) string {
	for _, target := range plan.Targets {
		if strings.HasSuffix(target, ".json") || strings.HasSuffix(target, "cert.pem") {
			return filepath.Dir(target)
		}
	}
	return ""
}
