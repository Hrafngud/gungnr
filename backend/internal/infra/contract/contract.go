package contract

import (
	"strings"
	"time"
)

const (
	VersionV1 = "v1"
)

const (
	QuickServiceExposureInternal      = "internal"
	QuickServiceExposureHostPublished = "host_published"
	QuickServicePublishLoopbackHost   = "127.0.0.1"
	QuickServiceDefaultNetwork        = "gungnr_quick_internal"
	QuickServiceDefaultPIDsLimit      = 128
	QuickServiceDefaultMemory         = "512m"
	QuickServiceDefaultCPUs           = "1.0"
	QuickServiceManagedLabelKey       = "io.gungnr.quick_service"
	QuickServiceManagedLabelValue     = "true"
	QuickServiceExposureLabelKey      = "io.gungnr.quick_service.exposure"
	QuickServiceNetworkLabelKey       = "io.gungnr.quick_service.network"
	QuickServiceNetworkLabelValue     = "true"
)

type TaskType string

const (
	// Canonical tunnel restart task used by the current API workflow.
	TaskTypeRestartTunnel TaskType = "restart_tunnel"

	// Task catalog staged for upcoming host-lifecycle bridge migrations.
	TaskTypeTunnelRestart          TaskType = "tunnel_restart"
	TaskTypeComposeUpStack         TaskType = "compose_up_stack"
	TaskTypeDockerStopContainer    TaskType = "docker_stop_container"
	TaskTypeDockerRestartContainer TaskType = "docker_restart_container"
	TaskTypeDockerRemoveContainer  TaskType = "docker_remove_container"
	TaskTypeDockerListContainers   TaskType = "docker_list_containers"
	TaskTypeDockerSystemDF         TaskType = "docker_system_df"
	TaskTypeDockerListVolumes      TaskType = "docker_list_volumes"
	TaskTypeDockerContainerLogs    TaskType = "docker_container_logs"
	TaskTypeDockerRuntimeCheck     TaskType = "docker_runtime_check"
	TaskTypeHostListenTCPPorts     TaskType = "host_listen_tcp_ports"
	TaskTypeDockerPublishedPorts   TaskType = "docker_published_ports"
	TaskTypeHostRuntimeStats       TaskType = "host_runtime_stats"
	TaskTypeHostRuntimeStream      TaskType = "host_runtime_stream"
	TaskTypeDockerRunQuickService  TaskType = "docker_run_quick_service"
	TaskTypeProjectFileWriteAtomic TaskType = "project_file_write_atomic"
	TaskTypeProjectFileCopy        TaskType = "project_file_copy"
	TaskTypeProjectFileRemove      TaskType = "project_file_remove"
	TaskTypeHostPortScan           TaskType = "host_port_scan"
	TaskTypeAPIHealthProbe         TaskType = "api_health_probe"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

func IsTerminalStatus(status Status) bool {
	return status == StatusSucceeded || status == StatusFailed
}

type Intent struct {
	Version   string         `json:"version"`
	IntentID  string         `json:"intent_id"`
	RequestID string         `json:"request_id"`
	TaskType  TaskType       `json:"task_type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type Claim struct {
	Version   string    `json:"version"`
	IntentID  string    `json:"intent_id"`
	Owner     string    `json:"owner"`
	ClaimedAt time.Time `json:"claimed_at"`
}

type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type Result struct {
	Version    string         `json:"version"`
	IntentID   string         `json:"intent_id"`
	RequestID  string         `json:"request_id"`
	TaskType   TaskType       `json:"task_type"`
	Status     Status         `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt time.Time      `json:"finished_at"`
	LogPath    string         `json:"log_path"`
	LogTail    []string       `json:"log_tail,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	Error      *Error         `json:"error,omitempty"`
}

func (r Result) Terminal() bool {
	return IsTerminalStatus(r.Status)
}

type RestartTunnelPayload struct {
	ConfigPath string `json:"config_path"`
}

type ComposeUpStackPayload struct {
	Project       string   `json:"project"`
	ProjectDir    string   `json:"project_dir,omitempty"`
	ConfigFiles   []string `json:"config_files,omitempty"`
	Build         bool     `json:"build,omitempty"`
	ForceRecreate bool     `json:"force_recreate,omitempty"`
}

type DockerStopContainerPayload struct {
	Container string `json:"container"`
}

type DockerRestartContainerPayload struct {
	Container string `json:"container"`
}

type DockerRemoveContainerPayload struct {
	Container     string `json:"container"`
	RemoveVolumes bool   `json:"remove_volumes,omitempty"`
}

type DockerListContainersPayload struct {
	IncludeAll bool `json:"include_all,omitempty"`
}

type DockerSystemDFPayload struct{}

type DockerListVolumesPayload struct{}

type DockerContainerLogsPayload struct {
	Container  string `json:"container"`
	Tail       int    `json:"tail,omitempty"`
	Follow     bool   `json:"follow,omitempty"`
	Timestamps bool   `json:"timestamps,omitempty"`
	Since      string `json:"since,omitempty"`
}

type DockerRuntimeCheckPayload struct{}

type HostListenTCPPortsPayload struct{}

type DockerPublishedPortsPayload struct{}

type HostRuntimeStatsPayload struct{}

type HostRuntimeStreamPayload struct{}

type DockerRunQuickServicePayload struct {
	Image         string `json:"image"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	ContainerName string `json:"container_name,omitempty"`
	ExposureMode  string `json:"exposure_mode,omitempty"`
	PublishHost   string `json:"publish_host,omitempty"`
	NetworkName   string `json:"network_name,omitempty"`
}

type ProjectFileWriteAtomicPayload struct {
	BasePath      string `json:"base_path"`
	Path          string `json:"path"`
	Content       string `json:"content"`
	Mode          uint32 `json:"mode,omitempty"`
	PreserveMode  bool   `json:"preserve_mode,omitempty"`
	CreateParents bool   `json:"create_parents,omitempty"`
}

type ProjectFileCopyPayload struct {
	BasePath        string `json:"base_path"`
	SourcePath      string `json:"source_path"`
	DestinationPath string `json:"destination_path"`
	Mode            uint32 `json:"mode,omitempty"`
	CreateParents   bool   `json:"create_parents,omitempty"`
}

type ProjectFileRemovePayload struct {
	BasePath       string `json:"base_path"`
	Path           string `json:"path"`
	IgnoreNotExist bool   `json:"ignore_not_exist,omitempty"`
}

type HostPortScanPayload struct {
	StartPort int `json:"start_port"`
	EndPort   int `json:"end_port"`
}

type APIHealthProbePayload struct {
	URL            string `json:"url"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func NormalizeQuickServiceExposureMode(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = strings.NewReplacer("-", "_", " ", "_").Replace(normalized)
	switch normalized {
	case QuickServiceExposureInternal, "internal_only":
		return QuickServiceExposureInternal
	case QuickServiceExposureHostPublished, "published", "publish":
		return QuickServiceExposureHostPublished
	default:
		return ""
	}
}

func NormalizeQuickServicePublishHost(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", QuickServicePublishLoopbackHost:
		return QuickServicePublishLoopbackHost
	case "localhost":
		return QuickServicePublishLoopbackHost
	default:
		return ""
	}
}
