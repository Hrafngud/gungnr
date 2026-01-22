package cloudflare

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	ErrTunnelNotRemote  = errors.New("tunnel is locally managed; remote configuration updates require config_src=cloudflare")
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

type TokenStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type ZoneInfo struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Account ZoneAccount `json:"account"`
}

type ZoneAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
	return c.EnsureDNSForZone(ctx, hostname, strings.TrimSpace(c.cfg.CloudflareZoneID))
}

func (c *Client) EnsureDNSForZone(ctx context.Context, hostname, zoneID string) error {
	if strings.TrimSpace(hostname) == "" {
		return ErrMissingHostname
	}
	if err := c.ensureAuth(); err != nil {
		return err
	}
	if strings.TrimSpace(zoneID) == "" {
		return ErrMissingZoneID
	}

	tunnelID, err := c.resolveTunnelID(ctx)
	if err != nil {
		return err
	}

	record, err := c.selectDNSRecord(ctx, hostname, zoneID)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("%s.cfargotunnel.com", tunnelID)
	if record != nil {
		if record.Content == content && record.Proxied {
			return nil
		}
		return c.updateDNSRecord(ctx, zoneID, record.ID, dnsRecordRequest{
			Type:    "CNAME",
			Name:    hostname,
			Content: content,
			Proxied: true,
		})
	}

	return c.createDNSRecord(ctx, zoneID, dnsRecordRequest{
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

	if err := c.ensureRemoteManaged(ctx, tunnelID); err != nil {
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

func (c *Client) VerifyToken(ctx context.Context) (TokenStatus, error) {
	if err := c.ensureToken(); err != nil {
		return TokenStatus{}, err
	}
	var result TokenStatus
	if err := c.do(ctx, http.MethodGet, "/user/tokens/verify", nil, &result); err != nil {
		return TokenStatus{}, err
	}
	return result, nil
}

func (c *Client) ListZones(ctx context.Context) ([]ZoneInfo, error) {
	if err := c.ensureAuth(); err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("account.id", c.cfg.CloudflareAccountID)
	query.Set("per_page", "200")
	path := fmt.Sprintf("/zones?%s", query.Encode())

	var result []ZoneInfo
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Zone(ctx context.Context, zoneID string) (ZoneInfo, error) {
	if err := c.ensureToken(); err != nil {
		return ZoneInfo{}, err
	}
	if strings.TrimSpace(zoneID) == "" {
		return ZoneInfo{}, ErrMissingZoneID
	}
	path := fmt.Sprintf("/zones/%s", strings.TrimSpace(zoneID))
	var result ZoneInfo
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return ZoneInfo{}, err
	}
	return result, nil
}

func (c *Client) ensureAuth() error {
	if strings.TrimSpace(c.cfg.CloudflareAPIToken) == "" {
		return ErrMissingToken
	}
	if strings.TrimSpace(c.cfg.CloudflareAccountID) == "" {
		return ErrMissingAccountID
	}
	return nil
}

func (c *Client) ensureToken() error {
	if strings.TrimSpace(c.cfg.CloudflareAPIToken) == "" {
		return ErrMissingToken
	}
	return nil
}

func (c *Client) resolveTunnelID(ctx context.Context) (string, error) {
	raw := strings.TrimSpace(c.cfg.CloudflaredTunnel)
	fallbackID := strings.TrimSpace(c.cfg.CloudflareTunnelID)
	if looksLikeUUID(raw) {
		return raw, nil
	}
	configID, configErr := c.tunnelIDFromConfig()
	if looksLikeUUID(configID) {
		return configID, nil
	}
	if raw == "" {
		if looksLikeUUID(fallbackID) {
			return fallbackID, nil
		}
		if configErr != nil {
			return "", fmt.Errorf("cloudflared tunnel name is not set and config read failed: %w", configErr)
		}
		return "", ErrMissingTunnel
	}

	path := fmt.Sprintf("/accounts/%s/cfd_tunnel", c.cfg.CloudflareAccountID)
	query := url.Values{}
	query.Set("name", raw)
	path = path + "?" + query.Encode()

	var result []tunnelInfo
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		if looksLikeUUID(fallbackID) {
			return fallbackID, nil
		}
		return "", err
	}
	for _, tunnel := range result {
		if strings.EqualFold(tunnel.Name, raw) {
			return tunnel.ID, nil
		}
	}
	if looksLikeUUID(fallbackID) {
		return fallbackID, nil
	}
	if configErr != nil {
		return "", fmt.Errorf("tunnel %q not found in Cloudflare account (config read failed: %v)", raw, configErr)
	}
	return "", fmt.Errorf("tunnel %q not found in Cloudflare account", raw)
}

func (c *Client) TunnelIDFromConfig() (string, error) {
	return c.tunnelIDFromConfig()
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

	rawConfig := result
	if nested, ok := result["config"].(map[string]any); ok {
		rawConfig = nested
	}
	if rawConfig != nil {
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
		if originRequest, ok := config.Raw["originRequest"]; ok {
			payload["originRequest"] = originRequest
		} else if originRequest, ok := config.Raw["origin_request"]; ok {
			payload["originRequest"] = originRequest
		}
		if warpRouting, ok := config.Raw["warpRouting"]; ok {
			payload["warpRouting"] = warpRouting
		} else if warpRouting, ok := config.Raw["warp_routing"]; ok {
			payload["warpRouting"] = warpRouting
		}
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

func (c *Client) selectDNSRecord(ctx context.Context, hostname, zoneID string) (*dnsRecord, error) {
	records, err := c.listDNSRecordsByName(ctx, hostname, zoneID)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}

	cnameIndex := -1
	for i, record := range records {
		if strings.EqualFold(record.Type, "CNAME") {
			if cnameIndex != -1 {
				return nil, fmt.Errorf("multiple CNAME records exist for %s; remove duplicates before continuing", hostname)
			}
			cnameIndex = i
		}
	}
	if cnameIndex != -1 {
		return &records[cnameIndex], nil
	}
	if len(records) == 1 {
		return &records[0], nil
	}
	return nil, fmt.Errorf("multiple DNS records exist for %s (%s); remove conflicting records before creating a CNAME", hostname, describeDNSRecords(records))
}

func (c *Client) listDNSRecordsByName(ctx context.Context, hostname, zoneID string) ([]dnsRecord, error) {
	query := url.Values{}
	query.Set("name", hostname)

	path := fmt.Sprintf("/zones/%s/dns_records?%s", zoneID, query.Encode())
	var result []dnsRecord
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) createDNSRecord(ctx context.Context, zoneID string, req dnsRecordRequest) error {
	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)
	return c.do(ctx, http.MethodPost, path, req, nil)
}

func (c *Client) updateDNSRecord(ctx context.Context, zoneID, id string, req dnsRecordRequest) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, id)
	return c.do(ctx, http.MethodPut, path, req, nil)
}

type tunnelInfo struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Status       string        `json:"status"`
	Connections  []interface{} `json:"connections"`
	ConfigSrc    string        `json:"config_src"`
	RemoteConfig *bool         `json:"remote_config"`
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
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.CloudflareAPIToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare api request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read cloudflare response: %w", err)
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	trimmedBody := bytes.TrimSpace(rawBody)
	if len(trimmedBody) == 0 {
		return fmt.Errorf("cloudflare api empty response (%s %s) status=%d%s", method, path, resp.StatusCode, formatCFRay(resp))
	}
	if trimmedBody[0] != '{' {
		return fmt.Errorf("cloudflare api non-json response (%s %s) status=%d%s; content-type=%s; body=%s",
			method,
			path,
			resp.StatusCode,
			formatCFRay(resp),
			contentType,
			compactBody(rawBody),
		)
	}

	var wrapper apiResponse[json.RawMessage]
	if err := json.Unmarshal(rawBody, &wrapper); err != nil {
		return fmt.Errorf("decode cloudflare response (status %d)%s: %w; body=%s", resp.StatusCode, formatCFRay(resp), err, compactBody(rawBody))
	}

	if !wrapper.Success {
		return fmt.Errorf("cloudflare api error (%s %s) status=%d%s: %s%s",
			method,
			path,
			resp.StatusCode,
			formatCFRay(resp),
			describeErrors(wrapper.Errors),
			formatCFBody(rawBody),
		)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("cloudflare api http %d%s%s", resp.StatusCode, formatCFRay(resp), formatCFBody(rawBody))
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
	var parts []string
	for i, entry := range errors {
		if i >= 3 {
			parts = append(parts, fmt.Sprintf("and %d more", len(errors)-i))
			break
		}
		message := entry.Message
		if message == "" {
			message = "cloudflare api error"
		}
		if entry.Code != 0 {
			message = fmt.Sprintf("%s (code %d)", message, entry.Code)
		}
		parts = append(parts, message)
	}
	desc := strings.Join(parts, "; ")
	if errors[0].Code == 10000 || errors[0].Code == 10001 {
		return fmt.Sprintf("%s. Check that the account ID, zone ID, and tunnel name/ID all belong to the same Cloudflare account as the token; 10000 often indicates an account/tunnel mismatch even when the token itself is valid.", desc)
	}
	return desc
}

func describeDNSRecords(records []dnsRecord) string {
	var types []string
	for _, record := range records {
		if record.Type == "" {
			continue
		}
		types = append(types, record.Type)
	}
	if len(types) == 0 {
		return "unknown types"
	}
	return strings.Join(types, ", ")
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

func formatCFBody(raw []byte) string {
	body := compactBody(raw)
	if body == "" || body == "<empty>" {
		return ""
	}
	return fmt.Sprintf("; response=%s", body)
}

func formatCFRay(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	ray := strings.TrimSpace(resp.Header.Get("CF-RAY"))
	if ray == "" {
		return ""
	}
	return fmt.Sprintf(" (cf-ray %s)", ray)
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

func (c *Client) ensureRemoteManaged(ctx context.Context, tunnelID string) error {
	info, err := c.getTunnelInfo(ctx, tunnelID)
	if err != nil {
		return err
	}
	if strings.EqualFold(info.ConfigSrc, "local") {
		return fmt.Errorf("tunnel %s is locally managed (config_src=%s, remote_config=%s); Cloudflare API updates require config_src=cloudflare: %w",
			describeTunnelName(info, tunnelID), describeConfigSrc(info.ConfigSrc), describeRemoteConfig(info.RemoteConfig), ErrTunnelNotRemote)
	}
	if strings.EqualFold(info.ConfigSrc, "cloudflare") {
		return nil
	}
	if info.RemoteConfig != nil && !*info.RemoteConfig {
		return fmt.Errorf("tunnel %s is locally managed (config_src=%s, remote_config=%s); Cloudflare API updates require config_src=cloudflare: %w",
			describeTunnelName(info, tunnelID), describeConfigSrc(info.ConfigSrc), describeRemoteConfig(info.RemoteConfig), ErrTunnelNotRemote)
	}
	return nil
}

func (c *Client) getTunnelInfo(ctx context.Context, tunnelID string) (tunnelInfo, error) {
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s", c.cfg.CloudflareAccountID, tunnelID)
	var result tunnelInfo
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return tunnelInfo{}, err
	}
	return result, nil
}

func describeTunnelName(info tunnelInfo, fallback string) string {
	if strings.TrimSpace(info.Name) != "" {
		return info.Name
	}
	return fallback
}

func describeConfigSrc(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func describeRemoteConfig(value *bool) string {
	if value == nil {
		return "unknown"
	}
	if *value {
		return "true"
	}
	return "false"
}

func (c *Client) tunnelIDFromConfig() (string, error) {
	path := strings.TrimSpace(c.cfg.CloudflaredConfig)
	if path == "" {
		return "", nil
	}
	path = expandUserPath(path)

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var credentialsFile string
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		key, value, ok := parseCloudflaredConfigLine(scanner.Text())
		if !ok {
			continue
		}
		if key == "tunnel" {
			if looksLikeUUID(value) {
				return value, nil
			}
			continue
		}
		if key == "credentials-file" || key == "credential-file" {
			if credentialsFile == "" {
				credentialsFile = expandUserPath(value)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if credentialsFile != "" {
		if id := tunnelIDFromCredentialsFile(credentialsFile); looksLikeUUID(id) {
			return id, nil
		}
		base := strings.TrimSuffix(filepath.Base(credentialsFile), filepath.Ext(credentialsFile))
		if looksLikeUUID(base) {
			return base, nil
		}
	}
	return "", nil
}

func parseCloudflaredConfigLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		return "", "", false
	}
	if hash := strings.Index(value, "#"); hash != -1 {
		value = strings.TrimSpace(value[:hash])
	}
	value = strings.Trim(value, "\"'")
	return key, value, true
}

func expandUserPath(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if trimmed == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return trimmed
		}
		return home
	}
	if strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return trimmed
		}
		return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
	}
	return trimmed
}

func tunnelIDFromCredentialsFile(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	for _, key := range []string{"TunnelID", "tunnel_id", "tunnelID"} {
		if value, ok := payload[key].(string); ok && looksLikeUUID(value) {
			return value
		}
	}
	return ""
}
