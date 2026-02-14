package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterSettings(r gin.IRoutes, c *controller.SettingsController) {
	if c == nil {
		return
	}
	r.GET("/settings", c.Get)
	r.PUT("/settings", c.Update)
	r.GET("/settings/cloudflared/preview", c.CloudflaredPreview)
	r.POST("/settings/cloudflare/sync", c.SyncCloudflareFromEnv)
}
