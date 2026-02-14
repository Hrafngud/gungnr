package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterAudit(r gin.IRoutes, c *controller.AuditController) {
	if c == nil {
		return
	}
	r.GET("/audit-logs", c.List)
}
