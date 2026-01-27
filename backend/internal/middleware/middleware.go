package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"go-notes/internal/config"
)

func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	corsConfig := config.DefaultCORSConfig(allowedOrigins)
	return cors.New(corsConfig.ToGinCORSConfig())
}
