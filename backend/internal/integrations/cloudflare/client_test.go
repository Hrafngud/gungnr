package cloudflare

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"go-notes/internal/config"

	"github.com/stretchr/testify/require"
)

func TestDeleteTunnelCNAMERecordDeletesMatchingCNAME(t *testing.T) {
	t.Parallel()

	deleteCalls := 0
	client := newTestCloudflareClient(t, func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/client/v4/zones/zone-1/dns_records":
			require.Equal(t, "app.example.com", r.URL.Query().Get("name"))
			return cloudflareSuccessResponse(t, []map[string]any{
				{
					"id":      "rec-1",
					"type":    "CNAME",
					"name":    "app.example.com",
					"content": "tunnel-1.cfargotunnel.com",
					"proxied": true,
				},
			}), nil
		case r.Method == http.MethodDelete && r.URL.Path == "/client/v4/zones/zone-1/dns_records/rec-1":
			deleteCalls++
			return cloudflareSuccessResponse(t, map[string]any{}), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})
	result, err := client.DeleteTunnelCNAMERecord(context.Background(), "zone-1", "rec-1", "app.example.com", "tunnel-1.cfargotunnel.com")
	require.NoError(t, err)
	require.True(t, result.Deleted)
	require.Empty(t, result.SkipReason)
	require.Equal(t, 1, deleteCalls)
}

func TestDeleteTunnelCNAMERecordSkipsWhenTargetDrifts(t *testing.T) {
	t.Parallel()

	deleteCalls := 0
	client := newTestCloudflareClient(t, func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/client/v4/zones/zone-1/dns_records":
			require.Equal(t, "app.example.com", r.URL.Query().Get("name"))
			return cloudflareSuccessResponse(t, []map[string]any{
				{
					"id":      "rec-1",
					"type":    "CNAME",
					"name":    "app.example.com",
					"content": "other-target.cfargotunnel.com",
					"proxied": true,
				},
			}), nil
		case r.Method == http.MethodDelete:
			deleteCalls++
			t.Fatalf("unexpected delete request: %s", r.URL.Path)
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})
	result, err := client.DeleteTunnelCNAMERecord(context.Background(), "zone-1", "rec-1", "app.example.com", "tunnel-1.cfargotunnel.com")
	require.NoError(t, err)
	require.False(t, result.Deleted)
	require.Equal(t, "target other-target.cfargotunnel.com no longer matches tunnel-1.cfargotunnel.com", result.SkipReason)
	require.Equal(t, 0, deleteCalls)
}

func TestDeleteTunnelCNAMERecordSkipsWhenRecordIsNotCNAME(t *testing.T) {
	t.Parallel()

	deleteCalls := 0
	client := newTestCloudflareClient(t, func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/client/v4/zones/zone-1/dns_records":
			require.Equal(t, "app.example.com", r.URL.Query().Get("name"))
			return cloudflareSuccessResponse(t, []map[string]any{
				{
					"id":      "rec-1",
					"type":    "A",
					"name":    "app.example.com",
					"content": "192.0.2.10",
					"proxied": true,
				},
			}), nil
		case r.Method == http.MethodDelete:
			deleteCalls++
			t.Fatalf("unexpected delete request: %s", r.URL.Path)
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})
	result, err := client.DeleteTunnelCNAMERecord(context.Background(), "zone-1", "rec-1", "app.example.com", "tunnel-1.cfargotunnel.com")
	require.NoError(t, err)
	require.False(t, result.Deleted)
	require.Equal(t, "type is A", result.SkipReason)
	require.Equal(t, 0, deleteCalls)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestCloudflareClient(t *testing.T, transport roundTripFunc) *Client {
	t.Helper()

	return &Client{
		cfg: config.Config{
			CloudflareAPIToken: "token-1",
		},
		client: &http.Client{
			Transport: transport,
		},
	}
}

func cloudflareSuccessResponse(t *testing.T, result any) *http.Response {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"success": true,
		"errors":  []any{},
		"result":  result,
	})
	require.NoError(t, err)

	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(string(body))),
	}
}
