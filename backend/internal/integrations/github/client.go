package github

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	gogithub "github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

var ErrMissingToken = errors.New("github app installation token is required")

const responseBodyHeader = "X-Warp-Response-Body"

type Client struct {
	api *gogithub.Client
}

type RepoAccessDiagnostics struct {
	Checked   bool   `json:"checked"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func NewTokenClient(token string) *Client {
	trimmed := strings.TrimSpace(token)
	var api *gogithub.Client
	if trimmed != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: trimmed})
		httpClient := WrapHTTPClient(oauth2.NewClient(context.Background(), ts))
		api = gogithub.NewClient(httpClient)
	}
	return &Client{api: api}
}

func (c *Client) CreateRepoFromTemplate(ctx context.Context, templateOwner, templateRepo, name, targetOwner string, private bool) (*gogithub.Repository, error) {
	if c.api == nil {
		return nil, ErrMissingToken
	}
	templateOwner = strings.TrimSpace(templateOwner)
	templateRepo = normalizeRepoName(templateRepo)
	if templateOwner == "" || templateRepo == "" {
		return nil, errors.New("template owner and repo are required")
	}

	owner := strings.TrimSpace(targetOwner)
	if owner == "" {
		owner = strings.TrimSpace(templateOwner)
	}

	req := &gogithub.TemplateRepoRequest{
		Name:    gogithub.String(name),
		Owner:   gogithub.String(owner),
		Private: gogithub.Bool(private),
	}

	repo, _, err := c.api.Repositories.CreateFromTemplate(ctx, templateOwner, templateRepo, req)
	if err != nil {
		detail := FormatError(err)
		if detail == "" {
			return nil, fmt.Errorf("create repo from template: %w", err)
		}
		return nil, fmt.Errorf("create repo from template: %w; %s", err, detail)
	}

	return repo, nil
}

func (c *Client) ValidateTemplateRepo(ctx context.Context, templateOwner, templateRepo string) error {
	if c.api == nil {
		return ErrMissingToken
	}
	templateOwner = strings.TrimSpace(templateOwner)
	templateRepo = normalizeRepoName(templateRepo)
	if templateOwner == "" || templateRepo == "" {
		return errors.New("template owner and repo are required")
	}

	repo, _, err := c.api.Repositories.Get(ctx, templateOwner, templateRepo)
	if err != nil {
		guidance := ""
		if isNotFoundError(err) {
			guidance = fmt.Sprintf("github app installation cannot access %s/%s; install the app on the repo owner or update the installation ID", templateOwner, templateRepo)
		}
		detail := FormatError(err)
		base := "template repo lookup failed"
		if guidance != "" {
			base = fmt.Sprintf("%s: %s", base, guidance)
		}
		if detail == "" {
			return fmt.Errorf("%s: %w", base, err)
		}
		return fmt.Errorf("%s: %w; %s", base, err, detail)
	}
	if repo == nil {
		return fmt.Errorf("template repo lookup failed: empty response for %s/%s", templateOwner, templateRepo)
	}
	if !repo.GetIsTemplate() {
		return fmt.Errorf("github repo %s/%s is not marked as a template; enable \"Template repository\" in the repo settings", templateOwner, templateRepo)
	}
	return nil
}

func (c *Client) CheckRepoAccess(ctx context.Context, templateOwner, templateRepo string) RepoAccessDiagnostics {
	if c.api == nil {
		return RepoAccessDiagnostics{
			Checked: false,
			Error:   ErrMissingToken.Error(),
		}
	}
	templateOwner = strings.TrimSpace(templateOwner)
	templateRepo = normalizeRepoName(templateRepo)
	if templateOwner == "" || templateRepo == "" {
		return RepoAccessDiagnostics{
			Checked: false,
			Error:   "template owner and repo are required",
		}
	}

	repo, resp, err := c.api.Repositories.Get(ctx, templateOwner, templateRepo)
	diagnostics := RepoAccessDiagnostics{
		Checked: true,
	}
	if diagnostics.RequestID == "" {
		diagnostics.RequestID = requestIDFromResponse(resp.Response)
	}
	if err != nil {
		diagnostics.Available = false
		detail := FormatError(err)
		if detail == "" {
			diagnostics.Error = err.Error()
		} else {
			diagnostics.Error = detail
		}
		if diagnostics.RequestID == "" {
			diagnostics.RequestID = requestIDFromError(err)
		}
		return diagnostics
	}
	if repo == nil {
		diagnostics.Available = false
		diagnostics.Error = fmt.Sprintf("template repo lookup failed: empty response for %s/%s", templateOwner, templateRepo)
		return diagnostics
	}

	diagnostics.Available = true
	return diagnostics
}

func FormatError(err error) string {
	return formatGitHubError(err)
}

func formatGitHubError(err error) string {
	switch typed := err.(type) {
	case *gogithub.RateLimitError:
		return fmt.Sprintf("github rate limit exceeded: %s%s", strings.TrimSpace(typed.Message), formatGitHubResponseMeta(typed.Response))
	case *gogithub.AbuseRateLimitError:
		retry := ""
		if typed.RetryAfter != nil && *typed.RetryAfter > 0 {
			retry = fmt.Sprintf(" retry_after=%s", typed.RetryAfter.Round(time.Second))
		}
		return fmt.Sprintf("github abuse detection triggered: %s%s%s", strings.TrimSpace(typed.Message), retry, formatGitHubResponseMeta(typed.Response))
	case *gogithub.AcceptedError:
		return "github request accepted but still processing"
	case *gogithub.ErrorResponse:
		message := strings.TrimSpace(typed.Message)
		if message == "" {
			message = "github api error"
		}
		errs := formatGitHubErrors(typed.Errors)
		docs := ""
		if typed.DocumentationURL != "" {
			docs = fmt.Sprintf(" docs=%s", typed.DocumentationURL)
		}
		return fmt.Sprintf("%s%s%s%s", message, errs, docs, formatGitHubResponseMeta(typed.Response))
	default:
		return ""
	}
}

func formatGitHubErrors(errors []gogithub.Error) string {
	if len(errors) == 0 {
		return ""
	}
	parts := make([]string, 0, len(errors))
	for i, entry := range errors {
		if i >= 3 {
			parts = append(parts, fmt.Sprintf("and %d more", len(errors)-i))
			break
		}
		fragment := strings.TrimSpace(entry.Message)
		if fragment == "" {
			fragment = entry.Code
		}
		if entry.Resource != "" || entry.Field != "" {
			fragment = fmt.Sprintf("%s (%s.%s)", fragment, entry.Resource, entry.Field)
		}
		parts = append(parts, fragment)
	}
	return fmt.Sprintf("; errors=%s", strings.Join(parts, "; "))
}

func formatGitHubResponseMeta(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	status := strings.TrimSpace(resp.Status)
	if status == "" {
		status = fmt.Sprintf("%d", resp.StatusCode)
	}
	requestID := strings.TrimSpace(resp.Header.Get("X-GitHub-Request-Id"))
	if requestID == "" {
		requestID = strings.TrimSpace(resp.Header.Get("X-Request-Id"))
	}
	meta := fmt.Sprintf(" status=%s", status)
	if requestID != "" {
		meta = fmt.Sprintf("%s request_id=%s", meta, requestID)
	}
	body := strings.TrimSpace(resp.Header.Get(responseBodyHeader))
	if body != "" {
		meta = fmt.Sprintf("%s response=%s", meta, body)
	}
	return fmt.Sprintf(" (%s)", strings.TrimSpace(meta))
}

func requestIDFromResponse(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	requestID := strings.TrimSpace(resp.Header.Get("X-GitHub-Request-Id"))
	if requestID == "" {
		requestID = strings.TrimSpace(resp.Header.Get("X-Request-Id"))
	}
	return requestID
}

func requestIDFromError(err error) string {
	switch typed := err.(type) {
	case *gogithub.RateLimitError:
		return requestIDFromResponse(typed.Response)
	case *gogithub.AbuseRateLimitError:
		return requestIDFromResponse(typed.Response)
	case *gogithub.AcceptedError:
		return ""
	case *gogithub.ErrorResponse:
		return requestIDFromResponse(typed.Response)
	default:
		return ""
	}
}

func isNotFoundError(err error) bool {
	typed, ok := err.(*gogithub.ErrorResponse)
	if !ok || typed.Response == nil {
		return false
	}
	return typed.Response.StatusCode == http.StatusNotFound
}

func WrapHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		return nil
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	if _, ok := base.(*responseCaptureTransport); ok {
		return client
	}
	client.Transport = &responseCaptureTransport{base: base}
	return client
}

type responseCaptureTransport struct {
	base http.RoundTripper
}

func (t *responseCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	if resp.StatusCode < http.StatusBadRequest {
		return resp, err
	}
	raw, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	if readErr == nil {
		body := compactBodySnippet(raw)
		if body != "" {
			resp.Header.Set(responseBodyHeader, body)
		}
	}
	return resp, err
}

func compactBodySnippet(raw []byte) string {
	const maxLen = 600
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\r", " ")
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	if len(trimmed) > maxLen {
		return trimmed[:maxLen] + "..."
	}
	return trimmed
}

func normalizeRepoName(repo string) string {
	trimmed := strings.TrimSpace(repo)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, ".git") {
		trimmed = strings.TrimSpace(trimmed[:len(trimmed)-4])
	}
	return trimmed
}
