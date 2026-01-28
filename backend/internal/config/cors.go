package config

import (
	"slices"
	"time"

	"github.com/gin-contrib/cors"
)

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

func DefaultCORSConfig(allowedOrigins []string) CORSConfig {
	return CORSConfig{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
}

func (c CORSConfig) ToGinCORSConfig() cors.Config {
	cfg := cors.Config{
		AllowHeaders:     c.AllowedHeaders,
		AllowMethods:     c.AllowedMethods,
		AllowCredentials: c.AllowCredentials,
		MaxAge:           c.MaxAge,
	}

	allowAll := len(c.AllowedOrigins) == 0 || slices.Contains(c.AllowedOrigins, "*")
	if allowAll {
		// Browsers reject `Access-Control-Allow-Origin: *` when credentials are enabled.
		// Use an origin func to echo the request origin instead of a wildcard header.
		if c.AllowCredentials {
			cfg.AllowOriginFunc = func(string) bool { return true }
			return cfg
		}
		cfg.AllowAllOrigins = true
		return cfg
	}

	cfg.AllowOrigins = c.AllowedOrigins
	return cfg
}
