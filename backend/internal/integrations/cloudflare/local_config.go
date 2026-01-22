package cloudflare

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var ErrMissingConfigPath = fmt.Errorf("cloudflared config path is not set")

func UpdateLocalIngress(configPath, hostname string, port int) error {
	if strings.TrimSpace(hostname) == "" {
		return ErrMissingHostname
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	trimmed := strings.TrimSpace(configPath)
	if trimmed == "" {
		return ErrMissingConfigPath
	}
	path := expandUserPath(trimmed)
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read cloudflared config: %w", err)
	}

	var payload map[string]any
	if err := yaml.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("parse cloudflared config: %w", err)
	}
	if payload == nil {
		payload = map[string]any{}
	}

	ingress := coerceIngress(payload["ingress"])
	service := fmt.Sprintf("http://localhost:%d", port)
	payload["ingress"] = ensureIngressRule(ingress, hostname, service)

	updated, err := yaml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode cloudflared config: %w", err)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return fmt.Errorf("write cloudflared config: %w", err)
	}
	return nil
}
