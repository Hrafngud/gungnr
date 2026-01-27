package cloudflared

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type PersistenceResult struct {
	RunScript     string
	EnsureScript  string
	CronInstalled bool
	CronDetail    string
}

const (
	cronMarkerBoot  = "gungnr-cloudflared"
	cronMarkerWatch = "gungnr-cloudflared-watch"
)

func SetupAutoStart(configPath, stateDir string) (PersistenceResult, error) {
	configPath = strings.TrimSpace(configPath)
	stateDir = strings.TrimSpace(stateDir)
	if configPath == "" {
		return PersistenceResult{}, errors.New("cloudflared config path is empty")
	}
	if stateDir == "" {
		return PersistenceResult{}, errors.New("state directory is empty")
	}
	if _, err := os.Stat(configPath); err != nil {
		return PersistenceResult{}, fmt.Errorf("cloudflared config not found at %s: %w", configPath, err)
	}

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return PersistenceResult{}, fmt.Errorf("create state directory %s: %w", stateDir, err)
	}

	logPath := filepath.Join(filepath.Dir(configPath), "cloudflared.log")
	runScript := filepath.Join(stateDir, "cloudflared-run.sh")
	ensureScript := filepath.Join(stateDir, "cloudflared-ensure.sh")

	if err := writeRunScript(runScript, configPath, logPath); err != nil {
		return PersistenceResult{}, err
	}
	if err := writeEnsureScript(ensureScript, runScript, configPath); err != nil {
		return PersistenceResult{}, err
	}

	cronInstalled, cronDetail, err := installCron(ensureScript)
	if err != nil {
		return PersistenceResult{}, err
	}

	return PersistenceResult{
		RunScript:     runScript,
		EnsureScript:  ensureScript,
		CronInstalled: cronInstalled,
		CronDetail:    cronDetail,
	}, nil
}

func writeRunScript(path, configPath, logPath string) error {
	content := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

CONFIG_PATH=%s
LOG_PATH=%s

if ! command -v cloudflared >/dev/null 2>&1; then
  echo "cloudflared not found in PATH" >&2
  exit 1
fi

exec cloudflared --config "$CONFIG_PATH" tunnel run >>"$LOG_PATH" 2>&1
`, strconv.Quote(configPath), strconv.Quote(logPath))

	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write cloudflared run script %s: %w", path, err)
	}
	return nil
}

func writeEnsureScript(path, runScript, configPath string) error {
	content := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

RUN_SCRIPT=%s
CONFIG_PATH=%s

if ! command -v cloudflared >/dev/null 2>&1; then
  exit 0
fi

if pgrep -f -- "--config $CONFIG_PATH" >/dev/null 2>&1; then
  exit 0
fi

nohup "$RUN_SCRIPT" >/dev/null 2>&1 &
`, strconv.Quote(runScript), strconv.Quote(configPath))

	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write cloudflared ensure script %s: %w", path, err)
	}
	return nil
}

func installCron(ensureScript string) (bool, string, error) {
	if _, err := exec.LookPath("crontab"); err != nil {
		return false, "crontab command not found; skipping auto-start wiring", nil
	}

	existing, err := readCrontab()
	if err != nil {
		return false, "", err
	}

	cleaned := removeManagedCronLines(existing)
	quotedEnsure := strconv.Quote(ensureScript)
	bootLine := fmt.Sprintf("@reboot %s # %s\n", quotedEnsure, cronMarkerBoot)
	watchLine := fmt.Sprintf("*/5 * * * * %s # %s\n", quotedEnsure, cronMarkerWatch)

	var builder strings.Builder
	builder.WriteString(cleaned)
	if cleaned != "" && !strings.HasSuffix(cleaned, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString(bootLine)
	builder.WriteString(watchLine)

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(builder.String())
	if output, err := cmd.CombinedOutput(); err != nil {
		return false, "", fmt.Errorf("install crontab entries: %s", strings.TrimSpace(string(output)))
	}

	return true, "installed @reboot and 5-minute tunnel watchdog via crontab", nil
}

func readCrontab() (string, error) {
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}
	message := strings.ToLower(string(output))
	if strings.Contains(message, "no crontab") {
		return "", nil
	}
	return "", fmt.Errorf("read crontab: %s", strings.TrimSpace(string(output)))
}

func removeManagedCronLines(existing string) string {
	if strings.TrimSpace(existing) == "" {
		return ""
	}
	var builder strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(existing))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			builder.WriteString("\n")
			continue
		}
		if strings.Contains(line, cronMarkerBoot) || strings.Contains(line, cronMarkerWatch) {
			continue
		}
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return builder.String()
}
