package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterAuth(r gin.IRoutes, c *controller.AuthController) {
	if c == nil {
		return
	}
	r.GET("/auth/login", c.Login)
	r.GET("/auth/callback", c.Callback)
	r.GET("/auth/me", c.Me)
	r.POST("/auth/logout", c.Logout)
	r.POST("/test-token", c.TestToken)
}
