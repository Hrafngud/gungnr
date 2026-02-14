package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterHealth(r gin.IRoutes, c *controller.HealthController) {
	if c == nil {
		return
	}
	r.GET("/healthz", c.Healthz)
	r.GET("/health/docker", c.Docker)
	r.GET("/health/tunnel", c.Tunnel)
}
