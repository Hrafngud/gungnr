package service

import (
	"context"
	"reflect"
	"testing"

	"go-notes/internal/errs"
)

func TestWorkbenchMutateStoredSnapshotPortManualSetDeterministic(t *testing.T) {
	t.Parallel()

	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    2,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "web", Image: "nginx:stable"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:        "api",
				ContainerPort:      80,
				HostPort:           intPtr(8080),
				Protocol:           "tcp",
				AssignmentStrategy: workbenchPortStrategyAuto,
				AllocationStatus:   workbenchPortAllocationAssigned,
			},
			{
				ServiceName:   "web",
				ContainerPort: 8080,
				Protocol:      "tcp",
			},
		},
	}
	request := WorkbenchPortMutationRequest{
		Selector: WorkbenchPortSelector{
			ServiceName:   "web",
			ContainerPort: 8080,
			Protocol:      "tcp",
		},
		Action:         workbenchPortMutationActionSetManual,
		ManualHostPort: intPtr(9090),
	}

	firstService := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	if err := firstService.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save first snapshot: %v", err)
	}
	secondService := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	if err := secondService.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save second snapshot: %v", err)
	}

	firstSnapshot, firstSummary, firstErr := firstService.MutateStoredSnapshotPort(context.Background(), "demo", request)
	if firstErr != nil {
		t.Fatalf("first mutation: %v", firstErr)
	}
	secondSnapshot, secondSummary, secondErr := secondService.MutateStoredSnapshotPort(context.Background(), "demo", request)
	if secondErr != nil {
		t.Fatalf("second mutation: %v", secondErr)
	}

	if !reflect.DeepEqual(firstSnapshot.Ports, secondSnapshot.Ports) {
		t.Fatalf("expected deterministic mutated ports\nfirst=%#v\nsecond=%#v", firstSnapshot.Ports, secondSnapshot.Ports)
	}
	if !reflect.DeepEqual(firstSummary, secondSummary) {
		t.Fatalf("expected deterministic mutation summary\nfirst=%#v\nsecond=%#v", firstSummary, secondSummary)
	}
	if !firstSummary.Changed {
		t.Fatal("expected changed=true for manual set mutation")
	}
	if firstSummary.Status != workbenchPortAllocationAssigned {
		t.Fatalf("expected assigned status, got %q", firstSummary.Status)
	}
	if firstSummary.CurrentStrategy != workbenchPortStrategyManual {
		t.Fatalf("expected current strategy manual, got %q", firstSummary.CurrentStrategy)
	}
	if firstSummary.AssignedHostPort == nil || *firstSummary.AssignedHostPort != 9090 {
		t.Fatalf("expected assigned host port 9090, got %#v", firstSummary.AssignedHostPort)
	}
	if firstSnapshot.Revision != 3 {
		t.Fatalf("expected revision=3 after mutation persistence, got %d", firstSnapshot.Revision)
	}

	mutatedWeb := findWorkbenchPort(firstSnapshot.Ports, "web")
	if mutatedWeb == nil || mutatedWeb.HostPort == nil || *mutatedWeb.HostPort != 9090 {
		t.Fatalf("expected web host port 9090, got %#v", mutatedWeb)
	}
	if mutatedWeb.AssignmentStrategy != workbenchPortStrategyManual {
		t.Fatalf("expected manual assignment strategy, got %q", mutatedWeb.AssignmentStrategy)
	}
}

func TestWorkbenchMutateStoredSnapshotPortManualConflictValidation(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    5,
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
				AllocationStatus:   workbenchPortAllocationAssigned,
			},
			{
				ServiceName:   "web",
				ContainerPort: 8080,
				Protocol:      "tcp",
			},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	request := WorkbenchPortMutationRequest{
		Selector: WorkbenchPortSelector{
			ServiceName:   "web",
			ContainerPort: 8080,
			Protocol:      "tcp",
		},
		Action:         workbenchPortMutationActionSetManual,
		ManualHostPort: intPtr(9000),
	}
	resolved, summary, err := svc.MutateStoredSnapshotPort(context.Background(), "demo", request)
	if err == nil {
		t.Fatal("expected conflict validation error")
	}
	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}
	if summary.Changed {
		t.Fatal("expected changed=false on conflict validation error")
	}
	if summary.Status != workbenchPortAllocationConflict {
		t.Fatalf("expected conflict status, got %q", summary.Status)
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
	if len(issues) != 1 || issues[0].Code != "WB-MUTATE-MANUAL-CONFLICT" {
		t.Fatalf("expected WB-MUTATE-MANUAL-CONFLICT, got %#v", issues)
	}

	webPort := findWorkbenchPort(resolved.Ports, "web")
	if webPort == nil || webPort.HostPort != nil {
		t.Fatalf("expected web host port to remain unchanged, got %#v", webPort)
	}

	stored, exists, loadErr := svc.loadStoredWorkbenchSnapshot(context.Background(), "demo")
	if loadErr != nil {
		t.Fatalf("load stored snapshot: %v", loadErr)
	}
	if !exists {
		t.Fatal("expected stored snapshot to exist")
	}
	if stored.Revision != initial.Revision {
		t.Fatalf("expected unchanged stored revision %d, got %d", initial.Revision, stored.Revision)
	}
}

func TestWorkbenchMutateStoredSnapshotPortClearManualResetToAuto(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    10,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "web", Image: "nginx:stable"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:   "api",
				ContainerPort: 80,
				HostPort:      intPtr(9000),
				Protocol:      "tcp",
			},
			{
				ServiceName:        "web",
				ContainerPort:      8080,
				HostPort:           intPtr(9000),
				Protocol:           "tcp",
				AssignmentStrategy: workbenchPortStrategyManual,
				AllocationStatus:   workbenchPortAllocationAssigned,
			},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	request := WorkbenchPortMutationRequest{
		Selector: WorkbenchPortSelector{
			ServiceName:   "web",
			ContainerPort: 8080,
			Protocol:      "tcp",
		},
		Action: workbenchPortMutationActionClearManual,
	}

	resolved, summary, err := svc.MutateStoredSnapshotPort(context.Background(), "demo", request)
	if err != nil {
		t.Fatalf("clear manual mutation: %v", err)
	}
	if !summary.Changed {
		t.Fatal("expected changed=true when clearing manual assignment")
	}
	if summary.CurrentStrategy != workbenchPortStrategyAuto {
		t.Fatalf("expected auto strategy after clear, got %q", summary.CurrentStrategy)
	}
	if summary.AssignedHostPort == nil || *summary.AssignedHostPort != 9001 {
		t.Fatalf("expected reassigned host port 9001, got %#v", summary.AssignedHostPort)
	}
	if summary.Attempts != 2 {
		t.Fatalf("expected attempts=2 for fallback, got %d", summary.Attempts)
	}

	webPort := findWorkbenchPort(resolved.Ports, "web")
	if webPort == nil || webPort.HostPort == nil || *webPort.HostPort != 9001 {
		t.Fatalf("expected resolved web host port 9001, got %#v", webPort)
	}
	if webPort.AssignmentStrategy != workbenchPortStrategyAuto {
		t.Fatalf("expected auto strategy, got %q", webPort.AssignmentStrategy)
	}
	if resolved.Revision != 11 {
		t.Fatalf("expected revision=11 after mutation persistence, got %d", resolved.Revision)
	}
}

func TestWorkbenchSuggestStoredSnapshotHostPortsDeterministic(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    4,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "cache", Image: "redis:7"},
		},
		Ports: []WorkbenchComposePort{
			{
				ServiceName:   "api",
				ContainerPort: 6379,
				HostPort:      intPtr(6379),
				Protocol:      "tcp",
			},
			{
				ServiceName:   "cache",
				ContainerPort: 6380,
				Protocol:      "tcp",
			},
		},
		Modules: []WorkbenchStackModule{
			{ModuleType: "redis", ServiceName: "cache"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	request := WorkbenchPortSuggestionRequest{
		Selector: WorkbenchPortSelector{
			ServiceName:   "cache",
			ContainerPort: 6380,
			Protocol:      "tcp",
		},
		Limit: 3,
	}

	firstSnapshot, firstSummary, firstErr := svc.SuggestStoredSnapshotHostPorts(context.Background(), "demo", request)
	if firstErr != nil {
		t.Fatalf("first suggestion request: %v", firstErr)
	}
	secondSnapshot, secondSummary, secondErr := svc.SuggestStoredSnapshotHostPorts(context.Background(), "demo", request)
	if secondErr != nil {
		t.Fatalf("second suggestion request: %v", secondErr)
	}

	if !reflect.DeepEqual(firstSummary, secondSummary) {
		t.Fatalf("expected deterministic suggestions\nfirst=%#v\nsecond=%#v", firstSummary, secondSummary)
	}
	if !reflect.DeepEqual(firstSnapshot.Ports, secondSnapshot.Ports) {
		t.Fatalf("expected snapshot to remain unchanged across suggestions\nfirst=%#v\nsecond=%#v", firstSnapshot.Ports, secondSnapshot.Ports)
	}
	if firstSummary.Source != workbenchPortSourceModuleDefault {
		t.Fatalf("expected module default source, got %q", firstSummary.Source)
	}
	if firstSummary.PreferredHostPort == nil || *firstSummary.PreferredHostPort != 6379 {
		t.Fatalf("expected preferred host port 6379, got %#v", firstSummary.PreferredHostPort)
	}
	if firstSummary.SuggestionCount != 3 {
		t.Fatalf("expected 3 suggestions, got %d", firstSummary.SuggestionCount)
	}

	gotPorts := workbenchSuggestionPorts(firstSummary.Suggestions)
	expectedPorts := []int{6380, 6381, 6382}
	if !reflect.DeepEqual(gotPorts, expectedPorts) {
		t.Fatalf("unexpected suggestion ports got=%v want=%v", gotPorts, expectedPorts)
	}

	stored, exists, loadErr := svc.loadStoredWorkbenchSnapshot(context.Background(), "demo")
	if loadErr != nil {
		t.Fatalf("load stored snapshot: %v", loadErr)
	}
	if !exists {
		t.Fatal("expected stored snapshot to exist")
	}
	if stored.Revision != initial.Revision {
		t.Fatalf("expected read-only suggestion flow to keep revision=%d, got %d", initial.Revision, stored.Revision)
	}
}

func workbenchSuggestionPorts(suggestions []WorkbenchPortSuggestion) []int {
	ports := make([]int, 0, len(suggestions))
	for _, suggestion := range suggestions {
		ports = append(ports, suggestion.HostPort)
	}
	return ports
}
