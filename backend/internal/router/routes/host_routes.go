package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterHost(r gin.IRoutes, c *controller.HostController) {
	if c == nil {
		return
	}
	r.GET("/host/docker", c.ListDocker)
	r.GET("/host/docker/usage", c.DockerUsage)
	r.GET("/host/docker/logs", c.StreamDockerLogs)
	r.POST("/host/docker/stop", c.StopDocker)
	r.POST("/host/docker/restart", c.RestartDocker)
	r.POST("/host/docker/remove", c.RemoveDocker)
}
