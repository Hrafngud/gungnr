package sanitize

import "strings"

// String trims whitespace.
func String(s string) string {
	return strings.TrimSpace(s)
}

// Lower trims and lowercases a string.
func Lower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Role normalizes a role string.
func Role(role string) string {
	return Lower(role)
}

// ProjectName normalizes a project name.
func ProjectName(name string) string {
	return Lower(name)
}

// Subdomain normalizes a subdomain.
func Subdomain(s string) string {
	return Lower(s)
}

// ContainerRef trims whitespace from a container reference.
func ContainerRef(name string) string {
	return String(name)
}

// UserLogin trims and strips leading '@' from a GitHub login.
func UserLogin(login string) string {
	return strings.TrimPrefix(String(login), "@")
}
