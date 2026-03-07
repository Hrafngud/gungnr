package service

import (
	"context"
	"reflect"
	"testing"

	"go-notes/internal/errs"
)

func TestWorkbenchAddOptionalServiceDeterministic(t *testing.T) {
	t.Parallel()

	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    4,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "ghcr.io/example/api:latest"},
		},
	}
	request := WorkbenchOptionalServiceAddRequest{EntryKey: "minio"}

	firstSvc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	if err := firstSvc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save first snapshot: %v", err)
	}
	secondSvc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	if err := secondSvc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save second snapshot: %v", err)
	}

	firstSnapshot, firstSummary, firstErr := firstSvc.AddOptionalService(context.Background(), "demo", request)
	if firstErr != nil {
		t.Fatalf("first add optional service: %v", firstErr)
	}
	secondSnapshot, secondSummary, secondErr := secondSvc.AddOptionalService(context.Background(), "demo", request)
	if secondErr != nil {
		t.Fatalf("second add optional service: %v", secondErr)
	}

	if !reflect.DeepEqual(firstSnapshot.ManagedServices, secondSnapshot.ManagedServices) {
		t.Fatalf("expected deterministic managed services\nfirst=%#v\nsecond=%#v", firstSnapshot.ManagedServices, secondSnapshot.ManagedServices)
	}
	if !reflect.DeepEqual(firstSummary, secondSummary) {
		t.Fatalf("expected deterministic optional-service summary\nfirst=%#v\nsecond=%#v", firstSummary, secondSummary)
	}
	if !firstSummary.Changed {
		t.Fatal("expected changed=true for add optional service")
	}
	if firstSummary.EntryKey != "minio" || firstSummary.ServiceName != "minio" {
		t.Fatalf("unexpected summary identity: %#v", firstSummary)
	}
	if !firstSummary.ComposeGenerationReady {
		t.Fatal("expected composeGenerationReady=true")
	}
	if firstSnapshot.Revision != 5 {
		t.Fatalf("expected revision=5 after add optional service, got %d", firstSnapshot.Revision)
	}
	if got, want := len(firstSnapshot.ManagedServices), 1; got != want {
		t.Fatalf("expected %d managed service, got %d", want, got)
	}
	if firstSnapshot.ManagedServices[0] != (WorkbenchManagedService{EntryKey: "minio", ServiceName: "minio"}) {
		t.Fatalf("unexpected managed service record: %#v", firstSnapshot.ManagedServices[0])
	}
}

func TestWorkbenchAddOptionalServiceRejectsDuplicateAndUnsupported(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    8,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "ghcr.io/example/api:latest"},
		},
		ManagedServices: []WorkbenchManagedService{
			{EntryKey: "minio", ServiceName: "minio"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	_, duplicateSummary, duplicateErr := svc.AddOptionalService(context.Background(), "demo", WorkbenchOptionalServiceAddRequest{EntryKey: "minio"})
	if duplicateErr == nil {
		t.Fatal("expected duplicate optional-service validation error")
	}
	if duplicateSummary.Changed {
		t.Fatal("expected changed=false on duplicate optional-service validation")
	}
	assertWorkbenchOptionalServiceIssueCode(t, duplicateErr, "WB-OPTIONAL-SERVICE-DUPLICATE")

	_, unsupportedSummary, unsupportedErr := svc.AddOptionalService(context.Background(), "demo", WorkbenchOptionalServiceAddRequest{EntryKey: "unknown"})
	if unsupportedErr == nil {
		t.Fatal("expected unsupported optional-service validation error")
	}
	if unsupportedSummary.Changed {
		t.Fatal("expected changed=false on unsupported optional-service validation")
	}
	assertWorkbenchOptionalServiceIssueCode(t, unsupportedErr, "WB-OPTIONAL-SERVICE-KEY-UNSUPPORTED")
}

func TestWorkbenchRemoveOptionalServiceUpdatesManagedStateAndRejectsMissing(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    12,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "ghcr.io/example/api:latest"},
		},
		ManagedServices: []WorkbenchManagedService{
			{EntryKey: "redis", ServiceName: "redis"},
		},
		Modules: []WorkbenchStackModule{
			{ModuleType: "redis", ServiceName: "api"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	snapshot, summary, err := svc.RemoveOptionalService(context.Background(), "demo", "redis")
	if err != nil {
		t.Fatalf("RemoveOptionalService: %v", err)
	}
	if !summary.Changed {
		t.Fatal("expected changed=true for remove optional service")
	}
	if summary.EntryKey != "redis" || summary.ServiceName != "redis" {
		t.Fatalf("unexpected remove summary identity: %#v", summary)
	}
	if snapshot.Revision != 13 {
		t.Fatalf("expected revision=13 after remove optional service, got %d", snapshot.Revision)
	}
	if got, want := len(snapshot.ManagedServices), 0; got != want {
		t.Fatalf("expected %d managed services after removal, got %d", want, got)
	}
	if got, want := len(snapshot.Modules), 1; got != want {
		t.Fatalf("expected legacy modules to remain untouched, got %d", got)
	}

	_, missingSummary, missingErr := svc.RemoveOptionalService(context.Background(), "demo", "redis")
	if missingErr == nil {
		t.Fatal("expected missing optional-service validation error")
	}
	if missingSummary.Changed {
		t.Fatal("expected changed=false on missing optional-service validation")
	}
	assertWorkbenchOptionalServiceIssueCode(t, missingErr, "WB-OPTIONAL-SERVICE-NOT-FOUND")
}

func assertWorkbenchOptionalServiceIssueCode(t *testing.T, opErr error, expectedCode string) {
	t.Helper()

	typed, ok := errs.From(opErr)
	if !ok {
		t.Fatalf("expected typed error, got %T", opErr)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}

	details, ok := typed.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected details map, got %T", typed.Details)
	}
	issuesAny, ok := details["issues"]
	if !ok {
		t.Fatalf("expected issues in details: %#v", details)
	}
	issues, ok := issuesAny.([]WorkbenchMutationIssue)
	if !ok {
		t.Fatalf("expected []WorkbenchMutationIssue, got %T", issuesAny)
	}
	if len(issues) != 1 || issues[0].Code != expectedCode {
		t.Fatalf("expected issue code %q, got %#v", expectedCode, issues)
	}
}
