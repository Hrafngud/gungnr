package service

import (
	"fmt"
	"strings"

	"go-notes/internal/errs"
)

// Select a published HTTP ingress mapping without assuming its container port.
// Service identity takes precedence over port numbers; equally likely mappings
// must be resolved explicitly rather than forwarding to an arbitrary service.
func workbenchSelectIngressPort(snapshot WorkbenchStackSnapshot) (WorkbenchPortSelector, error) {
	snapshot = normalizeWorkbenchStackSnapshot(snapshot)
	services := make(map[string]WorkbenchComposeService, len(snapshot.Services))
	for _, service := range snapshot.Services {
		services[service.ServiceName] = service
	}

	bestPriority := 5
	candidates := []WorkbenchPortSelector{}
	for _, port := range snapshot.Ports {
		if port.Protocol != "tcp" || port.ContainerPort < 1 || port.ContainerPort > 65535 {
			continue
		}
		priority, eligible := workbenchIngressPortPriority(services[port.ServiceName], port)
		if !eligible || priority > bestPriority {
			continue
		}
		if priority < bestPriority {
			bestPriority = priority
			candidates = candidates[:0]
		}
		candidates = append(candidates, WorkbenchPortSelector{
			ServiceName: port.ServiceName, ContainerPort: port.ContainerPort,
			Protocol: port.Protocol, HostIP: port.HostIP,
		})
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	// Prefer a single conventional HTTP mapping when an ingress also publishes
	// HTTPS or administrative ports, preserving existing port-80 templates.
	var httpCandidates []WorkbenchPortSelector
	for _, candidate := range candidates {
		if candidate.ContainerPort == 80 {
			httpCandidates = append(httpCandidates, candidate)
		}
	}
	if len(httpCandidates) == 1 {
		return httpCandidates[0], nil
	}

	code := "WB-JOB-INGRESS-PORT-MISSING"
	message := "template has no eligible published TCP ingress port"
	if len(candidates) > 0 {
		code = "WB-JOB-INGRESS-PORT-AMBIGUOUS"
		labels := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			labels = append(labels, fmt.Sprintf("%s:%d/%s", candidate.ServiceName, candidate.ContainerPort, candidate.Protocol))
		}
		message = "template has multiple possible ingress mappings: " + strings.Join(labels, ", ")
	}
	return WorkbenchPortSelector{}, errs.WithDetails(
		errs.New(errs.CodeWorkbenchValidationFailed, message),
		map[string]any{
			"project": snapshot.ProjectName, "composePath": snapshot.ComposePath,
			"issueCount": 1, "candidates": candidates,
			"issues": []WorkbenchValidationIssue{{
				Class: workbenchValidationClassSchema, Code: code, Path: "$.ports", Message: message,
			}},
		},
	)
}

func workbenchIngressPortPriority(service WorkbenchComposeService, port WorkbenchComposePort) (int, bool) {
	name := strings.ToLower(strings.TrimSpace(port.ServiceName))
	switch name {
	case "proxy", "ingress", "gateway", "edge", "nginx":
		return 0, true
	}
	if profilePort, known := workbenchServiceProfilePortFromImage(service.Image); known {
		if profilePort == 80 {
			return 1, true
		}
		return 0, false
	}
	if profilePort, known := workbenchServiceProfilePortFromName(name); known {
		if profilePort == 80 {
			return 2, true
		}
		return 0, false
	}
	// Never infer ingress from a lone database/cache/metrics/storage mapping.
	switch port.ContainerPort {
	case 5432, 3306, 6379, 27017, 9090:
		return 0, false
	case 80:
		return 3, true
	default:
		return 4, true
	}
}
