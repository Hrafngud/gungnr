package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/apierror"
	"go-notes/internal/errs"
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
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeCloudflareUnavailable, "cloudflare service unavailable", nil)
		return
	}
	result, err := c.service.Preflight(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusBadGateway, err, errs.CodeCloudflarePreflight, err.Error())
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
		apierror.Respond(ctx, http.StatusInternalServerError, errs.CodeCloudflareUnavailable, "cloudflare service unavailable", nil)
		return
	}
	zones, err := c.service.Zones(ctx.Request.Context())
	if err != nil {
		apierror.RespondWithError(ctx, http.StatusBadGateway, err, errs.CodeCloudflareZones, err.Error())
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
