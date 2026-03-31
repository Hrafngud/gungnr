package controller

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/respond"
	"go-notes/internal/service"
	"go-notes/internal/utils/httpx"
)

type AuditController struct {
	service *service.AuditService
}

func NewAuditController(service *service.AuditService) *AuditController {
	return &AuditController{service: service}
}

func (c *AuditController) List(ctx *gin.Context) {
	limit := httpx.ParseIntQuery(ctx, "limit", 0)
	if limit <= 0 {
		limit = 0
	}
	if limit > 500 {
		limit = 500
	}

	logs, err := c.service.List(ctx.Request.Context(), limit)
	if err != nil {
		respond.Err(ctx, err, errs.CodeAuditListFailed, "failed to load audit logs")
		return
	}

	respond.OK(ctx, gin.H{"logs": models.NewAuditLogResponses(logs)})
}
