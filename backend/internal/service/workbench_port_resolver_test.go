package service

import (
	"context"
	"reflect"
	"testing"

	"go-notes/internal/errs"
)

func TestResolveWorkbenchSnapshotPortsDeterministicPreferredModuleAndFallback(t *testing.T) {
	t.Parallel()

	snapshot := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    3,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "cache", Image: "redis:7"},
			{ServiceName: "worker", Image: "busybox:latest"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:   "api",
				ContainerPort: 80,
				HostPort:      intPtr(8080),
				Protocol:      "tcp",
			},
			{
				ServiceName:   "cache",
				ContainerPort: 6379,
				Protocol:      "tcp",
			},
			{
				ServiceName:   "worker",
				ContainerPort: 8080,
				Protocol:      "tcp",
			},
		},
		Modules: []WorkbenchStackModule{
			{ModuleType: "redis", ServiceName: "cache"},
		},
	}

	firstResolved, firstSummary, firstErr := resolveWorkbenchSnapshotPorts(snapshot)
	if firstErr != nil {
		t.Fatalf("first resolve: %v", firstErr)
	}
	secondResolved, secondSummary, secondErr := resolveWorkbenchSnapshotPorts(snapshot)
	if secondErr != nil {
		t.Fatalf("second resolve: %v", secondErr)
	}

	if !reflect.DeepEqual(firstResolved.Ports, secondResolved.Ports) {
		t.Fatalf("expected deterministic resolved ports\nfirst=%#v\nsecond=%#v", firstResolved.Ports, secondResolved.Ports)
	}
	if !reflect.DeepEqual(firstSummary.Outcomes, secondSummary.Outcomes) {
		t.Fatalf("expected deterministic outcomes\nfirst=%#v\nsecond=%#v", firstSummary.Outcomes, secondSummary.Outcomes)
	}

	if !firstSummary.Changed {
		t.Fatal("expected summary changed=true on first resolve")
	}
	if firstSummary.Assigned != 3 || firstSummary.Conflict != 0 || firstSummary.Unavailable != 0 {
		t.Fatalf(
			"unexpected summary counts assigned=%d conflict=%d unavailable=%d",
			firstSummary.Assigned,
			firstSummary.Conflict,
			firstSummary.Unavailable,
		)
	}

	apiPort := findWorkbenchPort(firstResolved.Ports, "api")
	if apiPort == nil || apiPort.HostPort == nil || *apiPort.HostPort != 8080 {
		t.Fatalf("expected api host port 8080, got %#v", apiPort)
	}
	cachePort := findWorkbenchPort(firstResolved.Ports, "cache")
	if cachePort == nil || cachePort.HostPort == nil || *cachePort.HostPort != 6379 {
		t.Fatalf("expected cache host port 6379 from module default, got %#v", cachePort)
	}
	workerPort := findWorkbenchPort(firstResolved.Ports, "worker")
	if workerPort == nil || workerPort.HostPort == nil || *workerPort.HostPort != 8081 {
		t.Fatalf("expected worker host port fallback to 8081, got %#v", workerPort)
	}

	apiOutcome := findWorkbenchOutcome(firstSummary.Outcomes, "api")
	if apiOutcome == nil || apiOutcome.Source != workbenchPortSourceComposeHostPort || apiOutcome.Status != workbenchPortAllocationAssigned {
		t.Fatalf("unexpected api outcome: %#v", apiOutcome)
	}
	cacheOutcome := findWorkbenchOutcome(firstSummary.Outcomes, "cache")
	if cacheOutcome == nil || cacheOutcome.Source != workbenchPortSourceModuleDefault || cacheOutcome.Status != workbenchPortAllocationAssigned {
		t.Fatalf("unexpected cache outcome: %#v", cacheOutcome)
	}
	workerOutcome := findWorkbenchOutcome(firstSummary.Outcomes, "worker")
	if workerOutcome == nil || workerOutcome.Source != workbenchPortSourceContainerPort || workerOutcome.Status != workbenchPortAllocationAssigned {
		t.Fatalf("unexpected worker outcome: %#v", workerOutcome)
	}
	if workerOutcome.Attempts != 2 {
		t.Fatalf("expected worker fallback attempts=2, got %d", workerOutcome.Attempts)
	}
}

func TestResolveWorkbenchSnapshotPortsUnsupportedRequestedPortValidation(t *testing.T) {
	t.Parallel()

	snapshot := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:   "api",
				ContainerPort: 80,
				HostPortRaw:   "${API_PORT}",
				Protocol:      "tcp",
			},
		},
	}

	_, summary, err := resolveWorkbenchSnapshotPorts(snapshot)
	if err == nil {
		t.Fatal("expected validation error for unsupported requested host port")
	}
	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}

	if summary.Assigned != 0 || summary.Conflict != 0 || summary.Unavailable != 1 {
		t.Fatalf(
			"unexpected summary counts assigned=%d conflict=%d unavailable=%d",
			summary.Assigned,
			summary.Conflict,
			summary.Unavailable,
		)
	}

	details, ok := typed.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected details map, got %T", typed.Details)
	}
	issuesAny, ok := details["issues"]
	if !ok {
		t.Fatalf("expected issues in details: %#v", details)
	}
	issues, ok := issuesAny.([]WorkbenchPortResolutionIssue)
	if !ok {
		t.Fatalf("expected []WorkbenchPortResolutionIssue, got %T", issuesAny)
	}
	if len(issues) != 1 || issues[0].Code != "WB-RESOLVE-PORT-REQUESTED-UNSUPPORTED" {
		t.Fatalf("expected unsupported-port issue, got %#v", issues)
	}
}

func TestResolveWorkbenchSnapshotPortsManualConflictValidation(t *testing.T) {
	t.Parallel()

	snapshot := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "web", Image: "nginx:stable"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:        "api",
				ContainerPort:      80,
				HostPort:           intPtr(9000),
				Protocol:           "tcp",
				AssignmentStrategy: workbenchPortStrategyManual,
			},
			{
				ServiceName:        "web",
				ContainerPort:      8080,
				HostPort:           intPtr(9000),
				Protocol:           "tcp",
				AssignmentStrategy: workbenchPortStrategyManual,
			},
		},
	}

	_, summary, err := resolveWorkbenchSnapshotPorts(snapshot)
	if err == nil {
		t.Fatal("expected manual conflict validation error")
	}
	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}

	if summary.Assigned != 1 || summary.Conflict != 1 || summary.Unavailable != 0 {
		t.Fatalf(
			"unexpected summary counts assigned=%d conflict=%d unavailable=%d",
			summary.Assigned,
			summary.Conflict,
			summary.Unavailable,
		)
	}

	details, ok := typed.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected details map, got %T", typed.Details)
	}
	issuesAny, ok := details["issues"]
	if !ok {
		t.Fatalf("expected issues in details: %#v", details)
	}
	issues, ok := issuesAny.([]WorkbenchPortResolutionIssue)
	if !ok {
		t.Fatalf("expected []WorkbenchPortResolutionIssue, got %T", issuesAny)
	}
	if len(issues) != 1 || issues[0].Code != "WB-RESOLVE-MANUAL-CONFLICT" {
		t.Fatalf("expected manual conflict issue, got %#v", issues)
	}
}

func TestResolveWorkbenchSnapshotPortsUnavailableExhausted(t *testing.T) {
	t.Parallel()

	snapshot := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "web", Image: "nginx:stable"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:   "api",
				ContainerPort: 80,
				HostPort:      intPtr(65535),
				Protocol:      "tcp",
			},
			{
				ServiceName:   "web",
				ContainerPort: 65535,
				Protocol:      "tcp",
			},
		},
	}

	_, summary, err := resolveWorkbenchSnapshotPorts(snapshot)
	if err == nil {
		t.Fatal("expected exhaustion validation error")
	}
	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}

	if summary.Assigned != 1 || summary.Conflict != 0 || summary.Unavailable != 1 {
		t.Fatalf(
			"unexpected summary counts assigned=%d conflict=%d unavailable=%d",
			summary.Assigned,
			summary.Conflict,
			summary.Unavailable,
		)
	}

	webOutcome := findWorkbenchOutcome(summary.Outcomes, "web")
	if webOutcome == nil {
		t.Fatalf("expected web outcome in %#v", summary.Outcomes)
	}
	if webOutcome.Status != workbenchPortAllocationUnavailable || webOutcome.Source != workbenchPortSourceContainerPort {
		t.Fatalf("unexpected web exhaustion outcome: %#v", webOutcome)
	}
}

func TestWorkbenchResolveStoredSnapshotPortsPersistsChangesAndIsIdempotent(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    4,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "web", Image: "nginx:stable"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:   "api",
				ContainerPort: 80,
				HostPort:      intPtr(8080),
				Protocol:      "tcp",
			},
			{
				ServiceName:   "web",
				ContainerPort: 8080,
				Protocol:      "tcp",
			},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save initial snapshot: %v", err)
	}

	firstResolved, firstSummary, err := svc.ResolveStoredSnapshotPorts(context.Background(), "demo")
	if err != nil {
		t.Fatalf("first ResolveStoredSnapshotPorts: %v", err)
	}
	if !firstSummary.Changed {
		t.Fatal("expected first resolve to persist changes")
	}
	if firstResolved.Revision != 5 {
		t.Fatalf("expected revision=5 after persisted mutation, got %d", firstResolved.Revision)
	}
	webPort := findWorkbenchPort(firstResolved.Ports, "web")
	if webPort == nil || webPort.HostPort == nil || *webPort.HostPort != 8081 {
		t.Fatalf("expected persisted fallback host port 8081 for web, got %#v", webPort)
	}

	secondResolved, secondSummary, err := svc.ResolveStoredSnapshotPorts(context.Background(), "demo")
	if err != nil {
		t.Fatalf("second ResolveStoredSnapshotPorts: %v", err)
	}
	if secondSummary.Changed {
		t.Fatal("expected second resolve to be idempotent (changed=false)")
	}
	if secondResolved.Revision != firstResolved.Revision {
		t.Fatalf("expected stable revision %d, got %d", firstResolved.Revision, secondResolved.Revision)
	}
}

func TestResolveWorkbenchSnapshotPortsSynthesizesManagedServicePorts(t *testing.T) {
	t.Parallel()

	snapshot := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    2,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:   "api",
				ContainerPort: 6379,
				HostPort:      intPtr(6379),
				Protocol:      "tcp",
			},
		},
		ManagedServices: []WorkbenchManagedService{
			{EntryKey: "redis", ServiceName: "redis"},
		},
	}

	resolved, summary, err := resolveWorkbenchSnapshotPorts(snapshot)
	if err != nil {
		t.Fatalf("resolveWorkbenchSnapshotPorts: %v", err)
	}
	if !summary.Changed {
		t.Fatal("expected managed service port synthesis to change snapshot ports")
	}

	redisPort := findWorkbenchPort(resolved.Ports, "redis")
	if redisPort == nil {
		t.Fatalf("expected synthesized redis port in %#v", resolved.Ports)
	}
	if redisPort.HostPort == nil || *redisPort.HostPort != 6380 {
		t.Fatalf("expected redis host port fallback to 6380, got %#v", redisPort)
	}

	redisOutcome := findWorkbenchOutcome(summary.Outcomes, "redis")
	if redisOutcome == nil {
		t.Fatalf("expected redis outcome in %#v", summary.Outcomes)
	}
	if redisOutcome.AssignedHostPort == nil || *redisOutcome.AssignedHostPort != 6380 {
		t.Fatalf("expected redis assigned host port 6380, got %#v", redisOutcome)
	}
	if redisOutcome.Source != workbenchPortSourceContainerPort {
		t.Fatalf("expected redis source %q, got %#v", workbenchPortSourceContainerPort, redisOutcome)
	}
}

func findWorkbenchPort(ports []WorkbenchComposePort, serviceName string) *WorkbenchComposePort {
	for idx := range ports {
		if ports[idx].ServiceName == serviceName {
			return &ports[idx]
		}
	}
	return nil
}

func findWorkbenchOutcome(outcomes []WorkbenchPortResolveOutcome, serviceName string) *WorkbenchPortResolveOutcome {
	for idx := range outcomes {
		if outcomes[idx].ServiceName == serviceName {
			return &outcomes[idx]
		}
	}
	return nil
}
