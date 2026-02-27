package netbird

import (
	"strings"
	"testing"
)

func TestDecodeWithFallback_UnknownTopLevelArrayKey(t *testing.T) {
	raw := []byte(`{"payload":[{"id":"peer-1","name":"panel-host","ip":"100.64.0.10","connected":true}]}`)

	var peers []Peer
	if err := decodeWithFallback(raw, &peers, "peers", "items", "data", "result"); err != nil {
		t.Fatalf("decodeWithFallback returned error: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].ID != "peer-1" {
		t.Fatalf("expected peer id peer-1, got %q", peers[0].ID)
	}
}

func TestDecodeWithFallback_UnknownNestedArrayKey(t *testing.T) {
	raw := []byte(`{"wrapper":{"records":[{"id":"group-1","name":"gungnr-panel","peers":["peer-1"]}]}}`)

	var groups []Group
	if err := decodeWithFallback(raw, &groups, "groups", "items", "data", "result"); err != nil {
		t.Fatalf("decodeWithFallback returned error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].ID != "group-1" {
		t.Fatalf("expected group id group-1, got %q", groups[0].ID)
	}
}

func TestDecodeWithFallback_ErrorIncludesAvailableKeys(t *testing.T) {
	raw := []byte(`{"code":200,"message":"ok"}`)

	var peers []Peer
	err := decodeWithFallback(raw, &peers, "peers", "items", "data", "result")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	message := err.Error()
	if !strings.Contains(message, "available keys") {
		t.Fatalf("expected error to include available keys, got: %s", message)
	}
	if !strings.Contains(message, "message") {
		t.Fatalf("expected error to include envelope keys, got: %s", message)
	}
}

func TestDecodeWithFallback_GroupPeersAsObjects(t *testing.T) {
	raw := []byte(`[{"id":"group-1","name":"Admins","peers":[{"id":"peer-1","name":"host"},{"id":"peer-2","name":"laptop"}]}]`)

	var groups []Group
	if err := decodeWithFallback(raw, &groups, "groups", "items", "data", "result"); err != nil {
		t.Fatalf("decodeWithFallback returned error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(groups[0].Peers))
	}
	if groups[0].Peers[0] != "peer-1" || groups[0].Peers[1] != "peer-2" {
		t.Fatalf("unexpected peer ids: %v", groups[0].Peers)
	}
}

func TestDecodeWithFallback_PolicyRuleRefsAsObjects(t *testing.T) {
	raw := []byte(`[{"id":"policy-1","name":"Admins to Panel","enabled":true,"rules":[{"name":"allow","enabled":true,"action":"accept","bidirectional":false,"protocol":"tcp","ports":["8080"],"sources":[{"id":"group-admins","name":"Admins"}],"destinations":[{"id":"group-panel","name":"Panel"}]}]}]`)

	var policies []Policy
	if err := decodeWithFallback(raw, &policies, "policies", "items", "data", "result"); err != nil {
		t.Fatalf("decodeWithFallback returned error: %v", err)
	}
	if len(policies) != 1 || len(policies[0].Rules) != 1 {
		t.Fatalf("unexpected policy decode result: %+v", policies)
	}
	rule := policies[0].Rules[0]
	if len(rule.Sources) != 1 || rule.Sources[0] != "group-admins" {
		t.Fatalf("unexpected sources: %v", rule.Sources)
	}
	if len(rule.Destinations) != 1 || rule.Destinations[0] != "group-panel" {
		t.Fatalf("unexpected destinations: %v", rule.Destinations)
	}
}
