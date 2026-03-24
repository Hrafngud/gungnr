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
	keepaliveComposeFileName = "keepalive-compose-file"
	keepaliveLastRunFileName = "keepalive-last-run.json"
	keepaliveLogFileName     = "keepalive.log"
	keepaliveLockFileName    = "keepalive.lock"
	keepaliveHealthURL       = "http://localhost/healthz"

	defaultAPIHealthTimeoutSeconds = 180
	defaultProjectRetryCount       = 2
	defaultProjectRetryBackoffSec  = 5
	defaultProjectTimeoutSeconds   = 180
)

var errKeepaliveAlreadyRunning = errors.New("keepalive recovery already running")

type keepaliveContext struct {
	ConfigPath  string
	StateDir    string
	ComposePath string
	LastRunPath string
	LogPath     string
	LockPath    string
	EnvPath     string
	Env         map[string]string
	DockerLog   string
}

type keepaliveRunControls struct {
	APIHealthTimeout              time.Duration `json:"apiHealthTimeout"`
	APIHealthTimeoutRaw           string        `json:"apiHealthTimeoutRaw"`
	ProjectRetryCount             int           `json:"projectRetryCount"`
	ProjectRetryBackoff           time.Duration `json:"projectRetryBackoff"`
	ProjectRetryBackoffRaw        string        `json:"projectRetryBackoffRaw"`
	ProjectTimeout                time.Duration `json:"projectTimeout"`
	ProjectTimeoutRaw             string        `json:"projectTimeoutRaw"`
	SupervisorFailOnProjectErrors bool          `json:"supervisorFailOnProjectErrors"`
}

type keepaliveRecovery struct {
	PanelRecovered           bool              `json:"panelRecovered"`
	TunnelRestarted          bool              `json:"tunnelRestarted"`
	APIHealthy               bool              `json:"apiHealthy"`
	ProjectRecoveryAttempted bool              `json:"projectRecoveryAttempted"`
	ProjectsQueued           int               `json:"projectsQueued"`
	ProjectsRecovered        int               `json:"projectsRecovered"`
	ProjectsFailed           int               `json:"projectsFailed"`
	FailedProjects           []string          `json:"failedProjects,omitempty"`
	FailedProjectErrors      map[string]string `json:"failedProjectErrors,omitempty"`
	PanelProject             string            `json:"panelProject,omitempty"`
	PanelComposeFile         string            `json:"panelComposeFile,omitempty"`
	TunnelLogPath            string            `json:"tunnelLogPath,omitempty"`
	PanelError               string            `json:"panelError,omitempty"`
	TunnelError              string            `json:"tunnelError,omitempty"`
	HealthError              string            `json:"healthError,omitempty"`
	ProjectsError            string            `json:"projectsError,omitempty"`
}

type keepaliveLastRun struct {
	Trigger     string               `json:"trigger"`
	Result      string               `json:"result"`
	StartedAt   time.Time            `json:"startedAt"`
	FinishedAt  time.Time            `json:"finishedAt"`
	DurationSec int64                `json:"durationSec"`
	Controls    keepaliveRunControls `json:"controls"`
	Recovery    keepaliveRecovery    `json:"recovery"`
	Remediation []string             `json:"remediation,omitempty"`
}

type keepaliveLogger struct {
	file *os.File
}

type keepaliveSection struct {
	Title string
	Lines []string
}

type keepaliveSetupResult struct {
	Supervisor  supervisor.SetupResult
	ComposeFile string
}

func KeepaliveToggle() (string, error) {
	ctx, err := resolveKeepaliveContext(false)
	if err != nil {
		return "", err
	}

	supervisorStatus, err := supervisor.Status(ctx.StateDir)
	if err != nil {
		return "", err
	}

	status := keepaliveStatus(supervisorStatus)
	if status == "enabled" || status == "partial" {
		return KeepaliveDisable()
	}
	return KeepaliveEnable()
}

func KeepaliveEnable() (string, error) {
	ctx, err := resolveKeepaliveContext(true)
	if err != nil {
		return "", err
	}

	setupResult, err := configureKeepalive(ctx)
	if err != nil {
		return "", err
	}

	output := formatKeepaliveOutput("Keepalive", []keepaliveSection{
		{
			Title: "Action",
			Lines: []string{
				"State: enabled",
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

func KeepaliveRecover(trigger string) (string, error) {
	ctx, err := resolveKeepaliveContext(true)
	if err != nil {
		return "", err
	}

	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		trigger = "manual"
	}

	run, runErr := executeKeepaliveRecovery(ctx, trigger)
	output := formatKeepaliveOutput("Keepalive Recovery", []keepaliveSection{
		{
			Title: "Summary",
			Lines: append([]string{"Trigger: " + trigger}, recoverySummaryLines(run)...),
		},
		{
			Title: "Paths",
			Lines: []string{
				"Panel compose file: " + nonEmptyOrFallback(run.Recovery.PanelComposeFile, "n/a"),
				"Tunnel log: " + nonEmptyOrFallback(run.Recovery.TunnelLogPath, "n/a"),
				"Recovery log: " + ctx.LogPath,
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
	if source == "" {
		source = supervisor.SupervisorNone
	}

	output := formatKeepaliveOutput("Keepalive", []keepaliveSection{
		{
			Title: "Action",
			Lines: []string{
				"State: disabled",
				"Previous supervisor source: " + string(source),
			},
		},
		{
			Title: "Removed",
			Lines: []string{
				"Systemd-system timer: " + boolLabel(teardown.SystemdSystemTimerRemoved),
				"Systemd-system service: " + boolLabel(teardown.SystemdSystemServiceRemoved),
				"Systemd user timer: " + boolLabel(teardown.SystemdTimerRemoved),
				"Systemd user service: " + boolLabel(teardown.SystemdServiceRemoved),
				"Crontab entries: " + boolLabel(teardown.CronRemoved),
				"Run script: " + boolLabel(teardown.RunScriptRemoved),
				"Ensure script: " + boolLabel(teardown.EnsureScriptRemoved),
				"Compose file pointer: " + boolLabel(composeRemoved),
				"Last-run metadata: " + boolLabel(lastRunRemoved),
				"Recovery lock file: " + boolLabel(lockRemoved),
			},
		},
	})
	return output, nil
}

func configureKeepalive(ctx keepaliveContext) (keepaliveSetupResult, error) {
	composeFile, err := resolveComposeFileForSetup(ctx)
	if err != nil {
		return keepaliveSetupResult{}, err
	}

	autoStart, err := supervisor.Setup(ctx.ConfigPath, ctx.StateDir)
	if err != nil {
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
		ComposePath: filepath.Join(stateDir, keepaliveComposeFileName),
		LastRunPath: filepath.Join(stateDir, keepaliveLastRunFileName),
		LogPath:     filepath.Join(stateDir, keepaliveLogFileName),
		LockPath:    filepath.Join(stateDir, keepaliveLockFileName),
		EnvPath:     envPath,
		Env:         env,
		DockerLog:   filepath.Join(stateDir, "docker-compose.log"),
	}, nil
}

func executeKeepaliveRecovery(ctx keepaliveContext, trigger string) (keepaliveLastRun, error) {
	controls := keepaliveControlsFromEnv(ctx.Env)
	now := time.Now().UTC()
	result := keepaliveLastRun{
		Trigger:   trigger,
		Result:    "failed",
		StartedAt: now,
		Controls:  controls,
	}

	logger, err := newKeepaliveLogger(ctx.LogPath)
	if err != nil {
		result.Recovery.PanelError = fmt.Sprintf("open keepalive log %s: %v", ctx.LogPath, err)
		result.Remediation = remediationHintsFromErrors(result.Recovery.PanelError)
		result.FinishedAt = time.Now().UTC()
		result.DurationSec = int64(result.FinishedAt.Sub(result.StartedAt).Seconds())
		_ = writeKeepaliveLastRun(ctx.LastRunPath, result)
		return result, fmt.Errorf("open keepalive log %s: %w", ctx.LogPath, err)
	}
	defer logger.Close()

	lockFile, lockErr := acquireKeepaliveLock(ctx.LockPath)
	if lockErr != nil {
		result.Recovery.PanelError = lockErr.Error()
		result.Remediation = remediationHintsFromErrors(result.Recovery.PanelError)
		result.FinishedAt = time.Now().UTC()
		result.DurationSec = int64(result.FinishedAt.Sub(result.StartedAt).Seconds())
		_ = writeKeepaliveLastRun(ctx.LastRunPath, result)
		logger.Error("lock", lockErr.Error())
		return result, lockErr
	}
	defer releaseKeepaliveLock(lockFile)

	logger.Info("recovery", fmt.Sprintf("starting keepalive recovery (trigger=%s)", trigger))
	recovery := runKeepaliveRecovery(ctx, trigger, controls, logger)
	result.Recovery = recovery

	runErr := evaluateRecoveryError(trigger, recovery, controls)
	if runErr == nil {
		if recovery.ProjectsFailed > 0 || strings.TrimSpace(recovery.ProjectsError) != "" {
			result.Result = "degraded"
			logger.Warn("recovery", "keepalive recovery completed with project-level warnings")
		} else {
			result.Result = "success"
			logger.Info("recovery", "keepalive recovery completed successfully")
		}
	} else {
		result.Result = "failed"
		logger.Error("recovery", runErr.Error())
	}

	result.Remediation = remediationHintsFromErrors(recovery.PanelError, recovery.TunnelError, recovery.HealthError, recovery.ProjectsError)
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

func runKeepaliveRecovery(ctx keepaliveContext, trigger string, controls keepaliveRunControls, logger *keepaliveLogger) keepaliveRecovery {
	result := keepaliveRecovery{}

	composeFile, err := resolveComposeFileForRecovery(ctx)
	if err != nil {
		result.PanelError = err.Error()
		logger.Error("panel", result.PanelError)
		return result
	}
	result.PanelComposeFile = composeFile

	if _, err := os.Stat(ctx.EnvPath); err != nil {
		if os.IsNotExist(err) {
			result.PanelError = fmt.Sprintf("bootstrap .env not found at %s", ctx.EnvPath)
		} else {
			result.PanelError = fmt.Sprintf("unable to access %s: %v", ctx.EnvPath, err)
		}
		logger.Error("panel", result.PanelError)
		return result
	}

	if err := docker.CheckDockerAccess(); err != nil {
		result.PanelError = err.Error()
		logger.Error("panel", result.PanelError)
		return result
	}
	if err := docker.CheckCompose(); err != nil {
		result.PanelError = err.Error()
		logger.Error("panel", result.PanelError)
		return result
	}

	logger.Info("panel", fmt.Sprintf("rebuilding panel compose stack with %s", composeFile))
	if err := docker.RebuildCompose(composeFile, ctx.EnvPath, ctx.DockerLog); err != nil {
		result.PanelError = err.Error()
		logger.Error("panel", result.PanelError)
		return result
	}
	result.PanelRecovered = true

	logger.Info("tunnel", fmt.Sprintf("ensuring cloudflared tunnel process from %s", ctx.ConfigPath))
	startedTunnel, tunnelLogPath, err := cloudflared.EnsureTunnelRunning(ctx.ConfigPath)
	if err != nil {
		result.TunnelError = err.Error()
		logger.Error("tunnel", result.TunnelError)
		return result
	}
	if startedTunnel {
		logger.Warn("tunnel", "cloudflared process was not running; started a new tunnel process")
	} else {
		logger.Info("tunnel", "cloudflared process already running; skipped restart")
	}
	result.TunnelRestarted = true
	result.TunnelLogPath = tunnelLogPath

	logger.Info("health", fmt.Sprintf("waiting for panel health (%s timeout=%s)", keepaliveHealthURL, controls.APIHealthTimeoutRaw))
	if err := health.WaitForHTTPHealth(keepaliveHealthURL, controls.APIHealthTimeout); err != nil {
		result.HealthError = err.Error()
		logger.Error("health", result.HealthError)
		return result
	}
	result.APIHealthy = true

	projects, err := docker.DiscoverComposeProjects(true)
	if err != nil {
		result.ProjectsError = err.Error()
		logger.Error("projects", result.ProjectsError)
	} else {
		result.PanelProject = detectPanelProject(projects, composeFile)
		if result.PanelProject == "" {
			containers, listErr := docker.ListComposeContainers(true)
			if listErr != nil {
				logger.Warn("projects", fmt.Sprintf("unable to derive panel project from running containers: %v", listErr))
			} else {
				result.PanelProject = findCoreComposeProject(containers)
			}
		}
		queue := buildProjectRecoveryQueue(projects, result.PanelProject)
		if len(queue) == 0 {
			logger.Info("projects", "no managed project stacks detected for recovery")
		} else {
			result.ProjectRecoveryAttempted = true
			result.ProjectsQueued = len(queue)
			result.FailedProjectErrors = make(map[string]string)

			for _, project := range queue {
				projectLogPath := filepath.Join(ctx.StateDir, "keepalive-project-"+sanitizeProjectName(project.Name)+".log")

				if err := recoverProjectStackWithRetry(project, projectLogPath, controls, logger); err != nil {
					result.ProjectsFailed++
					result.FailedProjects = append(result.FailedProjects, project.Name)
					result.FailedProjectErrors[project.Name] = err.Error()
					logger.Error("projects", fmt.Sprintf("project %s recovery failed: %v", project.Name, err))
					continue
				}

				logger.Info("health", fmt.Sprintf("waiting for panel health after project %s", project.Name))
				if err := health.WaitForHTTPHealth(keepaliveHealthURL, controls.APIHealthTimeout); err != nil {
					result.ProjectsFailed++
					result.FailedProjects = append(result.FailedProjects, project.Name)
					result.FailedProjectErrors[project.Name] = err.Error()
					logger.Error("health", fmt.Sprintf("project %s post-restart health check failed: %v", project.Name, err))
					continue
				}

				result.ProjectsRecovered++
				logger.Info("projects", fmt.Sprintf("project %s recovered", project.Name))
			}
		}
	}

	sort.Strings(result.FailedProjects)
	if result.ProjectsFailed > 0 {
		summary := fmt.Sprintf("%d project stack(s) failed to recover", result.ProjectsFailed)
		if strings.TrimSpace(result.ProjectsError) != "" {
			result.ProjectsError = result.ProjectsError + "; " + summary
		} else {
			result.ProjectsError = summary
		}
	}
	if len(result.FailedProjectErrors) == 0 {
		result.FailedProjectErrors = nil
	}

	if strings.EqualFold(strings.TrimSpace(trigger), "supervisor") &&
		!controls.SupervisorFailOnProjectErrors &&
		strings.TrimSpace(result.ProjectsError) != "" {
		logger.Warn("projects", "project recovery reported failures; keeping tunnel recovery non-fatal for supervisor trigger")
	}

	if strings.TrimSpace(result.ProjectsError) != "" {
		logger.Warn("tunnel", "skipping final tunnel restart because project recovery is degraded")
		return result
	}

	logger.Info("tunnel", "restarting cloudflared tunnel (final pass)")
	finalTunnelLogPath, err := RunTunnel()
	if err != nil {
		result.TunnelError = err.Error()
		logger.Error("tunnel", result.TunnelError)
		return result
	}
	result.TunnelRestarted = true
	result.TunnelLogPath = finalTunnelLogPath

	logger.Info("health", fmt.Sprintf("waiting for panel health after final tunnel restart (%s timeout=%s)", keepaliveHealthURL, controls.APIHealthTimeoutRaw))
	if err := health.WaitForHTTPHealth(keepaliveHealthURL, controls.APIHealthTimeout); err != nil {
		result.HealthError = err.Error()
		logger.Error("health", result.HealthError)
		return result
	}
	result.APIHealthy = true

	return result
}

func detectPanelProject(projects []docker.ComposeProject, panelComposeFile string) string {
	panelComposeFile = strings.TrimSpace(panelComposeFile)
	for _, project := range projects {
		for _, configFile := range project.ConfigFiles {
			if sameFilePath(configFile, panelComposeFile) {
				return project.Name
			}
		}
	}
	return ""
}

func buildProjectRecoveryQueue(projects []docker.ComposeProject, panelProject string) []docker.ComposeProject {
	queue := make([]docker.ComposeProject, 0, len(projects))
	for _, project := range projects {
		if strings.TrimSpace(project.Name) == "" {
			continue
		}
		if panelProject != "" && strings.EqualFold(strings.TrimSpace(project.Name), strings.TrimSpace(panelProject)) {
			continue
		}
		queue = append(queue, project)
	}
	sort.Slice(queue, func(i, j int) bool {
		return strings.ToLower(queue[i].Name) < strings.ToLower(queue[j].Name)
	})
	return queue
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

func sameFilePath(a, b string) bool {
	left := strings.TrimSpace(a)
	right := strings.TrimSpace(b)
	if left == "" || right == "" {
		return false
	}
	if absLeft, err := filepath.Abs(left); err == nil {
		left = absLeft
	}
	if absRight, err := filepath.Abs(right); err == nil {
		right = absRight
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func sanitizeProjectName(name string) string {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return "project"
	}
	var builder strings.Builder
	for _, char := range trimmed {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			continue
		}
		builder.WriteRune('-')
	}
	value := strings.Trim(builder.String(), "-")
	if value == "" {
		return "project"
	}
	return value
}

func recoverProjectStackWithRetry(project docker.ComposeProject, projectLogPath string, controls keepaliveRunControls, logger *keepaliveLogger) error {
	attempts := controls.ProjectRetryCount
	if attempts < 1 {
		attempts = 1
	}

	backoff := controls.ProjectRetryBackoff
	if backoff <= 0 {
		backoff = 1 * time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		logger.Info(
			"projects",
			fmt.Sprintf(
				"rebuilding project stack %s (attempt %d/%d timeout=%s)",
				project.Name,
				attempt,
				attempts,
				controls.ProjectTimeoutRaw,
			),
		)

		err := docker.RebuildComposeProjectWithTimeout(project, projectLogPath, controls.ProjectTimeout)
		if err == nil {
			return nil
		}

		lastErr = err
		logger.Warn("projects", fmt.Sprintf("project %s attempt %d failed: %v", project.Name, attempt, err))
		if attempt == attempts {
			break
		}

		sleep := backoff * time.Duration(attempt)
		logger.Info("projects", fmt.Sprintf("retrying project %s in %s", project.Name, sleep))
		time.Sleep(sleep)
	}

	if lastErr == nil {
		return errors.New("project recovery failed")
	}
	return lastErr
}

func keepaliveControlsFromEnv(env map[string]string) keepaliveRunControls {
	healthTimeoutSec := parsePositiveIntEnv(env, "KEEPALIVE_API_HEALTH_TIMEOUT_SECONDS", defaultAPIHealthTimeoutSeconds)
	projectRetryCount := parsePositiveIntEnv(env, "KEEPALIVE_PROJECT_RETRY_COUNT", defaultProjectRetryCount)
	projectRetryBackoffSec := parsePositiveIntEnv(env, "KEEPALIVE_PROJECT_RETRY_BACKOFF_SECONDS", defaultProjectRetryBackoffSec)
	projectTimeoutSec := parsePositiveIntEnv(env, "KEEPALIVE_PROJECT_TIMEOUT_SECONDS", defaultProjectTimeoutSeconds)
	supervisorFailOnProjectErrors := parseBoolEnv(env, "KEEPALIVE_SUPERVISOR_FAIL_ON_PROJECT_ERRORS", false)

	return keepaliveRunControls{
		APIHealthTimeout:              time.Duration(healthTimeoutSec) * time.Second,
		APIHealthTimeoutRaw:           fmt.Sprintf("%ds", healthTimeoutSec),
		ProjectRetryCount:             projectRetryCount,
		ProjectRetryBackoff:           time.Duration(projectRetryBackoffSec) * time.Second,
		ProjectRetryBackoffRaw:        fmt.Sprintf("%ds", projectRetryBackoffSec),
		ProjectTimeout:                time.Duration(projectTimeoutSec) * time.Second,
		ProjectTimeoutRaw:             fmt.Sprintf("%ds", projectTimeoutSec),
		SupervisorFailOnProjectErrors: supervisorFailOnProjectErrors,
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

func parseBoolEnv(env map[string]string, key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(env[key]))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func evaluateRecoveryError(trigger string, recovery keepaliveRecovery, controls keepaliveRunControls) error {
	if recovery.PanelError != "" {
		return errors.New(recovery.PanelError)
	}
	if !recovery.PanelRecovered {
		return errors.New("panel recovery did not complete")
	}
	if recovery.TunnelError != "" {
		return errors.New(recovery.TunnelError)
	}
	if !recovery.TunnelRestarted {
		return errors.New("tunnel restart did not complete")
	}
	if recovery.HealthError != "" {
		return errors.New(recovery.HealthError)
	}
	if !recovery.APIHealthy {
		return errors.New("panel health did not become ready")
	}

	projectFailures := strings.TrimSpace(recovery.ProjectsError) != "" || recovery.ProjectsFailed > 0
	if projectFailures {
		supervisorTrigger := strings.EqualFold(strings.TrimSpace(trigger), "supervisor")
		if supervisorTrigger && !controls.SupervisorFailOnProjectErrors {
			return nil
		}
		if recovery.ProjectsError != "" {
			return errors.New(recovery.ProjectsError)
		}
		if recovery.ProjectsFailed > 0 {
			return fmt.Errorf("%d project stack(s) failed to recover", recovery.ProjectsFailed)
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

	return "", fmt.Errorf("unable to resolve docker-compose.yml for keepalive setup: %w. Run `gungnr keepalive` from the repo root or set GUNGNR_COMPOSE_FILE in %s", err, ctx.EnvPath)
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
	case supervisor.SupervisorSystemdSystem:
		if supervisorStatus.RunScriptExists &&
			supervisorStatus.EnsureScriptExists &&
			supervisorStatus.SystemdSystem.TimerEnabled {
			return "enabled"
		}
		return "partial"
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
		supervisorStatus.SystemdSystem.ServiceFileExists ||
		supervisorStatus.SystemdSystem.TimerFileExists ||
		supervisorStatus.Systemd.ServiceFileExists ||
		supervisorStatus.Systemd.TimerFileExists ||
		supervisorStatus.Cron.HasBoot ||
		supervisorStatus.Cron.HasWatch {
		return "partial"
	}

	return "disabled"
}

func writeKeepaliveComposeFile(path, composeFile string) error {
	if err := os.WriteFile(path, []byte(strings.TrimSpace(composeFile)+"\n"), 0o600); err != nil {
		return fmt.Errorf("write keepalive compose file %s: %w", path, err)
	}
	return nil
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
		"Panel recovered: " + boolLabel(run.Recovery.PanelRecovered),
		"Tunnel ensured: " + boolLabel(run.Recovery.TunnelRestarted),
		"Panel API healthy: " + boolLabel(run.Recovery.APIHealthy),
		fmt.Sprintf("Project recovery attempted: %s", boolLabel(run.Recovery.ProjectRecoveryAttempted)),
		fmt.Sprintf("Projects queued: %d", run.Recovery.ProjectsQueued),
		fmt.Sprintf("Projects recovered: %d", run.Recovery.ProjectsRecovered),
		fmt.Sprintf("Projects failed: %d", run.Recovery.ProjectsFailed),
		"Panel project: " + nonEmptyOrFallback(run.Recovery.PanelProject, "n/a"),
		"Panel error: " + nonEmptyOrFallback(run.Recovery.PanelError, "none"),
		"Tunnel error: " + nonEmptyOrFallback(run.Recovery.TunnelError, "none"),
		"Health error: " + nonEmptyOrFallback(run.Recovery.HealthError, "none"),
		"Projects error: " + nonEmptyOrFallback(run.Recovery.ProjectsError, "none"),
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
			strings.Contains(msg, "unable to resolve docker-compose.yml") ||
			strings.Contains(msg, "compose file path not configured") ||
			strings.Contains(msg, "compose file not found at") ||
			(strings.Contains(msg, "compose file path") && strings.Contains(msg, "is a directory")) {
			hints = append(hints, "Compose resolution failed. Re-run `gungnr keepalive` from the project root or set absolute `GUNGNR_COMPOSE_FILE` in `~/gungnr/.env`.")
		}
		if strings.Contains(msg, "network") && strings.Contains(msg, "not found") {
			hints = append(hints, "A project Docker network is missing. Recreate the stack with `docker compose up --build --force-recreate -d` in the project directory.")
		}
		if strings.Contains(msg, "port is already allocated") || strings.Contains(msg, "bind for") {
			hints = append(hints, "A host port required by a project is already in use. Free the port or change the project port mapping before retrying keepalive.")
		}
		if strings.Contains(msg, "cloudflared not found") {
			hints = append(hints, "Install cloudflared and ensure it is available in PATH for non-interactive sessions.")
		}
		if strings.Contains(msg, "system-level keepalive unit management requires sudo permission") ||
			(strings.Contains(msg, "sudo") && strings.Contains(msg, "system-level")) {
			hints = append(hints, "System-level unit management needs sudo approval. Re-run `gungnr keepalive` and allow elevation to install `/etc/systemd/system/gungnr.service` + timer.")
		}
		if strings.Contains(msg, "bootstrap .env not found") {
			hints = append(hints, "Bootstrap environment is missing. Run `gungnr bootstrap` before enabling keepalive recovery.")
		}
		if strings.Contains(msg, "keepalive recovery already running") {
			hints = append(hints, "A previous keepalive run is still in progress. Wait for completion or inspect `~/gungnr/state/keepalive.log`.")
		}
		if strings.Contains(msg, "timed out") && strings.Contains(msg, "health") {
			hints = append(hints, "Panel health timed out. Inspect compose logs (`~/gungnr/state/docker-compose.log`) and service status before retrying.")
		}
		if strings.Contains(msg, "timed out") && strings.Contains(msg, "up --build --force-recreate -d") {
			hints = append(hints, "A project compose rebuild timed out. Check the project log under `~/gungnr/state/keepalive-project-*.log` and retry after resolving slow pulls/builds.")
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
