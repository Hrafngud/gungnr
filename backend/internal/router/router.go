package router

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
	"go-notes/internal/middleware"
)

type Dependencies struct {
	Health         *controller.HealthController
	Auth           *controller.AuthController
	AllowedOrigins []string
}

func NewRouter(deps Dependencies) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware(deps.AllowedOrigins))

	if deps.Health != nil {
		deps.Health.Register(r)
	}
	if deps.Auth != nil {
		deps.Auth.Register(r)
	}

	return r
}
