package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	AppEnv            string
	Port              string
	DatabaseURL       string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	AllowedOrigins    []string
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

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		// only warn when config file is missing; env vars still apply
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := Config{
		AppEnv:            v.GetString("APP_ENV"),
		Port:              v.GetString("PORT"),
		DatabaseURL:       v.GetString("DATABASE_URL"),
		DBMaxOpenConns:    v.GetInt("DB_MAX_OPEN_CONNS"),
		DBMaxIdleConns:    v.GetInt("DB_MAX_IDLE_CONNS"),
		DBConnMaxLifetime: time.Duration(v.GetInt("DB_CONN_MAX_LIFETIME_MIN")) * time.Minute,
		AllowedOrigins:    parseCSV(v.GetString("CORS_ALLOWED_ORIGINS")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
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
