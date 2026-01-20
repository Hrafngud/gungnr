package cloudflare

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Zone struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Account struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"account"`
}

type ZoneResponse struct {
	Success bool       `json:"success"`
	Errors  []APIError `json:"errors"`
	Result  []Zone     `json:"result"`
}

type AccountResponse struct {
	Success bool       `json:"success"`
	Errors  []APIError `json:"errors"`
	Result  struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"result"`
}

type DNSRecordResponse struct {
	Success bool       `json:"success"`
	Errors  []APIError `json:"errors"`
	Result  []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"result"`
}

func FetchZone(token, baseDomain string) (*Zone, error) {
	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s&status=active", url.QueryEscape(baseDomain))
	body, err := apiRequest(token, endpoint)
	if err != nil {
		return nil, err
	}

	var payload ZoneResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("unable to decode Cloudflare zone response: %w", err)
	}

	if !payload.Success {
		return nil, fmt.Errorf("Cloudflare zone lookup failed: %s", errorsToString(payload.Errors))
	}

	var match *Zone
	for i := range payload.Result {
		if strings.EqualFold(payload.Result[i].Name, baseDomain) {
			match = &payload.Result[i]
			break
		}
	}

	if match == nil {
		return nil, fmt.Errorf("no active zone found for %s", baseDomain)
	}
	if match.Account.ID == "" {
		return nil, errors.New("zone response missing account id")
	}

	return match, nil
}

func VerifyAccountAccess(token, accountID string) (string, error) {
	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s", url.PathEscape(accountID))
	body, err := apiRequest(token, endpoint)
	if err != nil {
		return "", err
	}

	var payload AccountResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("unable to decode Cloudflare account response: %w", err)
	}
	if !payload.Success {
		return "", fmt.Errorf("Cloudflare account lookup failed: %s", errorsToString(payload.Errors))
	}
	if payload.Result.ID == "" {
		return "", errors.New("Cloudflare account response missing id")
	}

	return payload.Result.Name, nil
}

func VerifyDNSRecord(token, zoneID, hostname string) error {
	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s", url.PathEscape(zoneID), url.QueryEscape(hostname))
	body, err := apiRequest(token, endpoint)
	if err != nil {
		return err
	}

	var payload DNSRecordResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("unable to decode Cloudflare DNS response: %w", err)
	}
	if !payload.Success {
		return fmt.Errorf("Cloudflare DNS lookup failed: %s", errorsToString(payload.Errors))
	}
	if len(payload.Result) == 0 {
		return fmt.Errorf("DNS record for %s not found after routing", hostname)
	}

	return nil
}

func apiRequest(token, endpoint string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cloudflare request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read Cloudflare response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Cloudflare request failed: %s", strings.TrimSpace(string(body)))
	}

	return body, nil
}

func errorsToString(errs []APIError) string {
	if len(errs) == 0 {
		return "unknown error"
	}
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err.Code != 0 {
			parts = append(parts, fmt.Sprintf("%d: %s", err.Code, err.Message))
		} else if err.Message != "" {
			parts = append(parts, err.Message)
		}
	}
	if len(parts) == 0 {
		return "unknown error"
	}
	return strings.Join(parts, "; ")
}
