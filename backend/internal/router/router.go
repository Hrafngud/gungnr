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
	HostJobs       *controller.HostJobsController
	Settings       *controller.SettingsController
	Onboarding     *controller.OnboardingController
	Host           *controller.HostController
	Audit          *controller.AuditController
	GitHub         *controller.GitHubController
	Cloudflare     *controller.CloudflareController
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
	if deps.HostJobs != nil {
		deps.HostJobs.RegisterPublic(api)
	}

	authed := api
	if deps.AuthMiddleware != nil {
		authed = api.Group("")
		authed.Use(deps.AuthMiddleware)
	}
	if deps.Projects != nil {
		deps.Projects.Register(authed)
	}
	if deps.Jobs != nil {
		deps.Jobs.Register(authed)
	}
	if deps.HostJobs != nil {
		deps.HostJobs.RegisterAuthed(authed)
	}
	if deps.Settings != nil {
		deps.Settings.Register(authed)
	}
	if deps.Onboarding != nil {
		deps.Onboarding.Register(authed)
	}
	if deps.Host != nil {
		deps.Host.Register(authed)
	}
	if deps.Audit != nil {
		deps.Audit.Register(authed)
	}
	if deps.GitHub != nil {
		deps.GitHub.Register(authed)
	}
	if deps.Cloudflare != nil {
		deps.Cloudflare.Register(authed)
	}

	return r
}
