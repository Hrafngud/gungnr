package service

import (
	"testing"

	"go-notes/internal/utils/cryptox"
)

func TestLoadSettingsEncryptedPayloadSupportsLegacySingleWorkbenchSnapshot(t *testing.T) {
	t.Parallel()

	encrypted := encryptedSettingsPayloadForTest(t, `{
		"workbench": {
			"projectName": "Demo",
			"composePath": "/tmp/demo/docker-compose.yml",
			"modelVersion": 1,
			"revision": 4,
			"services": []
		}
	}`)

	payload, err := loadSettingsEncryptedPayload("test-session-secret", encrypted)
	if err != nil {
		t.Fatalf("loadSettingsEncryptedPayload: %v", err)
	}

	if len(payload.Workbench) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(payload.Workbench))
	}
	snapshot, ok := payload.Workbench["demo"]
	if !ok {
		t.Fatalf("expected snapshot key %q", "demo")
	}
	if snapshot.ProjectName != "demo" {
		t.Fatalf("expected normalized projectName demo, got %q", snapshot.ProjectName)
	}
	if snapshot.Revision != 4 {
		t.Fatalf("expected revision 4, got %d", snapshot.Revision)
	}
}

func TestLoadSettingsEncryptedPayloadSkipsInvalidWorkbenchEntries(t *testing.T) {
	t.Parallel()

	encrypted := encryptedSettingsPayloadForTest(t, `{
		"workbench": {
			"demo": {
				"projectName": "demo",
				"composePath": "/tmp/demo/docker-compose.yml",
				"modelVersion": 1,
				"revision": 2,
				"services": []
			},
			"broken": "oops"
		}
	}`)

	payload, err := loadSettingsEncryptedPayload("test-session-secret", encrypted)
	if err != nil {
		t.Fatalf("loadSettingsEncryptedPayload: %v", err)
	}

	if len(payload.Workbench) != 1 {
		t.Fatalf("expected one valid snapshot, got %d", len(payload.Workbench))
	}
	if _, ok := payload.Workbench["demo"]; !ok {
		t.Fatalf("expected valid demo snapshot")
	}
	if _, ok := payload.Workbench["broken"]; ok {
		t.Fatalf("did not expect invalid broken snapshot entry to survive decode")
	}
}

func TestLoadSettingsEncryptedPayloadPreservesNetBirdWhenWorkbenchIsInvalid(t *testing.T) {
	t.Parallel()

	encrypted := encryptedSettingsPayloadForTest(t, `{
		"netbird": {
			"apiBaseUrl": "https://api.netbird.example",
			"apiToken": "secret"
		},
		"workbench": "invalid"
	}`)

	payload, err := loadSettingsEncryptedPayload("test-session-secret", encrypted)
	if err != nil {
		t.Fatalf("loadSettingsEncryptedPayload: %v", err)
	}

	if payload.NetBird == nil {
		t.Fatal("expected netbird payload to decode")
	}
	if payload.NetBird.APIBaseURL != "https://api.netbird.example" {
		t.Fatalf("expected netbird apiBaseUrl to survive, got %q", payload.NetBird.APIBaseURL)
	}
	if len(payload.Workbench) != 0 {
		t.Fatalf("expected invalid workbench payload to be dropped, got %d entries", len(payload.Workbench))
	}
}

func encryptedSettingsPayloadForTest(t *testing.T, raw string) string {
	t.Helper()

	encrypted, err := cryptox.EncryptWithSecret("test-session-secret", raw)
	if err != nil {
		t.Fatalf("encrypt payload: %v", err)
	}
	return encrypted
}
