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

func TestRegisterProjectsIncludesWorkbenchCatalogRoute(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterProjects(router, &controller.ProjectsController{})

	for _, route := range router.Routes() {
		if route.Method == "GET" && route.Path == "/projects/:name/workbench/catalog" {
			return
		}
	}

	t.Fatal("expected GET /projects/:name/workbench/catalog route to be registered")
}

func TestRegisterProjectsIncludesWorkbenchServiceMutationRoutes(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterProjects(router, &controller.ProjectsController{})

	foundAdd := false
	foundRemove := false
	for _, route := range router.Routes() {
		if route.Method == "POST" && route.Path == "/projects/:name/workbench/services" {
			foundAdd = true
		}
		if route.Method == "DELETE" && route.Path == "/projects/:name/workbench/services/:serviceName" {
			foundRemove = true
		}
	}

	if !foundAdd {
		t.Fatal("expected POST /projects/:name/workbench/services route to be registered")
	}
	if !foundRemove {
		t.Fatal("expected DELETE /projects/:name/workbench/services/:serviceName route to be registered")
	}
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
