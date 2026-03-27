package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go-notes/internal/infra/contract"
	"go-notes/internal/infra/queue"
)

const defaultPollInterval = 500 * time.Millisecond
const (
	tunnelMetricsAddress    = "127.0.0.1:20241"
	tunnelReadyURL          = "http://127.0.0.1:20241/ready"
	tunnelReadyProbeTimeout = 20 * time.Second
)

type commandExecutor interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

type tunnelLifecycle interface {
	Restart(ctx context.Context, configPath string) (string, []string, error)
}

type defaultCommandExecutor struct{}

func (defaultCommandExecutor) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.CombinedOutput()
}

type Runner struct {
	queue        *queue.Filesystem
	pollInterval time.Duration
	owner        string
	templatesDir string
	logger       *log.Logger
	exec         commandExecutor
	tunnel       tunnelLifecycle
}

func New(q *queue.Filesystem, pollInterval time.Duration, templatesDir string, logger *log.Logger) *Runner {
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	if logger == nil {
		logger = log.Default()
	}
	hostname, _ := os.Hostname()
	owner := fmt.Sprintf("api-worker:%s:%d", hostname, os.Getpid())
	return &Runner{
		queue:        q,
		pollInterval: pollInterval,
		owner:        owner,
		templatesDir: strings.TrimSpace(templatesDir),
		logger:       logger,
		exec:         defaultCommandExecutor{},
		tunnel:       newCloudflaredTunnelLifecycle(logger),
	}
}

func (r *Runner) Run(ctx context.Context) {
	if r == nil || r.queue == nil {
		return
	}
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		if err := ctx.Err(); err != nil {
			return
		}
		if err := r.ProcessOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.logger.Printf("warn: infra worker cycle failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Runner) ProcessOnce(ctx context.Context) error {
	ids, err := r.queue.ListIntentIDs(ctx)
	if err != nil {
		return err
	}
	for _, intentID := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}

		intent, err := r.queue.ReadIntent(ctx, intentID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			r.logger.Printf("warn: infra worker read intent %s failed: %v", intentID, err)
			continue
		}
		if !r.supportsTask(intent.TaskType) {
			continue
		}

		result, err := r.queue.ReadResult(ctx, intentID)
		if err == nil && result.Terminal() {
			continue
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			r.logger.Printf("warn: infra worker read result %s failed: %v", intentID, err)
			continue
		}

		_, claimed, err := r.queue.ClaimIntent(ctx, intentID, r.owner)
		if err != nil {
			r.logger.Printf("warn: infra worker claim %s failed: %v", intentID, err)
			continue
		}
		if !claimed {
			continue
		}

		if err := r.handleIntent(ctx, intent); err != nil {
			r.logger.Printf("warn: infra worker handle intent %s failed: %v", intentID, err)
		}
	}
	return nil
}

func (r *Runner) supportsTask(taskType contract.TaskType) bool {
	switch taskType {
	case contract.TaskTypeRestartTunnel,
		contract.TaskTypeDockerStopContainer,
		contract.TaskTypeDockerRestartContainer,
		contract.TaskTypeDockerRemoveContainer,
		contract.TaskTypeDockerListContainers,
		contract.TaskTypeDockerSystemDF,
		contract.TaskTypeDockerListVolumes,
		contract.TaskTypeComposeUpStack,
		contract.TaskTypeHostRuntimeStats:
		return true
	default:
		return false
	}
}

func (r *Runner) SupportedTasks() []contract.TaskType {
	return []contract.TaskType{
		contract.TaskTypeRestartTunnel,
		contract.TaskTypeDockerStopContainer,
		contract.TaskTypeDockerRestartContainer,
		contract.TaskTypeDockerRemoveContainer,
		contract.TaskTypeDockerListContainers,
		contract.TaskTypeDockerSystemDF,
		contract.TaskTypeDockerListVolumes,
		contract.TaskTypeComposeUpStack,
		contract.TaskTypeHostRuntimeStats,
	}
}

func (r *Runner) ValidateTaskCoverage(required []contract.TaskType) error {
	missing := make([]string, 0)
	for _, taskType := range required {
		if r.supportsTask(taskType) {
			continue
		}
		missing = append(missing, string(taskType))
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("infra worker missing required task handlers: %s", strings.Join(missing, ", "))
}

type taskOutcome struct {
	err     error
	logTail []string
	logPath string
	data    map[string]any
}

func (r *Runner) handleIntent(ctx context.Context, intent contract.Intent) error {
	startedAt := time.Now().UTC()
	if _, err := r.queue.WriteResult(ctx, contract.Result{
		Version:   contract.VersionV1,
		IntentID:  intent.IntentID,
		RequestID: intent.RequestID,
		TaskType:  intent.TaskType,
		Status:    contract.StatusRunning,
		CreatedAt: intent.CreatedAt,
		StartedAt: startedAt,
	}); err != nil {
		return fmt.Errorf("write running result for %s: %w", intent.IntentID, err)
	}

	outcome := taskOutcome{}
	switch intent.TaskType {
	case contract.TaskTypeRestartTunnel:
		outcome = r.handleRestartTunnel(ctx, intent)
	case contract.TaskTypeDockerStopContainer:
		outcome = r.handleDockerStop(ctx, intent)
	case contract.TaskTypeDockerRestartContainer:
		outcome = r.handleDockerRestart(ctx, intent)
	case contract.TaskTypeDockerRemoveContainer:
		outcome = r.handleDockerRemove(ctx, intent)
	case contract.TaskTypeDockerListContainers:
		outcome = r.handleDockerListContainers(ctx, intent)
	case contract.TaskTypeDockerSystemDF:
		outcome = r.handleDockerSystemDF(ctx, intent)
	case contract.TaskTypeDockerListVolumes:
		outcome = r.handleDockerListVolumes(ctx, intent)
	case contract.TaskTypeComposeUpStack:
		outcome = r.handleComposeUpStack(ctx, intent)
	case contract.TaskTypeHostRuntimeStats:
		outcome = r.handleHostRuntimeStats(ctx, intent)
	default:
		outcome.err = fmt.Errorf("unsupported task type: %s", intent.TaskType)
	}

	final := contract.Result{
		Version:    contract.VersionV1,
		IntentID:   intent.IntentID,
		RequestID:  intent.RequestID,
		TaskType:   intent.TaskType,
		Status:     contract.StatusSucceeded,
		CreatedAt:  intent.CreatedAt,
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
		LogTail:    outcome.logTail,
		LogPath:    outcome.logPath,
		Data:       outcome.data,
	}
	if outcome.err != nil {
		final.Status = contract.StatusFailed
		final.Error = &contract.Error{
			Code:    "INFRA-500-EXEC",
			Message: outcome.err.Error(),
		}
	}

	if _, err := r.queue.WriteResult(ctx, final); err != nil {
		return fmt.Errorf("write final result for %s: %w", intent.IntentID, err)
	}
	if err := os.Remove(r.queue.ClaimPath(intent.IntentID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		r.logger.Printf("warn: remove claim for %s failed: %v", intent.IntentID, err)
	}
	return nil
}

func (r *Runner) handleRestartTunnel(ctx context.Context, intent contract.Intent) taskOutcome {
	var payload contract.RestartTunnelPayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}
	configPath := strings.TrimSpace(payload.ConfigPath)
	if configPath == "" {
		return taskOutcome{err: fmt.Errorf("config_path is required")}
	}
	if r.tunnel == nil {
		return taskOutcome{err: fmt.Errorf("tunnel lifecycle unavailable")}
	}
	logPath, tail, err := r.tunnel.Restart(ctx, configPath)
	return taskOutcome{
		err:     err,
		logTail: tail,
		logPath: logPath,
	}
}

func (r *Runner) handleDockerStop(ctx context.Context, intent contract.Intent) taskOutcome {
	var payload contract.DockerStopContainerPayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}
	container := strings.TrimSpace(payload.Container)
	if container == "" {
		return taskOutcome{err: fmt.Errorf("container is required")}
	}
	output, err := r.exec.Run(ctx, "", "docker", "stop", container)
	return taskOutcome{
		err:     commandError(err, output, "docker stop %s", container),
		logTail: tailLines(output, 25),
	}
}

func (r *Runner) handleDockerRestart(ctx context.Context, intent contract.Intent) taskOutcome {
	var payload contract.DockerRestartContainerPayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}
	container := strings.TrimSpace(payload.Container)
	if container == "" {
		return taskOutcome{err: fmt.Errorf("container is required")}
	}
	output, err := r.exec.Run(ctx, "", "docker", "restart", container)
	return taskOutcome{
		err:     commandError(err, output, "docker restart %s", container),
		logTail: tailLines(output, 25),
	}
}

func (r *Runner) handleDockerRemove(ctx context.Context, intent contract.Intent) taskOutcome {
	var payload contract.DockerRemoveContainerPayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}
	container := strings.TrimSpace(payload.Container)
	if container == "" {
		return taskOutcome{err: fmt.Errorf("container is required")}
	}
	args := []string{"rm", "-f"}
	if payload.RemoveVolumes {
		args = append(args, "-v")
	}
	args = append(args, container)
	output, err := r.exec.Run(ctx, "", "docker", args...)
	return taskOutcome{
		err:     commandError(err, output, "docker %s", strings.Join(args, " ")),
		logTail: tailLines(output, 25),
	}
}

func (r *Runner) handleDockerListContainers(ctx context.Context, intent contract.Intent) taskOutcome {
	var payload contract.DockerListContainersPayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}

	args := []string{"ps"}
	if payload.IncludeAll {
		args = append(args, "-a")
	}
	args = append(args, "--format", "{{json .}}")

	output, err := r.exec.Run(ctx, "", "docker", args...)
	return taskOutcome{
		err:     commandError(err, output, "docker %s", strings.Join(args, " ")),
		logTail: tailLines(output, 25),
		data: map[string]any{
			"lines": parseLines(output),
		},
	}
}

func (r *Runner) handleDockerSystemDF(ctx context.Context, _ contract.Intent) taskOutcome {
	args := []string{"system", "df", "--format", "{{json .}}"}
	output, err := r.exec.Run(ctx, "", "docker", args...)
	return taskOutcome{
		err:     commandError(err, output, "docker %s", strings.Join(args, " ")),
		logTail: tailLines(output, 25),
		data: map[string]any{
			"lines": parseLines(output),
		},
	}
}

func (r *Runner) handleDockerListVolumes(ctx context.Context, _ contract.Intent) taskOutcome {
	args := []string{"volume", "ls", "--format", "{{json .}}"}
	output, err := r.exec.Run(ctx, "", "docker", args...)
	return taskOutcome{
		err:     commandError(err, output, "docker %s", strings.Join(args, " ")),
		logTail: tailLines(output, 25),
		data: map[string]any{
			"lines": parseLines(output),
		},
	}
}

func (r *Runner) handleComposeUpStack(ctx context.Context, intent contract.Intent) taskOutcome {
	var payload contract.ComposeUpStackPayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}
	payload.Project = strings.TrimSpace(payload.Project)
	payload.ProjectDir = strings.TrimSpace(payload.ProjectDir)
	if payload.Project == "" {
		return taskOutcome{err: fmt.Errorf("project is required")}
	}

	projectDir, err := r.resolveProjectDir(payload.Project, payload.ProjectDir)
	if err != nil {
		return taskOutcome{err: err}
	}

	args := []string{"compose"}
	for _, configFile := range payload.ConfigFiles {
		file := strings.TrimSpace(configFile)
		if file == "" {
			continue
		}
		args = append(args, "-f", file)
	}
	args = append(args, "up")
	if payload.Build {
		args = append(args, "--build")
	}
	if payload.ForceRecreate {
		args = append(args, "--force-recreate")
	}
	args = append(args, "-d")

	output, err := r.exec.Run(ctx, projectDir, "docker", args...)
	return taskOutcome{
		err:     commandError(err, output, "docker %s", strings.Join(args, " ")),
		logTail: tailLines(output, 40),
	}
}

func (r *Runner) resolveProjectDir(project, projectDir string) (string, error) {
	if projectDir != "" {
		info, err := os.Stat(projectDir)
		if err == nil && info.IsDir() {
			return projectDir, nil
		}
		if err == nil {
			return "", fmt.Errorf("project path is not a directory: %s", projectDir)
		}
		return "", fmt.Errorf("project directory missing: %s", projectDir)
	}
	baseDir := strings.TrimSpace(r.templatesDir)
	if baseDir == "" {
		return "", fmt.Errorf("templates directory is not configured")
	}

	exact := filepath.Join(baseDir, project)
	if info, err := os.Stat(exact); err == nil {
		if info.IsDir() {
			return exact, nil
		}
		return "", fmt.Errorf("project path is not a directory: %s", exact)
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", fmt.Errorf("read templates directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), project) {
			return filepath.Join(baseDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("project directory missing: %s", project)
}

func decodePayload(payload map[string]any, out any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return nil
}

func commandError(runErr error, output []byte, format string, args ...any) error {
	if runErr == nil {
		return nil
	}
	command := fmt.Sprintf(format, args...)
	text := strings.TrimSpace(string(output))
	if text == "" {
		return fmt.Errorf("%s failed: %w", command, runErr)
	}
	return fmt.Errorf("%s failed: %w: %s", command, runErr, text)
}

func parseLines(output []byte) []string {
	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	return clean
}

func tailLines(output []byte, max int) []string {
	if max <= 0 {
		return nil
	}
	clean := parseLines(output)
	if len(clean) == 0 {
		return nil
	}
	if len(clean) <= max {
		return clean
	}
	return clean[len(clean)-max:]
}

type cloudflaredTunnelLifecycle struct {
	logger *log.Logger
}

type tunnelRunIdentity struct {
	uid      uint32
	gid      uint32
	username string
	homeDir  string
}

func newCloudflaredTunnelLifecycle(logger *log.Logger) tunnelLifecycle {
	return &cloudflaredTunnelLifecycle{logger: logger}
}

func (l *cloudflaredTunnelLifecycle) Restart(ctx context.Context, configPath string) (string, []string, error) {
	configPath = expandUserPath(configPath)
	logPath := filepath.Join(filepath.Dir(configPath), "cloudflared-restart-worker.log")
	runAs, err := resolveTunnelRunIdentity(configPath, os.Geteuid())
	if err != nil {
		return logPath, nil, err
	}

	pids, err := findTunnelPIDs(ctx, configPath)
	if err != nil {
		return logPath, nil, err
	}
	for _, pid := range pids {
		if killErr := syscall.Kill(pid, syscall.SIGTERM); killErr != nil && !errors.Is(killErr, syscall.ESRCH) {
			return logPath, nil, fmt.Errorf("stop existing cloudflared process %d: %w", pid, killErr)
		}
	}

	if err := waitForTunnelExit(ctx, configPath, 5*time.Second); err != nil {
		return logPath, nil, err
	}
	if err := startTunnelProcess(configPath, logPath, tunnelMetricsAddress, runAs); err != nil {
		return logPath, nil, err
	}
	if err := waitForTunnelStart(ctx, configPath, 10*time.Second); err != nil {
		return logPath, readLogTail(logPath, 25), err
	}
	if err := waitForTunnelReady(ctx, tunnelReadyURL, tunnelReadyProbeTimeout); err != nil {
		return logPath, readLogTail(logPath, 25), err
	}
	return logPath, readLogTail(logPath, 25), nil
}

func findTunnelPIDs(ctx context.Context, configPath string) ([]int, error) {
	pattern := fmt.Sprintf("cloudflared.*--config[[:space:]]+%s.*tunnel run", regexp.QuoteMeta(configPath))
	cmd := exec.CommandContext(ctx, "pgrep", "-f", pattern)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		if trimmed == "" {
			return nil, fmt.Errorf("pgrep cloudflared failed: %w", err)
		}
		return nil, fmt.Errorf("pgrep cloudflared failed: %w: %s", err, trimmed)
	}
	if trimmed == "" {
		return nil, nil
	}
	lines := strings.Split(trimmed, "\n")
	pids := make([]int, 0, len(lines))
	for _, line := range lines {
		id := strings.TrimSpace(line)
		if id == "" {
			continue
		}
		pid, convErr := strconv.Atoi(id)
		if convErr != nil {
			return nil, fmt.Errorf("parse cloudflared pid %q: %w", id, convErr)
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func waitForTunnelExit(ctx context.Context, configPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		pids, err := findTunnelPIDs(ctx, configPath)
		if err != nil {
			return err
		}
		if len(pids) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("cloudflared still running for config %s (pids: %v)", configPath, pids)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func waitForTunnelStart(ctx context.Context, configPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		pids, err := findTunnelPIDs(ctx, configPath)
		if err != nil {
			return err
		}
		if len(pids) > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("cloudflared process did not start for config %s", configPath)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func startTunnelProcess(configPath, logPath, metricsAddress string, runAs *tunnelRunIdentity) error {
	if strings.TrimSpace(configPath) == "" {
		return fmt.Errorf("config path is required")
	}
	if strings.TrimSpace(metricsAddress) == "" {
		return fmt.Errorf("metrics address is required")
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open cloudflared log file: %w", err)
	}
	defer file.Close()

	cmd := exec.Command("cloudflared", "--config", configPath, "--metrics", metricsAddress, "tunnel", "run")
	if runAs != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: runAs.uid,
				Gid: runAs.gid,
			},
		}
		cmd.Env = withTunnelRunIdentityEnv(os.Environ(), runAs)
	}
	cmd.Stdout = file
	cmd.Stderr = file
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cloudflared tunnel run failed: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release cloudflared process: %w", err)
	}
	return nil
}

func waitForTunnelReady(ctx context.Context, readyURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, readyURL, nil)
		if err != nil {
			return fmt.Errorf("build readiness request: %w", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("tunnel readiness check failed for %s: %w", readyURL, err)
			}
			return fmt.Errorf("tunnel readiness check failed for %s with status %d", readyURL, resp.StatusCode)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func readLogTail(logPath string, max int) []string {
	if max <= 0 {
		return nil
	}
	payload, err := os.ReadFile(logPath)
	if err != nil {
		return nil
	}
	return tailLines(payload, max)
}

func expandUserPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed[0] != '~' {
		return trimmed
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return strings.TrimPrefix(trimmed, "~")
	}
	switch trimmed {
	case "~":
		return home
	default:
		return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
	}
}

func resolveTunnelRunIdentity(configPath string, effectiveUID int) (*tunnelRunIdentity, error) {
	if effectiveUID != 0 {
		return nil, nil
	}

	uid, gid, ok, err := ownerFromPath(configPath)
	if err != nil {
		return nil, err
	}
	if !ok || uid == 0 {
		return nil, nil
	}

	identity := &tunnelRunIdentity{
		uid: uint32(uid),
		gid: uint32(gid),
	}
	if account, lookupErr := user.LookupId(strconv.Itoa(uid)); lookupErr == nil {
		identity.username = strings.TrimSpace(account.Username)
		identity.homeDir = strings.TrimSpace(account.HomeDir)
	}
	return identity, nil
}

func ownerFromPath(path string) (int, int, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, false, nil
		}
		return 0, 0, false, fmt.Errorf("stat %s: %w", path, err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false, fmt.Errorf("read ownership for %s", path)
	}
	return int(stat.Uid), int(stat.Gid), true, nil
}

func withTunnelRunIdentityEnv(base []string, runAs *tunnelRunIdentity) []string {
	if runAs == nil {
		return base
	}
	updated := make([]string, 0, len(base)+3)
	hasUser := false
	hasLogName := false
	hasHome := false
	for _, raw := range base {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			continue
		}
		switch key {
		case "USER":
			hasUser = true
			if strings.TrimSpace(runAs.username) != "" {
				value = runAs.username
			}
		case "LOGNAME":
			hasLogName = true
			if strings.TrimSpace(runAs.username) != "" {
				value = runAs.username
			}
		case "HOME":
			hasHome = true
			if strings.TrimSpace(runAs.homeDir) != "" {
				value = runAs.homeDir
			}
		}
		updated = append(updated, key+"="+value)
	}
	if !hasUser && strings.TrimSpace(runAs.username) != "" {
		updated = append(updated, "USER="+runAs.username)
	}
	if !hasLogName && strings.TrimSpace(runAs.username) != "" {
		updated = append(updated, "LOGNAME="+runAs.username)
	}
	if !hasHome && strings.TrimSpace(runAs.homeDir) != "" {
		updated = append(updated, "HOME="+runAs.homeDir)
	}
	return updated
}
