package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterJobs(r gin.IRoutes, c *controller.JobsController) {
	if c == nil {
		return
	}
	r.GET("/jobs", c.List)
	r.GET("/jobs/:id", c.Get)
	r.GET("/jobs/:id/stream", c.Stream)
	r.POST("/jobs/:id/stop", c.Stop)
	r.POST("/jobs/:id/retry", c.Retry)
}
