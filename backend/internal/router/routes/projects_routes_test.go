package routes

import (
	"testing"

	"github.com/gin-gonic/gin"

	"go-notes/internal/controller"
)

func TestRegisterProjectsIncludesWorkbenchSnapshotRoute(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterProjects(router, &controller.ProjectsController{})

	for _, route := range router.Routes() {
		if route.Method == "GET" && route.Path == "/projects/:name/workbench" {
			return
		}
	}

	t.Fatal("expected GET /projects/:name/workbench route to be registered")
}

func TestRegisterProjectsIncludesWorkbenchModuleMutationRoute(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterProjects(router, &controller.ProjectsController{})

	for _, route := range router.Routes() {
		if route.Method == "POST" && route.Path == "/projects/:name/workbench/modules" {
			return
		}
	}

	t.Fatal("expected POST /projects/:name/workbench/modules route to be registered")
}
