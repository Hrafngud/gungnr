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
	defaultDockerConfigDir  = "gungnr-docker-config"
)

type commandExecutor interface {
	Run(ctx context.Context, req commandRequest) ([]byte, error)
}

type tunnelLifecycle interface {
	Restart(ctx context.Context, configPath string) (string, []string, error)
}

type defaultCommandExecutor struct{}

type commandRequest struct {
	Dir  string
	Env  []string
	Name string
	Args []string
}

type dockerRuntimeInfo struct {
	DockerRootDir   string   `json:"DockerRootDir"`
	SecurityOptions []string `json:"SecurityOptions"`
	Warnings        []string `json:"Warnings"`
	Rootless        bool     `json:"Rootless"`
}

func (defaultCommandExecutor) Run(ctx context.Context, req commandRequest) ([]byte, error) {
	cmd := exec.CommandContext(ctx, req.Name, req.Args...)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	if len(req.Env) > 0 {
		cmd.Env = req.Env
	}
	return cmd.CombinedOutput()
}

type Runner struct {
	queue        *queue.Filesystem
	pollInterval time.Duration
	owner        string
	templatesDir string
	dockerTmpDir string
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
		dockerTmpDir: os.TempDir(),
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
		contract.TaskTypeDockerContainerLogs,
		contract.TaskTypeDockerRuntimeCheck,
		contract.TaskTypeDockerRunQuickService,
		contract.TaskTypeHostListenTCPPorts,
		contract.TaskTypeDockerPublishedPorts,
		contract.TaskTypeComposeUpStack,
		contract.TaskTypeHostRuntimeStats,
		contract.TaskTypeProjectFileWriteAtomic,
		contract.TaskTypeProjectFileCopy,
		contract.TaskTypeProjectFileRemove:
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
		contract.TaskTypeDockerContainerLogs,
		contract.TaskTypeDockerRuntimeCheck,
		contract.TaskTypeDockerRunQuickService,
		contract.TaskTypeHostListenTCPPorts,
		contract.TaskTypeDockerPublishedPorts,
		contract.TaskTypeComposeUpStack,
		contract.TaskTypeHostRuntimeStats,
		contract.TaskTypeProjectFileWriteAtomic,
		contract.TaskTypeProjectFileCopy,
		contract.TaskTypeProjectFileRemove,
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
	case contract.TaskTypeDockerContainerLogs:
		outcome = r.handleDockerContainerLogs(ctx, intent)
	case contract.TaskTypeDockerRuntimeCheck:
		outcome = r.handleDockerRuntimeCheck(ctx, intent)
	case contract.TaskTypeDockerRunQuickService:
		outcome = r.handleDockerRunQuickService(ctx, intent)
	case contract.TaskTypeHostListenTCPPorts:
		outcome = r.handleHostListenTCPPorts(ctx, intent)
	case contract.TaskTypeDockerPublishedPorts:
		outcome = r.handleDockerPublishedPorts(ctx, intent)
	case contract.TaskTypeComposeUpStack:
		outcome = r.handleComposeUpStack(ctx, intent)
	case contract.TaskTypeHostRuntimeStats:
		outcome = r.handleHostRuntimeStats(ctx, intent)
	case contract.TaskTypeProjectFileWriteAtomic:
		outcome = r.handleProjectFileWriteAtomic(ctx, intent)
	case contract.TaskTypeProjectFileCopy:
		outcome = r.handleProjectFileCopy(ctx, intent)
	case contract.TaskTypeProjectFileRemove:
		outcome = r.handleProjectFileRemove(ctx, intent)
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
	output, err := r.runDockerCommand(ctx, "", "stop", container)
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
	output, err := r.runDockerCommand(ctx, "", "restart", container)
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
	output, err := r.runDockerCommand(ctx, "", args...)
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

	output, err := r.runDockerCommand(ctx, "", args...)
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
	output, err := r.runDockerCommand(ctx, "", args...)
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
	output, err := r.runDockerCommand(ctx, "", args...)
	return taskOutcome{
		err:     commandError(err, output, "docker %s", strings.Join(args, " ")),
		logTail: tailLines(output, 25),
		data: map[string]any{
			"lines": parseLines(output),
		},
	}
}

func (r *Runner) handleDockerContainerLogs(ctx context.Context, intent contract.Intent) taskOutcome {
	var payload contract.DockerContainerLogsPayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}

	container := strings.TrimSpace(payload.Container)
	if container == "" {
		return taskOutcome{err: fmt.Errorf("container is required")}
	}

	tail := payload.Tail
	includeTail := true
	if tail <= 0 {
		if strings.TrimSpace(payload.Since) != "" {
			includeTail = false
		} else {
			tail = 200
		}
	}
	if tail > 5000 {
		tail = 5000
	}

	args := []string{"logs"}
	if payload.Follow {
		args = append(args, "-f")
	}
	if payload.Timestamps {
		args = append(args, "--timestamps")
	}
	if strings.TrimSpace(payload.Since) != "" {
		args = append(args, "--since", strings.TrimSpace(payload.Since))
	}
	if includeTail {
		args = append(args, "--tail", strconv.Itoa(tail))
	}
	args = append(args, container)

	output, err := r.runDockerCommand(ctx, "", args...)
	return taskOutcome{
		err:     commandError(err, output, "docker %s", strings.Join(args, " ")),
		logTail: tailLines(output, 40),
		data: map[string]any{
			"lines": parseLinesPreserveWhitespace(output),
		},
	}
}

func (r *Runner) handleDockerRuntimeCheck(ctx context.Context, _ contract.Intent) taskOutcome {
	versionArgs := []string{"version", "--format", "{{.Server.Version}}"}
	versionOutput, err := r.runDockerCommand(ctx, "", versionArgs...)
	if err != nil {
		return taskOutcome{
			err:     commandError(err, versionOutput, "docker %s", strings.Join(versionArgs, " ")),
			logTail: tailLines(versionOutput, 25),
		}
	}

	infoArgs := []string{"info", "--format", "{{json .}}"}
	infoOutput, err := r.runDockerCommand(ctx, "", infoArgs...)
	if err != nil {
		return taskOutcome{
			err:     commandError(err, infoOutput, "docker %s", strings.Join(infoArgs, " ")),
			logTail: tailLines(infoOutput, 25),
		}
	}

	runtimeInfo, parseErr := parseDockerRuntimeInfo(infoOutput)
	if parseErr != nil {
		return taskOutcome{
			err:     fmt.Errorf("parse docker info runtime payload: %w", parseErr),
			logTail: tailLines(infoOutput, 25),
		}
	}

	serverVersion := strings.TrimSpace(string(versionOutput))
	return taskOutcome{
		logTail: tailLines(versionOutput, 25),
		data: map[string]any{
			"lines":            parseLines(versionOutput),
			"server_version":   serverVersion,
			"docker_root_dir":  strings.TrimSpace(runtimeInfo.DockerRootDir),
			"security_options": runtimeInfo.SecurityOptions,
			"warnings":         runtimeInfo.Warnings,
			"rootless":         dockerRuntimeUsesRootless(runtimeInfo),
			"userns_remap":     dockerRuntimeUsesUsernsRemap(runtimeInfo),
		},
	}
}

func parseDockerRuntimeInfo(output []byte) (dockerRuntimeInfo, error) {
	jsonOutput, externalWarnings, err := extractDockerInfoJSONPayload(output)
	if err != nil {
		return dockerRuntimeInfo{}, err
	}

	var payload dockerRuntimeInfo
	if err := json.Unmarshal(jsonOutput, &payload); err != nil {
		return dockerRuntimeInfo{}, err
	}
	payload.Warnings = append(externalWarnings, payload.Warnings...)
	return payload, nil
}

func extractDockerInfoJSONPayload(output []byte) ([]byte, []string, error) {
	raw := strings.TrimSpace(string(output))
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end < start {
		return nil, nil, fmt.Errorf("docker info output did not contain a JSON payload")
	}

	var warnings []string
	if prefix := strings.TrimSpace(raw[:start]); prefix != "" {
		warnings = append(warnings, parseLinesPreserveWhitespace([]byte(prefix))...)
	}
	if suffix := strings.TrimSpace(raw[end+1:]); suffix != "" {
		warnings = append(warnings, parseLinesPreserveWhitespace([]byte(suffix))...)
	}

	return []byte(raw[start : end+1]), warnings, nil
}

func dockerRuntimeUsesRootless(info dockerRuntimeInfo) bool {
	if info.Rootless {
		return true
	}
	return dockerSecurityOptionPresent(info.SecurityOptions, "rootless")
}

func dockerRuntimeUsesUsernsRemap(info dockerRuntimeInfo) bool {
	return dockerSecurityOptionPresent(info.SecurityOptions, "name=userns") || dockerSecurityOptionPresent(info.SecurityOptions, "userns")
}

func dockerSecurityOptionPresent(options []string, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return false
	}
	for _, option := range options {
		normalized := strings.ToLower(strings.TrimSpace(option))
		if strings.Contains(normalized, needle) {
			return true
		}
	}
	return false
}

func (r *Runner) handleDockerRunQuickService(ctx context.Context, intent contract.Intent) taskOutcome {
	var payload contract.DockerRunQuickServicePayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}

	payload.Image = strings.TrimSpace(payload.Image)
	payload.ContainerName = strings.TrimSpace(payload.ContainerName)
	payload.ExposureMode = contract.NormalizeQuickServiceExposureMode(payload.ExposureMode)
	payload.PublishHost = contract.NormalizeQuickServicePublishHost(payload.PublishHost)
	payload.NetworkName = strings.TrimSpace(payload.NetworkName)
	if payload.Image == "" {
		return taskOutcome{err: fmt.Errorf("image is required")}
	}
	if payload.ExposureMode == "" {
		return taskOutcome{err: fmt.Errorf("exposure_mode must be internal or host_published")}
	}
	if payload.NetworkName == "" {
		return taskOutcome{err: fmt.Errorf("network_name is required")}
	}
	if payload.HostPort < 1 || payload.HostPort > 65535 {
		return taskOutcome{err: fmt.Errorf("host_port must be between 1 and 65535")}
	}
	if payload.PublishHost == "" {
		return taskOutcome{err: fmt.Errorf("publish_host must be loopback-only")}
	}
	if payload.ContainerPort < 1 || payload.ContainerPort > 65535 {
		return taskOutcome{err: fmt.Errorf("container_port must be between 1 and 65535")}
	}
	if err := r.ensureQuickServiceNetwork(ctx, payload.NetworkName); err != nil {
		return taskOutcome{err: err}
	}

	args := []string{
		"run",
		"-d",
		"--restart",
		"unless-stopped",
		"--network",
		payload.NetworkName,
		"--security-opt",
		"no-new-privileges:true",
		"--cap-drop",
		"ALL",
	}
	if payload.ContainerPort < 1024 {
		args = append(args, "--cap-add", "NET_BIND_SERVICE")
	}
	args = append(args,
		"--pids-limit",
		strconv.Itoa(contract.QuickServiceDefaultPIDsLimit),
		"--memory",
		contract.QuickServiceDefaultMemory,
		"--memory-swap",
		contract.QuickServiceDefaultMemory,
		"--cpus",
		contract.QuickServiceDefaultCPUs,
		"--label",
		contract.QuickServiceManagedLabelKey+"="+contract.QuickServiceManagedLabelValue,
		"--label",
		contract.QuickServiceExposureLabelKey+"="+payload.ExposureMode,
		"-p",
		fmt.Sprintf("%s:%d:%d", payload.PublishHost, payload.HostPort, payload.ContainerPort),
	)
	if payload.ContainerName != "" {
		args = append(args, "--name", payload.ContainerName)
	}
	args = append(args, payload.Image)

	output, err := r.runDockerCommand(ctx, "", args...)
	return taskOutcome{
		err:     commandError(err, output, "docker %s", strings.Join(args, " ")),
		logTail: tailLines(output, 40),
		data: map[string]any{
			"lines": parseLines(output),
		},
	}
}

func (r *Runner) ensureQuickServiceNetwork(ctx context.Context, networkName string) error {
	inspectOutput, inspectErr := r.runDockerCommand(ctx, "", "network", "inspect", networkName)
	if inspectErr == nil {
		return nil
	}
	if !dockerNetworkMissing(inspectErr, inspectOutput) {
		return commandError(inspectErr, inspectOutput, "docker %s", strings.Join([]string{"network", "inspect", networkName}, " "))
	}

	createArgs := []string{
		"network",
		"create",
		"--driver",
		"bridge",
		"--internal",
		"--label",
		contract.QuickServiceManagedLabelKey + "=" + contract.QuickServiceManagedLabelValue,
		"--label",
		contract.QuickServiceNetworkLabelKey + "=" + contract.QuickServiceNetworkLabelValue,
		networkName,
	}
	createOutput, createErr := r.runDockerCommand(ctx, "", createArgs...)
	if createErr != nil {
		if dockerNetworkAlreadyExists(createErr, createOutput) {
			return nil
		}
		return commandError(createErr, createOutput, "docker %s", strings.Join(createArgs, " "))
	}
	return nil
}

func dockerNetworkMissing(err error, output []byte) bool {
	if err == nil {
		return false
	}
	combined := strings.ToLower(strings.TrimSpace(string(output) + " " + err.Error()))
	return strings.Contains(combined, "no such network") || strings.Contains(combined, "not found")
}

func dockerNetworkAlreadyExists(err error, output []byte) bool {
	if err == nil {
		return false
	}
	combined := strings.ToLower(strings.TrimSpace(string(output) + " " + err.Error()))
	return strings.Contains(combined, "already exists")
}

func (r *Runner) handleProjectFileWriteAtomic(_ context.Context, intent contract.Intent) taskOutcome {
	var payload contract.ProjectFileWriteAtomicPayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}

	basePath, err := r.resolveProjectMutationBase(payload.BasePath)
	if err != nil {
		return taskOutcome{err: err}
	}
	targetPath, err := resolveProjectMutationPath(basePath, payload.Path)
	if err != nil {
		return taskOutcome{err: err}
	}

	mode := os.FileMode(payload.Mode & 0o777)
	if mode == 0 {
		mode = 0o600
	}
	if info, statErr := os.Lstat(targetPath); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return taskOutcome{err: fmt.Errorf("refusing to write through symlinked file")}
		}
		if info.IsDir() {
			return taskOutcome{err: fmt.Errorf("target path points to a directory")}
		}
		if payload.PreserveMode || payload.Mode == 0 {
			mode = info.Mode().Perm()
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return taskOutcome{err: fmt.Errorf("stat target path: %w", statErr)}
	}

	info, err := writeFileAtomically(targetPath, []byte(payload.Content), mode, payload.CreateParents)
	if err != nil {
		return taskOutcome{err: err}
	}
	return taskOutcome{
		data: map[string]any{
			"path":       targetPath,
			"size_bytes": info.Size(),
			"updated_at": info.ModTime().UTC().Format(time.RFC3339Nano),
		},
	}
}

func (r *Runner) handleProjectFileCopy(_ context.Context, intent contract.Intent) taskOutcome {
	var payload contract.ProjectFileCopyPayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}

	basePath, err := r.resolveProjectMutationBase(payload.BasePath)
	if err != nil {
		return taskOutcome{err: err}
	}
	sourcePath, err := resolveProjectMutationPath(basePath, payload.SourcePath)
	if err != nil {
		return taskOutcome{err: err}
	}
	targetPath, err := resolveProjectMutationPath(basePath, payload.DestinationPath)
	if err != nil {
		return taskOutcome{err: err}
	}

	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return taskOutcome{err: fmt.Errorf("stat source path: %w", err)}
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return taskOutcome{err: fmt.Errorf("refusing to copy symlinked source file")}
	}
	if sourceInfo.IsDir() {
		return taskOutcome{err: fmt.Errorf("source path points to a directory")}
	}

	if targetInfo, statErr := os.Lstat(targetPath); statErr == nil {
		if targetInfo.Mode()&os.ModeSymlink != 0 {
			return taskOutcome{err: fmt.Errorf("refusing to write through symlinked destination file")}
		}
		if targetInfo.IsDir() {
			return taskOutcome{err: fmt.Errorf("destination path points to a directory")}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return taskOutcome{err: fmt.Errorf("stat destination path: %w", statErr)}
	}

	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return taskOutcome{err: fmt.Errorf("read source file: %w", err)}
	}

	mode := sourceInfo.Mode().Perm()
	if payload.Mode > 0 {
		mode = os.FileMode(payload.Mode & 0o777)
		if mode == 0 {
			mode = sourceInfo.Mode().Perm()
		}
	}

	info, err := writeFileAtomically(targetPath, raw, mode, payload.CreateParents)
	if err != nil {
		return taskOutcome{err: err}
	}
	return taskOutcome{
		data: map[string]any{
			"path":       targetPath,
			"size_bytes": info.Size(),
			"updated_at": info.ModTime().UTC().Format(time.RFC3339Nano),
		},
	}
}

func (r *Runner) handleProjectFileRemove(_ context.Context, intent contract.Intent) taskOutcome {
	var payload contract.ProjectFileRemovePayload
	if err := decodePayload(intent.Payload, &payload); err != nil {
		return taskOutcome{err: err}
	}

	basePath, err := r.resolveProjectMutationBase(payload.BasePath)
	if err != nil {
		return taskOutcome{err: err}
	}
	targetPath, err := resolveProjectMutationPath(basePath, payload.Path)
	if err != nil {
		return taskOutcome{err: err}
	}

	if targetInfo, statErr := os.Lstat(targetPath); statErr == nil {
		if targetInfo.Mode()&os.ModeSymlink != 0 {
			return taskOutcome{err: fmt.Errorf("refusing to remove symlinked path")}
		}
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return taskOutcome{err: fmt.Errorf("stat target path: %w", statErr)}
	}

	if err := os.Remove(targetPath); err != nil {
		if errors.Is(err, os.ErrNotExist) && payload.IgnoreNotExist {
			return taskOutcome{
				data: map[string]any{
					"path":    targetPath,
					"removed": false,
				},
			}
		}
		return taskOutcome{err: fmt.Errorf("remove target path: %w", err)}
	}
	return taskOutcome{
		data: map[string]any{
			"path":    targetPath,
			"removed": true,
		},
	}
}

func (r *Runner) handleHostListenTCPPorts(ctx context.Context, _ contract.Intent) taskOutcome {
	args := []string{"-ltnH"}
	output, err := r.runCommand(ctx, "", nil, "ss", args...)
	return taskOutcome{
		err:     commandError(err, output, "ss %s", strings.Join(args, " ")),
		logTail: tailLines(output, 25),
		data: map[string]any{
			"lines": parseLines(output),
		},
	}
}

func (r *Runner) handleDockerPublishedPorts(ctx context.Context, _ contract.Intent) taskOutcome {
	args := []string{"ps", "--format", "{{.Ports}}"}
	output, err := r.runDockerCommand(ctx, "", args...)
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

	output, err := r.runDockerCommand(ctx, projectDir, args...)
	return taskOutcome{
		err:     commandError(err, output, "docker %s", strings.Join(args, " ")),
		logTail: tailLines(output, 40),
	}
}

func (r *Runner) runCommand(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	return runExecutorCommand(ctx, r.exec, dir, env, name, args...)
}

func (r *Runner) runDockerCommand(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return runExecutorDockerCommand(ctx, r.exec, dir, r.dockerTmpDir, args...)
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

func (r *Runner) resolveProjectMutationBase(rawBasePath string) (string, error) {
	basePath := strings.TrimSpace(rawBasePath)
	if basePath == "" {
		return "", fmt.Errorf("base_path is required")
	}
	if !filepath.IsAbs(basePath) {
		templatesRoot := strings.TrimSpace(r.templatesDir)
		if templatesRoot == "" {
			return "", fmt.Errorf("templates directory is not configured")
		}
		basePath = filepath.Join(templatesRoot, basePath)
	}
	basePath = filepath.Clean(basePath)

	if resolved, err := filepath.EvalSymlinks(basePath); err == nil {
		basePath = filepath.Clean(resolved)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("resolve base path: %w", err)
	}

	info, err := os.Stat(basePath)
	if err != nil {
		return "", fmt.Errorf("base path missing: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("base path is not a directory")
	}
	return basePath, nil
}

func resolveProjectMutationPath(basePath, rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(basePath, path)
	}
	path = filepath.Clean(path)
	if !pathWithinBase(basePath, path) {
		return "", fmt.Errorf("path resolves outside base path")
	}

	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = filepath.Clean(resolved)
		if !pathWithinBase(basePath, path) {
			return "", fmt.Errorf("path resolves outside base path")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("resolve path: %w", err)
	} else {
		parent := filepath.Dir(path)
		if resolvedParent, parentErr := filepath.EvalSymlinks(parent); parentErr == nil {
			candidate := filepath.Clean(filepath.Join(resolvedParent, filepath.Base(path)))
			if !pathWithinBase(basePath, candidate) {
				return "", fmt.Errorf("path resolves outside base path")
			}
			path = candidate
		} else if !errors.Is(parentErr, os.ErrNotExist) {
			return "", fmt.Errorf("resolve parent path: %w", parentErr)
		}
	}
	return path, nil
}

func pathWithinBase(basePath, targetPath string) bool {
	base := filepath.Clean(strings.TrimSpace(basePath))
	target := filepath.Clean(strings.TrimSpace(targetPath))
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}

func writeFileAtomically(targetPath string, content []byte, mode os.FileMode, createParents bool) (os.FileInfo, error) {
	if strings.TrimSpace(targetPath) == "" {
		return nil, fmt.Errorf("target path is empty")
	}
	if mode == 0 {
		mode = 0o600
	}

	dir := filepath.Dir(targetPath)
	if createParents {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create parent directory: %w", err)
		}
	} else {
		dirInfo, err := os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("stat parent directory: %w", err)
		}
		if !dirInfo.IsDir() {
			return nil, fmt.Errorf("parent path is not a directory")
		}
	}

	tempFile, err := os.CreateTemp(dir, ".infra-file-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if err := tempFile.Chmod(mode); err != nil {
		_ = tempFile.Close()
		return nil, fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return nil, fmt.Errorf("sync temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return nil, fmt.Errorf("replace target file: %w", err)
	}
	cleanupTemp = false

	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("stat target file: %w", err)
	}
	return info, nil
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

func runExecutorCommand(ctx context.Context, exec commandExecutor, dir string, env []string, name string, args ...string) ([]byte, error) {
	if exec == nil {
		return nil, fmt.Errorf("executor unavailable")
	}
	request := commandRequest{
		Dir:  dir,
		Env:  cloneEnv(env),
		Name: name,
		Args: append([]string(nil), args...),
	}
	return exec.Run(ctx, request)
}

func runExecutorDockerCommand(ctx context.Context, exec commandExecutor, dir, dockerTmpDir string, args ...string) ([]byte, error) {
	env, err := prepareDockerCommandEnv(os.Environ(), dockerTmpDir)
	if err != nil {
		return nil, err
	}
	return runExecutorCommand(ctx, exec, dir, env, "docker", args...)
}

func prepareDockerCommandEnv(baseEnv []string, dockerTmpDir string) ([]string, error) {
	env := cloneEnv(baseEnv)
	if value, ok := envValue(env, "DOCKER_CONFIG"); ok && strings.TrimSpace(value) != "" {
		return env, nil
	}
	if hasDefaultDockerConfig(env) {
		return env, nil
	}

	root := strings.TrimSpace(dockerTmpDir)
	if root == "" {
		root = os.TempDir()
	}
	configDir := filepath.Join(root, defaultDockerConfigDir)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return nil, fmt.Errorf("ensure docker config dir: %w", err)
	}
	return setEnvValue(env, "DOCKER_CONFIG", configDir), nil
}

func hasDefaultDockerConfig(env []string) bool {
	homeDir, ok := envValue(env, "HOME")
	homeDir = strings.TrimSpace(homeDir)
	if !ok || homeDir == "" {
		return false
	}

	configPath := filepath.Join(homeDir, ".docker", "config.json")
	info, err := os.Stat(configPath)
	return err == nil && !info.IsDir()
}

func cloneEnv(baseEnv []string) []string {
	if len(baseEnv) == 0 {
		return nil
	}
	cloned := make([]string, len(baseEnv))
	copy(cloned, baseEnv)
	return cloned
}

func envValue(env []string, key string) (string, bool) {
	for _, raw := range env {
		currentKey, value, ok := strings.Cut(raw, "=")
		if ok && currentKey == key {
			return value, true
		}
	}
	return "", false
}

func setEnvValue(env []string, key, value string) []string {
	updated := make([]string, 0, len(env)+1)
	replaced := false
	for _, raw := range env {
		currentKey, _, ok := strings.Cut(raw, "=")
		if ok && currentKey == key {
			if !replaced {
				updated = append(updated, key+"="+value)
				replaced = true
			}
			continue
		}
		updated = append(updated, raw)
	}
	if !replaced {
		updated = append(updated, key+"="+value)
	}
	return updated
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

func parseLinesPreserveWhitespace(output []byte) []string {
	if len(output) == 0 {
		return nil
	}
	raw := string(output)
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
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
