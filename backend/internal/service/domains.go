package service

import (
	"strings"

	"go-notes/internal/errs"
)

func normalizeDomain(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func normalizeDomainList(input []string) []string {
	seen := make(map[string]bool)
	output := make([]string, 0, len(input))
	for _, entry := range input {
		normalized := normalizeDomain(entry)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		output = append(output, normalized)
	}
	return output
}

func selectDomain(requested, base string, additional []string) (string, error) {
	normalizedBase := normalizeDomain(base)
	normalizedRequested := normalizeDomain(requested)
	if normalizedRequested == "" {
		if normalizedBase == "" {
			return "", errBaseDomainUnset()
		}
		return normalizedBase, nil
	}
	if err := ValidateDomain(normalizedRequested); err != nil {
		return "", err
	}
	if normalizedRequested == normalizedBase {
		return normalizedBase, nil
	}
	for _, domain := range additional {
		if normalizedRequested == normalizeDomain(domain) {
			return normalizedRequested, nil
		}
	}
	return "", errDomainNotConfigured(normalizedRequested)
}

func errBaseDomainUnset() error {
	return errs.New(errs.CodeDomainMissing, "base domain is not configured")
}

func errDomainNotConfigured(domain string) error {
	return errs.New(errs.CodeDomainNotConfigured, "domain is not configured: "+domain)
}
