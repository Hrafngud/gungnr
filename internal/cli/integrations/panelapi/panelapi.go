package panelapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
}

type APIError struct {
	StatusCode int    `json:"statusCode"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	ErrorText  string `json:"error"`
}

func (e *APIError) Error() string {
	if e == nil {
		return "api request failed"
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = strings.TrimSpace(e.ErrorText)
	}
	if message == "" {
		message = "api request failed"
	}
	code := strings.TrimSpace(e.Code)
	if code == "" {
		return message
	}
	return fmt.Sprintf("%s (%s)", message, code)
}

type TestTokenResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"tokenType"`
}

type NetBirdOperation struct {
	Operation string `json:"operation"`
}

type NetBirdServiceRebindingOperation struct {
	Service     string `json:"service"`
	ProjectName string `json:"projectName"`
}

type NetBirdRedeployTargets struct {
	Panel    bool                        `json:"panel"`
	Projects []NetBirdRedeployTargetInfo `json:"projects"`
}

type NetBirdRedeployTargetInfo struct {
	ProjectName string `json:"projectName"`
}

type NetBirdModePlan struct {
	CurrentMode                string                             `json:"currentMode"`
	TargetMode                 string                             `json:"targetMode"`
	AllowLocalhost             bool                               `json:"allowLocalhost"`
	GroupOperations            []NetBirdOperation                 `json:"groupOperations"`
	PolicyOperations           []NetBirdOperation                 `json:"policyOperations"`
	ServiceRebindingOperations []NetBirdServiceRebindingOperation `json:"serviceRebindingOperations"`
	RedeployTargets            NetBirdRedeployTargets             `json:"redeployTargets"`
	Warnings                   []string                           `json:"warnings"`
}

type NetBirdModeApplyRequest struct {
	TargetMode     string   `json:"targetMode"`
	AllowLocalhost bool     `json:"allowLocalhost"`
	APIBaseURL     string   `json:"apiBaseUrl,omitempty"`
	APIToken       string   `json:"apiToken"`
	HostPeerID     string   `json:"hostPeerId,omitempty"`
	AdminPeerIDs   []string `json:"adminPeerIds,omitempty"`
}

type NetBirdStatus struct {
	ClientInstalled       bool                   `json:"clientInstalled"`
	DaemonRunning         bool                   `json:"daemonRunning"`
	Connected             bool                   `json:"connected"`
	PeerID                string                 `json:"peerId,omitempty"`
	PeerName              string                 `json:"peerName,omitempty"`
	WG0IP                 string                 `json:"wg0Ip,omitempty"`
	CurrentMode           string                 `json:"currentMode"`
	LastPolicySyncAt      *time.Time             `json:"lastPolicySyncAt,omitempty"`
	LastPolicySyncStatus  string                 `json:"lastPolicySyncStatus"`
	LastPolicySyncWarning int                    `json:"lastPolicySyncWarnings"`
	APIReachable          bool                   `json:"apiReachable"`
	APIReachability       NetBirdAPIReachability `json:"apiReachability"`
	ManagedGroups         int                    `json:"managedGroups"`
	ManagedPolicies       int                    `json:"managedPolicies"`
	Warnings              []string               `json:"warnings"`
}

type NetBirdAPIReachability struct {
	Source  string `json:"source"`
	Message string `json:"message,omitempty"`
}

type Job struct {
	ID     uint64 `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

type JobDetail struct {
	Job
	LogLines []string `json:"logLines"`
}

func NewClient(baseURL string) (*Client, error) {
	normalized, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}

	return &Client{
		baseURL: normalized,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

func NormalizeBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("panel api base URL is required")
	}
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "http://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid panel api base URL: %w", err)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("invalid panel api base URL: host is required")
	}

	parsed.Path = strings.TrimSpace(parsed.Path)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	base := strings.TrimRight(parsed.String(), "/")
	base = strings.TrimSuffix(base, "/api/v1")
	return base, nil
}

func (c *Client) SetAuthToken(token string) {
	if c == nil {
		return
	}
	c.authToken = strings.TrimSpace(token)
}

func (c *Client) IssueTestToken(ctx context.Context, login, password string) (TestTokenResponse, error) {
	request := map[string]string{
		"login":    strings.TrimSpace(login),
		"password": strings.TrimSpace(password),
	}
	var response TestTokenResponse
	if err := c.requestJSON(ctx, http.MethodPost, "/test-token", request, &response, false); err != nil {
		return TestTokenResponse{}, err
	}
	return response, nil
}

func (c *Client) PlanNetBirdMode(ctx context.Context, targetMode string, allowLocalhost bool) (NetBirdModePlan, error) {
	request := map[string]any{
		"targetMode":     strings.TrimSpace(targetMode),
		"allowLocalhost": allowLocalhost,
	}

	var response struct {
		Plan NetBirdModePlan `json:"plan"`
	}
	if err := c.requestJSON(ctx, http.MethodPost, "/api/v1/netbird/mode/plan", request, &response, true); err != nil {
		return NetBirdModePlan{}, err
	}
	return response.Plan, nil
}

func (c *Client) ApplyNetBirdMode(ctx context.Context, request NetBirdModeApplyRequest) (Job, error) {
	var response struct {
		Job Job `json:"job"`
	}
	if err := c.requestJSON(ctx, http.MethodPost, "/api/v1/netbird/mode/apply", request, &response, true); err != nil {
		return Job{}, err
	}
	return response.Job, nil
}

func (c *Client) GetNetBirdStatus(ctx context.Context) (NetBirdStatus, error) {
	var response struct {
		Status NetBirdStatus `json:"status"`
	}
	if err := c.requestJSON(ctx, http.MethodGet, "/api/v1/netbird/status", nil, &response, true); err != nil {
		return NetBirdStatus{}, err
	}
	return response.Status, nil
}

func (c *Client) GetJob(ctx context.Context, id uint64) (JobDetail, error) {
	var response JobDetail
	path := fmt.Sprintf("/api/v1/jobs/%d", id)
	if err := c.requestJSON(ctx, http.MethodGet, path, nil, &response, true); err != nil {
		return JobDetail{}, err
	}
	return response, nil
}

func (c *Client) requestJSON(ctx context.Context, method, path string, requestBody any, responseBody any, withAuth bool) error {
	if c == nil {
		return fmt.Errorf("panel api client is required")
	}

	urlValue := strings.TrimRight(c.baseURL, "/") + path
	var bodyReader io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode request payload: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlValue, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withAuth {
		token := strings.TrimSpace(c.authToken)
		if token == "" {
			return fmt.Errorf("panel auth token is required")
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, payload)
	}
	if responseBody == nil {
		return nil
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, responseBody); err != nil {
		return fmt.Errorf("decode response payload: %w", err)
	}
	return nil
}

func parseAPIError(statusCode int, payload []byte) error {
	apiErr := &APIError{StatusCode: statusCode}
	trimmed := strings.TrimSpace(string(payload))
	if trimmed != "" {
		_ = json.Unmarshal(payload, apiErr)
	}
	if strings.TrimSpace(apiErr.Message) == "" && strings.TrimSpace(apiErr.ErrorText) == "" {
		apiErr.Message = fmt.Sprintf("request failed with status %d", statusCode)
	}
	return apiErr
}
