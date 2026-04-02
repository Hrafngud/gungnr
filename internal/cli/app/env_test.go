package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePanelRuntimeEnv(t *testing.T) {
	runtimeEnv, err := resolvePanelRuntimeEnv("/home/tester/gungnr", func() (string, error) {
		return "952", nil
	})
	if err != nil {
		t.Fatalf("ResolvePanelRuntimeEnv() error = %v", err)
	}
	if runtimeEnv.InfraQueueRoot != "/home/tester/gungnr/templates/.infra" {
		t.Fatalf("expected InfraQueueRoot %q, got %q", "/home/tester/gungnr/templates/.infra", runtimeEnv.InfraQueueRoot)
	}
	if runtimeEnv.DockerSocketGID != "952" {
		t.Fatalf("expected DockerSocketGID %q, got %q", "952", runtimeEnv.DockerSocketGID)
	}
	if runtimeEnv.DockerNetworkMode != "compat" {
		t.Fatalf("expected DockerNetworkMode %q, got %q", "compat", runtimeEnv.DockerNetworkMode)
	}
}

func TestResolvePanelRuntimeEnvRequiresSocketGroup(t *testing.T) {
	_, err := resolvePanelRuntimeEnv("/home/tester/gungnr", func() (string, error) {
		return "", nil
	})
	if err == nil || err.Error() != "DOCKER_SOCKET_GID is required" {
		t.Fatalf("expected missing DOCKER_SOCKET_GID error, got %v", err)
	}
}

func TestResolvePanelRuntimeEnvPropagatesResolverError(t *testing.T) {
	_, err := resolvePanelRuntimeEnv("/home/tester/gungnr", func() (string, error) {
		return "", errors.New("socket unavailable")
	})
	if err == nil || err.Error() != "socket unavailable" {
		t.Fatalf("expected socket resolver error, got %v", err)
	}
}

func TestBootstrapEnvValidateRequiresDockerSocketGID(t *testing.T) {
	env := BootstrapEnv{
		SessionSecret:       "secret",
		GitHubClientID:      "client-id",
		GitHubClientSecret:  "client-secret",
		GitHubCallbackURL:   "https://panel.example.com/auth/callback",
		SuperUserGitHubName: "octocat",
		SuperUserGitHubID:   1,
		TemplatesDir:        "/templates",
		Domain:              "example.com",
		CloudflareAPIToken:  "token",
		CloudflareAccountID: "account",
		CloudflareZoneID:    "zone",
		CloudflareTunnelID:  "tunnel",
		CloudflaredConfig:   "/home/tester/.cloudflared/config.yml",
		CloudflaredTunnel:   "panel",
		CloudflaredDir:      "/home/tester/.cloudflared",
		InfraQueueRoot:      "/templates/.infra",
		DockerNetworkMode:   "compat",
		DatabaseURL:         "postgres://notes:notes@db:5432/notes?sslmode=disable",
	}

	err := env.Validate()
	if err == nil || err.Error() != "DOCKER_SOCKET_GID is required" {
		t.Fatalf("expected missing DOCKER_SOCKET_GID error, got %v", err)
	}
}

func TestRefreshPanelRuntimeEnvEntries(t *testing.T) {
	dataDir := t.TempDir()
	envPath := filepath.Join(dataDir, ".env")
	if err := os.WriteFile(envPath, []byte(strings.Join([]string{
		"SESSION_SECRET=secret",
		"DOCKER_SOCKET_GID=0",
		"INFRA_QUEUE_ROOT=/stale",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	err := refreshPanelRuntimeEnvEntries(envPath, dataDir, func() (string, error) {
		return "952", nil
	})
	if err != nil {
		t.Fatalf("refreshPanelRuntimeEnvEntries() error = %v", err)
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	updated := string(content)
	if !strings.Contains(updated, "DOCKER_SOCKET_GID=952") {
		t.Fatalf("expected refreshed socket gid in env, got %q", updated)
	}
	if !strings.Contains(updated, "INFRA_QUEUE_ROOT="+filepath.Join(dataDir, "templates", ".infra")) {
		t.Fatalf("expected refreshed infra queue root in env, got %q", updated)
	}
	if !strings.Contains(updated, "DOCKER_NETWORK_GUARDRAILS_MODE=compat") {
		t.Fatalf("expected refreshed docker network mode in env, got %q", updated)
	}
}
