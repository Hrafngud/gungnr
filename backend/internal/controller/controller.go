package controller

import "github.com/gin-gonic/gin"

type HealthController struct{}

func NewHealthController() *HealthController {
	return &HealthController{}
}

func (h *HealthController) Register(r *gin.Engine) {
	r.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"status": "ok"})
	})
}
