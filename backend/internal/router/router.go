package router

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
	"go-notes/internal/middleware"
)

type Dependencies struct {
	Health         *controller.HealthController
	Notes          *controller.NoteController
	AllowedOrigins []string
}

func NewRouter(deps Dependencies) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware(deps.AllowedOrigins))

	if deps.Health != nil {
		deps.Health.Register(r)
	}

	api := r.Group("/api/v1")
	if deps.Notes != nil {
		deps.Notes.Register(api)
	}

	return r
}
