package netbird

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.netbird.io"

var ErrMissingToken = errors.New("netbird api token is required")

type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

type Peer struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IP         string `json:"ip"`
	DNSLabel   string `json:"dns_label"`
	LastSeen   string `json:"last_seen,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	Connected  bool   `json:"connected"`
	SSHEnabled bool   `json:"ssh_enabled"`
}

type Group struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Peers []string `json:"peers"`
}

type GroupRequest struct {
	Name  string   `json:"name"`
	Peers []string `json:"peers"`
}

type Policy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Enabled     bool         `json:"enabled"`
	Rules       []PolicyRule `json:"rules"`
}

type PolicyRule struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Enabled       bool     `json:"enabled"`
	Action        string   `json:"action"`
	Bidirectional bool     `json:"bidirectional"`
	Protocol      string   `json:"protocol"`
	Ports         []string `json:"ports"`
	Sources       []string `json:"sources"`
	Destinations  []string `json:"destinations"`
}

type referenceObject struct {
	ID string `json:"id"`
}

func (g *Group) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Peers json.RawMessage `json:"peers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	peers, err := decodeStringReferences(raw.Peers)
	if err != nil {
		return fmt.Errorf("decode group peers: %w", err)
	}
	g.ID = strings.TrimSpace(raw.ID)
	g.Name = strings.TrimSpace(raw.Name)
	g.Peers = peers
	return nil
}

func (r *PolicyRule) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name          string          `json:"name"`
		Description   string          `json:"description,omitempty"`
		Enabled       bool            `json:"enabled"`
		Action        string          `json:"action"`
		Bidirectional bool            `json:"bidirectional"`
		Protocol      string          `json:"protocol"`
		Ports         json.RawMessage `json:"ports"`
		Sources       json.RawMessage `json:"sources"`
		Destinations  json.RawMessage `json:"destinations"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	ports, err := decodePorts(raw.Ports)
	if err != nil {
		return fmt.Errorf("decode policy rule ports: %w", err)
	}
	sources, err := decodeStringReferences(raw.Sources)
	if err != nil {
		return fmt.Errorf("decode policy rule sources: %w", err)
	}
	destinations, err := decodeStringReferences(raw.Destinations)
	if err != nil {
		return fmt.Errorf("decode policy rule destinations: %w", err)
	}
	r.Name = strings.TrimSpace(raw.Name)
	r.Description = strings.TrimSpace(raw.Description)
	r.Enabled = raw.Enabled
	r.Action = strings.TrimSpace(raw.Action)
	r.Bidirectional = raw.Bidirectional
	r.Protocol = strings.TrimSpace(raw.Protocol)
	r.Ports = ports
	r.Sources = sources
	r.Destinations = destinations
	return nil
}

type PolicyRequest struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Enabled     bool         `json:"enabled"`
	Rules       []PolicyRule `json:"rules"`
}

func NewClient(baseURL, token string) *Client {
	return NewClientWithHTTP(baseURL, token, nil)
}

func NewClientWithHTTP(baseURL, token string, httpClient *http.Client) *Client {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBaseURL == "" {
		trimmedBaseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 12 * time.Second}
	}
	return &Client{
		baseURL: trimmedBaseURL,
		token:   strings.TrimSpace(token),
		client:  httpClient,
	}
}

func (c *Client) ListPeers(ctx context.Context) ([]Peer, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/peers", nil)
	if err != nil {
		return nil, err
	}
	var peers []Peer
	if err := decodeWithFallback(raw, &peers, "peers", "items", "data", "result"); err != nil {
		return nil, err
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Name == peers[j].Name {
			return peers[i].ID < peers[j].ID
		}
		return peers[i].Name < peers[j].Name
	})
	return peers, nil
}

func (c *Client) ListGroups(ctx context.Context) ([]Group, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/groups", nil)
	if err != nil {
		return nil, err
	}
	var groups []Group
	if err := decodeWithFallback(raw, &groups, "groups", "items", "data", "result"); err != nil {
		return nil, err
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Name == groups[j].Name {
			return groups[i].ID < groups[j].ID
		}
		return groups[i].Name < groups[j].Name
	})
	return groups, nil
}

func (c *Client) CreateGroup(ctx context.Context, input GroupRequest) (Group, error) {
	raw, err := c.do(ctx, http.MethodPost, "/api/groups", input)
	if err != nil {
		return Group{}, err
	}
	var group Group
	if err := decodeWithFallback(raw, &group, "group", "data", "result"); err != nil {
		return Group{}, err
	}
	return group, nil
}

func (c *Client) UpdateGroup(ctx context.Context, groupID string, input GroupRequest) (Group, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return Group{}, fmt.Errorf("group id is required")
	}
	raw, err := c.do(ctx, http.MethodPut, "/api/groups/"+groupID, input)
	if err != nil {
		return Group{}, err
	}
	var group Group
	if err := decodeWithFallback(raw, &group, "group", "data", "result"); err != nil {
		return Group{}, err
	}
	return group, nil
}

func (c *Client) DeleteGroup(ctx context.Context, groupID string) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return fmt.Errorf("group id is required")
	}
	_, err := c.do(ctx, http.MethodDelete, "/api/groups/"+groupID, nil)
	return err
}

func (c *Client) ListPolicies(ctx context.Context) ([]Policy, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/policies", nil)
	if err != nil {
		return nil, err
	}
	var policies []Policy
	if err := decodeWithFallback(raw, &policies, "policies", "items", "data", "result"); err != nil {
		return nil, err
	}
	sort.Slice(policies, func(i, j int) bool {
		if policies[i].Name == policies[j].Name {
			return policies[i].ID < policies[j].ID
		}
		return policies[i].Name < policies[j].Name
	})
	return policies, nil
}

func (c *Client) CreatePolicy(ctx context.Context, input PolicyRequest) (Policy, error) {
	raw, err := c.do(ctx, http.MethodPost, "/api/policies", input)
	if err != nil {
		return Policy{}, err
	}
	var policy Policy
	if err := decodeWithFallback(raw, &policy, "policy", "data", "result"); err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func (c *Client) UpdatePolicy(ctx context.Context, policyID string, input PolicyRequest) (Policy, error) {
	policyID = strings.TrimSpace(policyID)
	if policyID == "" {
		return Policy{}, fmt.Errorf("policy id is required")
	}
	raw, err := c.do(ctx, http.MethodPut, "/api/policies/"+policyID, input)
	if err != nil {
		return Policy{}, err
	}
	var policy Policy
	if err := decodeWithFallback(raw, &policy, "policy", "data", "result"); err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func (c *Client) DeletePolicy(ctx context.Context, policyID string) error {
	policyID = strings.TrimSpace(policyID)
	if policyID == "" {
		return fmt.Errorf("policy id is required")
	}
	_, err := c.do(ctx, http.MethodDelete, "/api/policies/"+policyID, nil)
	return err
}

func (c *Client) do(ctx context.Context, method, path string, payload any) ([]byte, error) {
	if strings.TrimSpace(c.token) == "" {
		return nil, ErrMissingToken
	}

	var bodyReader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal netbird payload: %w", err)
		}
		bodyReader = bytes.NewReader(body)
	}

	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create netbird request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token "+c.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("netbird api request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read netbird response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("netbird api error (%s %s) status=%d: %s", method, path, resp.StatusCode, describeNetBirdError(raw))
	}

	if len(bytes.TrimSpace(raw)) == 0 {
		return []byte("{}"), nil
	}
	return raw, nil
}

func decodeWithFallback[T any](raw []byte, out *T, keys ...string) error {
	if err := json.Unmarshal(raw, out); err == nil {
		return nil
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode netbird response: %w", err)
	}

	candidates := make([]json.RawMessage, 0, len(keys)+len(envelope)+8)
	seenKeys := map[string]struct{}{}
	appendKeyCandidate := func(rawKey string) {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			return
		}
		if _, seen := seenKeys[key]; seen {
			return
		}
		value, ok := envelope[key]
		if !ok {
			return
		}
		seenKeys[key] = struct{}{}
		candidates = append(candidates, value)
	}

	for _, key := range keys {
		appendKeyCandidate(key)
	}
	for _, fallbackKey := range []string{"result", "results", "data", "items"} {
		appendKeyCandidate(fallbackKey)
	}
	availableKeys := make([]string, 0, len(envelope))
	for key := range envelope {
		availableKeys = append(availableKeys, key)
	}
	sort.Strings(availableKeys)
	for _, key := range availableKeys {
		appendKeyCandidate(key)
	}

	for _, candidate := range candidates {
		if len(bytes.TrimSpace(candidate)) == 0 {
			continue
		}
		if err := json.Unmarshal(candidate, out); err == nil {
			return nil
		}

		var nested map[string]json.RawMessage
		if err := json.Unmarshal(candidate, &nested); err != nil {
			continue
		}
		for _, key := range keys {
			if value, ok := nested[key]; ok {
				if err := json.Unmarshal(value, out); err == nil {
					return nil
				}
			}
		}
		for _, fallbackKey := range []string{"result", "results", "data", "items"} {
			if value, ok := nested[fallbackKey]; ok {
				if err := json.Unmarshal(value, out); err == nil {
					return nil
				}
			}
		}
		nestedKeys := make([]string, 0, len(nested))
		for key := range nested {
			nestedKeys = append(nestedKeys, key)
		}
		sort.Strings(nestedKeys)
		for _, key := range nestedKeys {
			if value, ok := nested[key]; ok {
				if err := json.Unmarshal(value, out); err == nil {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("decode netbird response: no compatible payload for keys %v (available keys: %v)", keys, availableKeys)
}

func describeNetBirdError(raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "empty response body"
	}

	var asMap map[string]any
	if err := json.Unmarshal(trimmed, &asMap); err != nil {
		return compactBody(raw)
	}

	parts := make([]string, 0, 4)
	for _, key := range []string{"message", "error", "detail"} {
		if value, ok := asMap[key]; ok {
			text := strings.TrimSpace(fmt.Sprintf("%v", value))
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
	if value, ok := asMap["code"]; ok {
		parts = append(parts, fmt.Sprintf("code=%v", value))
	}
	if len(parts) == 0 {
		return compactBody(raw)
	}
	return strings.Join(parts, "; ")
}

func compactBody(raw []byte) string {
	const maxLen = 600
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "<empty>"
	}
	if len(trimmed) > maxLen {
		return trimmed[:maxLen] + "..."
	}
	return trimmed
}

func decodeStringReferences(raw json.RawMessage) ([]string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return []string{}, nil
	}

	var values []string
	if err := json.Unmarshal(trimmed, &values); err == nil {
		return normalizeStringSlice(values), nil
	}

	var refs []referenceObject
	if err := json.Unmarshal(trimmed, &refs); err == nil {
		values = make([]string, 0, len(refs))
		for _, ref := range refs {
			values = append(values, strings.TrimSpace(ref.ID))
		}
		return normalizeStringSlice(values), nil
	}

	var value string
	if err := json.Unmarshal(trimmed, &value); err == nil {
		return normalizeStringSlice([]string{value}), nil
	}

	return nil, fmt.Errorf("unsupported reference payload: %s", compactBody(trimmed))
}

func decodePorts(raw json.RawMessage) ([]string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return []string{}, nil
	}

	var asStrings []string
	if err := json.Unmarshal(trimmed, &asStrings); err == nil {
		return normalizeStringSlice(asStrings), nil
	}

	var asInts []int
	if err := json.Unmarshal(trimmed, &asInts); err == nil {
		values := make([]string, 0, len(asInts))
		for _, item := range asInts {
			values = append(values, strconv.Itoa(item))
		}
		return normalizeStringSlice(values), nil
	}

	var mixed []any
	if err := json.Unmarshal(trimmed, &mixed); err == nil {
		values := make([]string, 0, len(mixed))
		for _, item := range mixed {
			switch typed := item.(type) {
			case string:
				values = append(values, typed)
			case float64:
				values = append(values, strconv.FormatFloat(typed, 'f', -1, 64))
			}
		}
		return normalizeStringSlice(values), nil
	}

	return nil, fmt.Errorf("unsupported ports payload: %s", compactBody(trimmed))
}

func normalizeStringSlice(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}
