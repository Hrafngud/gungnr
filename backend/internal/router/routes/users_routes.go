package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterUsers(r gin.IRoutes, c *controller.UsersController) {
	if c == nil {
		return
	}
	r.GET("/users", c.List)
}

func RegisterUsersAdmin(r gin.IRoutes, c *controller.UsersController) {
	if c == nil {
		return
	}
	r.POST("/users", c.Create)
	r.PATCH("/users/:id/role", c.UpdateRole)
	r.DELETE("/users/:id", c.Delete)
}
