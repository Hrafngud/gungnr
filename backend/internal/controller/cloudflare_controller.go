package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/service"
)

type CloudflareController struct {
	service *service.CloudflareService
}

func NewCloudflareController(service *service.CloudflareService) *CloudflareController {
	return &CloudflareController{service: service}
}

func (c *CloudflareController) Register(r gin.IRoutes) {
	r.GET("/cloudflare/preflight", c.Preflight)
}

func (c *CloudflareController) Preflight(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "cloudflare service unavailable"})
		return
	}
	result, err := c.service.Preflight(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, result)
}
