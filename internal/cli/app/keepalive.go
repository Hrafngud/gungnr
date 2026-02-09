package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gungnr-cli/internal/cli/integrations/cloudflared"
	"gungnr-cli/internal/cli/integrations/docker"
	"gungnr-cli/internal/cli/integrations/filesystem"
	"gungnr-cli/internal/cli/integrations/health"
	"gungnr-cli/internal/cli/integrations/supervisor"
)

const (
	keepaliveModeCore = "core"
	keepaliveModeAll  = "all"

	keepaliveModeFileName    = "keepalive-mode"
	keepaliveComposeFileName = "keepalive-compose-file"
	keepaliveLastRunFileName = "keepalive-last-run.json"
	keepaliveLogFileName     = "keepalive.log"
	keepaliveLockFileName    = "keepalive.lock"
	keepaliveHealthURL       = "http://localhost/healthz"

	defaultAPIHealthTimeoutSeconds = 180
	defaultManagedRetryCount       = 3
	defaultManagedBackoffSeconds   = 3
	defaultManagedTimeoutSeconds   = 45
)

var errKeepaliveAlreadyRunning = errors.New("keepalive recovery already running")

type keepaliveContext struct {
	ConfigPath   string
	StateDir     string
	ModePath     string
	ComposePath  string
	LastRunPath  string
	LogPath      string
	LockPath     string
	EnvPath      string
	Env          map[string]string
	DockerLog    string
	ComposeFile  string
	ComposeFound bool
}

type keepaliveRunControls struct {
	APIHealthTimeout       time.Duration `json:"apiHealthTimeout"`
	ManagedRetryCount      int           `json:"managedRetryCount"`
	ManagedRetryBackoff    time.Duration `json:"managedRetryBackoff"`
	ManagedStartTimeout    time.Duration `json:"managedStartTimeout"`
	APIHealthTimeoutRaw    string        `json:"apiHealthTimeoutRaw"`
	ManagedBackoffRaw      string        `json:"managedBackoffRaw"`
	ManagedStartTimeoutRaw string        `json:"managedStartTimeoutRaw"`
}

type keepaliveAllRecovery struct {
	CoreRecovered            bool              `json:"coreRecovered"`
	APIHealthy               bool              `json:"apiHealthy"`
	ManagedRecoveryAttempted bool              `json:"managedRecoveryAttempted"`
	ManagedProjects          int               `json:"managedProjects"`
	ManagedProjectsRecovered int               `json:"managedProjectsRecovered"`
	ManagedProjectsFailed    int               `json:"managedProjectsFailed"`
	FailedProjects           []string          `json:"failedProjects,omitempty"`
	FailedProjectErrors      map[string]string `json:"failedProjectErrors,omitempty"`
	CoreProject              string            `json:"coreProject,omitempty"`
	ComposeFile              string            `json:"composeFile,omitempty"`
	CoreError                string            `json:"coreError,omitempty"`
	HealthError              string            `json:"healthError,omitempty"`
	ManagedError             string            `json:"managedError,omitempty"`
}

type keepaliveLastRun struct {
	Mode        string               `json:"mode"`
	Trigger     string               `json:"trigger"`
	Result      string               `json:"result"`
	StartedAt   time.Time            `json:"startedAt"`
	FinishedAt  time.Time            `json:"finishedAt"`
	DurationSec int64                `json:"durationSec"`
	Controls    keepaliveRunControls `json:"controls"`
	Recovery    keepaliveAllRecovery `json:"recovery"`
	Remediation []string             `json:"remediation,omitempty"`
}

type keepaliveLogger struct {
	file *os.File
}

type keepaliveSection struct {
	Title string
	Lines []string
}

func KeepaliveEnable() (string, error) {
	return keepaliveEnableWithMode(keepaliveModeCore, "enable")
}

func KeepaliveAll() (string, error) {
	ctx, err := resolveKeepaliveContext(true)
	if err != nil {
		return "", err
	}

	setupResult, err := configureKeepalive(ctx, keepaliveModeAll)
	if err != nil {
		return "", err
	}

	run, runErr := executeKeepaliveRecovery(ctx, keepaliveModeAll, "manual-all")
	output := formatKeepaliveOutput("Keepalive All", []keepaliveSection{
		{
			Title: "Configuration",
			Lines: []string{
				"Action: all",
				"Mode: " + keepaliveModeAll,
				"Supervisor: " + string(setupResult.Supervisor.Supervisor),
				"Configured: " + boolLabel(setupResult.Supervisor.Installed),
				"Detail: " + nonEmptyOrFallback(setupResult.Supervisor.Detail, "n/a"),
			},
		},
		{
			Title: "Recovery",
			Lines: recoverySummaryLines(run),
		},
		{
			Title: "Paths",
			Lines: []string{
				"Compose file: " + nonEmptyOrFallback(run.Recovery.ComposeFile, setupResult.ComposeFile),
				"Run script: " + setupResult.Supervisor.RunScript,
				"Ensure script: " + setupResult.Supervisor.EnsureScript,
				"Log file: " + ctx.LogPath,
				"Last-run metadata: " + ctx.LastRunPath,
			},
		},
		{
			Title: "Remediation",
			Lines: run.Remediation,
		},
	})

	return output, runErr
}

func KeepaliveRecover() (string, error) {
	ctx, err := resolveKeepaliveContext(true)
	if err != nil {
		return "", err
	}

	mode := readKeepaliveMode(ctx.ModePath)
	if mode == "" {
		mode = keepaliveModeCore
	}

	trigger := strings.TrimSpace(os.Getenv("GUNGNR_KEEPALIVE_TRIGGER"))
	if trigger == "" {
		trigger = "manual"
	}

	run, runErr := executeKeepaliveRecovery(ctx, mode, trigger)
	output := formatKeepaliveOutput("Keepalive Recovery", []keepaliveSection{
		{
			Title: "Summary",
			Lines: append([]string{
				"Mode: " + mode,
				"Trigger: " + trigger,
			}, recoverySummaryLines(run)...),
		},
		{
			Title: "Paths",
			Lines: []string{
				"Compose file: " + nonEmptyOrFallback(run.Recovery.ComposeFile, "n/a"),
				"Log file: " + ctx.LogPath,
				"Last-run metadata: " + ctx.LastRunPath,
			},
		},
		{
			Title: "Remediation",
			Lines: run.Remediation,
		},
	})
	return output, runErr
}

func KeepaliveDisable() (string, error) {
	ctx, err := resolveKeepaliveContext(false)
	if err != nil {
		return "", err
	}

	teardown, err := supervisor.Teardown(ctx.StateDir)
	if err != nil {
		return "", err
	}
	modeRemoved, err := removeFileIfExists(ctx.ModePath)
	if err != nil {
		return "", err
	}
	composeRemoved, err := removeFileIfExists(ctx.ComposePath)
	if err != nil {
		return "", err
	}
	lastRunRemoved, err := removeFileIfExists(ctx.LastRunPath)
	if err != nil {
		return "", err
	}
	lockRemoved, err := removeFileIfExists(ctx.LockPath)
	if err != nil {
		return "", err
	}

	source := teardown.Source
	if source == "" || source == supervisor.SupervisorNone {
		source = supervisor.SupervisorCron
	}

	output := formatKeepaliveOutput("Keepalive Disable", []keepaliveSection{
		{
			Title: "Configuration",
			Lines: []string{
				"Action: disable",
				"Supervisor source: " + string(source),
			},
		},
		{
			Title: "Removed",
			Lines: []string{
				"Systemd timer: " + boolLabel(teardown.SystemdTimerRemoved),
				"Systemd service: " + boolLabel(teardown.SystemdServiceRemoved),
				"Crontab entries: " + boolLabel(teardown.CronRemoved),
				"Run script: " + boolLabel(teardown.RunScriptRemoved),
				"Ensure script: " + boolLabel(teardown.EnsureScriptRemoved),
				"Mode file: " + boolLabel(modeRemoved),
				"Compose file pointer: " + boolLabel(composeRemoved),
				"Last-run metadata: " + boolLabel(lastRunRemoved),
				"Recovery lock file: " + boolLabel(lockRemoved),
			},
		},
	})
	return output, nil
}

func KeepaliveStatus() (string, error) {
	ctx, err := resolveKeepaliveContext(false)
	if err != nil {
		return "", err
	}

	supervisorStatus, err := supervisor.Status(ctx.StateDir)
	if err != nil {
		return "", err
	}

	status := keepaliveStatus(supervisorStatus)
	mode := readKeepaliveMode(ctx.ModePath)
	if mode == "" {
		mode = keepaliveModeCore
	}

	source := supervisorStatus.Source
	if source == "" || source == supervisor.SupervisorNone {
		source = supervisor.SupervisorCron
	}

	composeFile := strings.TrimSpace(readSingleLineFile(ctx.ComposePath))
	composeExists := false
	if composeFile != "" {
		if info, statErr := os.Stat(composeFile); statErr == nil && !info.IsDir() {
			composeExists = true
		}
	}

	lastRun, lastRunErr := readKeepaliveLastRun(ctx.LastRunPath)
	remediation := remediationHintsFromStatus(supervisorStatus)
	if lastRunErr != nil {
		remediation = append(remediation, "Last-run metadata is unreadable. Remove and regenerate with `gungnr keepalive recover`.")
	}
	if lastRun != nil {
		remediation = append(remediation, lastRun.Remediation...)
	}
	remediation = uniqueNonEmpty(remediation)

	lastRunLines := []string{"No keepalive run metadata recorded yet."}
	if lastRunErr != nil {
		lastRunLines = []string{"Metadata error: " + lastRunErr.Error()}
	} else if lastRun != nil {
		lastRunLines = append(lastRunLines[:0],
			"Result: "+lastRun.Result,
			"Trigger: "+lastRun.Trigger,
			"Started: "+formatTimestamp(lastRun.StartedAt),
			"Finished: "+formatTimestamp(lastRun.FinishedAt),
			fmt.Sprintf("Duration: %ds", lastRun.DurationSec),
		)
		lastRunLines = append(lastRunLines, recoverySummaryLines(*lastRun)...)
	}

	output := formatKeepaliveOutput("Keepalive Status", []keepaliveSection{
		{
			Title: "Configuration",
			Lines: []string{
				"Status: " + status,
				"Mode: " + mode,
				"Supervisor source: " + string(source),
				"Supervisor active: " + string(supervisorStatus.Active),
				"Systemd available: " + boolLabel(supervisorStatus.Systemd.Available),
				"Systemd reason: " + nonEmptyOrFallback(supervisorStatus.Systemd.UnavailableReason, "n/a"),
				"Cron available: " + boolLabel(supervisorStatus.Cron.Available),
			},
		},
		{
			Title: "Artifacts",
			Lines: []string{
				"Run script: " + boolLabel(supervisorStatus.RunScriptExists) + " (" + supervisorStatus.RunScript + ")",
				"Ensure script: " + boolLabel(supervisorStatus.EnsureScriptExists) + " (" + supervisorStatus.EnsureScript + ")",
				"Systemd timer file: " + boolLabel(supervisorStatus.Systemd.TimerFileExists),
				"Systemd service file: " + boolLabel(supervisorStatus.Systemd.ServiceFileExists),
				"Systemd timer enabled: " + boolLabel(supervisorStatus.Systemd.TimerEnabled),
				"Systemd timer active: " + boolLabel(supervisorStatus.Systemd.TimerActive),
				"Cron @reboot entry: " + boolLabel(supervisorStatus.Cron.HasBoot),
				"Cron 5-minute entry: " + boolLabel(supervisorStatus.Cron.HasWatch),
			},
		},
		{
			Title: "Paths",
			Lines: []string{
				"Cloudflared config: " + ctx.ConfigPath,
				"Bootstrap env: " + ctx.EnvPath,
				"Compose file pointer: " + nonEmptyOrFallback(composeFile, "not set"),
				"Compose file exists: " + boolLabel(composeExists),
				"Keepalive log: " + ctx.LogPath,
				"Last-run metadata: " + ctx.LastRunPath,
			},
		},
		{
			Title: "Last Run",
			Lines: lastRunLines,
		},
		{
			Title: "Remediation",
			Lines: remediation,
		},
	})
	return output, nil
}

func keepaliveEnableWithMode(mode, action string) (string, error) {
	ctx, err := resolveKeepaliveContext(true)
	if err != nil {
		return "", err
	}

	setupResult, err := configureKeepalive(ctx, mode)
	if err != nil {
		return "", err
	}

	output := formatKeepaliveOutput("Keepalive Enable", []keepaliveSection{
		{
			Title: "Configuration",
			Lines: []string{
				"Action: " + action,
				"Mode: " + mode,
				"Supervisor: " + string(setupResult.Supervisor.Supervisor),
				"Configured: " + boolLabel(setupResult.Supervisor.Installed),
				"Detail: " + nonEmptyOrFallback(setupResult.Supervisor.Detail, "n/a"),
			},
		},
		{
			Title: "Paths",
			Lines: []string{
				"Compose file: " + setupResult.ComposeFile,
				"Run script: " + setupResult.Supervisor.RunScript,
				"Ensure script: " + setupResult.Supervisor.EnsureScript,
				"Recovery log: " + ctx.LogPath,
				"Last-run metadata: " + ctx.LastRunPath,
			},
		},
	})
	return output, nil
}

type keepaliveSetupResult struct {
	Supervisor  supervisor.SetupResult
	ComposeFile string
}

func configureKeepalive(ctx keepaliveContext, mode string) (keepaliveSetupResult, error) {
	composeFile, err := resolveComposeFileForSetup(ctx)
	if err != nil {
		return keepaliveSetupResult{}, err
	}

	autoStart, err := supervisor.Setup(ctx.ConfigPath, ctx.StateDir)
	if err != nil {
		return keepaliveSetupResult{}, err
	}
	if err := writeKeepaliveMode(ctx.ModePath, mode); err != nil {
		return keepaliveSetupResult{}, err
	}
	if err := writeKeepaliveComposeFile(ctx.ComposePath, composeFile); err != nil {
		return keepaliveSetupResult{}, err
	}

	return keepaliveSetupResult{Supervisor: autoStart, ComposeFile: composeFile}, nil
}

func resolveKeepaliveContext(requireConfig bool) (keepaliveContext, error) {
	paths, err := filesystem.DefaultPaths()
	if err != nil {
		return keepaliveContext{}, err
	}

	envPath := filepath.Join(paths.DataDir, ".env")
	env := readEnvFile(envPath)

	cloudflaredDir := strings.TrimSpace(env["CLOUDFLARED_DIR"])
	if cloudflaredDir == "" {
		cloudflaredDir = paths.CloudflaredDir
	}

	configPath := strings.TrimSpace(env["CLOUDFLARED_CONFIG"])
	if configPath == "" {
		configPath = filepath.Join(cloudflaredDir, "config.yml")
	}

	stateDir := filepath.Join(paths.DataDir, "state")
	if requireConfig {
		if _, err := os.Stat(envPath); err != nil {
			if os.IsNotExist(err) {
				return keepaliveContext{}, fmt.Errorf("bootstrap .env not found at %s", envPath)
			}
			return keepaliveContext{}, fmt.Errorf("unable to access %s: %w", envPath, err)
		}
		if _, err := os.Stat(configPath); err != nil {
			if os.IsNotExist(err) {
				return keepaliveContext{}, fmt.Errorf("cloudflared config not found at %s", configPath)
			}
			return keepaliveContext{}, fmt.Errorf("unable to access %s: %w", configPath, err)
		}
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			return keepaliveContext{}, fmt.Errorf("unable to create state directory %s: %w", stateDir, err)
		}
	}

	return keepaliveContext{
		ConfigPath:  configPath,
		StateDir:    stateDir,
		ModePath:    filepath.Join(stateDir, keepaliveModeFileName),
		ComposePath: filepath.Join(stateDir, keepaliveComposeFileName),
		LastRunPath: filepath.Join(stateDir, keepaliveLastRunFileName),
		LogPath:     filepath.Join(stateDir, keepaliveLogFileName),
		LockPath:    filepath.Join(stateDir, keepaliveLockFileName),
		EnvPath:     envPath,
		Env:         env,
		DockerLog:   filepath.Join(stateDir, "docker-compose.log"),
	}, nil
}

func executeKeepaliveRecovery(ctx keepaliveContext, mode, trigger string) (keepaliveLastRun, error) {
	controls := keepaliveControlsFromEnv(ctx.Env)
	now := time.Now().UTC()
	result := keepaliveLastRun{
		Mode:      mode,
		Trigger:   trigger,
		Result:    "failed",
		StartedAt: now,
		Controls:  controls,
	}

	logger, err := newKeepaliveLogger(ctx.LogPath)
	if err != nil {
		result.Recovery.CoreError = fmt.Sprintf("open keepalive log %s: %v", ctx.LogPath, err)
		result.Remediation = remediationHintsFromErrors(result.Recovery.CoreError)
		result.FinishedAt = time.Now().UTC()
		result.DurationSec = int64(result.FinishedAt.Sub(result.StartedAt).Seconds())
		_ = writeKeepaliveLastRun(ctx.LastRunPath, result)
		return result, fmt.Errorf("open keepalive log %s: %w", ctx.LogPath, err)
	}
	defer logger.Close()

	lockFile, lockErr := acquireKeepaliveLock(ctx.LockPath)
	if lockErr != nil {
		result.Recovery.CoreError = lockErr.Error()
		result.Remediation = remediationHintsFromErrors(result.Recovery.CoreError)
		result.FinishedAt = time.Now().UTC()
		result.DurationSec = int64(result.FinishedAt.Sub(result.StartedAt).Seconds())
		_ = writeKeepaliveLastRun(ctx.LastRunPath, result)
		logger.Error("lock", lockErr.Error())
		return result, lockErr
	}
	defer releaseKeepaliveLock(lockFile)

	logger.Info("recovery", fmt.Sprintf("starting keepalive recovery (mode=%s trigger=%s)", mode, trigger))
	recovery := runKeepaliveRecovery(ctx, mode, controls, logger)
	result.Recovery = recovery

	runErr := evaluateRecoveryError(mode, recovery)
	if runErr == nil {
		result.Result = "success"
		logger.Info("recovery", "keepalive recovery completed successfully")
	} else {
		result.Result = "failed"
		logger.Error("recovery", runErr.Error())
	}

	result.Remediation = remediationHintsFromErrors(recovery.CoreError, recovery.HealthError, recovery.ManagedError)
	if len(recovery.FailedProjectErrors) > 0 {
		for _, msg := range recovery.FailedProjectErrors {
			result.Remediation = append(result.Remediation, remediationHintsFromErrors(msg)...)
		}
	}
	result.Remediation = uniqueNonEmpty(result.Remediation)

	result.FinishedAt = time.Now().UTC()
	result.DurationSec = int64(result.FinishedAt.Sub(result.StartedAt).Seconds())

	if err := writeKeepaliveLastRun(ctx.LastRunPath, result); err != nil {
		persistErr := fmt.Errorf("persist keepalive metadata at %s: %w", ctx.LastRunPath, err)
		if runErr != nil {
			return result, fmt.Errorf("%s; %w", runErr.Error(), persistErr)
		}
		return result, persistErr
	}

	return result, runErr
}

func runKeepaliveRecovery(ctx keepaliveContext, mode string, controls keepaliveRunControls, logger *keepaliveLogger) keepaliveAllRecovery {
	result := keepaliveAllRecovery{}

	composeFile, err := resolveComposeFileForRecovery(ctx)
	if err != nil {
		result.CoreError = err.Error()
		logger.Error("core", result.CoreError)
		return result
	}
	result.ComposeFile = composeFile

	if _, err := os.Stat(ctx.EnvPath); err != nil {
		if os.IsNotExist(err) {
			result.CoreError = fmt.Sprintf("bootstrap .env not found at %s", ctx.EnvPath)
		} else {
			result.CoreError = fmt.Sprintf("unable to access %s: %v", ctx.EnvPath, err)
		}
		logger.Error("core", result.CoreError)
		return result
	}

	if err := docker.CheckDockerAccess(); err != nil {
		result.CoreError = err.Error()
		logger.Error("core", result.CoreError)
		return result
	}
	if err := docker.CheckCompose(); err != nil {
		result.CoreError = err.Error()
		logger.Error("core", result.CoreError)
		return result
	}
	if err := cloudflared.CheckInstalled(); err != nil {
		result.CoreError = err.Error()
		logger.Error("core", result.CoreError)
		return result
	}

	logger.Info("core", fmt.Sprintf("ensuring compose stack with %s", composeFile))
	if err := docker.EnsureComposeRunning(composeFile, ctx.EnvPath, ctx.DockerLog); err != nil {
		result.CoreError = err.Error()
		logger.Error("core", result.CoreError)
		return result
	}
	logger.Info("core", fmt.Sprintf("ensuring cloudflared tunnel process from %s", ctx.ConfigPath))
	restartedTunnel, _, err := cloudflared.EnsureTunnelRunning(ctx.ConfigPath)
	if err != nil {
		result.CoreError = err.Error()
		logger.Error("core", result.CoreError)
		return result
	}
	if restartedTunnel {
		logger.Warn("core", "cloudflared process was not running; started a new tunnel process")
	} else {
		logger.Info("core", "cloudflared process already running; skipped restart")
	}
	result.CoreRecovered = true

	logger.Info("health", fmt.Sprintf("waiting for API health (%s timeout=%s)", keepaliveHealthURL, controls.APIHealthTimeoutRaw))
	if err := health.WaitForHTTPHealth(keepaliveHealthURL, controls.APIHealthTimeout); err != nil {
		result.HealthError = err.Error()
		logger.Error("health", result.HealthError)
		return result
	}
	result.APIHealthy = true

	if mode != keepaliveModeAll {
		logger.Info("managed", "mode is core; skipping managed project recovery")
		return result
	}

	containers, err := docker.ListComposeContainers(true)
	if err != nil {
		result.ManagedError = err.Error()
		logger.Error("managed", result.ManagedError)
		return result
	}

	coreProject := findCoreComposeProject(containers)
	result.CoreProject = coreProject
	if coreProject == "" {
		result.ManagedError = "core compose project not detected; skipped managed project recovery"
		logger.Warn("managed", result.ManagedError)
		return result
	}

	projectContainers := groupManagedProjectContainers(containers, coreProject)
	projectNames := sortedProjectNames(projectContainers)
	result.ManagedRecoveryAttempted = true
	result.ManagedProjects = len(projectNames)
	result.FailedProjectErrors = make(map[string]string)

	for _, project := range projectNames {
		containerIDs := projectContainers[project]
		projectErr := startManagedProjectWithRetry(project, containerIDs, controls, logger)
		if projectErr != nil {
			result.ManagedProjectsFailed++
			result.FailedProjects = append(result.FailedProjects, project)
			result.FailedProjectErrors[project] = projectErr.Error()
			continue
		}
		result.ManagedProjectsRecovered++
	}

	sort.Strings(result.FailedProjects)
	if result.ManagedProjectsFailed > 0 {
		result.ManagedError = fmt.Sprintf("%d managed project(s) failed to recover", result.ManagedProjectsFailed)
	}
	if len(result.FailedProjectErrors) == 0 {
		result.FailedProjectErrors = nil
	}
	return result
}

func startManagedProjectWithRetry(project string, containerIDs []string, controls keepaliveRunControls, logger *keepaliveLogger) error {
	attempts := controls.ManagedRetryCount
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		logger.Info("managed", fmt.Sprintf("starting project %s (attempt %d/%d, timeout=%s)", project, attempt, attempts, controls.ManagedStartTimeoutRaw))
		err := docker.StartContainersWithTimeout(containerIDs, controls.ManagedStartTimeout)
		if err == nil {
			logger.Info("managed", fmt.Sprintf("project %s recovered", project))
			return nil
		}
		lastErr = err
		logger.Warn("managed", fmt.Sprintf("project %s attempt %d failed: %v", project, attempt, err))
		if attempt == attempts {
			break
		}
		sleep := controls.ManagedRetryBackoff * time.Duration(attempt)
		if sleep <= 0 {
			sleep = 1 * time.Second
		}
		time.Sleep(sleep)
	}

	if lastErr == nil {
		return errors.New("managed recovery failed")
	}
	return lastErr
}

func keepaliveControlsFromEnv(env map[string]string) keepaliveRunControls {
	healthTimeoutSec := parsePositiveIntEnv(env, "KEEPALIVE_API_HEALTH_TIMEOUT_SECONDS", defaultAPIHealthTimeoutSeconds)
	retryCount := parsePositiveIntEnv(env, "KEEPALIVE_MANAGED_RETRY_COUNT", defaultManagedRetryCount)
	backoffSec := parsePositiveIntEnv(env, "KEEPALIVE_MANAGED_BACKOFF_SECONDS", defaultManagedBackoffSeconds)
	timeoutSec := parsePositiveIntEnv(env, "KEEPALIVE_MANAGED_TIMEOUT_SECONDS", defaultManagedTimeoutSeconds)

	return keepaliveRunControls{
		APIHealthTimeout:       time.Duration(healthTimeoutSec) * time.Second,
		ManagedRetryCount:      retryCount,
		ManagedRetryBackoff:    time.Duration(backoffSec) * time.Second,
		ManagedStartTimeout:    time.Duration(timeoutSec) * time.Second,
		APIHealthTimeoutRaw:    fmt.Sprintf("%ds", healthTimeoutSec),
		ManagedBackoffRaw:      fmt.Sprintf("%ds", backoffSec),
		ManagedStartTimeoutRaw: fmt.Sprintf("%ds", timeoutSec),
	}
}

func parsePositiveIntEnv(env map[string]string, key string, fallback int) int {
	value := strings.TrimSpace(env[key])
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func evaluateRecoveryError(mode string, recovery keepaliveAllRecovery) error {
	if recovery.CoreError != "" {
		return errors.New(recovery.CoreError)
	}
	if !recovery.CoreRecovered {
		return errors.New("core recovery did not complete")
	}
	if recovery.HealthError != "" {
		return errors.New(recovery.HealthError)
	}
	if !recovery.APIHealthy {
		return errors.New("api health did not become ready")
	}
	if mode == keepaliveModeAll {
		if recovery.ManagedError != "" {
			return errors.New(recovery.ManagedError)
		}
		if recovery.ManagedProjectsFailed > 0 {
			return fmt.Errorf("%d managed project(s) failed to recover", recovery.ManagedProjectsFailed)
		}
	}
	return nil
}

func resolveComposeFileForSetup(ctx keepaliveContext) (string, error) {
	if envCompose := strings.TrimSpace(ctx.Env["GUNGNR_COMPOSE_FILE"]); envCompose != "" {
		resolved, err := resolveComposePath(envCompose)
		if err != nil {
			return "", fmt.Errorf("invalid GUNGNR_COMPOSE_FILE value %q: %w", envCompose, err)
		}
		return resolved, nil
	}

	composeFile, err := docker.FindComposeFile()
	if err == nil {
		resolved, resolveErr := resolveComposePath(composeFile)
		if resolveErr != nil {
			return "", resolveErr
		}
		return resolved, nil
	}

	persisted := strings.TrimSpace(readSingleLineFile(ctx.ComposePath))
	if persisted != "" {
		resolved, resolveErr := resolveComposePath(persisted)
		if resolveErr == nil {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("unable to resolve docker-compose.yml for keepalive setup: %w. Run `gungnr keepalive enable` from the repo root or set GUNGNR_COMPOSE_FILE in %s", err, ctx.EnvPath)
}

func resolveComposeFileForRecovery(ctx keepaliveContext) (string, error) {
	var envIssue string

	if envCompose := strings.TrimSpace(ctx.Env["GUNGNR_COMPOSE_FILE"]); envCompose != "" {
		if filepath.IsAbs(envCompose) {
			resolved, err := resolveComposePath(envCompose)
			if err == nil {
				return resolved, nil
			}
			envIssue = fmt.Sprintf("GUNGNR_COMPOSE_FILE is set but invalid (%q): %v", envCompose, err)
		} else {
			envIssue = fmt.Sprintf("GUNGNR_COMPOSE_FILE is relative (%q) and cannot be used for non-interactive recovery", envCompose)
		}
	}

	persisted := strings.TrimSpace(readSingleLineFile(ctx.ComposePath))
	if persisted == "" {
		if envIssue != "" {
			return "", fmt.Errorf("%s; keepalive compose file path not configured at %s", envIssue, ctx.ComposePath)
		}
		return "", fmt.Errorf("keepalive compose file path not configured at %s", ctx.ComposePath)
	}
	resolved, err := resolveComposePath(persisted)
	if err != nil {
		if envIssue != "" {
			return "", fmt.Errorf("%s; configured keepalive compose file %q is invalid: %w", envIssue, persisted, err)
		}
		return "", fmt.Errorf("configured keepalive compose file %q is invalid: %w", persisted, err)
	}
	return resolved, nil
}

func resolveComposePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("compose file path is empty")
	}

	resolved := trimmed
	if !filepath.IsAbs(resolved) {
		abs, err := filepath.Abs(resolved)
		if err != nil {
			return "", fmt.Errorf("resolve absolute compose path: %w", err)
		}
		resolved = abs
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("compose file not found at %s", resolved)
		}
		return "", fmt.Errorf("unable to access compose file %s: %w", resolved, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("compose file path %s is a directory", resolved)
	}
	return resolved, nil
}

func keepaliveStatus(supervisorStatus supervisor.StatusResult) string {
	switch supervisorStatus.Active {
	case supervisor.SupervisorSystemd:
		if supervisorStatus.RunScriptExists && supervisorStatus.EnsureScriptExists && supervisorStatus.Systemd.TimerEnabled {
			return "enabled"
		}
		return "partial"
	case supervisor.SupervisorCron:
		if supervisorStatus.RunScriptExists && supervisorStatus.EnsureScriptExists && supervisorStatus.Cron.HasBoot && supervisorStatus.Cron.HasWatch {
			return "enabled"
		}
		return "partial"
	}

	if supervisorStatus.RunScriptExists ||
		supervisorStatus.EnsureScriptExists ||
		supervisorStatus.Systemd.ServiceFileExists ||
		supervisorStatus.Systemd.TimerFileExists ||
		supervisorStatus.Cron.HasBoot ||
		supervisorStatus.Cron.HasWatch {
		return "partial"
	}

	return "disabled"
}

func writeKeepaliveMode(path, mode string) error {
	if err := os.WriteFile(path, []byte(mode+"\n"), 0o600); err != nil {
		return fmt.Errorf("write keepalive mode file %s: %w", path, err)
	}
	return nil
}

func writeKeepaliveComposeFile(path, composeFile string) error {
	if err := os.WriteFile(path, []byte(strings.TrimSpace(composeFile)+"\n"), 0o600); err != nil {
		return fmt.Errorf("write keepalive compose file %s: %w", path, err)
	}
	return nil
}

func readKeepaliveMode(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	mode := strings.TrimSpace(string(content))
	if mode != keepaliveModeCore && mode != keepaliveModeAll {
		return ""
	}
	return mode
}

func readSingleLineFile(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

func writeKeepaliveLastRun(path string, run keepaliveLastRun) error {
	payload, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal keepalive last-run payload: %w", err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write keepalive last-run file %s: %w", path, err)
	}
	return nil
}

func readKeepaliveLastRun(path string) (*keepaliveLastRun, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read keepalive last-run file %s: %w", path, err)
	}
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return nil, nil
	}

	var run keepaliveLastRun
	if err := json.Unmarshal([]byte(trimmed), &run); err != nil {
		return nil, fmt.Errorf("parse keepalive last-run payload: %w", err)
	}
	return &run, nil
}

func newKeepaliveLogger(path string) (*keepaliveLogger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &keepaliveLogger{file: file}, nil
}

func (l *keepaliveLogger) Close() {
	if l == nil || l.file == nil {
		return
	}
	_ = l.file.Close()
}

func (l *keepaliveLogger) Info(phase, message string) {
	l.log("info", phase, message)
}

func (l *keepaliveLogger) Warn(phase, message string) {
	l.log("warn", phase, message)
}

func (l *keepaliveLogger) Error(phase, message string) {
	l.log("error", phase, message)
}

func (l *keepaliveLogger) log(level, phase, message string) {
	if l == nil || l.file == nil {
		return
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	_, _ = fmt.Fprintf(l.file, "%s level=%s phase=%s msg=%q\n", timestamp, level, phase, message)
}

func acquireKeepaliveLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open keepalive lock file %s: %w", path, err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errKeepaliveAlreadyRunning
		}
		return nil, fmt.Errorf("acquire keepalive lock %s: %w", path, err)
	}

	if err := file.Truncate(0); err == nil {
		_, _ = file.Seek(0, 0)
		_, _ = file.WriteString(fmt.Sprintf("pid=%d started_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339)))
	}
	return file, nil
}

func releaseKeepaliveLock(file *os.File) {
	if file == nil {
		return
	}
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	_ = file.Close()
}

func findCoreComposeProject(containers []docker.ComposeContainer) string {
	requiredServices := []string{"db", "api", "web", "proxy"}
	serviceSets := make(map[string]map[string]struct{})
	containerCounts := make(map[string]int)

	for _, container := range containers {
		project := strings.TrimSpace(container.Project)
		if project == "" {
			continue
		}
		if _, ok := serviceSets[project]; !ok {
			serviceSets[project] = make(map[string]struct{})
		}
		containerCounts[project]++
		service := strings.ToLower(strings.TrimSpace(container.Service))
		if service != "" {
			serviceSets[project][service] = struct{}{}
		}
	}

	bestProject := ""
	bestScore := -1
	bestCount := -1
	for project, services := range serviceSets {
		score := 0
		for _, required := range requiredServices {
			if _, ok := services[required]; ok {
				score++
			}
		}
		if score < 3 {
			continue
		}
		count := containerCounts[project]
		if score > bestScore ||
			(score == bestScore && count > bestCount) ||
			(score == bestScore && count == bestCount && (bestProject == "" || project < bestProject)) {
			bestProject = project
			bestScore = score
			bestCount = count
		}
	}
	return bestProject
}

func groupManagedProjectContainers(containers []docker.ComposeContainer, coreProject string) map[string][]string {
	grouped := make(map[string][]string)
	for _, container := range containers {
		project := strings.TrimSpace(container.Project)
		if project == "" {
			continue
		}
		if strings.EqualFold(project, coreProject) {
			continue
		}
		id := strings.TrimSpace(container.ID)
		if id == "" {
			continue
		}
		grouped[project] = append(grouped[project], id)
	}
	return grouped
}

func sortedProjectNames(projects map[string][]string) []string {
	names := make([]string, 0, len(projects))
	for project := range projects {
		names = append(names, project)
	}
	sort.Strings(names)
	return names
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

func formatKeepaliveOutput(title string, sections []keepaliveSection) string {
	var builder strings.Builder
	builder.WriteString(title)
	for _, section := range sections {
		lines := uniqueNonEmpty(section.Lines)
		if len(lines) == 0 {
			continue
		}
		builder.WriteString("\n\n")
		builder.WriteString(section.Title)
		for _, line := range lines {
			builder.WriteString("\n- ")
			builder.WriteString(line)
		}
	}
	return builder.String()
}

func recoverySummaryLines(run keepaliveLastRun) []string {
	lines := []string{
		"Result: " + run.Result,
		"Core recovered: " + boolLabel(run.Recovery.CoreRecovered),
		"API healthy: " + boolLabel(run.Recovery.APIHealthy),
		fmt.Sprintf("Managed recovery attempted: %s", boolLabel(run.Recovery.ManagedRecoveryAttempted)),
		fmt.Sprintf("Managed projects: %d", run.Recovery.ManagedProjects),
		fmt.Sprintf("Managed projects recovered: %d", run.Recovery.ManagedProjectsRecovered),
		fmt.Sprintf("Managed projects failed: %d", run.Recovery.ManagedProjectsFailed),
		"Core project: " + nonEmptyOrFallback(run.Recovery.CoreProject, "n/a"),
		"Core error: " + nonEmptyOrFallback(run.Recovery.CoreError, "none"),
		"Health error: " + nonEmptyOrFallback(run.Recovery.HealthError, "none"),
		"Managed error: " + nonEmptyOrFallback(run.Recovery.ManagedError, "none"),
	}
	if len(run.Recovery.FailedProjects) > 0 {
		lines = append(lines, "Failed projects: "+strings.Join(run.Recovery.FailedProjects, ", "))
	}
	return lines
}

func boolLabel(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func nonEmptyOrFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return "n/a"
	}
	return value.UTC().Format(time.RFC3339)
}

func remediationHintsFromStatus(status supervisor.StatusResult) []string {
	hints := []string{}
	reason := strings.ToLower(strings.TrimSpace(status.Systemd.UnavailableReason))
	if reason != "" {
		if strings.Contains(reason, "failed to connect") ||
			strings.Contains(reason, "dbus") ||
			strings.Contains(reason, "no medium") ||
			strings.Contains(reason, "session") {
			hints = append(hints, "Systemd user session is unavailable. Enable linger (`sudo loginctl enable-linger $USER`) or use cron fallback by re-running `gungnr keepalive enable`.")
		}
	}
	return uniqueNonEmpty(hints)
}

func remediationHintsFromErrors(messages ...string) []string {
	hints := []string{}
	for _, message := range messages {
		msg := strings.ToLower(strings.TrimSpace(message))
		if msg == "" {
			continue
		}

		if strings.Contains(msg, "docker access failed") ||
			(strings.Contains(msg, "docker") && strings.Contains(msg, "permission denied")) {
			hints = append(hints, "Docker permissions are blocking recovery. Ensure the user can access `/var/run/docker.sock` (docker group + re-login).")
		}
		if strings.Contains(msg, "docker compose not available") ||
			strings.Contains(msg, "docker-compose.yml") ||
			strings.Contains(msg, "compose file") {
			hints = append(hints, "Compose resolution failed. Re-run `gungnr keepalive enable` from the project root or set absolute `GUNGNR_COMPOSE_FILE` in `~/gungnr/.env`.")
		}
		if strings.Contains(msg, "cloudflared not found") {
			hints = append(hints, "Install cloudflared and ensure it is available in PATH for non-interactive sessions.")
		}
		if strings.Contains(msg, "systemctl --user") ||
			strings.Contains(msg, "failed to connect") ||
			strings.Contains(msg, "session") {
			hints = append(hints, "Systemd user services are unavailable. Enable linger (`sudo loginctl enable-linger $USER`) or rely on cron fallback.")
		}
		if strings.Contains(msg, "bootstrap .env not found") {
			hints = append(hints, "Bootstrap environment is missing. Run `gungnr bootstrap` before enabling keepalive recovery.")
		}
		if strings.Contains(msg, "keepalive recovery already running") {
			hints = append(hints, "A previous keepalive run is still in progress. Wait for completion or inspect `~/gungnr/state/keepalive.log`.")
		}
		if strings.Contains(msg, "timed out") && strings.Contains(msg, "health") {
			hints = append(hints, "API health timed out. Inspect compose logs (`~/gungnr/state/docker-compose.log`) and service status before retrying.")
		}
	}
	return uniqueNonEmpty(hints)
}

func uniqueNonEmpty(lines []string) []string {
	seen := make(map[string]struct{}, len(lines))
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
