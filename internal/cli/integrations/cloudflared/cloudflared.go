package cloudflared

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gungnr-cli/internal/cli/integrations/command"
	"gungnr-cli/internal/cli/integrations/filesystem"
)

type TunnelInfo struct {
	ID              string
	Name            string
	CredentialsFile string
}

func CheckInstalled() error {
	if _, err := exec.LookPath("cloudflared"); err != nil {
		return errors.New("cloudflared not found in PATH. Install cloudflared and retry")
	}

	if _, err := command.Run("cloudflared", "--version"); err != nil {
		return fmt.Errorf("cloudflared check failed: %w", err)
	}

	return nil
}

func Login() error {
	return command.RunInteractive("cloudflared", "tunnel", "login")
}

func WaitForOriginCert(cloudflaredDir string, timeout time.Duration) (string, error) {
	credentialsPath := filepath.Join(cloudflaredDir, "cert.pem")
	if err := filesystem.WaitForFile(credentialsPath, timeout); err != nil {
		return "", err
	}
	return credentialsPath, nil
}

func CreateTunnel(cloudflaredDir, tunnelName string) (*TunnelInfo, error) {
	output, err := command.Run("cloudflared", "tunnel", "create", tunnelName)
	if err != nil {
		return nil, err
	}

	tunnelID, err := parseTunnelID(output)
	if err != nil {
		return nil, err
	}

	credentialsFile := filepath.Join(cloudflaredDir, tunnelID+".json")
	if err := filesystem.WaitForFile(credentialsFile, 2*time.Minute); err != nil {
		return nil, fmt.Errorf("cloudflared tunnel credentials not found after create: %w", err)
	}

	return &TunnelInfo{ID: tunnelID, Name: tunnelName, CredentialsFile: credentialsFile}, nil
}

func WriteConfig(cloudflaredDir string, tunnel *TunnelInfo, hostname string) (string, error) {
	if err := os.MkdirAll(cloudflaredDir, 0o755); err != nil {
		return "", fmt.Errorf("unable to create cloudflared directory: %w", err)
	}

	configPath := filepath.Join(cloudflaredDir, "config.yml")
	if _, err := os.Stat(configPath); err == nil {
		if err := filesystem.CopyFile(configPath, configPath+".bak"); err != nil {
			return "", fmt.Errorf("unable to backup existing config: %w", err)
		}
	}

	credentialsFile := tunnel.CredentialsFile
	if credentialsFile == "" {
		credentialsFile = filepath.Join(cloudflaredDir, tunnel.ID+".json")
	}

	originCert := filepath.Join(cloudflaredDir, "cert.pem")
	config := fmt.Sprintf("tunnel: %s\ncredentials-file: %s\norigincert: %s\ningress:\n  - hostname: %s\n    service: http://localhost:80\n  - service: http_status:404\n", tunnel.ID, credentialsFile, originCert, hostname)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return "", fmt.Errorf("unable to write cloudflared config: %w", err)
	}

	return configPath, nil
}

func StartTunnel(configPath string) (string, error) {
	logPath := filepath.Join(filepath.Dir(configPath), "cloudflared.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("open cloudflared log file: %w", err)
	}
	cmd := exec.Command("cloudflared", "--config", configPath, "tunnel", "run")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return "", fmt.Errorf("cloudflared tunnel run failed: %w", err)
	}
	_ = logFile.Close()
	return logPath, nil
}

func WaitForRunning(tunnelID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		output, err := command.Run("cloudflared", "tunnel", "info", tunnelID)
		if err == nil {
			if connections, ok := parseActiveConnections(output); ok {
				if connections > 0 {
					return nil
				}
				lastErr = fmt.Errorf("active connections reported as %d", connections)
			} else if connectors, ok := parseConnectorCount(output); ok {
				if connectors > 0 {
					return nil
				}
				lastErr = fmt.Errorf("tunnel reported %d connectors", connectors)
			} else if strings.Contains(strings.ToLower(output), "status: healthy") {
				return nil
			} else {
				lastErr = fmt.Errorf("unable to confirm tunnel status from output: %s", output)
			}
		} else {
			lastErr = err
		}

		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return errors.New("tunnel did not report active connections before timeout")
		}
		time.Sleep(5 * time.Second)
	}
}

func RouteDNS(tunnelID, hostname string) error {
	_, err := command.Run("cloudflared", "tunnel", "route", "dns", tunnelID, hostname)
	return err
}

func parseTunnelID(output string) (string, error) {
	re := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	match := re.FindString(output)
	if match == "" {
		return "", fmt.Errorf("unable to parse tunnel ID from cloudflared output: %s", output)
	}
	return match, nil
}

func parseActiveConnections(output string) (int, bool) {
	re := regexp.MustCompile(`(?i)active connections:\s*([0-9]+)`)
	match := re.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return value, true
}

func parseConnectorCount(output string) (int, bool) {
	if !strings.Contains(output, "CONNECTOR ID") {
		return 0, false
	}
	re := regexp.MustCompile(`(?m)^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)
	matches := re.FindAllString(output, -1)
	if len(matches) == 0 {
		return 0, false
	}
	return len(matches), true
}
