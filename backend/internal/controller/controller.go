package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/respond"
	"go-notes/internal/service"
)

type HealthController struct {
	service *service.HealthService
}

func NewHealthController(service *service.HealthService) *HealthController {
	return &HealthController{service: service}
}

func (h *HealthController) Healthz(ctx *gin.Context) {
	respond.OK(ctx, gin.H{"status": "ok"})
}

func (h *HealthController) Docker(ctx *gin.Context) {
	if h.service == nil {
		respond.ErrManual(ctx, http.StatusOK, errs.CodeInternal, "health service unavailable")
		return
	}
	respond.OK(ctx, h.service.Docker(ctx.Request.Context()))
}

func (h *HealthController) Tunnel(ctx *gin.Context) {
	if h.service == nil {
		respond.ErrManual(ctx, http.StatusOK, errs.CodeInternal, "health service unavailable")
		return
	}
	health := h.service.Tunnel(ctx.Request.Context())
	switch health.Status {
	case "missing":
		respond.ErrManual(ctx, http.StatusFailedDependency, errs.CodeInternal, "tunnel missing")
		return
	case "error":
		respond.ErrManual(ctx, http.StatusBadGateway, errs.CodeInternal, "tunnel error")
		return
	}
	respond.OK(ctx, health)
}
