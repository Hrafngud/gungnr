package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func WorkbenchSourceFingerprint(raw []byte) (normalized string, fingerprint string) {
	normalized = normalizeWorkbenchSourceForFingerprint(raw)
	sum := sha256.Sum256([]byte(normalized))
	fingerprint = "sha256:" + hex.EncodeToString(sum[:])
	return normalized, fingerprint
}

func normalizeWorkbenchSourceForFingerprint(raw []byte) string {
	normalized := string(raw)
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.TrimPrefix(normalized, "\uFEFF")

	lines := strings.Split(normalized, "\n")
	lines = trimTrailingBlankLines(lines)
	for len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last != "---" && last != "..." {
			break
		}
		lines = lines[:len(lines)-1]
		lines = trimTrailingBlankLines(lines)
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func trimTrailingBlankLines(lines []string) []string {
	idx := len(lines) - 1
	for idx >= 0 && strings.TrimSpace(lines[idx]) == "" {
		idx--
	}
	if idx < 0 {
		return nil
	}
	return lines[:idx+1]
}
