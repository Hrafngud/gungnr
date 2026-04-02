package controller

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/errs"
	"go-notes/internal/models"
	"go-notes/internal/respond"
	"go-notes/internal/service"
)

type CloudflareController struct {
	service *service.CloudflareService
}

func NewCloudflareController(service *service.CloudflareService) *CloudflareController {
	return &CloudflareController{service: service}
}

func (c *CloudflareController) Preflight(ctx *gin.Context) {
	if c.service == nil {
		respond.Err(ctx, errs.New(errs.CodeCloudflareUnavailable, "cloudflare service unavailable"), errs.CodeCloudflareUnavailable, "cloudflare service unavailable")
		return
	}
	result, err := c.service.Preflight(ctx.Request.Context())
	if err != nil {
		respond.Err(ctx, err, errs.CodeCloudflarePreflight, err.Error())
		return
	}
	respond.OK(ctx, result)
}

func (c *CloudflareController) Zones(ctx *gin.Context) {
	if c.service == nil {
		respond.Err(ctx, errs.New(errs.CodeCloudflareUnavailable, "cloudflare service unavailable"), errs.CodeCloudflareUnavailable, "cloudflare service unavailable")
		return
	}
	zones, err := c.service.Zones(ctx.Request.Context())
	if err != nil {
		respond.Err(ctx, err, errs.CodeCloudflareZones, err.Error())
		return
	}
	response := make([]models.CloudflareZoneResponse, 0, len(zones))
	for _, zone := range zones {
		if zone.ID == "" || zone.Name == "" {
			continue
		}
		response = append(response, models.CloudflareZoneResponse{ID: zone.ID, Name: zone.Name})
	}
	respond.OK(ctx, gin.H{"zones": response})
}
