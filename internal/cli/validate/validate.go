package validate

import (
	"errors"
	"fmt"
	"strings"
)

func NonEmpty(label, value string) error {
	if strings.TrimSpace(value) == "" {
		if label == "" {
			return errors.New("value is required")
		}
		return fmt.Errorf("%s is required", label)
	}
	return nil
}

func NormalizeDomain(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimSuffix(trimmed, "/")
	return strings.ToLower(trimmed)
}

func Domain(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("value is required")
	}
	if strings.Contains(value, "/") || strings.Contains(value, " ") {
		return errors.New("enter a base domain without paths or spaces")
	}
	if !strings.Contains(value, ".") {
		return errors.New("enter a valid base domain (example.com)")
	}
	return nil
}

func NormalizeCloudflareToken(token string) string {
	value := strings.TrimSpace(token)
	value = strings.Trim(value, "\"'")
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "authorization:") {
		value = strings.TrimSpace(value[len("authorization:"):])
		lower = strings.ToLower(value)
	}
	if strings.HasPrefix(lower, "bearer ") {
		value = strings.TrimSpace(value[len("bearer "):])
	}
	return value
}
