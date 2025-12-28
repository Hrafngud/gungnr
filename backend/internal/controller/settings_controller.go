package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/service"
)

type SettingsController struct {
	service *service.SettingsService
}

func NewSettingsController(service *service.SettingsService) *SettingsController {
	return &SettingsController{service: service}
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
