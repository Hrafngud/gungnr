package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCORSMiddlewareAllowsConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CORSMiddleware([]string{"http://localhost:4173"}))
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "http://localhost:4173")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "http://localhost:4173", resp.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddlewareAllowsWildcard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CORSMiddleware([]string{"*"}))
	r.Any("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "http://api.local/ping", nil)
	req.Header.Set("Origin", "http://frontend.local")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	require.Contains(t, []int{http.StatusNoContent, http.StatusOK}, resp.Code)
	require.Equal(t, "*", resp.Header().Get("Access-Control-Allow-Origin"))
}
