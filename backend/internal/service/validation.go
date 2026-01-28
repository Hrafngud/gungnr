package service

import (
	"regexp"

	"go-notes/internal/errs"
)

var (
	projectNameRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,61}[a-z0-9])?$`)
	subdomainRe   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	domainRe      = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
)

func ValidateProjectName(name string) error {
	if name == "" {
		return errs.New(errs.CodeValidationName, "project name is required")
	}
	if !projectNameRe.MatchString(name) {
		return errs.New(errs.CodeValidationName, "project name must be lowercase alphanumerics or dashes")
	}
	return nil
}

func ValidateServiceName(name string) error {
	if name == "" {
		return errs.New(errs.CodeValidationName, "service name is required")
	}
	if !projectNameRe.MatchString(name) {
		return errs.New(errs.CodeValidationName, "service name must be lowercase alphanumerics or dashes")
	}
	return nil
}

func ValidateSubdomain(subdomain string) error {
	if subdomain == "" {
		return errs.New(errs.CodeValidationSubdomain, "subdomain is required")
	}
	if !subdomainRe.MatchString(subdomain) {
		return errs.New(errs.CodeValidationSubdomain, "subdomain must be lowercase alphanumerics or dashes")
	}
	return nil
}

func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return errs.New(errs.CodeValidationPort, "port must be between 1 and 65535")
	}
	return nil
}

func ValidateDomain(domain string) error {
	if domain == "" {
		return errs.New(errs.CodeValidationDomain, "domain is required")
	}
	if !domainRe.MatchString(domain) {
		return errs.New(errs.CodeValidationDomain, "domain must be a valid hostname")
	}
	return nil
}
