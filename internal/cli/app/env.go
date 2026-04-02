package app

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"gungnr-cli/internal/cli/integrations/docker"
	"gungnr-cli/internal/cli/integrations/filesystem"
)

const (
	DefaultPostgresUser     = "notes"
	DefaultPostgresPassword = "notes"
	DefaultPostgresDB       = "notes"
)

type BootstrapEnv struct {
	AppEnv              string
	Port                string
	DatabaseURL         string
	DBMaxOpenConns      int
	DBMaxIdleConns      int
	DBConnMaxLifetime   int
	CORSAllowedOrigins  string
	SessionSecret       string
	SessionTTLHours     int
	CookieDomain        string
	GitHubClientID      string
	GitHubClientSecret  string
	GitHubCallbackURL   string
	GitHubTemplateOwner string
	GitHubTemplateRepo  string
	GitHubRepoOwner     string
	GitHubRepoPrivate   bool
	SuperUserGitHubName string
	SuperUserGitHubID   int64
	TemplatesDir        string
	Domain              string
	CloudflareAPIToken  string
	CloudflareAccountID string
	CloudflareZoneID    string
	CloudflareTunnelID  string
	CloudflaredConfig   string
	CloudflaredTunnel   string
	CloudflaredDir      string
	InfraQueueRoot      string
	DockerSocketGID     string
	DockerNetworkMode   string
	PostgresUser        string
	PostgresPassword    string
	PostgresDB          string
	ViteAPIBaseURL      string
}

type PanelRuntimeEnv struct {
	InfraQueueRoot    string
	DockerSocketGID   string
	DockerNetworkMode string
}

func ResolvePanelRuntimeEnv(dataDir string) (PanelRuntimeEnv, error) {
	return resolvePanelRuntimeEnv(dataDir, docker.DockerSocketGID)
}

func RefreshPanelRuntimeEnvEntries(envPath, dataDir string) error {
	return refreshPanelRuntimeEnvEntries(envPath, dataDir, docker.DockerSocketGID)
}

func resolvePanelRuntimeEnv(dataDir string, socketGIDResolver func() (string, error)) (PanelRuntimeEnv, error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return PanelRuntimeEnv{}, errors.New("runtime data directory is required")
	}
	if socketGIDResolver == nil {
		return PanelRuntimeEnv{}, errors.New("docker socket group resolver is required")
	}

	socketGID, err := socketGIDResolver()
	if err != nil {
		return PanelRuntimeEnv{}, err
	}

	runtimeEnv := PanelRuntimeEnv{
		InfraQueueRoot:    filepath.Join(dataDir, "templates", ".infra"),
		DockerSocketGID:   socketGID,
		DockerNetworkMode: "compat",
	}
	if err := runtimeEnv.Validate(); err != nil {
		return PanelRuntimeEnv{}, err
	}
	return runtimeEnv, nil
}

func refreshPanelRuntimeEnvEntries(envPath, dataDir string, socketGIDResolver func() (string, error)) error {
	envPath = strings.TrimSpace(envPath)
	if envPath == "" {
		return errors.New("runtime env path is required")
	}

	runtimeEnv, err := resolvePanelRuntimeEnv(dataDir, socketGIDResolver)
	if err != nil {
		return err
	}
	if err := filesystem.UpsertEnvFileEntries(envPath, runtimeEnv.Entries()); err != nil {
		return fmt.Errorf("unable to refresh runtime env entries in %s: %w", envPath, err)
	}
	return nil
}

func (env PanelRuntimeEnv) Validate() error {
	if strings.TrimSpace(env.InfraQueueRoot) == "" {
		return errors.New("INFRA_QUEUE_ROOT is required")
	}
	if strings.TrimSpace(env.DockerSocketGID) == "" {
		return errors.New("DOCKER_SOCKET_GID is required")
	}
	if strings.TrimSpace(env.DockerNetworkMode) == "" {
		return errors.New("DOCKER_NETWORK_GUARDRAILS_MODE is required")
	}
	return nil
}

func (env PanelRuntimeEnv) Entries() []filesystem.EnvEntry {
	return []filesystem.EnvEntry{
		{Key: "INFRA_QUEUE_ROOT", Value: env.InfraQueueRoot},
		{Key: "DOCKER_SOCKET_GID", Value: env.DockerSocketGID},
		{Key: "DOCKER_NETWORK_GUARDRAILS_MODE", Value: env.DockerNetworkMode},
	}
}

func (env PanelRuntimeEnv) Apply(target *BootstrapEnv) error {
	if target == nil {
		return errors.New("bootstrap env target is required")
	}
	if err := env.Validate(); err != nil {
		return err
	}
	target.InfraQueueRoot = env.InfraQueueRoot
	target.DockerSocketGID = env.DockerSocketGID
	target.DockerNetworkMode = env.DockerNetworkMode
	return nil
}

func (env BootstrapEnv) Validate() error {
	required := map[string]string{
		"SESSION_SECRET":                 env.SessionSecret,
		"GITHUB_CLIENT_ID":               env.GitHubClientID,
		"GITHUB_CLIENT_SECRET":           env.GitHubClientSecret,
		"GITHUB_CALLBACK_URL":            env.GitHubCallbackURL,
		"SUPERUSER_GH_NAME":              env.SuperUserGitHubName,
		"SUPER_GH_ID":                    strconv.FormatInt(env.SuperUserGitHubID, 10),
		"TEMPLATES_DIR":                  env.TemplatesDir,
		"DOMAIN":                         env.Domain,
		"CLOUDFLARE_API_TOKEN":           env.CloudflareAPIToken,
		"CLOUDFLARE_ACCOUNT_ID":          env.CloudflareAccountID,
		"CLOUDFLARE_ZONE_ID":             env.CloudflareZoneID,
		"CLOUDFLARE_TUNNEL_ID":           env.CloudflareTunnelID,
		"CLOUDFLARED_CONFIG":             env.CloudflaredConfig,
		"CLOUDFLARED_TUNNEL_NAME":        env.CloudflaredTunnel,
		"CLOUDFLARED_DIR":                env.CloudflaredDir,
		"INFRA_QUEUE_ROOT":               env.InfraQueueRoot,
		"DOCKER_SOCKET_GID":              env.DockerSocketGID,
		"DOCKER_NETWORK_GUARDRAILS_MODE": env.DockerNetworkMode,
	}

	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}

	if env.SuperUserGitHubID == 0 {
		return errors.New("SUPER_GH_ID must be non-zero")
	}
	if env.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	return nil
}

func (env BootstrapEnv) Entries() []filesystem.EnvEntry {
	entries := []filesystem.EnvEntry{
		{Key: "APP_ENV", Value: env.AppEnv},
		{Key: "PORT", Value: env.Port},
		{Key: "POSTGRES_USER", Value: env.PostgresUser},
		{Key: "POSTGRES_PASSWORD", Value: env.PostgresPassword},
		{Key: "POSTGRES_DB", Value: env.PostgresDB},
		{Key: "DATABASE_URL", Value: env.DatabaseURL},
		{Key: "DB_MAX_OPEN_CONNS", Value: strconv.Itoa(env.DBMaxOpenConns)},
		{Key: "DB_MAX_IDLE_CONNS", Value: strconv.Itoa(env.DBMaxIdleConns)},
		{Key: "DB_CONN_MAX_LIFETIME_MIN", Value: strconv.Itoa(env.DBConnMaxLifetime)},
		{Key: "CORS_ALLOWED_ORIGINS", Value: env.CORSAllowedOrigins},
		{Key: "SESSION_SECRET", Value: env.SessionSecret},
		{Key: "SESSION_TTL_HOURS", Value: strconv.Itoa(env.SessionTTLHours)},
		{Key: "COOKIE_DOMAIN", Value: env.CookieDomain},
		{Key: "SUPERUSER_GH_NAME", Value: env.SuperUserGitHubName},
		{Key: "SUPER_GH_ID", Value: strconv.FormatInt(env.SuperUserGitHubID, 10)},
		{Key: "GITHUB_CLIENT_ID", Value: env.GitHubClientID},
		{Key: "GITHUB_CLIENT_SECRET", Value: env.GitHubClientSecret},
		{Key: "GITHUB_CALLBACK_URL", Value: env.GitHubCallbackURL},
		{Key: "GITHUB_TEMPLATE_OWNER", Value: env.GitHubTemplateOwner},
		{Key: "GITHUB_TEMPLATE_REPO", Value: env.GitHubTemplateRepo},
		{Key: "GITHUB_REPO_PRIVATE", Value: strconv.FormatBool(env.GitHubRepoPrivate)},
		{Key: "TEMPLATES_DIR", Value: env.TemplatesDir},
		{Key: "DOMAIN", Value: env.Domain},
		{Key: "CLOUDFLARE_API_TOKEN", Value: env.CloudflareAPIToken},
		{Key: "CLOUDFLARE_ACCOUNT_ID", Value: env.CloudflareAccountID},
		{Key: "CLOUDFLARE_ZONE_ID", Value: env.CloudflareZoneID},
		{Key: "CLOUDFLARE_TUNNEL_ID", Value: env.CloudflareTunnelID},
		{Key: "CLOUDFLARED_CONFIG", Value: env.CloudflaredConfig},
		{Key: "CLOUDFLARED_TUNNEL_NAME", Value: env.CloudflaredTunnel},
		{Key: "CLOUDFLARED_DIR", Value: env.CloudflaredDir},
		{Key: "INFRA_QUEUE_ROOT", Value: env.InfraQueueRoot},
		{Key: "DOCKER_SOCKET_GID", Value: env.DockerSocketGID},
		{Key: "DOCKER_NETWORK_GUARDRAILS_MODE", Value: env.DockerNetworkMode},
		{Key: "VITE_API_BASE_URL", Value: env.ViteAPIBaseURL},
	}

	if strings.TrimSpace(env.GitHubRepoOwner) != "" {
		entries = append(entries, filesystem.EnvEntry{Key: "GITHUB_REPO_OWNER", Value: env.GitHubRepoOwner})
	}

	return entries
}

func GenerateSessionSecret(bytesLen int) (string, error) {
	if bytesLen <= 0 {
		return "", errors.New("secret length must be positive")
	}
	buffer := make([]byte, bytesLen)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("unable to generate random secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func BuildDatabaseURL(user, password, name string) string {
	return fmt.Sprintf("postgres://%s:%s@db:5432/%s?sslmode=disable", url.PathEscape(user), url.PathEscape(password), url.PathEscape(name))
}

func BuildCORSOrigins(hostname string) string {
	origins := []string{
		fmt.Sprintf("https://%s", hostname),
		"http://localhost:4173",
		"http://127.0.0.1:4173",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
	}

	seen := make(map[string]struct{}, len(origins))
	var unique []string
	for _, origin := range origins {
		if origin == "" {
			continue
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		unique = append(unique, origin)
	}

	return strings.Join(unique, ",")
}
