package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

// Dependencies bundles controllers used by the route composition layer.
type Dependencies struct {
	Health     *controller.HealthController
	Auth       *controller.AuthController
	Projects   *controller.ProjectsController
	Jobs       *controller.JobsController
	Settings   *controller.SettingsController
	Host       *controller.HostController
	NetBird    *controller.NetBirdController
	Audit      *controller.AuditController
	Users      *controller.UsersController
	GitHub     *controller.GitHubController
	Cloudflare *controller.CloudflareController
}

// Register wires all public and authenticated route modules.
func Register(root gin.IRoutes, authed gin.IRoutes, admin gin.IRoutes, deps Dependencies) {
	RegisterHealth(root, deps.Health)
	RegisterAuth(root, deps.Auth)

	RegisterProjects(authed, deps.Projects)
	RegisterJobs(authed, deps.Jobs)
	RegisterSettings(authed, deps.Settings)
	RegisterHost(authed, deps.Host)
	RegisterNetBird(authed, deps.NetBird)
	RegisterAudit(authed, deps.Audit)
	RegisterUsers(authed, deps.Users)
	RegisterUsersAdmin(admin, deps.Users)
	RegisterGitHub(authed, deps.GitHub)
	RegisterCloudflare(authed, deps.Cloudflare)
}
