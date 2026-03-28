package config

import "testing"

func TestNormalizeDBHostPublishMode(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty defaults disabled", raw: "", want: "disabled"},
		{name: "explicit disabled", raw: "disabled", want: "disabled"},
		{name: "loopback accepted", raw: "loopback", want: "loopback"},
		{name: "loopback accepted case-insensitive", raw: " LoOpBaCk ", want: "loopback"},
		{name: "invalid fails closed", raw: "wildcard", want: "disabled"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeDBHostPublishMode(tc.raw); got != tc.want {
				t.Fatalf("expected mode %q, got %q", tc.want, got)
			}
		})
	}
}

func TestNormalizeDBHostPublishHost(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty defaults loopback", raw: "", want: "127.0.0.1"},
		{name: "loopback accepted", raw: "127.0.0.1", want: "127.0.0.1"},
		{name: "localhost normalized", raw: "localhost", want: "127.0.0.1"},
		{name: "non-loopback fails closed", raw: "0.0.0.0", want: "127.0.0.1"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeDBHostPublishHost(tc.raw); got != tc.want {
				t.Fatalf("expected host %q, got %q", tc.want, got)
			}
		})
	}
}

func TestNormalizeDBHostPublishPort(t *testing.T) {
	tests := []struct {
		name string
		raw  int
		want int
	}{
		{name: "default when zero", raw: 0, want: 5432},
		{name: "valid port accepted", raw: 15432, want: 15432},
		{name: "negative fails closed", raw: -1, want: 5432},
		{name: "too large fails closed", raw: 65536, want: 5432},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeDBHostPublishPort(tc.raw); got != tc.want {
				t.Fatalf("expected port %d, got %d", tc.want, got)
			}
		})
	}
}

func TestNormalizeDockerNetworkGuardrailsMode(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "defaults to enforced",
			raw:  "",
			want: "enforced",
		},
		{
			name: "compat accepted",
			raw:  "compat",
			want: "compat",
		},
		{
			name: "compat accepted case-insensitive",
			raw:  " CoMpAt ",
			want: "compat",
		},
		{
			name: "invalid values fail closed",
			raw:  "edge_compat",
			want: "enforced",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeDockerNetworkGuardrailsMode(tc.raw); got != tc.want {
				t.Fatalf("expected network mode %q, got %q", tc.want, got)
			}
		})
	}
}
