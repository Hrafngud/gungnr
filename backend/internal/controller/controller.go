package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/service"
)

type HealthController struct {
	service *service.HealthService
}

func NewHealthController(service *service.HealthService) *HealthController {
	return &HealthController{service: service}
}

func (h *HealthController) Healthz(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *HealthController) Docker(ctx *gin.Context) {
	if h.service == nil {
		ctx.JSON(http.StatusOK, gin.H{"status": "error", "code": errs.CodeInternal, "detail": "health service unavailable"})
		return
	}
	ctx.JSON(http.StatusOK, h.service.Docker(ctx.Request.Context()))
}

func (h *HealthController) Tunnel(ctx *gin.Context) {
	if h.service == nil {
		ctx.JSON(http.StatusOK, gin.H{"status": "error", "code": errs.CodeInternal, "detail": "health service unavailable"})
		return
	}
	health := h.service.Tunnel(ctx.Request.Context())
	status := http.StatusOK
	switch health.Status {
	case "missing":
		status = http.StatusFailedDependency
	case "error":
		status = http.StatusBadGateway
	}
	ctx.JSON(status, health)
}
