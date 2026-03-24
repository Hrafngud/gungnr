package supervisor

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Kind string

const (
	SupervisorSystemdSystem Kind = "systemd-system"
	SupervisorSystemd       Kind = "systemd"
	SupervisorCron          Kind = "cron"
	SupervisorNone          Kind = "none"
)

const (
	runScriptName    = "cloudflared-run.sh"
	ensureScriptName = "cloudflared-ensure.sh"
	keepaliveLogName = "keepalive.log"

	cronMarkerBoot  = "gungnr-cloudflared"
	cronMarkerWatch = "gungnr-cloudflared-watch"

	systemdServiceUnit = "gungnr-cloudflared-keepalive.service"
	systemdTimerUnit   = "gungnr-cloudflared-keepalive.timer"

	systemdSystemServiceUnit = "gungnr.service"
	systemdSystemTimerUnit   = "gungnr-keepalive.timer"
	systemdSystemUnitDir     = "/etc/systemd/system"
)

type SetupResult struct {
	Supervisor   Kind
	RunScript    string
	EnsureScript string
	Installed    bool
	Detail       string
}

type TeardownResult struct {
	Source                      Kind
	SystemdSystemTimerRemoved   bool
	SystemdSystemServiceRemoved bool
	SystemdSystemDetail         string
	SystemdTimerRemoved         bool
	SystemdServiceRemoved       bool
	CronRemoved                 bool
	RunScriptRemoved            bool
	EnsureScriptRemoved         bool
}

type StatusResult struct {
	Source             Kind
	Active             Kind
	RunScript          string
	EnsureScript       string
	RunScriptExists    bool
	EnsureScriptExists bool
	SystemdSystem      SystemdStatus
	Systemd            SystemdStatus
	Cron               CronStatus
}

type SystemdStatus struct {
	Available         bool
	UnavailableReason string
	ServiceUnit       string
	TimerUnit         string
	ServicePath       string
	TimerPath         string
	ServiceFileExists bool
	TimerFileExists   bool
	TimerEnabled      bool
	TimerActive       bool
}

type CronStatus struct {
	Available bool
	HasBoot   bool
	HasWatch  bool
	Content   string
}

func Setup(configPath, stateDir string) (SetupResult, error) {
	configPath = strings.TrimSpace(configPath)
	stateDir = strings.TrimSpace(stateDir)
	if configPath == "" {
		return SetupResult{}, errors.New("cloudflared config path is empty")
	}
	if stateDir == "" {
		return SetupResult{}, errors.New("state directory is empty")
	}

	info, err := os.Stat(configPath)
	if err != nil {
		return SetupResult{}, fmt.Errorf("cloudflared config not found at %s: %w", configPath, err)
	}
	if info.IsDir() {
		return SetupResult{}, fmt.Errorf("cloudflared config path %s is a directory", configPath)
	}

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return SetupResult{}, fmt.Errorf("create state directory %s: %w", stateDir, err)
	}

	keepaliveExecutable, err := resolveKeepaliveExecutable()
	if err != nil {
		return SetupResult{}, err
	}

	logPath := filepath.Join(stateDir, keepaliveLogName)
	runScript := filepath.Join(stateDir, runScriptName)
	ensureScript := filepath.Join(stateDir, ensureScriptName)

	if err := writeRunScript(runScript, keepaliveExecutable, logPath); err != nil {
		return SetupResult{}, err
	}
	if err := writeEnsureScript(ensureScript, runScript); err != nil {
		return SetupResult{}, err
	}

	systemdSystemState, err := probeSystemdSystemStatus()
	if err != nil {
		return SetupResult{}, err
	}
	if !systemdSystemState.Available {
		reason := strings.TrimSpace(systemdSystemState.UnavailableReason)
		if reason == "" {
			reason = "unknown reason"
		}
		return SetupResult{}, fmt.Errorf("system-level systemd is unavailable: %s", reason)
	}

	if err := installSystemdSystemTimer(systemdSystemState, ensureScript); err != nil {
		cleanupState := systemdSystemState
		if refreshed, refreshErr := probeSystemdSystemStatus(); refreshErr == nil {
			cleanupState = refreshed
		}
		_, _ = teardownSystemdSystemBestEffort(cleanupState)
		return SetupResult{}, err
	}

	if systemdUserState, userErr := probeSystemdStatus(); userErr == nil {
		_, _ = teardownSystemd(systemdUserState)
	}
	_, _ = removeCronManagedEntries()

	return SetupResult{
		Supervisor:   SupervisorSystemdSystem,
		RunScript:    runScript,
		EnsureScript: ensureScript,
		Installed:    true,
		Detail:       "installed system-level systemd recovery units (boot + 5-minute keepalive timer)",
	}, nil
}

func resolveKeepaliveExecutable() (string, error) {
	if path, err := exec.LookPath("gungnr"); err == nil {
		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			return "", fmt.Errorf("resolve gungnr executable path: %w", absErr)
		}
		return abs, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return "", errors.New("unable to resolve gungnr executable path from PATH; install gungnr to enable keepalive")
	}
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return "", errors.New("resolved gungnr executable path is empty")
	}

	if strings.Contains(executable, string(filepath.Separator)+"go-build") {
		return "", errors.New("keepalive requires an installed gungnr binary in PATH (not a temporary go run executable)")
	}

	return executable, nil
}

func Teardown(stateDir string) (TeardownResult, error) {
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		return TeardownResult{}, errors.New("state directory is empty")
	}

	status, statusErr := Status(stateDir)
	result := TeardownResult{
		Source: status.Source,
	}
	if statusErr != nil || result.Source == "" {
		result.Source = SupervisorNone
	}

	systemdSystemState, err := probeSystemdSystemStatus()
	if err != nil {
		return TeardownResult{}, err
	}
	systemdSystemResult, systemdSystemErr := teardownSystemdSystem(systemdSystemState)
	if systemdSystemErr != nil {
		return TeardownResult{}, systemdSystemErr
	}
	result.SystemdSystemServiceRemoved = systemdSystemResult.ServiceRemoved
	result.SystemdSystemTimerRemoved = systemdSystemResult.TimerRemoved

	runScript := filepath.Join(stateDir, runScriptName)
	ensureScript := filepath.Join(stateDir, ensureScriptName)
	runRemoved, err := removeFileIfExists(runScript)
	if err != nil {
		return TeardownResult{}, err
	}
	result.RunScriptRemoved = runRemoved

	ensureRemoved, err := removeFileIfExists(ensureScript)
	if err != nil {
		return TeardownResult{}, err
	}
	result.EnsureScriptRemoved = ensureRemoved

	systemdState, err := probeSystemdStatus()
	if err != nil {
		return TeardownResult{}, err
	}
	systemdResult, err := teardownSystemd(systemdState)
	if err != nil {
		return TeardownResult{}, err
	}
	result.SystemdServiceRemoved = systemdResult.ServiceRemoved
	result.SystemdTimerRemoved = systemdResult.TimerRemoved

	cronRemoved, err := removeCronManagedEntries()
	if err != nil {
		return TeardownResult{}, err
	}
	result.CronRemoved = cronRemoved

	return result, nil
}

func Status(stateDir string) (StatusResult, error) {
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		return StatusResult{}, errors.New("state directory is empty")
	}

	runScript := filepath.Join(stateDir, runScriptName)
	ensureScript := filepath.Join(stateDir, ensureScriptName)

	runExists, err := fileExists(runScript)
	if err != nil {
		return StatusResult{}, err
	}
	ensureExists, err := fileExists(ensureScript)
	if err != nil {
		return StatusResult{}, err
	}

	systemdSystemState, err := probeSystemdSystemStatus()
	if err != nil {
		return StatusResult{}, err
	}
	systemdState, err := probeSystemdStatus()
	if err != nil {
		return StatusResult{}, err
	}
	cronState, err := readCronStatus()
	if err != nil {
		return StatusResult{}, err
	}

	source := configuredSupervisorSource(systemdSystemState, systemdState, cronState)
	active := activeSupervisorSource(systemdSystemState, systemdState, cronState)

	if source == SupervisorNone && active != SupervisorNone {
		source = active
	}

	return StatusResult{
		Source:             source,
		Active:             active,
		RunScript:          runScript,
		EnsureScript:       ensureScript,
		RunScriptExists:    runExists,
		EnsureScriptExists: ensureExists,
		SystemdSystem:      systemdSystemState,
		Systemd:            systemdState,
		Cron:               cronState,
	}, nil
}

func writeRunScript(path, keepaliveExecutable, logPath string) error {
	content := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

KEEPALIVE_EXECUTABLE=%s
LOG_PATH=%s
SYSTEM_PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

export PATH="$SYSTEM_PATH:${PATH:-}"

if [[ ! -x "$KEEPALIVE_EXECUTABLE" ]]; then
  ts="$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)"
  msg="keepalive executable not found at $KEEPALIVE_EXECUTABLE"
  printf '%%s level=error phase=bootstrap msg="%%s"\n' "$ts" "$msg" >>"$LOG_PATH" 2>/dev/null || true
  echo "keepalive executable not found at $KEEPALIVE_EXECUTABLE" >&2
  exit 1
fi

ts="$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)"
printf '%%s level=info phase=bootstrap msg="launching keepalive executable %%s"\n' "$ts" "$KEEPALIVE_EXECUTABLE" >>"$LOG_PATH" 2>/dev/null || true

export GUNGNR_KEEPALIVE_TRIGGER=supervisor
exec "$KEEPALIVE_EXECUTABLE" keepalive >>"$LOG_PATH" 2>&1
`, strconv.Quote(keepaliveExecutable), strconv.Quote(logPath))

	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write cloudflared run script %s: %w", path, err)
	}
	return nil
}

func writeEnsureScript(path, runScript string) error {
	content := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

RUN_SCRIPT=%s

exec "$RUN_SCRIPT"
`, strconv.Quote(runScript))

	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write cloudflared ensure script %s: %w", path, err)
	}
	return nil
}

func installSystemdTimer(state SystemdStatus, ensureScript string) error {
	if state.ServicePath == "" || state.TimerPath == "" {
		return errors.New("systemd unit paths are unavailable")
	}
	if state.ServiceUnit == "" || state.TimerUnit == "" {
		return errors.New("systemd unit names are unavailable")
	}

	if err := os.MkdirAll(filepath.Dir(state.ServicePath), 0o755); err != nil {
		return fmt.Errorf("create systemd user unit directory: %w", err)
	}
	if err := os.WriteFile(state.ServicePath, []byte(buildSystemdServiceUnit(ensureScript)), 0o644); err != nil {
		return fmt.Errorf("write systemd service unit %s: %w", state.ServicePath, err)
	}
	if err := os.WriteFile(state.TimerPath, []byte(buildSystemdTimerUnit(state.ServiceUnit)), 0o644); err != nil {
		return fmt.Errorf("write systemd timer unit %s: %w", state.TimerPath, err)
	}

	if _, err := runSystemctlUser("daemon-reload"); err != nil {
		return err
	}
	if _, err := runSystemctlUser("enable", "--now", state.TimerUnit); err != nil {
		return err
	}
	return nil
}

func installSystemdSystemTimer(state SystemdStatus, ensureScript string) error {
	if state.ServicePath == "" || state.TimerPath == "" {
		return errors.New("systemd-system unit paths are unavailable")
	}
	if state.ServiceUnit == "" || state.TimerUnit == "" {
		return errors.New("systemd-system unit names are unavailable")
	}

	if err := ensureSystemPrivileges(); err != nil {
		return err
	}

	serviceContent, err := buildSystemdSystemServiceUnit(ensureScript)
	if err != nil {
		return err
	}
	timerContent := buildSystemdTimerUnit(state.ServiceUnit)

	if err := writeSystemUnitFile(state.ServicePath, serviceContent); err != nil {
		return fmt.Errorf("write system-level service unit %s: %w", state.ServicePath, err)
	}
	if err := writeSystemUnitFile(state.TimerPath, timerContent); err != nil {
		return fmt.Errorf("write system-level timer unit %s: %w", state.TimerPath, err)
	}

	if _, err := runSystemctlSystemPrivileged("daemon-reload"); err != nil {
		return err
	}
	if _, err := runSystemctlSystemPrivileged("enable", "--now", state.TimerUnit); err != nil {
		return err
	}
	return nil
}

func buildSystemdServiceUnit(ensureScript string) string {
	return fmt.Sprintf(`[Unit]
Description=Gungnr keepalive recovery check
After=network-online.target

[Service]
Type=oneshot
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
TimeoutStartSec=0
ExecStart=/usr/bin/env bash -lc %s
`, strconv.Quote(ensureScript))
}

func buildSystemdSystemServiceUnit(ensureScript string) (string, error) {
	var username, homeDir string
	if current, err := user.Current(); err == nil {
		username = strings.TrimSpace(current.Username)
		homeDir = strings.TrimSpace(current.HomeDir)
	}
	if username == "" {
		username = strings.TrimSpace(os.Getenv("USER"))
	}
	if homeDir == "" {
		homeDir = strings.TrimSpace(os.Getenv("HOME"))
	}
	if username == "" {
		return "", errors.New("resolve current user for system-level unit: username is empty")
	}
	if homeDir == "" {
		return "", errors.New("resolve current user for system-level unit: home directory is empty")
	}

	return fmt.Sprintf(`[Unit]
Description=Gungnr keepalive recovery check
After=network-online.target

[Service]
Type=oneshot
User=%s
Environment=HOME=%s
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
TimeoutStartSec=0
ExecStart=/usr/bin/env bash -lc %s
`, username, homeDir, strconv.Quote(ensureScript)), nil
}

func buildSystemdTimerUnit(serviceUnit string) string {
	return fmt.Sprintf(`[Unit]
Description=Gungnr keepalive reboot recovery timer

[Timer]
OnBootSec=1min
OnUnitInactiveSec=5min
Unit=%s
Persistent=true

[Install]
WantedBy=timers.target
`, serviceUnit)
}

type teardownSystemdResult struct {
	ServiceRemoved bool
	TimerRemoved   bool
}

func teardownSystemd(state SystemdStatus) (teardownSystemdResult, error) {
	result := teardownSystemdResult{}
	if state.ServicePath != "" {
		removed, err := removeFileIfExists(state.ServicePath)
		if err != nil {
			return teardownSystemdResult{}, err
		}
		result.ServiceRemoved = removed
	}
	if state.TimerPath != "" {
		removed, err := removeFileIfExists(state.TimerPath)
		if err != nil {
			return teardownSystemdResult{}, err
		}
		result.TimerRemoved = removed
	}

	if state.Available && state.ServiceUnit != "" && state.TimerUnit != "" {
		_ = runSystemctlUserBestEffort("disable", "--now", state.TimerUnit)
		_ = runSystemctlUserBestEffort("stop", state.ServiceUnit)
		if result.ServiceRemoved || result.TimerRemoved {
			_ = runSystemctlUserBestEffort("daemon-reload")
		}
	}

	return result, nil
}

func teardownSystemdSystem(state SystemdStatus) (teardownSystemdResult, error) {
	result := teardownSystemdResult{}
	if !state.ServiceFileExists && !state.TimerFileExists && !systemdIsActive(state) {
		return result, nil
	}

	if err := ensureSystemPrivileges(); err != nil {
		return result, err
	}

	if state.Available && state.ServiceUnit != "" && state.TimerUnit != "" {
		_ = runSystemctlSystemBestEffortPrivileged("disable", "--now", state.TimerUnit)
		_ = runSystemctlSystemBestEffortPrivileged("stop", state.ServiceUnit)
	}

	if state.ServicePath != "" {
		removed, err := removeSystemFileIfExists(state.ServicePath)
		if err != nil {
			return teardownSystemdResult{}, err
		}
		result.ServiceRemoved = removed
	}
	if state.TimerPath != "" {
		removed, err := removeSystemFileIfExists(state.TimerPath)
		if err != nil {
			return teardownSystemdResult{}, err
		}
		result.TimerRemoved = removed
	}

	if state.Available && (result.ServiceRemoved || result.TimerRemoved) {
		_ = runSystemctlSystemBestEffortPrivileged("daemon-reload")
	}
	return result, nil
}

func teardownSystemdSystemBestEffort(state SystemdStatus) (teardownSystemdResult, error) {
	result, err := teardownSystemdSystem(state)
	if err == nil {
		return result, nil
	}
	if isElevationDeniedError(err) {
		return result, nil
	}
	return teardownSystemdResult{}, err
}

func probeSystemdStatus() (SystemdStatus, error) {
	state := SystemdStatus{
		ServiceUnit: systemdServiceUnit,
		TimerUnit:   systemdTimerUnit,
	}

	servicePath, timerPath, err := systemdUnitPaths()
	if err == nil {
		state.ServicePath = servicePath
		state.TimerPath = timerPath

		serviceExists, existsErr := fileExists(servicePath)
		if existsErr != nil {
			return SystemdStatus{}, existsErr
		}
		timerExists, existsErr := fileExists(timerPath)
		if existsErr != nil {
			return SystemdStatus{}, existsErr
		}
		state.ServiceFileExists = serviceExists
		state.TimerFileExists = timerExists
	}

	available, reason := systemdUserAvailable()
	state.Available = available
	state.UnavailableReason = reason
	if !available {
		return state, nil
	}

	timerEnabled, err := systemdTimerEnabled(state.TimerUnit)
	if err == nil {
		state.TimerEnabled = timerEnabled
	}

	timerActive, err := systemdTimerActive(state.TimerUnit)
	if err == nil {
		state.TimerActive = timerActive
	}

	return state, nil
}

func probeSystemdSystemStatus() (SystemdStatus, error) {
	state := SystemdStatus{
		ServiceUnit: systemdSystemServiceUnit,
		TimerUnit:   systemdSystemTimerUnit,
	}

	state.ServicePath, state.TimerPath = systemdSystemUnitPaths()

	serviceExists, err := fileExists(state.ServicePath)
	if err != nil {
		return SystemdStatus{}, err
	}
	timerExists, err := fileExists(state.TimerPath)
	if err != nil {
		return SystemdStatus{}, err
	}
	state.ServiceFileExists = serviceExists
	state.TimerFileExists = timerExists

	available, reason := systemdSystemAvailable()
	state.Available = available
	state.UnavailableReason = reason
	if !available {
		return state, nil
	}

	timerEnabled, err := systemdTimerEnabledSystem(state.TimerUnit)
	if err == nil {
		state.TimerEnabled = timerEnabled
	}

	timerActive, err := systemdTimerActiveSystem(state.TimerUnit)
	if err == nil {
		state.TimerActive = timerActive
	}

	return state, nil
}

func systemdUnitPaths() (servicePath, timerPath string, err error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home directory for systemd units: %w", err)
	}
	if strings.TrimSpace(homeDir) == "" {
		return "", "", errors.New("home directory is empty")
	}

	unitDir := filepath.Join(homeDir, ".config", "systemd", "user")
	return filepath.Join(unitDir, systemdServiceUnit), filepath.Join(unitDir, systemdTimerUnit), nil
}

func systemdSystemUnitPaths() (servicePath, timerPath string) {
	return filepath.Join(systemdSystemUnitDir, systemdSystemServiceUnit), filepath.Join(systemdSystemUnitDir, systemdSystemTimerUnit)
}

func systemdUserAvailable() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "non-linux host"
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false, "systemctl not found in PATH"
	}
	if _, err := runSystemctlUser("show-environment"); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func systemdSystemAvailable() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "non-linux host"
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false, "systemctl not found in PATH"
	}
	// system-level setup may require sudo privileges; availability only checks capability baseline.
	return true, ""
}

func systemdTimerEnabled(unit string) (bool, error) {
	output, err := runSystemctlUser("is-enabled", unit)
	if err == nil {
		return strings.TrimSpace(output) == "enabled", nil
	}

	trimmed := strings.ToLower(strings.TrimSpace(output))
	switch trimmed {
	case "disabled", "static", "indirect", "generated", "transient", "masked":
		return false, nil
	}
	if strings.Contains(trimmed, "not-found") || strings.Contains(trimmed, "no such file") {
		return false, nil
	}
	return false, err
}

func systemdTimerActive(unit string) (bool, error) {
	output, err := runSystemctlUser("is-active", unit)
	if err == nil {
		return strings.TrimSpace(output) == "active", nil
	}

	trimmed := strings.ToLower(strings.TrimSpace(output))
	switch trimmed {
	case "inactive", "failed", "deactivating", "activating", "unknown":
		return false, nil
	}
	if strings.Contains(trimmed, "could not be found") || strings.Contains(trimmed, "not loaded") {
		return false, nil
	}
	return false, err
}

func systemdTimerEnabledSystem(unit string) (bool, error) {
	output, err := runSystemctlSystem("is-enabled", unit)
	if err == nil {
		return strings.TrimSpace(output) == "enabled", nil
	}

	trimmed := strings.ToLower(strings.TrimSpace(output))
	switch trimmed {
	case "disabled", "static", "indirect", "generated", "transient", "masked":
		return false, nil
	}
	if strings.Contains(trimmed, "not-found") || strings.Contains(trimmed, "no such file") {
		return false, nil
	}
	return false, err
}

func systemdTimerActiveSystem(unit string) (bool, error) {
	output, err := runSystemctlSystem("is-active", unit)
	if err == nil {
		return strings.TrimSpace(output) == "active", nil
	}

	trimmed := strings.ToLower(strings.TrimSpace(output))
	switch trimmed {
	case "inactive", "failed", "deactivating", "activating", "unknown":
		return false, nil
	}
	if strings.Contains(trimmed, "could not be found") || strings.Contains(trimmed, "not loaded") {
		return false, nil
	}
	return false, err
}

func runSystemctlUser(args ...string) (string, error) {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return trimmed, fmt.Errorf("systemctl --user %s failed: %w", strings.Join(args, " "), err)
		}
		return trimmed, fmt.Errorf("systemctl --user %s failed: %s", strings.Join(args, " "), trimmed)
	}
	return trimmed, nil
}

func runSystemctlUserBestEffort(args ...string) error {
	_, err := runSystemctlUser(args...)
	return err
}

func runSystemctlSystem(args ...string) (string, error) {
	cmd := exec.Command("systemctl", args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return trimmed, fmt.Errorf("systemctl %s failed: %w", strings.Join(args, " "), err)
		}
		return trimmed, fmt.Errorf("systemctl %s failed: %s", strings.Join(args, " "), trimmed)
	}
	return trimmed, nil
}

func runSystemctlSystemPrivileged(args ...string) (string, error) {
	if os.Geteuid() == 0 {
		return runSystemctlSystem(args...)
	}
	cmd := exec.Command("sudo", append([]string{"systemctl"}, args...)...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return trimmed, fmt.Errorf("sudo systemctl %s failed: %w", strings.Join(args, " "), err)
		}
		return trimmed, fmt.Errorf("sudo systemctl %s failed: %s", strings.Join(args, " "), trimmed)
	}
	return trimmed, nil
}

func runSystemctlSystemBestEffortPrivileged(args ...string) error {
	_, err := runSystemctlSystemPrivileged(args...)
	return err
}

func ensureSystemPrivileges() error {
	if os.Geteuid() == 0 {
		return nil
	}

	nonInteractive := exec.Command("sudo", "-n", "true")
	if err := nonInteractive.Run(); err == nil {
		return nil
	}

	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("system-level keepalive unit management requires sudo permission: %w", err)
	}
	return nil
}

func isElevationDeniedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "requires sudo permission") ||
		strings.Contains(msg, "a password is required") ||
		strings.Contains(msg, "no tty present") ||
		strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "incorrect password") ||
		strings.Contains(msg, "is not in the sudoers")
}

func detailWithSystemdSystemFallback(detail string, err error) string {
	base := strings.TrimSpace(detail)
	if base == "" {
		base = "configured keepalive fallback"
	}
	if err == nil {
		return base
	}
	return fmt.Sprintf("%s (system-level setup skipped: %s)", base, strings.TrimSpace(err.Error()))
}

func writeSystemUnitFile(path, content string) error {
	if os.Geteuid() == 0 {
		return os.WriteFile(path, []byte(content), 0o644)
	}

	cmd := exec.Command("sudo", "tee", path)
	cmd.Stdin = strings.NewReader(content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo tee %s: %s", path, strings.TrimSpace(string(output)))
	}
	chmod := exec.Command("sudo", "chmod", "0644", path)
	chmodOutput, chmodErr := chmod.CombinedOutput()
	if chmodErr != nil {
		return fmt.Errorf("sudo chmod %s: %s", path, strings.TrimSpace(string(chmodOutput)))
	}
	return nil
}

func removeSystemFileIfExists(path string) (bool, error) {
	if os.Geteuid() == 0 {
		return removeFileIfExists(path)
	}
	exists, err := fileExists(path)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	cmd := exec.Command("sudo", "rm", "-f", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("remove %s via sudo rm: %s", path, strings.TrimSpace(string(output)))
	}
	return true, nil
}

func installCronWatchdog(ensureScript string) (bool, string, error) {
	if _, err := exec.LookPath("crontab"); err != nil {
		return false, "", errors.New("crontab command not found; unable to configure cron fallback")
	}

	existing, err := readCrontabForUpdate()
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

	return true, "installed @reboot and 5-minute keepalive recovery watchdog via crontab", nil
}

func readCrontabForUpdate() (string, error) {
	cmd := exec.Command("crontab", "-l")
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}

	message := strings.ToLower(strings.TrimSpace(string(output)))
	if strings.Contains(message, "no crontab") {
		return "", nil
	}
	if strings.Contains(message, "permission denied") ||
		strings.Contains(message, "not allowed") ||
		strings.Contains(message, "pam configuration") {
		return "", errors.New("crontab is unavailable for the current user")
	}
	return "", fmt.Errorf("read crontab: %s", strings.TrimSpace(string(output)))
}

func readCronStatus() (CronStatus, error) {
	if _, err := exec.LookPath("crontab"); err != nil {
		return CronStatus{Available: false}, nil
	}

	content, available, err := readCrontabWithAvailability()
	if err != nil {
		return CronStatus{}, err
	}
	if !available {
		return CronStatus{Available: false}, nil
	}

	return CronStatus{
		Available: true,
		Content:   content,
		HasBoot:   strings.Contains(content, cronMarkerBoot),
		HasWatch:  strings.Contains(content, cronMarkerWatch),
	}, nil
}

func readCrontabWithAvailability() (content string, available bool, err error) {
	cmd := exec.Command("crontab", "-l")
	output, runErr := cmd.CombinedOutput()
	if runErr == nil {
		return string(output), true, nil
	}

	message := strings.ToLower(strings.TrimSpace(string(output)))
	if strings.Contains(message, "no crontab") {
		return "", true, nil
	}
	if strings.Contains(message, "permission denied") ||
		strings.Contains(message, "not allowed") ||
		strings.Contains(message, "pam configuration") {
		return "", false, nil
	}
	return "", true, fmt.Errorf("read crontab: %s", strings.TrimSpace(string(output)))
}

func removeCronManagedEntries() (bool, error) {
	if _, err := exec.LookPath("crontab"); err != nil {
		return false, nil
	}

	existing, available, err := readCrontabWithAvailability()
	if err != nil {
		return false, err
	}
	if !available {
		return false, nil
	}

	cleaned := removeManagedCronLines(existing)
	if cleaned == existing {
		return false, nil
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(cleaned)
	if output, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("update crontab entries: %s", strings.TrimSpace(string(output)))
	}
	return true, nil
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

func systemdIsActive(state SystemdStatus) bool {
	return state.TimerEnabled || state.TimerActive
}

func systemdHasArtifacts(state SystemdStatus) bool {
	return state.ServiceFileExists || state.TimerFileExists
}

func cronIsConfigured(state CronStatus) bool {
	return state.HasBoot || state.HasWatch
}

func configuredSupervisorSource(systemdSystemState, systemdUserState SystemdStatus, cronState CronStatus) Kind {
	switch {
	case systemdHasArtifacts(systemdSystemState):
		return SupervisorSystemdSystem
	case systemdHasArtifacts(systemdUserState):
		return SupervisorSystemd
	case cronIsConfigured(cronState):
		return SupervisorCron
	default:
		return SupervisorNone
	}
}

func activeSupervisorSource(systemdSystemState, systemdUserState SystemdStatus, cronState CronStatus) Kind {
	switch {
	case systemdIsActive(systemdSystemState):
		return SupervisorSystemdSystem
	case systemdIsActive(systemdUserState):
		return SupervisorSystemd
	case cronIsConfigured(cronState):
		return SupervisorCron
	default:
		return SupervisorNone
	}
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("unable to access %s: %w", path, err)
	}
	return !info.IsDir(), nil
}

func removeFileIfExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("unable to access %s: %w", path, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("%s is a directory", path)
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("remove %s: %w", path, err)
	}
	return true, nil
}
