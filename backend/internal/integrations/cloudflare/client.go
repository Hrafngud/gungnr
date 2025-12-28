package cloudflare

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go-notes/internal/config"
)

var (
	ErrMissingTunnel   = errors.New("CLOUDFLARED_TUNNEL_NAME is required")
	ErrMissingConfig   = errors.New("CLOUDFLARED_CONFIG is required")
	ErrMissingHostname = errors.New("hostname is required")
)

type Client struct {
	cfg config.Config
}

func NewClient(cfg config.Config) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) EnsureDNS(ctx context.Context, hostname string) error {
	if strings.TrimSpace(hostname) == "" {
		return ErrMissingHostname
	}
	if c.cfg.CloudflaredTunnel == "" {
		return ErrMissingTunnel
	}
	args := []string{"tunnel", "route", "dns"}
	if c.cfg.CloudflaredConfig != "" {
		args = append(args, "--config", c.cfg.CloudflaredConfig)
	}
	args = append(args, c.cfg.CloudflaredTunnel, hostname)

	cmd := exec.CommandContext(ctx, "cloudflared", args...)
	if env := c.commandEnv(); len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cloudflared dns route failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (c *Client) UpdateIngress(hostname string, port int) error {
	if strings.TrimSpace(hostname) == "" {
		return ErrMissingHostname
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	if c.cfg.CloudflaredConfig == "" {
		return ErrMissingConfig
	}

	raw, err := os.ReadFile(c.cfg.CloudflaredConfig)
	if err != nil {
		return fmt.Errorf("read cloudflared config: %w", err)
	}

	contents := string(raw)
	if strings.Contains(contents, "hostname: "+hostname) {
		return nil
	}

	lines := strings.Split(contents, "\n")
	insertIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "ingress:" {
			insertIdx = i + 1
			break
		}
	}
	if insertIdx == -1 {
		return fmt.Errorf("ingress section not found in cloudflared config")
	}

	newRule := []string{
		fmt.Sprintf("  - hostname: %s", hostname),
		fmt.Sprintf("    service: http://localhost:%d", port),
	}

	updated := append([]string{}, lines[:insertIdx]...)
	updated = append(updated, newRule...)
	updated = append(updated, lines[insertIdx:]...)

	foundCatchAll := false
	for _, line := range updated {
		if strings.Contains(line, "http_status:404") {
			foundCatchAll = true
			break
		}
	}
	if !foundCatchAll {
		return fmt.Errorf("cloudflared config missing http_status:404 catch-all")
	}

	if err := writeFileAtomic(c.cfg.CloudflaredConfig, []byte(strings.Join(updated, "\n"))); err != nil {
		return err
	}

	return nil
}

func (c *Client) RestartTunnel(ctx context.Context) error {
	if c.cfg.CloudflaredTunnel == "" {
		return ErrMissingTunnel
	}

	_ = exec.CommandContext(ctx, "pkill", "-f", "cloudflared tunnel run").Run()

	args := []string{"tunnel", "run"}
	if c.cfg.CloudflaredConfig != "" {
		args = append(args, "--config", c.cfg.CloudflaredConfig)
	}
	args = append(args, c.cfg.CloudflaredTunnel)

	cmd := exec.CommandContext(ctx, "cloudflared", args...)
	if env := c.commandEnv(); len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start cloudflared tunnel: %w", err)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	return nil
}

func (c *Client) commandEnv() []string {
	if strings.TrimSpace(c.cfg.CloudflareAPIToken) == "" {
		return nil
	}
	return []string{fmt.Sprintf("CLOUDFLARE_API_TOKEN=%s", c.cfg.CloudflareAPIToken)}
}

func writeFileAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat cloudflared config: %w", err)
	}

	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "cloudflared-config-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(temp.Name())

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(temp.Name(), info.Mode()); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("replace cloudflared config: %w", err)
	}
	return nil
}
