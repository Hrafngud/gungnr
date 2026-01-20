package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type AccessToken struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type User struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

func RequestDeviceCode(clientID string) (*DeviceCode, error) {
	form := map[string]string{
		"client_id": clientID,
		"scope":     "read:user",
	}

	var payload DeviceCode
	if err := postFormJSON("https://github.com/login/device/code", form, &payload); err != nil {
		return nil, err
	}

	if payload.DeviceCode == "" || payload.UserCode == "" || payload.VerificationURI == "" {
		return nil, errors.New("received incomplete device authorization response from GitHub")
	}

	if payload.Interval == 0 {
		payload.Interval = 5
	}

	return &payload, nil
}

func PollAccessToken(clientID string, deviceCode *DeviceCode) (*AccessToken, error) {
	interval := deviceCode.Interval
	if interval <= 0 {
		interval = 5
	}
	deadline := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return nil, errors.New("device code expired before authorization completed")
		}

		time.Sleep(time.Duration(interval) * time.Second)

		token, err := requestAccessToken(clientID, deviceCode.DeviceCode)
		if err != nil {
			return nil, err
		}

		if token.Error == "" && token.AccessToken != "" {
			return token, nil
		}

		switch token.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5
			continue
		case "access_denied":
			return nil, errors.New("device authorization denied in GitHub")
		case "expired_token":
			return nil, errors.New("device code expired before authorization completed")
		default:
			if token.Error != "" {
				if token.ErrorDescription != "" {
					return nil, fmt.Errorf("device authorization failed: %s", token.ErrorDescription)
				}
				return nil, fmt.Errorf("device authorization failed: %s", token.Error)
			}
		}
	}
}

func FetchUser(token string) (*User, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch GitHub user: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read GitHub user response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub user lookup failed: %s", strings.TrimSpace(string(body)))
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("unable to decode GitHub user response: %w", err)
	}

	if user.Login == "" || user.ID == 0 {
		return nil, errors.New("GitHub user response missing login or id")
	}

	return &user, nil
}

func requestAccessToken(clientID, deviceCode string) (*AccessToken, error) {
	form := map[string]string{
		"client_id":   clientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	}

	var payload AccessToken
	if err := postFormJSON("https://github.com/login/oauth/access_token", form, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

func postFormJSON(endpoint string, form map[string]string, out interface{}) error {
	values := url.Values{}
	for key, value := range form {
		values.Set(key, value)
	}
	body := values.Encode()

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read response from %s: %w", endpoint, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request to %s failed: %s", endpoint, strings.TrimSpace(string(payload)))
	}

	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("unable to decode response from %s: %w", endpoint, err)
	}

	return nil
}
