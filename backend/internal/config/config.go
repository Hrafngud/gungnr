package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	AppEnv              string
	Port                string
	DatabaseURL         string
	DBMaxOpenConns      int
	DBMaxIdleConns      int
	DBConnMaxLifetime   time.Duration
	AllowedOrigins      []string
	SessionSecret       string
	SessionTTL          time.Duration
	CookieDomain        string
	AdminLogin          string
	AdminPassword       string
	GitHubClientID      string
	GitHubClientSecret  string
	GitHubCallbackURL   string
	GitHubAllowedUsers  []string
	GitHubAllowedOrg    string
	GitHubTemplateOwner string
	GitHubTemplateRepo  string
	GitHubRepoOwner     string
	GitHubRepoPrivate   bool
	TemplatesDir        string
	Domain              string
	CloudflareAPIToken  string
	CloudflareAccountID string
	CloudflareZoneID    string
	CloudflareTunnelID  string
	CloudflaredConfig   string
	CloudflaredTunnel   string
}

func Load() (Config, error) {
	v := viper.New()
	_ = godotenv.Load()

	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("..")

	v.SetDefault("APP_ENV", "local")
	v.SetDefault("PORT", "8080")
	v.SetDefault("DATABASE_URL", "")
	v.SetDefault("DB_MAX_OPEN_CONNS", 20)
	v.SetDefault("DB_MAX_IDLE_CONNS", 10)
	v.SetDefault("DB_CONN_MAX_LIFETIME_MIN", 30)
	v.SetDefault("CORS_ALLOWED_ORIGINS", "http://localhost:4173,http://127.0.0.1:4173,http://localhost:5173,http://127.0.0.1:5173")
	v.SetDefault("SESSION_TTL_HOURS", 12)
	v.SetDefault("COOKIE_DOMAIN", "")
	v.SetDefault("ADMIN_LOGIN", "")
	v.SetDefault("ADMIN_PASSWORD", "")
	v.SetDefault("TEMPLATES_DIR", "/templates")
	v.SetDefault("GITHUB_REPO_PRIVATE", true)
	v.SetDefault("DOMAIN", "")
	v.SetDefault("CLOUDFLARE_API_TOKEN", "")
	v.SetDefault("CLOUDFLARE_ACCOUNT_ID", "")
	v.SetDefault("CLOUDFLARE_ZONE_ID", "")
	v.SetDefault("CLOUDFLARE_TUNNEL_ID", "")
	v.SetDefault("CLOUDFLARED_CONFIG", "~/.cloudflared/config.yml")
	v.SetDefault("CLOUDFLARED_TUNNEL_NAME", "")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		// only warn when config file is missing; env vars still apply
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := Config{
		AppEnv:              v.GetString("APP_ENV"),
		Port:                v.GetString("PORT"),
		DatabaseURL:         v.GetString("DATABASE_URL"),
		DBMaxOpenConns:      v.GetInt("DB_MAX_OPEN_CONNS"),
		DBMaxIdleConns:      v.GetInt("DB_MAX_IDLE_CONNS"),
		DBConnMaxLifetime:   time.Duration(v.GetInt("DB_CONN_MAX_LIFETIME_MIN")) * time.Minute,
		AllowedOrigins:      parseCSV(v.GetString("CORS_ALLOWED_ORIGINS")),
		SessionSecret:       v.GetString("SESSION_SECRET"),
		SessionTTL:          time.Duration(v.GetInt("SESSION_TTL_HOURS")) * time.Hour,
		CookieDomain:        strings.TrimSpace(v.GetString("COOKIE_DOMAIN")),
		AdminLogin:          v.GetString("ADMIN_LOGIN"),
		AdminPassword:       v.GetString("ADMIN_PASSWORD"),
		GitHubClientID:      v.GetString("GITHUB_CLIENT_ID"),
		GitHubClientSecret:  v.GetString("GITHUB_CLIENT_SECRET"),
		GitHubCallbackURL:   v.GetString("GITHUB_CALLBACK_URL"),
		GitHubAllowedUsers:  parseCSV(v.GetString("GITHUB_ALLOWED_USERS")),
		GitHubAllowedOrg:    v.GetString("GITHUB_ALLOWED_ORG"),
		GitHubTemplateOwner: v.GetString("GITHUB_TEMPLATE_OWNER"),
		GitHubTemplateRepo:  v.GetString("GITHUB_TEMPLATE_REPO"),
		GitHubRepoOwner:     v.GetString("GITHUB_REPO_OWNER"),
		GitHubRepoPrivate:   v.GetBool("GITHUB_REPO_PRIVATE"),
		TemplatesDir:        v.GetString("TEMPLATES_DIR"),
		Domain:              v.GetString("DOMAIN"),
		CloudflareAPIToken:  v.GetString("CLOUDFLARE_API_TOKEN"),
		CloudflareAccountID: v.GetString("CLOUDFLARE_ACCOUNT_ID"),
		CloudflareZoneID:    v.GetString("CLOUDFLARE_ZONE_ID"),
		CloudflareTunnelID:  v.GetString("CLOUDFLARE_TUNNEL_ID"),
		CloudflaredConfig:   v.GetString("CLOUDFLARED_CONFIG"),
		CloudflaredTunnel:   v.GetString("CLOUDFLARED_TUNNEL_NAME"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.SessionSecret == "" {
		return Config{}, fmt.Errorf("SESSION_SECRET is required")
	}
	if cfg.GitHubClientID == "" {
		return Config{}, fmt.Errorf("GITHUB_CLIENT_ID is required")
	}
	if cfg.GitHubClientSecret == "" {
		return Config{}, fmt.Errorf("GITHUB_CLIENT_SECRET is required")
	}
	if cfg.GitHubCallbackURL == "" {
		return Config{}, fmt.Errorf("GITHUB_CALLBACK_URL is required")
	}

	return cfg, nil
}

func parseCSV(input string) []string {
	parts := strings.Split(input, ",")
	var cleaned []string

	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	return cleaned
}
