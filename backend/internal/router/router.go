package router

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
	"go-notes/internal/middleware"
)

type Dependencies struct {
	Health         *controller.HealthController
	Auth           *controller.AuthController
	Projects       *controller.ProjectsController
	Jobs           *controller.JobsController
	AllowedOrigins []string
	AuthMiddleware gin.HandlerFunc
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

	api := r.Group("/api/v1")
	if deps.AuthMiddleware != nil {
		api.Use(deps.AuthMiddleware)
	}
	if deps.Projects != nil {
		deps.Projects.Register(api)
	}
	if deps.Jobs != nil {
		deps.Jobs.Register(api)
	}

	return r
}
