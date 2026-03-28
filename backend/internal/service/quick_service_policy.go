package service

import (
	"fmt"
	"strings"

	"go-notes/internal/infra/contract"
)

const quickServiceManagedNetworkSuffix = "_quick_internal"

func normalizeQuickServiceExposureRequest(raw string, requestedPort int) (string, error) {
	mode := contract.NormalizeQuickServiceExposureMode(raw)
	if mode != "" {
		return mode, nil
	}
	if strings.TrimSpace(raw) != "" {
		return "", fmt.Errorf("exposureMode must be internal or host_published")
	}
	if requestedPort > 0 {
		return contract.QuickServiceExposureHostPublished, nil
	}
	return contract.QuickServiceExposureInternal, nil
}

func NormalizeQuickServiceExposureMode(raw string, requestedPort int) (string, error) {
	return normalizeQuickServiceExposureRequest(raw, requestedPort)
}

func quickServiceRequiresPublishedPort(mode string) bool {
	return mode == contract.QuickServiceExposureHostPublished
}

func inferQuickServiceNetworkName(containers []DockerContainer) string {
	project := ""
	for _, container := range containers {
		candidate := sanitizeContainerName(container.Project)
		if candidate == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(container.Service)) {
		case "api", "proxy", "web", "db":
			return candidate + quickServiceManagedNetworkSuffix
		default:
			if project == "" {
				project = candidate
			}
		}
	}
	if project != "" {
		return project + quickServiceManagedNetworkSuffix
	}
	return contract.QuickServiceDefaultNetwork
}
