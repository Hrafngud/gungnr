package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/middleware"
	"go-notes/internal/service"
)

type SettingsController struct {
	service *service.SettingsService
	audit   *service.AuditService
}

func NewSettingsController(service *service.SettingsService, audit *service.AuditService) *SettingsController {
	return &SettingsController{service: service, audit: audit}
}

func (c *SettingsController) Register(r gin.IRoutes) {
	r.GET("/settings", c.Get)
	r.PUT("/settings", c.Update)
	r.GET("/settings/cloudflared/preview", c.CloudflaredPreview)
}

func (c *SettingsController) Get(ctx *gin.Context) {
	settings, err := c.service.Get(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load settings"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (c *SettingsController) Update(ctx *gin.Context) {
	var req service.SettingsPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	settings, err := c.service.Update(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update settings"})
		return
	}

	c.logAudit(ctx, "settings.update", "settings", map[string]any{
		"baseDomain":            req.BaseDomain,
		"githubTokenSet":        req.GitHubToken != "",
		"cloudflareTokenSet":    req.CloudflareToken != "",
		"cloudflaredConfigPath": req.CloudflaredConfigPath,
	})

	ctx.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (c *SettingsController) CloudflaredPreview(ctx *gin.Context) {
	preview, err := c.service.CloudflaredPreview(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"preview": preview})
}

func (c *SettingsController) logAudit(ctx *gin.Context, action, target string, metadata map[string]any) {
	if c.audit == nil {
		return
	}
	session, _ := middleware.SessionFromContext(ctx)
	_ = c.audit.Log(ctx.Request.Context(), service.AuditEntry{
		UserID:    session.UserID,
		UserLogin: session.Login,
		Action:    action,
		Target:    target,
		Metadata:  metadata,
	})
}
