package routes

import (
	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func RegisterGitHub(r gin.IRoutes, c *controller.GitHubController) {
	if c == nil {
		return
	}
	r.GET("/github/catalog", c.Catalog)
}
