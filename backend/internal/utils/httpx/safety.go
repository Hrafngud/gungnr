package httpx

import "regexp"

var safeRefPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

func IsSafeRef(value string) bool {
	return safeRefPattern.MatchString(value)
}
