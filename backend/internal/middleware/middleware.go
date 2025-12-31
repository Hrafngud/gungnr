package middleware

import (
	"slices"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func DefaultCORSConfig(allowedOrigins []string) cors.Config {
	cfg := cors.Config{
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	if len(allowedOrigins) == 0 || slices.Contains(allowedOrigins, "*") {
		// Echo the request origin while allowing any host; avoids '*' with credentials.
		cfg.AllowOriginFunc = func(_ string) bool { return true }
	} else {
		cfg.AllowOrigins = allowedOrigins
	}

	return cfg
}

func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return cors.New(DefaultCORSConfig(allowedOrigins))
}
