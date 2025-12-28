package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-notes/internal/service"
)

type HostController struct {
	service *service.HostService
}

func NewHostController(service *service.HostService) *HostController {
	return &HostController{service: service}
}

func (c *HostController) Register(r gin.IRoutes) {
	r.GET("/host/docker", c.ListDocker)
}

func (c *HostController) ListDocker(ctx *gin.Context) {
	containers, err := c.service.ListContainers(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"containers": containers})
}
