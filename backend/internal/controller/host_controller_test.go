package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"go-notes/internal/infra/contract"
	"go-notes/internal/service"
)

func TestHostListDockerReturnsDegradedInventoryShellWhenBridgeReadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewHostController(
		service.NewHostService("", nil, &hostControllerBridgeStub{
			listContainersResult: contract.Result{
				Status:   contract.StatusFailed,
				IntentID: "intent-docker-list-fail",
				Error: &contract.Error{
					Code:    "DOCKER-500",
					Message: "docker ps failed",
				},
			},
		}),
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/host/docker", nil)

	controller.ListDocker(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var payload struct {
		Containers  []service.DockerContainer      `json:"containers"`
		Diagnostics []service.DockerReadDiagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Containers) != 0 {
		t.Fatalf("expected empty degraded inventory shell, got %d containers", len(payload.Containers))
	}
	if len(payload.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(payload.Diagnostics))
	}
	if payload.Diagnostics[0].Scope != "containers" {
		t.Fatalf("expected containers scope, got %#v", payload.Diagnostics[0].Scope)
	}
}

func TestHostDockerUsageReturnsDegradedUsageShellWhenBridgeReadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewHostController(
		service.NewHostService("", nil, &hostControllerBridgeStub{
			systemDFResult: contract.Result{
				Status:   contract.StatusFailed,
				IntentID: "intent-docker-df-fail",
				Error: &contract.Error{
					Code:    "DOCKER-500",
					Message: "docker system df failed",
				},
			},
		}),
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/host/docker/usage?project=demo", nil)

	controller.DockerUsage(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var payload struct {
		Summary struct {
			TotalSize     string `json:"totalSize"`
			Project       string `json:"project"`
			ProjectCounts *struct {
				Containers int `json:"containers"`
				Images     int `json:"images"`
				Volumes    int `json:"volumes"`
			} `json:"projectCounts"`
		} `json:"summary"`
		Diagnostics []service.DockerReadDiagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Summary.TotalSize != "0B" {
		t.Fatalf("expected zeroed summary shell, got %#v", payload.Summary.TotalSize)
	}
	if payload.Summary.Project != "demo" {
		t.Fatalf("expected project passthrough, got %#v", payload.Summary.Project)
	}
	if payload.Summary.ProjectCounts == nil {
		t.Fatal("expected projectCounts shell")
	}
	if len(payload.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(payload.Diagnostics))
	}
	if payload.Diagnostics[0].Scope != "usage.summary" {
		t.Fatalf("expected usage.summary scope, got %#v", payload.Diagnostics[0].Scope)
	}
	if payload.Diagnostics[1].Scope != "usage.projectCounts" {
		t.Fatalf("expected usage.projectCounts scope, got %#v", payload.Diagnostics[1].Scope)
	}
}

func TestHostDockerUsagePreservesGlobalTotalsWhenOnlyProjectCountsFail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewHostController(
		service.NewHostService("", nil, &hostControllerBridgeStub{
			systemDFResult: contract.Result{
				Status: contract.StatusSucceeded,
				Data: map[string]any{
					"lines": []string{
						`{"Type":"Images","TotalCount":"8","Active":"2","Size":"3.2GB","Reclaimable":"1.1GB (34%)"}`,
						`{"Type":"Containers","TotalCount":"6","Active":"3","Size":"512MB","Reclaimable":"0B (0%)"}`,
						`{"Type":"Local Volumes","TotalCount":"5","Active":"4","Size":"1.5GB","Reclaimable":"0B (0%)"}`,
						`{"Type":"Build Cache","TotalCount":"2","Active":"0","Size":"120MB","Reclaimable":"120MB (100%)"}`,
					},
				},
			},
			listContainersResult: contract.Result{
				Status:   contract.StatusFailed,
				IntentID: "intent-docker-list-fail",
				Error: &contract.Error{
					Code:    "DOCKER-500",
					Message: "docker ps failed",
				},
			},
		}),
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/host/docker/usage?project=demo", nil)

	controller.DockerUsage(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var payload struct {
		Summary struct {
			TotalSize string `json:"totalSize"`
			Project   string `json:"project"`
			Images    struct {
				Count int `json:"count"`
			} `json:"images"`
			Containers struct {
				Count int `json:"count"`
			} `json:"containers"`
			Volumes struct {
				Count int `json:"count"`
			} `json:"volumes"`
			ProjectCounts *struct {
				Containers int `json:"containers"`
				Images     int `json:"images"`
				Volumes    int `json:"volumes"`
			} `json:"projectCounts"`
		} `json:"summary"`
		Diagnostics []service.DockerReadDiagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Summary.TotalSize == "0B" {
		t.Fatalf("expected preserved global totals, got zeroed summary %#v", payload.Summary.TotalSize)
	}
	if payload.Summary.Project != "demo" {
		t.Fatalf("expected project passthrough, got %#v", payload.Summary.Project)
	}
	if payload.Summary.Images.Count != 8 || payload.Summary.Containers.Count != 6 || payload.Summary.Volumes.Count != 5 {
		t.Fatalf("expected preserved global counts, got images=%d containers=%d volumes=%d", payload.Summary.Images.Count, payload.Summary.Containers.Count, payload.Summary.Volumes.Count)
	}
	if payload.Summary.ProjectCounts == nil {
		t.Fatal("expected degraded projectCounts shell")
	}
	if len(payload.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(payload.Diagnostics))
	}
	if payload.Diagnostics[0].Scope != "usage.projectCounts" {
		t.Fatalf("expected usage.projectCounts scope, got %#v", payload.Diagnostics[0].Scope)
	}
}

type hostControllerBridgeStub struct {
	listContainersResult contract.Result
	systemDFResult       contract.Result
}

func (s *hostControllerBridgeStub) StopContainer(_ context.Context, _, _ string) (contract.Result, error) {
	return contract.Result{}, nil
}

func (s *hostControllerBridgeStub) RestartContainer(_ context.Context, _, _ string) (contract.Result, error) {
	return contract.Result{}, nil
}

func (s *hostControllerBridgeStub) RemoveContainer(_ context.Context, _, _ string, _ bool) (contract.Result, error) {
	return contract.Result{}, nil
}

func (s *hostControllerBridgeStub) DockerListContainers(_ context.Context, _ string, _ bool) (contract.Result, error) {
	return s.listContainersResult, nil
}

func (s *hostControllerBridgeStub) DockerSystemDF(_ context.Context, _ string) (contract.Result, error) {
	return s.systemDFResult, nil
}

func (s *hostControllerBridgeStub) DockerListVolumes(_ context.Context, _ string) (contract.Result, error) {
	return contract.Result{}, nil
}

func (s *hostControllerBridgeStub) DockerContainerLogs(_ context.Context, _ string, _ contract.DockerContainerLogsPayload) (contract.Result, error) {
	return contract.Result{}, nil
}

func (s *hostControllerBridgeStub) DockerRuntimeCheck(_ context.Context, _ string) (contract.Result, error) {
	return contract.Result{}, nil
}

func (s *hostControllerBridgeStub) HostRuntimeStats(_ context.Context, _ string) (contract.Result, error) {
	return contract.Result{}, nil
}

func (s *hostControllerBridgeStub) HostRuntimeStream(_ context.Context, _ string) (contract.Result, error) {
	return contract.Result{}, nil
}

func (s *hostControllerBridgeStub) ComposeUpStack(_ context.Context, _ string, _ contract.ComposeUpStackPayload) (contract.Result, error) {
	return contract.Result{}, nil
}
