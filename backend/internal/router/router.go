package router

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
	"go-notes/internal/middleware"
	"go-notes/internal/router/routes"
)

type Dependencies struct {
	Health          *controller.HealthController
	Auth            *controller.AuthController
	Projects        *controller.ProjectsController
	Jobs            *controller.JobsController
	Settings        *controller.SettingsController
	Host            *controller.HostController
	Audit           *controller.AuditController
	Users           *controller.UsersController
	GitHub          *controller.GitHubController
	Cloudflare      *controller.CloudflareController
	AllowedOrigins  []string
	AuthMiddleware  gin.HandlerFunc
	UsersMiddleware gin.HandlerFunc
}

func NewRouter(deps Dependencies) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware(deps.AllowedOrigins))

	api := r.Group("/api/v1")

	authed := api
	if deps.AuthMiddleware != nil {
		authed = api.Group("")
		authed.Use(deps.AuthMiddleware)
	}

	adminGroup := authed
	if deps.UsersMiddleware != nil {
		adminGroup = authed.Group("")
		adminGroup.Use(deps.UsersMiddleware)
	}

	routes.Register(r, authed, adminGroup, routes.Dependencies{
		Health:     deps.Health,
		Auth:       deps.Auth,
		Projects:   deps.Projects,
		Jobs:       deps.Jobs,
		Settings:   deps.Settings,
		Host:       deps.Host,
		Audit:      deps.Audit,
		Users:      deps.Users,
		GitHub:     deps.GitHub,
		Cloudflare: deps.Cloudflare,
	})

	return r
}
