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
	path, payload, err := readLocalConfigPayload(configPath)
	if err != nil {
		return err
	}

	ingress := coerceIngress(payload["ingress"])
	service := fmt.Sprintf("http://localhost:%d", port)
	payload["ingress"] = ensureIngressRule(ingress, hostname, service)

	return writeLocalConfigPayload(path, payload)
}

func ListLocalIngressRules(configPath string) ([]IngressRule, error) {
	_, payload, err := readLocalConfigPayload(configPath)
	if err != nil {
		return nil, err
	}
	return ingressRulesFromConfig(coerceIngress(payload["ingress"])), nil
}

func RemoveLocalIngressHostnames(configPath string, hostnames []string) ([]IngressRule, error) {
	targets := normalizeHostnameSet(hostnames)
	if len(targets) == 0 {
		return []IngressRule{}, nil
	}

	path, payload, err := readLocalConfigPayload(configPath)
	if err != nil {
		return nil, err
	}

	ingress := coerceIngress(payload["ingress"])
	removed, nextIngress := removeIngressRules(ingress, targets)
	if len(removed) == 0 {
		return []IngressRule{}, nil
	}

	payload["ingress"] = ensureCatchAllRule(nextIngress)
	if err := writeLocalConfigPayload(path, payload); err != nil {
		return nil, err
	}
	return removed, nil
}

func readLocalConfigPayload(configPath string) (string, map[string]any, error) {
	trimmed := strings.TrimSpace(configPath)
	if trimmed == "" {
		return "", nil, ErrMissingConfigPath
	}
	path := expandUserPath(trimmed)
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("read cloudflared config: %w", err)
	}

	var payload map[string]any
	if err := yaml.Unmarshal(raw, &payload); err != nil {
		return "", nil, fmt.Errorf("parse cloudflared config: %w", err)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return path, payload, nil
}

func writeLocalConfigPayload(path string, payload map[string]any) error {
	updated, err := yaml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode cloudflared config: %w", err)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return fmt.Errorf("write cloudflared config: %w", err)
	}
	return nil
}
