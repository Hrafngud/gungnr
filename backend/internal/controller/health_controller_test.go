package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"go-notes/internal/config"
	"go-notes/internal/errs"
	"go-notes/internal/service"
)

func TestHealthDockerServiceUnavailablePreservesLegacyPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewHealthController(nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/health/docker", nil)

	controller.Docker(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("expected status payload %q, got %#v", "error", payload["status"])
	}
	if payload["detail"] != "health service unavailable" {
		t.Fatalf("expected detail payload, got %#v", payload["detail"])
	}
	if payload["code"] != string(errs.CodeInternal) {
		t.Fatalf("expected code %q, got %#v", errs.CodeInternal, payload["code"])
	}
	if _, ok := payload["message"]; ok {
		t.Fatalf("expected no generic error envelope message field, got %#v", payload["message"])
	}
}

func TestHealthTunnelErrorReturnsTunnelHealthPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewHealthController(service.NewHealthService(nil, nil, config.Config{}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/health/tunnel", nil)

	controller.Tunnel(ctx)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, recorder.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("expected tunnel health status %q, got %#v", "error", payload["status"])
	}
	if payload["detail"] != "settings service unavailable" {
		t.Fatalf("expected tunnel health detail, got %#v", payload["detail"])
	}
	if _, ok := payload["message"]; ok {
		t.Fatalf("expected tunnel health payload instead of generic error envelope, got %#v", payload["message"])
	}
}
