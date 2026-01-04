package service

import (
	"fmt"
	"regexp"
)

var (
	projectNameRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,61}[a-z0-9])?$`)
	subdomainRe   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
)

func ValidateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	if !projectNameRe.MatchString(name) {
		return fmt.Errorf("project name must be lowercase alphanumerics or dashes")
	}
	return nil
}

func ValidateServiceName(name string) error {
	if name == "" {
		return fmt.Errorf("service name is required")
	}
	if !projectNameRe.MatchString(name) {
		return fmt.Errorf("service name must be lowercase alphanumerics or dashes")
	}
	return nil
}

func ValidateSubdomain(subdomain string) error {
	if subdomain == "" {
		return fmt.Errorf("subdomain is required")
	}
	if !subdomainRe.MatchString(subdomain) {
		return fmt.Errorf("subdomain must be lowercase alphanumerics or dashes")
	}
	return nil
}

func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}
