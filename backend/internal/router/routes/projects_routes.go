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
	r.GET("/projects/:name", c.Detail)
	r.GET("/projects/:name/jobs", c.ListJobs)
	r.POST("/projects/:name/workbench/import", c.WorkbenchImport)
	r.POST("/projects/:name/workbench/compose/preview", c.WorkbenchComposePreview)
	r.POST("/projects/:name/workbench/compose/apply", c.WorkbenchComposeApply)
	r.POST("/projects/:name/workbench/compose/restore", c.WorkbenchComposeRestore)
	r.GET("/projects/:name/archive/plan", c.ArchivePlan)
	r.POST("/projects/:name/archive", c.Archive)
	r.POST("/projects/:name/stack/restart", c.RestartStack)
	r.POST("/projects/:name/containers/stop", c.StopContainer)
	r.POST("/projects/:name/containers/restart", c.RestartContainer)
	r.POST("/projects/:name/containers/remove", c.RemoveContainer)
	r.GET("/projects/:name/logs", c.StreamLogs)
	r.GET("/projects/:name/env", c.ReadEnv)
	r.PUT("/projects/:name/env", c.WriteEnv)
	r.POST("/projects/template", c.CreateFromTemplate)
	r.POST("/projects/existing", c.DeployExisting)
	r.POST("/projects/forward", c.ForwardLocal)
	r.POST("/projects/quick", c.QuickService)
}
