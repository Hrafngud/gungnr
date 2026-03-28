package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go-notes/internal/config"
	"go-notes/internal/integrations/cloudflare"
)

type DockerHealth struct {
	Status            string                           `json:"status"`
	Detail            string                           `json:"detail,omitempty"`
	Containers        int                              `json:"containers"`
	DBHostPublish     DBHostPublishHealthRef           `json:"dbHostPublish"`
	NetworkGuardrails DockerNetworkGuardrailsHealthRef `json:"networkGuardrails"`
	DaemonIsolation   DockerDaemonIsolationHealthRef   `json:"daemonIsolation"`
}

type DBHostPublishHealthRef struct {
	Mode    string `json:"mode"`
	Enabled bool   `json:"enabled"`
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
}

type DockerNetworkGuardrailsHealthRef struct {
	Mode          string `json:"mode"`
	ICCEnforced   bool   `json:"iccEnforced"`
	EdgeNetwork   string `json:"edgeNetwork"`
	CoreNetwork   string `json:"coreNetwork"`
	Fallback      bool   `json:"fallback"`
	FallbackNotes string `json:"fallbackNotes,omitempty"`
}

type DockerDaemonIsolationHealthRef struct {
	Mode            string   `json:"mode"`
	ActiveMode      string   `json:"activeMode,omitempty"`
	Supported       bool     `json:"supported"`
	Active          bool     `json:"active"`
	PreflightStatus string   `json:"preflightStatus"`
	ServerVersion   string   `json:"serverVersion,omitempty"`
	DockerRootDir   string   `json:"dockerRootDir,omitempty"`
	SocketPath      string   `json:"socketPath"`
	Rootless        bool     `json:"rootless"`
	UsernsRemap     bool     `json:"usernsRemap"`
	Blockers        []string `json:"blockers,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	RollbackMode    string   `json:"rollbackMode"`
	RollbackSteps   []string `json:"rollbackSteps,omitempty"`
}

type TunnelHealth struct {
	Status      string             `json:"status"`
	Detail      string             `json:"detail,omitempty"`
	Tunnel      string             `json:"tunnel,omitempty"`
	Connections int                `json:"connections"`
	ConfigPath  string             `json:"configPath,omitempty"`
	Diagnostics *TunnelDiagnostics `json:"diagnostics,omitempty"`
}

type TunnelDiagnostics struct {
	AccountID     string          `json:"accountId,omitempty"`
	ZoneID        string          `json:"zoneId,omitempty"`
	Tunnel        string          `json:"tunnel,omitempty"`
	Domain        string          `json:"domain,omitempty"`
	ConfigPath    string          `json:"configPath,omitempty"`
	TokenSet      bool            `json:"tokenSet,omitempty"`
	TunnelRefType string          `json:"tunnelRefType,omitempty"`
	Sources       SettingsSources `json:"sources,omitempty"`
}

type HealthService struct {
	host              *HostService
	settings          *SettingsService
	dbPublishMode     string
	dbPublishHost     string
	dbPublishPort     int
	dockerNetworkMode string
	daemonMode        string
}

func NewHealthService(host *HostService, settings *SettingsService, cfg config.Config) *HealthService {
	return &HealthService{
		host:              host,
		settings:          settings,
		dbPublishMode:     strings.TrimSpace(cfg.DBHostPublishMode),
		dbPublishHost:     strings.TrimSpace(cfg.DBHostPublishHost),
		dbPublishPort:     cfg.DBHostPublishPort,
		dockerNetworkMode: strings.TrimSpace(cfg.DockerNetworkMode),
		daemonMode:        strings.TrimSpace(cfg.DockerDaemonIsolation),
	}
}

func (s *HealthService) Docker(ctx context.Context) DockerHealth {
	dbPublish := DBHostPublishHealthRef{
		Mode:    "disabled",
		Enabled: false,
	}
	if strings.EqualFold(strings.TrimSpace(s.dbPublishMode), "loopback") {
		dbPublish.Mode = "loopback"
		dbPublish.Enabled = true
		dbPublish.Host = strings.TrimSpace(s.dbPublishHost)
		dbPublish.Port = s.dbPublishPort
	}
	networkGuardrails := DockerNetworkGuardrailsHealthRef{
		Mode:        "enforced",
		ICCEnforced: true,
		EdgeNetwork: "edge",
		CoreNetwork: "core",
	}
	if strings.EqualFold(strings.TrimSpace(s.dockerNetworkMode), "compat") {
		networkGuardrails.Mode = "compat"
		networkGuardrails.ICCEnforced = false
		networkGuardrails.Fallback = true
		networkGuardrails.FallbackNotes = "bridge ICC guardrail disabled via compat network fallback"
	}
	daemonIsolation := defaultDockerDaemonIsolationHealthRef(s.daemonMode)

	if s.host == nil {
		daemonIsolation.PreflightStatus = "error"
		daemonIsolation.Blockers = []string{"host service unavailable"}
		return DockerHealth{
			Status:            "error",
			Detail:            "host service unavailable",
			DBHostPublish:     dbPublish,
			NetworkGuardrails: networkGuardrails,
			DaemonIsolation:   daemonIsolation,
		}
	}

	// Keep host probe execution independent from request-context cancellation.
	// Some callers/proxies can apply short request deadlines that would
	// otherwise kill the docker command before it returns. Each bridge-backed
	// probe gets its own timeout budget so one slow read does not starve the next.
	runtimeCtx, runtimeCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer runtimeCancel()

	runtimeInfo, err := s.host.DockerRuntime(runtimeCtx)
	if err != nil {
		daemonIsolation.PreflightStatus = "error"
		daemonIsolation.Blockers = []string{err.Error()}
		return DockerHealth{
			Status:            "error",
			Detail:            err.Error(),
			DBHostPublish:     dbPublish,
			NetworkGuardrails: networkGuardrails,
			DaemonIsolation:   daemonIsolation,
		}
	}
	daemonIsolation = evaluateDockerDaemonIsolationHealth(s.daemonMode, runtimeInfo)

	countCtx, countCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer countCancel()

	count, err := s.host.CountRunningContainers(countCtx)
	if err != nil {
		return DockerHealth{
			Status:            "error",
			Detail:            err.Error(),
			DBHostPublish:     dbPublish,
			NetworkGuardrails: networkGuardrails,
			DaemonIsolation:   daemonIsolation,
		}
	}

	detail := fmt.Sprintf("%d containers running", count)
	return DockerHealth{
		Status:            "ok",
		Detail:            detail,
		Containers:        count,
		DBHostPublish:     dbPublish,
		NetworkGuardrails: networkGuardrails,
		DaemonIsolation:   daemonIsolation,
	}
}

const dockerSocketContractPath = "/var/run/docker.sock"

func defaultDockerDaemonIsolationHealthRef(mode string) DockerDaemonIsolationHealthRef {
	return DockerDaemonIsolationHealthRef{
		Mode:            strings.TrimSpace(mode),
		PreflightStatus: "ready",
		SocketPath:      dockerSocketContractPath,
		RollbackMode:    "disabled",
		RollbackSteps: []string{
			"Set DOCKER_DAEMON_ISOLATION_MODE=disabled in the panel runtime before restarting Gungnr.",
			"Restore the host Docker daemon to its non-isolated socket/configuration and restart docker.",
			"Restart the panel stack only after /var/run/docker.sock is reachable again from the current compose contract.",
		},
	}
}

func evaluateDockerDaemonIsolationHealth(mode string, runtime DockerRuntimeInfo) DockerDaemonIsolationHealthRef {
	ref := defaultDockerDaemonIsolationHealthRef(mode)
	ref.ServerVersion = strings.TrimSpace(runtime.ServerVersion)
	ref.DockerRootDir = strings.TrimSpace(runtime.DockerRootDir)
	ref.Rootless = runtime.Rootless
	ref.UsernsRemap = runtime.UsernsRemap
	ref.Warnings = append(ref.Warnings, runtime.Warnings...)

	activeMode, activeWarnings := detectDockerDaemonIsolationMode(runtime)
	ref.ActiveMode = activeMode
	ref.Warnings = append(ref.Warnings, activeWarnings...)
	ref.Active = activeMode == ref.Mode
	ref.Supported = ref.Active

	switch ref.Mode {
	case "disabled":
		ref.Supported = activeMode == "disabled"
		ref.Active = activeMode == "disabled"
		ref.PreflightStatus = "ready"
		if activeMode != "disabled" {
			ref.PreflightStatus = "warning"
			ref.Warnings = append(ref.Warnings, fmt.Sprintf("configured rollback mode is disabled but the daemon still reports %s isolation", activeMode))
		}
	case "userns-remap":
		if ref.Active {
			ref.PreflightStatus = "ready"
			break
		}
		ref.PreflightStatus = "blocked"
		ref.Blockers = append(ref.Blockers, fmt.Sprintf("docker daemon does not currently report %s as the active isolation mode", ref.Mode))
		ref.Blockers = append(ref.Blockers, "apply the host daemon userns-remap configuration and restart docker before keeping this mode selected")
	case "rootless":
		if ref.Active {
			ref.PreflightStatus = "ready"
			break
		}
		ref.PreflightStatus = "blocked"
		ref.Blockers = append(ref.Blockers, fmt.Sprintf("docker daemon does not currently report %s as the active isolation mode", ref.Mode))
		ref.Blockers = append(ref.Blockers, "current panel control paths stay supported only when the rootless daemon is presented through the mounted /var/run/docker.sock contract")
	default:
		ref.PreflightStatus = "blocked"
		ref.Blockers = append(ref.Blockers, "unsupported daemon isolation mode selected")
	}

	if ref.Mode != "disabled" && ref.ActiveMode != "" && ref.ActiveMode != ref.Mode {
		ref.Warnings = append(ref.Warnings, fmt.Sprintf("daemon currently reports %s while the selected rollout mode is %s", ref.ActiveMode, ref.Mode))
	}
	if ref.ServerVersion == "" {
		ref.Warnings = append(ref.Warnings, "docker runtime check did not report a server version")
	}

	return ref
}

func detectDockerDaemonIsolationMode(runtime DockerRuntimeInfo) (string, []string) {
	switch {
	case runtime.Rootless && runtime.UsernsRemap:
		return "rootless", []string{"daemon reports both rootless and user namespace remap; treating rootless as the active mode"}
	case runtime.Rootless:
		return "rootless", nil
	case runtime.UsernsRemap:
		return "userns-remap", nil
	default:
		return "disabled", nil
	}
}

func (s *HealthService) Tunnel(ctx context.Context) TunnelHealth {
	if s.settings == nil {
		return TunnelHealth{Status: "error", Detail: "settings service unavailable"}
	}

	cfg, sources, err := s.settings.ResolveConfigWithSources(ctx)
	if err != nil {
		return TunnelHealth{Status: "error", Detail: err.Error()}
	}

	diagnostics := &TunnelDiagnostics{
		AccountID:     strings.TrimSpace(cfg.CloudflareAccountID),
		ZoneID:        strings.TrimSpace(cfg.CloudflareZoneID),
		Tunnel:        strings.TrimSpace(cfg.CloudflaredTunnel),
		Domain:        strings.TrimSpace(cfg.Domain),
		ConfigPath:    strings.TrimSpace(cfg.CloudflaredConfig),
		TokenSet:      strings.TrimSpace(cfg.CloudflareAPIToken) != "",
		TunnelRefType: tunnelRefType(cfg.CloudflaredTunnel),
		Sources:       sources,
	}

	if strings.TrimSpace(cfg.CloudflareAPIToken) != "" &&
		strings.TrimSpace(cfg.CloudflareAccountID) != "" &&
		strings.TrimSpace(cfg.CloudflaredTunnel) != "" {
		client := cloudflare.NewClient(cfg)
		info, err := client.TunnelStatus(ctx)
		if err != nil {
			logTunnelHealthError("cloudflare api", err, diagnostics)
			status := "error"
			detail := err.Error()
			if errors.Is(err, cloudflare.ErrTunnelNotRemote) {
				status = "warning"
				detail = "Tunnel is locally managed; Cloudflare API checks require a remote-managed tunnel (config_src=cloudflare)."
			}
			return TunnelHealth{Status: status, Detail: detail, Diagnostics: diagnostics}
		}

		status := "ok"
		detail := fmt.Sprintf("Tunnel %s", info.Status)
		if info.Status == "" || strings.EqualFold(info.Status, "inactive") {
			status = "warning"
			detail = "Tunnel inactive"
		}
		if info.Connections == 0 {
			status = "warning"
			if detail == "" {
				detail = "No active connectors reported"
			}
		}

		return TunnelHealth{
			Status:      status,
			Detail:      detail,
			Tunnel:      info.Name,
			Connections: info.Connections,
			ConfigPath:  strings.TrimSpace(cfg.CloudflaredConfig),
			Diagnostics: diagnostics,
		}
	}

	configPath := strings.TrimSpace(cfg.CloudflaredConfig)
	if configPath == "" {
		logTunnelHealthError("cloudflared config", errors.New("cloudflared config path is not set"), diagnostics)
		return TunnelHealth{Status: "missing", Detail: "cloudflared config path is not set", Diagnostics: diagnostics}
	}
	configPath = expandUserPath(configPath)

	info, err := readCloudflaredConfig(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logTunnelHealthError("cloudflared config", err, diagnostics)
			return TunnelHealth{
				Status:      "missing",
				Detail:      fmt.Sprintf("cloudflared config not found at %s", configPath),
				ConfigPath:  configPath,
				Diagnostics: diagnostics,
			}
		}
		logTunnelHealthError("cloudflared config", err, diagnostics)
		return TunnelHealth{
			Status:      "error",
			Detail:      fmt.Sprintf("read cloudflared config: %v", err),
			ConfigPath:  configPath,
			Diagnostics: diagnostics,
		}
	}

	tunnelName := strings.TrimSpace(cfg.CloudflaredTunnel)
	if tunnelName == "" {
		tunnelName = info.Tunnel
	}
	if tunnelName == "" {
		logTunnelHealthError("cloudflared config", errors.New("cloudflared tunnel name is not set"), diagnostics)
		return TunnelHealth{
			Status:      "missing",
			Detail:      "cloudflared tunnel name is not set",
			ConfigPath:  configPath,
			Diagnostics: diagnostics,
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	if info.OriginCert == "" {
		info.OriginCert = findOriginCert(configPath)
	}

	args := []string{"tunnel"}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}
	if info.CredentialsFile != "" {
		args = append(args, "--credentials-file", info.CredentialsFile)
	}
	if info.OriginCert != "" {
		args = append(args, "--origincert", info.OriginCert)
	}
	args = append(args, "info", "--output", "json", tunnelName)

	cmd := exec.CommandContext(checkCtx, "cloudflared", args...)
	if strings.TrimSpace(cfg.CloudflareAPIToken) != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("CLOUDFLARE_API_TOKEN=%s", cfg.CloudflareAPIToken))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := compactOutput(output)
		if detail == "" {
			detail = err.Error()
		}
		logTunnelHealthCommandError(detail, tunnelName, configPath, info.CredentialsFile, info.OriginCert)
		if strings.Contains(detail, "Cannot determine default origin certificate path") {
			return TunnelHealth{
				Status:      "missing",
				Detail:      "Cloudflared origin cert is missing. Set origincert in config.yml or mount cert.pem.",
				Tunnel:      tunnelName,
				ConfigPath:  configPath,
				Diagnostics: diagnostics,
			}
		}
		return TunnelHealth{
			Status:      "error",
			Detail:      detail,
			Tunnel:      tunnelName,
			ConfigPath:  configPath,
			Diagnostics: diagnostics,
		}
	}

	parsed := false
	connections := 0
	var payload interface{}
	if err := json.Unmarshal(output, &payload); err == nil {
		parsed = true
		connections = countConnections(payload)
	}

	status := "ok"
	detail := "Tunnel info fetched"
	if parsed {
		if connections > 0 {
			detail = fmt.Sprintf("%d active connectors", connections)
		} else {
			status = "warning"
			detail = "No active connectors reported"
		}
	} else {
		detail = "Tunnel info fetched (unparsed output)"
	}

	return TunnelHealth{
		Status:      status,
		Detail:      detail,
		Tunnel:      tunnelName,
		Connections: connections,
		ConfigPath:  configPath,
		Diagnostics: diagnostics,
	}
}

type cloudflaredConfigInfo struct {
	Tunnel          string
	CredentialsFile string
	OriginCert      string
}

func readCloudflaredConfig(path string) (cloudflaredConfigInfo, error) {
	var info cloudflaredConfigInfo
	raw, err := os.ReadFile(path)
	if err != nil {
		return info, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		key, value, ok := parseCloudflaredConfigLine(scanner.Text())
		if !ok {
			continue
		}
		switch key {
		case "tunnel":
			if info.Tunnel == "" {
				info.Tunnel = value
			}
		case "credentials-file", "credential-file":
			if info.CredentialsFile == "" {
				info.CredentialsFile = expandUserPath(value)
			}
		case "origincert", "origin-cert":
			if info.OriginCert == "" {
				info.OriginCert = expandUserPath(value)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return info, err
	}

	if info.CredentialsFile == "" && info.Tunnel != "" {
		candidate := filepath.Join(filepath.Dir(path), info.Tunnel+".json")
		if _, err := os.Stat(candidate); err == nil {
			info.CredentialsFile = candidate
		}
	}
	if info.OriginCert == "" {
		info.OriginCert = findOriginCert(path)
	}
	return info, nil
}

func parseCloudflaredConfigLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		return "", "", false
	}
	if hash := strings.Index(value, "#"); hash != -1 {
		value = strings.TrimSpace(value[:hash])
	}
	value = strings.Trim(value, "\"'")
	return key, value, true
}

func findOriginCert(configPath string) string {
	candidates := []string{}
	if configPath != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(configPath), "cert.pem"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".cloudflared", "cert.pem"),
			filepath.Join(home, ".cloudflare-warp", "cert.pem"),
			filepath.Join(home, "cloudflare-warp", "cert.pem"),
		)
	}
	candidates = append(candidates,
		filepath.Join(string(filepath.Separator), "etc", "cloudflared", "cert.pem"),
		filepath.Join(string(filepath.Separator), "usr", "local", "etc", "cloudflared", "cert.pem"),
	)

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func compactOutput(output []byte) string {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return ""
	}
	compact := strings.Join(strings.Fields(trimmed), " ")
	const maxLen = 280
	if len(compact) > maxLen {
		return compact[:maxLen] + "..."
	}
	return compact
}

func countConnections(payload interface{}) int {
	switch value := payload.(type) {
	case []interface{}:
		return len(value)
	case map[string]interface{}:
		for _, key := range []string{"connections", "connectors", "activeConnectors"} {
			if raw, ok := value[key]; ok {
				if list, ok := raw.([]interface{}); ok {
					return len(list)
				}
			}
		}
	}
	return 0
}

func tunnelRefType(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if looksLikeUUID(trimmed) {
		return "id"
	}
	return "name"
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, ch := range value {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		case ch == '-':
		default:
			return false
		}
		if (i == 8 || i == 13 || i == 18 || i == 23) && ch != '-' {
			return false
		}
	}
	return true
}

func logTunnelHealthError(scope string, err error, diagnostics *TunnelDiagnostics) {
	if err == nil {
		return
	}
	if diagnostics == nil {
		log.Printf("tunnel health %s error: %v", scope, err)
		return
	}
	log.Printf(
		"tunnel health %s error: %v (acct=%s zone=%s tunnel=%s ref=%s domain=%s config=%s token=%t sources=acct:%s zone:%s token:%s config:%s)",
		scope,
		err,
		diagnostics.AccountID,
		diagnostics.ZoneID,
		diagnostics.Tunnel,
		diagnostics.TunnelRefType,
		diagnostics.Domain,
		diagnostics.ConfigPath,
		diagnostics.TokenSet,
		diagnostics.Sources.CloudflareAccountID,
		diagnostics.Sources.CloudflareZoneID,
		diagnostics.Sources.CloudflareToken,
		diagnostics.Sources.CloudflaredConfigPath,
	)
}

func logTunnelHealthCommandError(detail, tunnel, configPath, credentialsFile, originCert string) {
	log.Printf(
		"tunnel health cloudflared info error: %s (tunnel=%s config=%s creds=%s origincert=%s)",
		detail,
		tunnel,
		configPath,
		credentialsFile,
		originCert,
	)
}
