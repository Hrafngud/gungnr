package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go-notes/internal/integrations/cloudflare"
)

type DockerHealth struct {
	Status     string `json:"status"`
	Detail     string `json:"detail,omitempty"`
	Containers int    `json:"containers"`
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
	host     *HostService
	settings *SettingsService
}

func NewHealthService(host *HostService, settings *SettingsService) *HealthService {
	return &HealthService{host: host, settings: settings}
}

func (s *HealthService) Docker(ctx context.Context) DockerHealth {
	if s.host == nil {
		return DockerHealth{Status: "error", Detail: "host service unavailable"}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	containers, err := s.host.ListContainers(checkCtx)
	if err != nil {
		return DockerHealth{Status: "error", Detail: err.Error()}
	}

	detail := fmt.Sprintf("%d containers running", len(containers))
	return DockerHealth{
		Status:     "ok",
		Detail:     detail,
		Containers: len(containers),
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
		return TunnelHealth{Status: "missing", Detail: "cloudflared config path is not set", Diagnostics: diagnostics}
	}
	configPath = expandUserPath(configPath)

	info, err := readCloudflaredConfig(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TunnelHealth{
				Status:      "missing",
				Detail:      fmt.Sprintf("cloudflared config not found at %s", configPath),
				ConfigPath:  configPath,
				Diagnostics: diagnostics,
			}
		}
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
