package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterCloudflare(r gin.IRoutes, c *controller.CloudflareController) {
	if c == nil {
		return
	}
	r.GET("/cloudflare/preflight", c.Preflight)
	r.GET("/cloudflare/zones", c.Zones)
}
