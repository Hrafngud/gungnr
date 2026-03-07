package service

import (
	"fmt"
	"sort"
	"strings"
)

type workbenchOptionalServicePortDefinition struct {
	containerPort int
	protocol      string
	hostIP        string
}

type workbenchOptionalServiceRuntimeDefinition struct {
	image         string
	restartPolicy string
	command       []string
	environment   map[string]string
	ports         []workbenchOptionalServicePortDefinition
}

type workbenchComposeEnvironmentEntry struct {
	Key   string
	Value string
}

type workbenchComposeServiceExtras struct {
	Managed     bool
	Command     []string
	Environment []workbenchComposeEnvironmentEntry
}

type workbenchManagedServiceModel struct {
	EntryKey    string
	ServiceName string
	Service     WorkbenchComposeService
	Ports       []WorkbenchComposePort
	Extras      workbenchComposeServiceExtras
}

type workbenchManagedServiceIssue struct {
	Code     string
	Path     string
	Message  string
	EntryKey string
	Service  string
}

func workbenchBuildManagedServiceModels(snapshot WorkbenchStackSnapshot) ([]workbenchManagedServiceModel, []workbenchManagedServiceIssue) {
	normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
	importedServiceNames := make(map[string]struct{}, len(normalizedSnapshot.Services))
	for _, service := range normalizedSnapshot.Services {
		name := strings.TrimSpace(service.ServiceName)
		if name == "" {
			continue
		}
		importedServiceNames[strings.ToLower(name)] = struct{}{}
	}

	seenEntryKeys := make(map[string]struct{}, len(normalizedSnapshot.ManagedServices))
	seenServiceNames := make(map[string]struct{}, len(normalizedSnapshot.ManagedServices))
	models := make([]workbenchManagedServiceModel, 0, len(normalizedSnapshot.ManagedServices))
	issues := make([]workbenchManagedServiceIssue, 0)

	for idx, managedService := range normalizedSnapshot.ManagedServices {
		path := fmt.Sprintf("$.managedServices[%d]", idx)
		entryKey := strings.ToLower(strings.TrimSpace(managedService.EntryKey))
		serviceName := strings.TrimSpace(managedService.ServiceName)

		if entryKey == "" {
			issues = append(issues, workbenchManagedServiceIssue{
				Code:    "WB-MANAGED-SERVICE-KEY-REQUIRED",
				Path:    path + ".entryKey",
				Message: "managed service entryKey is required",
				Service: serviceName,
			})
			continue
		}
		if serviceName == "" {
			issues = append(issues, workbenchManagedServiceIssue{
				Code:     "WB-MANAGED-SERVICE-NAME-REQUIRED",
				Path:     path + ".serviceName",
				Message:  fmt.Sprintf("managed service %q must define serviceName", entryKey),
				EntryKey: entryKey,
			})
			continue
		}

		definition, ok := workbenchOptionalServiceDefinitionByKey(entryKey)
		if !ok {
			issues = append(issues, workbenchManagedServiceIssue{
				Code:     "WB-MANAGED-SERVICE-KEY-UNSUPPORTED",
				Path:     path + ".entryKey",
				Message:  fmt.Sprintf("managed service entry %q is not supported", entryKey),
				EntryKey: entryKey,
				Service:  serviceName,
			})
			continue
		}

		if _, exists := seenEntryKeys[entryKey]; exists {
			issues = append(issues, workbenchManagedServiceIssue{
				Code:     "WB-MANAGED-SERVICE-ENTRY-DUPLICATE",
				Path:     path + ".entryKey",
				Message:  fmt.Sprintf("managed service entry %q is duplicated in the stored snapshot", entryKey),
				EntryKey: entryKey,
				Service:  serviceName,
			})
			continue
		}
		normalizedServiceName := strings.ToLower(serviceName)
		if _, exists := seenServiceNames[normalizedServiceName]; exists {
			issues = append(issues, workbenchManagedServiceIssue{
				Code:     "WB-MANAGED-SERVICE-NAME-DUPLICATE",
				Path:     path + ".serviceName",
				Message:  fmt.Sprintf("managed service name %q is duplicated in the stored snapshot", serviceName),
				EntryKey: entryKey,
				Service:  serviceName,
			})
			continue
		}
		if _, exists := importedServiceNames[normalizedServiceName]; exists {
			issues = append(issues, workbenchManagedServiceIssue{
				Code:     "WB-MANAGED-SERVICE-NAME-CONFLICT",
				Path:     path + ".serviceName",
				Message:  fmt.Sprintf("managed service name %q conflicts with an imported compose service", serviceName),
				EntryKey: entryKey,
				Service:  serviceName,
			})
			continue
		}

		seenEntryKeys[entryKey] = struct{}{}
		seenServiceNames[normalizedServiceName] = struct{}{}

		models = append(models, workbenchManagedServiceModel{
			EntryKey:    entryKey,
			ServiceName: serviceName,
			Service: WorkbenchComposeService{
				ServiceName:   serviceName,
				Image:         strings.TrimSpace(definition.runtime.image),
				RestartPolicy: strings.TrimSpace(definition.runtime.restartPolicy),
			},
			Ports:  workbenchOptionalServicePorts(definition.runtime.ports, serviceName),
			Extras: workbenchOptionalServiceExtras(definition.runtime),
		})
	}

	sort.SliceStable(models, func(i, j int) bool {
		leftService := strings.ToLower(strings.TrimSpace(models[i].ServiceName))
		rightService := strings.ToLower(strings.TrimSpace(models[j].ServiceName))
		if leftService != rightService {
			return leftService < rightService
		}
		return strings.ToLower(strings.TrimSpace(models[i].EntryKey)) < strings.ToLower(strings.TrimSpace(models[j].EntryKey))
	})
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Code != issues[j].Code {
			return issues[i].Code < issues[j].Code
		}
		if issues[i].Path != issues[j].Path {
			return issues[i].Path < issues[j].Path
		}
		if issues[i].EntryKey != issues[j].EntryKey {
			return issues[i].EntryKey < issues[j].EntryKey
		}
		if issues[i].Service != issues[j].Service {
			return issues[i].Service < issues[j].Service
		}
		return issues[i].Message < issues[j].Message
	})
	return models, issues
}

func workbenchOptionalServicePorts(definitions []workbenchOptionalServicePortDefinition, serviceName string) []WorkbenchComposePort {
	if len(definitions) == 0 {
		return []WorkbenchComposePort{}
	}
	ports := make([]WorkbenchComposePort, 0, len(definitions))
	for _, definition := range definitions {
		protocol := strings.ToLower(strings.TrimSpace(definition.protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		ports = append(ports, WorkbenchComposePort{
			ServiceName:   serviceName,
			ContainerPort: definition.containerPort,
			Protocol:      protocol,
			HostIP:        normalizeHostIP(strings.TrimSpace(definition.hostIP)),
		})
	}
	return ports
}

func workbenchOptionalServiceExtras(definition workbenchOptionalServiceRuntimeDefinition) workbenchComposeServiceExtras {
	extras := workbenchComposeServiceExtras{
		Managed: true,
	}
	if len(definition.command) > 0 {
		extras.Command = append([]string(nil), definition.command...)
	}
	if len(definition.environment) > 0 {
		keys := make([]string, 0, len(definition.environment))
		for key := range definition.environment {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}
			keys = append(keys, trimmedKey)
		}
		sort.Strings(keys)
		extras.Environment = make([]workbenchComposeEnvironmentEntry, 0, len(keys))
		for _, key := range keys {
			extras.Environment = append(extras.Environment, workbenchComposeEnvironmentEntry{
				Key:   key,
				Value: definition.environment[key],
			})
		}
	}
	return extras
}

func workbenchManagedServiceIssuesToValidation(issues []workbenchManagedServiceIssue) []WorkbenchValidationIssue {
	out := make([]WorkbenchValidationIssue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, WorkbenchValidationIssue{
			Class:   workbenchValidationClassSchema,
			Code:    issue.Code,
			Path:    issue.Path,
			Message: issue.Message,
			Service: issue.Service,
		})
	}
	return out
}

func workbenchManagedServiceIssuesToPortResolution(issues []workbenchManagedServiceIssue) []WorkbenchPortResolutionIssue {
	out := make([]WorkbenchPortResolutionIssue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, WorkbenchPortResolutionIssue{
			Class:   workbenchPortIssueClassSchema,
			Code:    issue.Code,
			Path:    issue.Path,
			Message: issue.Message,
			Service: issue.Service,
		})
	}
	return out
}

func workbenchHasPortMapping(
	ports []WorkbenchComposePort,
	serviceName string,
	containerPort int,
	protocol string,
	hostIP string,
) bool {
	normalizedService := strings.TrimSpace(serviceName)
	normalizedProtocol := strings.ToLower(strings.TrimSpace(protocol))
	if normalizedProtocol == "" {
		normalizedProtocol = "tcp"
	}
	normalizedHostIP := normalizeHostIP(strings.TrimSpace(hostIP))

	for _, port := range ports {
		if strings.TrimSpace(port.ServiceName) != normalizedService {
			continue
		}
		if port.ContainerPort != containerPort {
			continue
		}
		currentProtocol := strings.ToLower(strings.TrimSpace(port.Protocol))
		if currentProtocol == "" {
			currentProtocol = "tcp"
		}
		if currentProtocol != normalizedProtocol {
			continue
		}
		if normalizeHostIP(strings.TrimSpace(port.HostIP)) != normalizedHostIP {
			continue
		}
		return true
	}
	return false
}
