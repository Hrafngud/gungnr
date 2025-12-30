package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go-notes/internal/config"
)

var (
	ErrMissingToken     = errors.New("CLOUDFLARE_API_TOKEN is required")
	ErrMissingAccountID = errors.New("CLOUDFLARE_ACCOUNT_ID is required")
	ErrMissingZoneID    = errors.New("CLOUDFLARE_ZONE_ID is required")
	ErrMissingTunnel    = errors.New("CLOUDFLARED_TUNNEL_NAME is required")
	ErrMissingHostname  = errors.New("hostname is required")
)

const apiBaseURL = "https://api.cloudflare.com/client/v4"

type Client struct {
	cfg    config.Config
	client *http.Client
}

type TunnelStatus struct {
	ID          string
	Name        string
	Status      string
	Connections int
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		cfg: cfg,
		client: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (c *Client) EnsureDNS(ctx context.Context, hostname string) error {
	if strings.TrimSpace(hostname) == "" {
		return ErrMissingHostname
	}
	if err := c.ensureAuth(); err != nil {
		return err
	}
	if strings.TrimSpace(c.cfg.CloudflareZoneID) == "" {
		return ErrMissingZoneID
	}

	tunnelID, err := c.resolveTunnelID(ctx)
	if err != nil {
		return err
	}

	record, err := c.findDNSRecord(ctx, hostname)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("%s.cfargotunnel.com", tunnelID)
	if record != nil {
		if record.Content == content && record.Proxied {
			return nil
		}
		return c.updateDNSRecord(ctx, record.ID, dnsRecordRequest{
			Type:    "CNAME",
			Name:    hostname,
			Content: content,
			Proxied: true,
		})
	}

	return c.createDNSRecord(ctx, dnsRecordRequest{
		Type:    "CNAME",
		Name:    hostname,
		Content: content,
		Proxied: true,
	})
}

func (c *Client) UpdateIngress(ctx context.Context, hostname string, port int) error {
	if strings.TrimSpace(hostname) == "" {
		return ErrMissingHostname
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	if err := c.ensureAuth(); err != nil {
		return err
	}

	tunnelID, err := c.resolveTunnelID(ctx)
	if err != nil {
		return err
	}

	config, err := c.getTunnelConfig(ctx, tunnelID)
	if err != nil {
		return err
	}

	service := fmt.Sprintf("http://localhost:%d", port)
	config.Ingress = ensureIngressRule(config.Ingress, hostname, service)

	return c.updateTunnelConfig(ctx, tunnelID, config)
}

func (c *Client) TunnelStatus(ctx context.Context) (TunnelStatus, error) {
	if err := c.ensureAuth(); err != nil {
		return TunnelStatus{}, err
	}

	tunnelID, err := c.resolveTunnelID(ctx)
	if err != nil {
		return TunnelStatus{}, err
	}

	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s", c.cfg.CloudflareAccountID, tunnelID)
	var result tunnelInfo
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return TunnelStatus{}, err
	}

	return TunnelStatus{
		ID:          result.ID,
		Name:        result.Name,
		Status:      result.Status,
		Connections: len(result.Connections),
	}, nil
}

func (c *Client) ensureAuth() error {
	if strings.TrimSpace(c.cfg.CloudflareAPIToken) == "" {
		return ErrMissingToken
	}
	if strings.TrimSpace(c.cfg.CloudflareAccountID) == "" {
		return ErrMissingAccountID
	}
	if strings.TrimSpace(c.cfg.CloudflaredTunnel) == "" {
		return ErrMissingTunnel
	}
	return nil
}

func (c *Client) resolveTunnelID(ctx context.Context) (string, error) {
	raw := strings.TrimSpace(c.cfg.CloudflaredTunnel)
	if raw == "" {
		return "", ErrMissingTunnel
	}
	if looksLikeUUID(raw) {
		return raw, nil
	}

	path := fmt.Sprintf("/accounts/%s/cfd_tunnel", c.cfg.CloudflareAccountID)
	query := url.Values{}
	query.Set("name", raw)
	path = path + "?" + query.Encode()

	var result []tunnelInfo
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return "", err
	}
	for _, tunnel := range result {
		if strings.EqualFold(tunnel.Name, raw) {
			return tunnel.ID, nil
		}
	}
	return "", fmt.Errorf("tunnel %q not found in Cloudflare account", raw)
}

type tunnelConfig struct {
	Ingress []map[string]any
	Raw     map[string]any
}

func (c *Client) getTunnelConfig(ctx context.Context, tunnelID string) (tunnelConfig, error) {
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/configurations", c.cfg.CloudflareAccountID, tunnelID)
	var result map[string]any
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return tunnelConfig{}, err
	}

	config := tunnelConfig{
		Raw:     map[string]any{},
		Ingress: []map[string]any{},
	}

	rawConfig, ok := result["config"].(map[string]any)
	if ok {
		config.Raw = rawConfig
		if ingress, ok := rawConfig["ingress"]; ok {
			config.Ingress = coerceIngress(ingress)
		}
	}

	return config, nil
}

func (c *Client) updateTunnelConfig(ctx context.Context, tunnelID string, config tunnelConfig) error {
	payload := map[string]any{}
	if config.Raw != nil {
		payload = config.Raw
	}
	payload["ingress"] = config.Ingress

	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/configurations", c.cfg.CloudflareAccountID, tunnelID)
	body := map[string]any{"config": payload}
	return c.do(ctx, http.MethodPut, path, body, nil)
}

type dnsRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

type dnsRecordRequest struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

func (c *Client) findDNSRecord(ctx context.Context, hostname string) (*dnsRecord, error) {
	query := url.Values{}
	query.Set("type", "CNAME")
	query.Set("name", hostname)

	path := fmt.Sprintf("/zones/%s/dns_records?%s", c.cfg.CloudflareZoneID, query.Encode())
	var result []dnsRecord
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	return &result[0], nil
}

func (c *Client) createDNSRecord(ctx context.Context, req dnsRecordRequest) error {
	path := fmt.Sprintf("/zones/%s/dns_records", c.cfg.CloudflareZoneID)
	return c.do(ctx, http.MethodPost, path, req, nil)
}

func (c *Client) updateDNSRecord(ctx context.Context, id string, req dnsRecordRequest) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", c.cfg.CloudflareZoneID, id)
	return c.do(ctx, http.MethodPut, path, req, nil)
}

type tunnelInfo struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Status      string        `json:"status"`
	Connections []interface{} `json:"connections"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type apiResponse[T any] struct {
	Success bool       `json:"success"`
	Errors  []apiError `json:"errors"`
	Result  T          `json:"result"`
}

func (c *Client) do(ctx context.Context, method, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	if payload == nil {
		body = nil
	}

	req, err := http.NewRequestWithContext(ctx, method, apiBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.CloudflareAPIToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare api request failed: %w", err)
	}
	defer resp.Body.Close()

	var wrapper apiResponse[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return fmt.Errorf("decode cloudflare response: %w", err)
	}

	if !wrapper.Success {
		return fmt.Errorf("cloudflare api error: %s", describeErrors(wrapper.Errors))
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(wrapper.Result, out); err != nil {
		return fmt.Errorf("decode cloudflare result: %w", err)
	}

	return nil
}

func describeErrors(errors []apiError) string {
	if len(errors) == 0 {
		return "unknown error"
	}
	first := errors[0]
	if first.Code == 0 {
		return first.Message
	}
	return fmt.Sprintf("%s (code %d)", first.Message, first.Code)
}

func coerceIngress(value any) []map[string]any {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	var result []map[string]any
	for _, entry := range list {
		if rule, ok := entry.(map[string]any); ok {
			result = append(result, rule)
		}
	}
	return result
}

func ensureIngressRule(existing []map[string]any, hostname, service string) []map[string]any {
	var rules []map[string]any
	var catchAll map[string]any
	found := false

	for _, rule := range existing {
		if isCatchAll(rule) {
			if catchAll == nil {
				catchAll = rule
			}
			continue
		}
		if host, ok := rule["hostname"].(string); ok && strings.EqualFold(host, hostname) {
			rule["service"] = service
			found = true
		}
		rules = append(rules, rule)
	}

	if !found {
		rules = append(rules, map[string]any{
			"hostname":      hostname,
			"service":       service,
			"originRequest": map[string]any{},
		})
	}

	if catchAll == nil {
		catchAll = map[string]any{"service": "http_status:404"}
	}
	rules = append(rules, catchAll)
	return rules
}

func isCatchAll(rule map[string]any) bool {
	if rule == nil {
		return false
	}
	if _, ok := rule["hostname"]; ok {
		return false
	}
	if _, ok := rule["path"]; ok {
		return false
	}
	service, ok := rule["service"].(string)
	return ok && strings.Contains(service, "http_status:404")
}

func looksLikeUUID(value string) bool {
	normalized := strings.TrimSpace(value)
	if len(normalized) != 36 {
		return false
	}
	for i, ch := range normalized {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		case ch >= 'A' && ch <= 'F':
		case ch == '-':
		default:
			return false
		}
		if (i == 8 || i == 13 || i == 18 || i == 23) && ch != '-' {
			return false
		}
	}
	return true
}
