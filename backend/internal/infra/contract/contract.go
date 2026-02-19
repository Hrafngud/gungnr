package contract

import (
	"time"
)

const (
	VersionV1 = "v1"
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
	TaskTypeDockerRunQuickService  TaskType = "docker_run_quick_service"
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
	Version    string    `json:"version"`
	IntentID   string    `json:"intent_id"`
	RequestID  string    `json:"request_id"`
	TaskType   TaskType  `json:"task_type"`
	Status     Status    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	LogPath    string    `json:"log_path"`
	LogTail    []string  `json:"log_tail,omitempty"`
	Error      *Error    `json:"error,omitempty"`
}

func (r Result) Terminal() bool {
	return IsTerminalStatus(r.Status)
}

type RestartTunnelPayload struct {
	ConfigPath string `json:"config_path"`
}

type ComposeUpStackPayload struct {
	Project     string   `json:"project"`
	ProjectDir  string   `json:"project_dir,omitempty"`
	ConfigFiles []string `json:"config_files,omitempty"`
	Build       bool     `json:"build,omitempty"`
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

type DockerRunQuickServicePayload struct {
	Image         string `json:"image"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	ContainerName string `json:"container_name,omitempty"`
}

type HostPortScanPayload struct {
	StartPort int `json:"start_port"`
	EndPort   int `json:"end_port"`
}

type APIHealthProbePayload struct {
	URL            string `json:"url"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
