package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterNetBird(r gin.IRoutes, c *controller.NetBirdController) {
	if c == nil {
		return
	}
	r.GET("/netbird/status", c.Status)
	r.GET("/netbird/config", c.ModeConfig)
	r.GET("/netbird/graph", c.Graph)
	r.GET("/netbird/acl/graph", c.ACLGraph)
	r.POST("/netbird/mode/plan", c.PlanMode)
	r.POST("/netbird/mode/apply", c.ApplyMode)
	r.PUT("/netbird/config", c.UpdateModeConfig)
	r.POST("/netbird/policies/reapply", c.ReapplyPolicies)
}
