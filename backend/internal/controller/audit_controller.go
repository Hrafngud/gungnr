package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
	"go-notes/internal/service"
)

type AuditController struct {
	service *service.AuditService
}

type auditLogResponse struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"userId"`
	UserLogin string    `json:"userLogin"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Metadata  string    `json:"metadata"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewAuditController(service *service.AuditService) *AuditController {
	return &AuditController{service: service}
}

func (c *AuditController) Register(r gin.IRoutes) {
	r.GET("/audit-logs", c.List)
}

func (c *AuditController) List(ctx *gin.Context) {
	limit := parseAuditLimit(ctx.Query("limit"))
	logs, err := c.service.List(ctx.Request.Context(), limit)
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusInternalServerError, err, errs.CodeAuditListFailed, "failed to load audit logs")
		return
	}

	response := make([]auditLogResponse, 0, len(logs))
	for _, log := range logs {
		response = append(response, auditLogResponse{
			ID:        log.ID,
			UserID:    log.UserID,
			UserLogin: log.UserLogin,
			Action:    log.Action,
			Target:    log.Target,
			Metadata:  log.Metadata,
			CreatedAt: log.CreatedAt,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"logs": response})
}

func parseAuditLimit(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0
	}
	if value > 500 {
		return 500
	}
	return value
}
