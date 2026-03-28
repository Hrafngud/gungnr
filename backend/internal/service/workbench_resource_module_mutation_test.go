package service

import (
	"context"
	"reflect"
	"testing"

	"go-notes/internal/errs"
)

func TestWorkbenchMutateStoredSnapshotResourceSetDeterministic(t *testing.T) {
	t.Parallel()

	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    4,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "web", Image: "nginx:stable"},
		},
		Resources: []WorkbenchComposeResource{
			{ServiceName: "api", LimitCPUs: "1.00"},
		},
	}
	request := WorkbenchResourceMutationRequest{
		Selector: WorkbenchResourceSelector{
			ServiceName: "web",
		},
		Action:            workbenchResourceMutationActionSet,
		LimitCPUs:         strPtr("0.50"),
		ReservationMemory: strPtr("256M"),
	}

	firstSvc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	if err := firstSvc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save first snapshot: %v", err)
	}
	secondSvc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	if err := secondSvc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save second snapshot: %v", err)
	}

	firstSnapshot, firstSummary, firstErr := firstSvc.MutateStoredSnapshotResource(context.Background(), "demo", request)
	if firstErr != nil {
		t.Fatalf("first resource mutation: %v", firstErr)
	}
	secondSnapshot, secondSummary, secondErr := secondSvc.MutateStoredSnapshotResource(context.Background(), "demo", request)
	if secondErr != nil {
		t.Fatalf("second resource mutation: %v", secondErr)
	}

	if !reflect.DeepEqual(firstSnapshot.Resources, secondSnapshot.Resources) {
		t.Fatalf("expected deterministic resources\nfirst=%#v\nsecond=%#v", firstSnapshot.Resources, secondSnapshot.Resources)
	}
	if !reflect.DeepEqual(firstSummary, secondSummary) {
		t.Fatalf("expected deterministic resource summary\nfirst=%#v\nsecond=%#v", firstSummary, secondSummary)
	}
	if !firstSummary.Changed {
		t.Fatal("expected changed=true for resource set mutation")
	}
	if !reflect.DeepEqual(firstSummary.UpdatedFields, []string{workbenchResourceFieldLimitCPUs, workbenchResourceFieldReservationMemory}) {
		t.Fatalf("unexpected updated fields: %#v", firstSummary.UpdatedFields)
	}
	if firstSummary.CurrentResource == nil {
		t.Fatal("expected current resource to be present")
	}
	if firstSummary.CurrentResource.ServiceName != "web" {
		t.Fatalf("expected current resource service=web, got %q", firstSummary.CurrentResource.ServiceName)
	}
	if firstSummary.CurrentResource.LimitCPUs != "0.50" || firstSummary.CurrentResource.ReservationMemory != "256M" {
		t.Fatalf("unexpected current resource values: %#v", firstSummary.CurrentResource)
	}
	if firstSnapshot.Revision != 5 {
		t.Fatalf("expected revision=5 after mutation persistence, got %d", firstSnapshot.Revision)
	}

	mutated := findWorkbenchResource(firstSnapshot.Resources, "web")
	if mutated == nil {
		t.Fatalf("expected mutated web resource, got %#v", firstSnapshot.Resources)
	}
	if mutated.LimitCPUs != "0.50" || mutated.ReservationMemory != "256M" {
		t.Fatalf("unexpected persisted web resource values: %#v", mutated)
	}
}

func TestWorkbenchMutateStoredSnapshotResourceClearRemovesEntry(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    7,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
			{ServiceName: "web", Image: "nginx:stable"},
		},
		Resources: []WorkbenchComposeResource{
			{
				ServiceName:       "web",
				LimitCPUs:         "0.50",
				LimitMemory:       "512M",
				ReservationCPUs:   "0.25",
				ReservationMemory: "256M",
			},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	request := WorkbenchResourceMutationRequest{
		Selector: WorkbenchResourceSelector{ServiceName: "web"},
		Action:   workbenchResourceMutationActionClear,
	}

	resolved, summary, err := svc.MutateStoredSnapshotResource(context.Background(), "demo", request)
	if err != nil {
		t.Fatalf("resource clear mutation: %v", err)
	}
	if !summary.Changed {
		t.Fatal("expected changed=true for clear mutation")
	}
	if summary.CurrentResource != nil {
		t.Fatalf("expected current resource to be removed, got %#v", summary.CurrentResource)
	}
	if findWorkbenchResource(resolved.Resources, "web") != nil {
		t.Fatalf("expected web resource entry removed, got %#v", resolved.Resources)
	}
	if resolved.Revision != 8 {
		t.Fatalf("expected revision=8 after mutation persistence, got %d", resolved.Revision)
	}
}

func TestWorkbenchMutateStoredSnapshotModuleDuplicateValidation(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    3,
		Services: []WorkbenchComposeService{
			{ServiceName: "cache", Image: "redis:7"},
		},
		Modules: []WorkbenchStackModule{
			{ModuleType: "redis", ServiceName: "cache"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	request := WorkbenchModuleMutationRequest{
		Selector: WorkbenchModuleSelector{
			ServiceName: "cache",
			ModuleType:  "redis",
		},
		Action: workbenchModuleMutationActionAdd,
	}
	_, summary, err := svc.MutateStoredSnapshotModule(context.Background(), "demo", request)
	if err == nil {
		t.Fatal("expected duplicate module validation error")
	}
	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}
	if summary.Changed {
		t.Fatal("expected changed=false on duplicate module validation")
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
	if len(issues) != 1 || issues[0].Code != "WB-MODULE-DUPLICATE" {
		t.Fatalf("expected WB-MODULE-DUPLICATE issue, got %#v", issues)
	}
}

func TestWorkbenchMutateStoredSnapshotModuleUnsupportedTargetValidation(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    9,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	request := WorkbenchModuleMutationRequest{
		Selector: WorkbenchModuleSelector{
			ServiceName: "cache",
			ModuleType:  "redis",
		},
		Action: workbenchModuleMutationActionAdd,
	}
	_, _, err := svc.MutateStoredSnapshotModule(context.Background(), "demo", request)
	if err == nil {
		t.Fatal("expected unsupported target validation error")
	}
	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
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
	if len(issues) != 1 || issues[0].Code != "WB-MODULE-TARGET-UNSUPPORTED" {
		t.Fatalf("expected WB-MODULE-TARGET-UNSUPPORTED issue, got %#v", issues)
	}
}

func TestWorkbenchMutateLegacyModuleCompatibilityAddsManagedService(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    3,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	snapshot, summary, err := svc.MutateLegacyModuleCompatibility(context.Background(), "demo", WorkbenchModuleMutationRequest{
		Selector: WorkbenchModuleSelector{
			ServiceName: "redis",
			ModuleType:  "redis",
		},
		Action: workbenchModuleMutationActionAdd,
	})
	if err != nil {
		t.Fatalf("MutateLegacyModuleCompatibility add: %v", err)
	}
	if !summary.Changed {
		t.Fatal("expected changed=true when adding redis through legacy compatibility path")
	}
	if summary.PreviousCount != 0 || summary.CurrentCount != 1 {
		t.Fatalf("unexpected count summary: %#v", summary)
	}
	if snapshot.Revision != 4 {
		t.Fatalf("expected revision=4 after compatibility add, got %d", snapshot.Revision)
	}
	if got, want := len(snapshot.ManagedServices), 1; got != want {
		t.Fatalf("expected %d managed service after compatibility add, got %d", want, got)
	}
	if snapshot.ManagedServices[0] != (WorkbenchManagedService{EntryKey: "redis", ServiceName: "redis"}) {
		t.Fatalf("unexpected managed service record: %#v", snapshot.ManagedServices[0])
	}
	if got := len(snapshot.Modules); got != 0 {
		t.Fatalf("expected legacy modules to stay unchanged, got %d entries", got)
	}
}

func TestWorkbenchMutateLegacyModuleCompatibilityAcceptsLegacySelectorAlias(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    5,
		Services: []WorkbenchComposeService{
			{ServiceName: "cache", Image: "redis:7"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	snapshot, summary, err := svc.MutateLegacyModuleCompatibility(context.Background(), "demo", WorkbenchModuleMutationRequest{
		Selector: WorkbenchModuleSelector{
			ServiceName: "cache",
			ModuleType:  "redis",
		},
		Action: workbenchModuleMutationActionAdd,
	})
	if err != nil {
		t.Fatalf("MutateLegacyModuleCompatibility add with legacy alias: %v", err)
	}
	if !summary.Changed {
		t.Fatal("expected changed=true when adding redis through legacy compatibility alias")
	}
	if summary.PreviousCount != 0 || summary.CurrentCount != 1 {
		t.Fatalf("unexpected count summary: %#v", summary)
	}
	if snapshot.Revision != 6 {
		t.Fatalf("expected revision=6 after compatibility add with legacy alias, got %d", snapshot.Revision)
	}
	if got, want := len(snapshot.ManagedServices), 1; got != want {
		t.Fatalf("expected %d managed service after compatibility add, got %d", want, got)
	}
	if snapshot.ManagedServices[0] != (WorkbenchManagedService{EntryKey: "redis", ServiceName: "redis"}) {
		t.Fatalf("unexpected managed service record: %#v", snapshot.ManagedServices[0])
	}
}

func TestWorkbenchMutateLegacyModuleCompatibilityRemoveNoOpWhenMissing(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    9,
		Services: []WorkbenchComposeService{
			{ServiceName: "redis", Image: "redis:7"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	snapshot, summary, err := svc.MutateLegacyModuleCompatibility(context.Background(), "demo", WorkbenchModuleMutationRequest{
		Selector: WorkbenchModuleSelector{
			ServiceName: "redis",
			ModuleType:  "redis",
		},
		Action: workbenchModuleMutationActionRemove,
	})
	if err != nil {
		t.Fatalf("MutateLegacyModuleCompatibility remove: %v", err)
	}
	if summary.Changed {
		t.Fatal("expected changed=false when removing missing redis managed service")
	}
	if summary.PreviousCount != 0 || summary.CurrentCount != 0 {
		t.Fatalf("unexpected count summary: %#v", summary)
	}
	if snapshot.Revision != 9 {
		t.Fatalf("expected unchanged revision on no-op remove, got %d", snapshot.Revision)
	}
	if got := len(snapshot.ManagedServices); got != 0 {
		t.Fatalf("expected no managed services, got %d", got)
	}
}

func TestWorkbenchMutateLegacyModuleCompatibilityRejectsTargetMismatch(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    1,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	_, summary, err := svc.MutateLegacyModuleCompatibility(context.Background(), "demo", WorkbenchModuleMutationRequest{
		Selector: WorkbenchModuleSelector{
			ServiceName: "api",
			ModuleType:  "redis",
		},
		Action: workbenchModuleMutationActionAdd,
	})
	if err == nil {
		t.Fatal("expected target mismatch validation error")
	}
	if summary.Changed {
		t.Fatal("expected changed=false on target mismatch")
	}
	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
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
	if len(issues) != 1 || issues[0].Code != "WB-MODULE-TARGET-UNSUPPORTED" {
		t.Fatalf("expected WB-MODULE-TARGET-UNSUPPORTED issue, got %#v", issues)
	}
}

func TestWorkbenchMutateStoredSnapshotResourceInvalidValueValidationPayload(t *testing.T) {
	t.Parallel()

	svc := NewWorkbenchServiceWithStorage(t.TempDir(), nil, &fakeSettingsRepo{}, "test-session-secret")
	initial := WorkbenchStackSnapshot{
		ProjectName: "demo",
		Revision:    11,
		Services: []WorkbenchComposeService{
			{ServiceName: "api", Image: "nginx:stable"},
		},
	}
	if err := svc.saveWorkbenchSnapshot(context.Background(), "demo", initial); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	request := WorkbenchResourceMutationRequest{
		Selector: WorkbenchResourceSelector{
			ServiceName: "api",
		},
		Action:    workbenchResourceMutationActionSet,
		LimitCPUs: strPtr("not-a-number"),
	}
	_, summary, err := svc.MutateStoredSnapshotResource(context.Background(), "demo", request)
	if err == nil {
		t.Fatal("expected invalid value validation error")
	}
	typed, ok := errs.From(err)
	if !ok {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typed.Code != errs.CodeWorkbenchValidationFailed {
		t.Fatalf("expected code %q, got %q", errs.CodeWorkbenchValidationFailed, typed.Code)
	}
	if summary.Changed {
		t.Fatal("expected changed=false on invalid value validation")
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
	if len(issues) != 1 || issues[0].Code != "WB-RESOURCE-LIMITCPUS-INVALID" {
		t.Fatalf("expected WB-RESOURCE-LIMITCPUS-INVALID issue, got %#v", issues)
	}
}

func findWorkbenchResource(resources []WorkbenchComposeResource, serviceName string) *WorkbenchComposeResource {
	for idx := range resources {
		if resources[idx].ServiceName == serviceName {
			return &resources[idx]
		}
	}
	return nil
}

func strPtr(value string) *string {
	v := value
	return &v
}
