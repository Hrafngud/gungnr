package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterProjects(r gin.IRoutes, c *controller.ProjectsController) {
	if c == nil {
		return
	}
	r.GET("/projects", c.List)
	r.GET("/projects/local", c.ListLocal)
	r.POST("/projects/template", c.CreateFromTemplate)
	r.POST("/projects/existing", c.DeployExisting)
	r.POST("/projects/forward", c.ForwardLocal)
	r.POST("/projects/quick", c.QuickService)
}
