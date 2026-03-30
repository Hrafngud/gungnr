package validation

import (
	"regexp"
	"strings"

	"go-notes/internal/errs"
	"go-notes/internal/models"
)

var (
	projectNameRe   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,61}[a-z0-9])?$`)
	subdomainRe     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	domainRe        = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
	safeRefRe       = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	containerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
)

// ProjectName validates a project name: lowercase alphanumerics or dashes, 3-63 chars.
func ProjectName(name string) error {
	if name == "" {
		return errs.New(errs.CodeValidationName, "project name is required")
	}
	if !projectNameRe.MatchString(name) {
		return errs.New(errs.CodeValidationName, "project name must be lowercase alphanumerics or dashes")
	}
	return nil
}

// ServiceName validates a service name with the same rules as project names.
func ServiceName(name string) error {
	if name == "" {
		return errs.New(errs.CodeValidationName, "service name is required")
	}
	if !projectNameRe.MatchString(name) {
		return errs.New(errs.CodeValidationName, "service name must be lowercase alphanumerics or dashes")
	}
	return nil
}

// Subdomain validates a DNS subdomain label.
func Subdomain(subdomain string) error {
	if subdomain == "" {
		return errs.New(errs.CodeValidationSubdomain, "subdomain is required")
	}
	if !subdomainRe.MatchString(subdomain) {
		return errs.New(errs.CodeValidationSubdomain, "subdomain must be lowercase alphanumerics or dashes")
	}
	return nil
}

// Port validates a TCP/UDP port number.
func Port(port int) error {
	if port < 1 || port > 65535 {
		return errs.New(errs.CodeValidationPort, "port must be between 1 and 65535")
	}
	return nil
}

// Domain validates a fully qualified domain name.
func Domain(domain string) error {
	if domain == "" {
		return errs.New(errs.CodeValidationDomain, "domain is required")
	}
	if !domainRe.MatchString(domain) {
		return errs.New(errs.CodeValidationDomain, "domain must be a valid hostname")
	}
	return nil
}

// ContainerRef validates a container reference used in API requests.
// Checks for non-empty and safe characters (alphanumeric, underscore, dot, dash).
func ContainerRef(name string) error {
	if name == "" {
		return errs.New(errs.CodeContainerName, "container is required")
	}
	if !safeRefRe.MatchString(name) {
		return errs.New(errs.CodeContainerName, "invalid container name")
	}
	return nil
}

// ProjectRef validates a project reference used in host API requests.
// Checks for non-empty, no path traversal, and safe characters.
func ProjectRef(name string) error {
	if name == "" {
		return errs.New(errs.CodeHostInvalidProject, "project is required")
	}
	if name == "." || name == ".." {
		return errs.New(errs.CodeHostInvalidProject, "invalid project name")
	}
	if !safeRefRe.MatchString(name) {
		return errs.New(errs.CodeHostInvalidProject, "invalid project name")
	}
	return nil
}

// ContainerName validates a Docker container name (stricter than ContainerRef).
func ContainerName(name string) error {
	if !containerNameRe.MatchString(name) {
		return errs.New(errs.CodeContainerName, "container name must use letters, numbers, '.', '_' or '-'")
	}
	return nil
}

// UserRole validates a user role assignment. Only "admin" and "user" are assignable.
func UserRole(role string) error {
	normalized := strings.ToLower(strings.TrimSpace(role))
	if normalized != models.RoleAdmin && normalized != models.RoleUser {
		return errs.New(errs.CodeUserInvalidRole, "role must be admin or user")
	}
	return nil
}

// ArchiveOptions validates project archive option constraints.
func ArchiveOptions(removeContainers, removeVolumes bool) error {
	if !removeContainers && removeVolumes {
		return errs.New(errs.CodeProjectInvalidBody, "removeVolumes requires removeContainers=true")
	}
	return nil
}

// NetBirdModeApplyFields validates required fields for a NetBird mode apply request
// after settings resolution. targetMode is the parsed mode, isLegacy indicates
// whether the target is the legacy mode (which has fewer required fields).
func NetBirdModeApplyFields(apiToken, hostPeerID string, adminPeerIDs []string, isLegacy bool) error {
	if apiToken == "" {
		return errs.New(errs.CodeNetBirdInvalidBody, "apiToken is required; save NetBird mode config first or provide apiToken in request")
	}
	if !isLegacy {
		if hostPeerID == "" {
			return errs.New(errs.CodeNetBirdInvalidBody, "hostPeerId is required for this mode")
		}
		if len(adminPeerIDs) == 0 {
			return errs.New(errs.CodeNetBirdInvalidBody, "adminPeerIds is required for this mode")
		}
	}
	return nil
}

// IsSafeRef checks if a value contains only safe reference characters.
func IsSafeRef(value string) bool {
	return safeRefRe.MatchString(value)
}
