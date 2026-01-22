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
	r.GET("/cloudflare/zones", c.Zones)
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

type CloudflareZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *CloudflareController) Zones(ctx *gin.Context) {
	if c.service == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "cloudflare service unavailable"})
		return
	}
	zones, err := c.service.Zones(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	response := make([]CloudflareZone, 0, len(zones))
	for _, zone := range zones {
		if zone.ID == "" || zone.Name == "" {
			continue
		}
		response = append(response, CloudflareZone{ID: zone.ID, Name: zone.Name})
	}
	ctx.JSON(http.StatusOK, gin.H{"zones": response})
}
