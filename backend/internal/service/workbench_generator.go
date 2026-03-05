package service

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go-notes/internal/errs"
	"gopkg.in/yaml.v3"
)

type workbenchComposeGenerationModel struct {
	snapshot        WorkbenchStackSnapshot
	services        []WorkbenchComposeService
	dependencies    map[string][]string
	ports           map[string][]WorkbenchComposePort
	resources       map[string]WorkbenchComposeResource
	networkRefs     map[string][]string
	topLevelNetwork []string
}

type workbenchHostBinding struct {
	serviceName string
	hostIP      string
	hostPort    string
	protocol    string
}

func (s *WorkbenchService) GenerateComposeFromStoredSnapshot(
	ctx context.Context,
	projectName string,
) (WorkbenchStackSnapshot, string, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchStackSnapshot{}, "", err
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, "", err
	}
	defer release()

	snapshot, exists, err := s.loadStoredWorkbenchSnapshot(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, "", err
	}
	if !exists {
		return WorkbenchStackSnapshot{}, "", errs.WithDetails(
			errs.New(errs.CodeWorkbenchSourceNotFound, fmt.Sprintf("workbench snapshot not found for project %q", normalizedProject)),
			map[string]any{
				"project": normalizedProject,
			},
		)
	}

	compose, err := generateWorkbenchCompose(snapshot)
	if err != nil {
		return WorkbenchStackSnapshot{}, "", err
	}

	return snapshot, compose, nil
}

func generateWorkbenchCompose(snapshot WorkbenchStackSnapshot) (string, error) {
	model, err := buildWorkbenchComposeGenerationModel(snapshot)
	if err != nil {
		return "", err
	}

	root := workbenchYAMLMappingNode()
	servicesNode := workbenchYAMLMappingNode()
	for _, service := range model.services {
		serviceNode := workbenchYAMLMappingNode()

		if image := strings.TrimSpace(service.Image); image != "" {
			workbenchYAMLAddMapEntry(serviceNode, "image", workbenchYAMLScalarNode(image))
		}
		if buildSource := strings.TrimSpace(service.BuildSource); buildSource != "" {
			workbenchYAMLAddMapEntry(serviceNode, "build", workbenchYAMLScalarNode(buildSource))
		}
		if restartPolicy := strings.TrimSpace(service.RestartPolicy); restartPolicy != "" {
			workbenchYAMLAddMapEntry(serviceNode, "restart", workbenchYAMLScalarNode(restartPolicy))
		}

		if deps := model.dependencies[service.ServiceName]; len(deps) > 0 {
			depSequence := workbenchYAMLSequenceNode()
			for _, dependency := range deps {
				depSequence.Content = append(depSequence.Content, workbenchYAMLScalarNode(dependency))
			}
			workbenchYAMLAddMapEntry(serviceNode, "depends_on", depSequence)
		}

		if ports := model.ports[service.ServiceName]; len(ports) > 0 {
			portSequence := workbenchYAMLSequenceNode()
			for _, port := range ports {
				portSequence.Content = append(portSequence.Content, workbenchYAMLScalarNode(formatWorkbenchComposePort(port)))
			}
			workbenchYAMLAddMapEntry(serviceNode, "ports", portSequence)
		}

		if resource, ok := model.resources[service.ServiceName]; ok {
			deployNode := workbenchYAMLMappingNode()
			resourcesNode := workbenchYAMLMappingNode()

			limitsNode := workbenchYAMLMappingNode()
			if cpus := strings.TrimSpace(resource.LimitCPUs); cpus != "" {
				workbenchYAMLAddMapEntry(limitsNode, "cpus", workbenchYAMLScalarNode(cpus))
			}
			if memory := strings.TrimSpace(resource.LimitMemory); memory != "" {
				workbenchYAMLAddMapEntry(limitsNode, "memory", workbenchYAMLScalarNode(memory))
			}
			if len(limitsNode.Content) > 0 {
				workbenchYAMLAddMapEntry(resourcesNode, "limits", limitsNode)
			}

			reservationsNode := workbenchYAMLMappingNode()
			if cpus := strings.TrimSpace(resource.ReservationCPUs); cpus != "" {
				workbenchYAMLAddMapEntry(reservationsNode, "cpus", workbenchYAMLScalarNode(cpus))
			}
			if memory := strings.TrimSpace(resource.ReservationMemory); memory != "" {
				workbenchYAMLAddMapEntry(reservationsNode, "memory", workbenchYAMLScalarNode(memory))
			}
			if len(reservationsNode.Content) > 0 {
				workbenchYAMLAddMapEntry(resourcesNode, "reservations", reservationsNode)
			}

			if len(resourcesNode.Content) > 0 {
				workbenchYAMLAddMapEntry(deployNode, "resources", resourcesNode)
			}
			if len(deployNode.Content) > 0 {
				workbenchYAMLAddMapEntry(serviceNode, "deploy", deployNode)
			}
		}

		if networks := model.networkRefs[service.ServiceName]; len(networks) > 0 {
			networkSequence := workbenchYAMLSequenceNode()
			for _, networkName := range networks {
				networkSequence.Content = append(networkSequence.Content, workbenchYAMLScalarNode(networkName))
			}
			workbenchYAMLAddMapEntry(serviceNode, "networks", networkSequence)
		}

		workbenchYAMLAddMapEntry(servicesNode, service.ServiceName, serviceNode)
	}
	workbenchYAMLAddMapEntry(root, "services", servicesNode)

	if len(model.topLevelNetwork) > 0 {
		networksNode := workbenchYAMLMappingNode()
		for _, networkName := range model.topLevelNetwork {
			workbenchYAMLAddMapEntry(networksNode, networkName, workbenchYAMLMappingNode())
		}
		workbenchYAMLAddMapEntry(root, "networks", networksNode)
	}

	encoded, err := encodeWorkbenchComposeYAML(root)
	if err != nil {
		return "", workbenchComposeGenerateError(
			model.snapshot,
			"failed to encode generated compose yaml",
			err,
		)
	}

	return encoded, nil
}

func buildWorkbenchComposeGenerationModel(snapshot WorkbenchStackSnapshot) (workbenchComposeGenerationModel, error) {
	normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
	model := workbenchComposeGenerationModel{
		snapshot:     normalizedSnapshot,
		services:     []WorkbenchComposeService{},
		dependencies: make(map[string][]string),
		ports:        make(map[string][]WorkbenchComposePort),
		resources:    make(map[string]WorkbenchComposeResource),
		networkRefs:  make(map[string][]string),
	}

	issues := []string{}
	serviceNames := make(map[string]struct{}, len(normalizedSnapshot.Services))
	for _, service := range normalizedSnapshot.Services {
		name := strings.TrimSpace(service.ServiceName)
		if name == "" {
			issues = append(issues, "service name is required")
			continue
		}
		if _, exists := serviceNames[name]; exists {
			issues = append(issues, fmt.Sprintf("duplicate service definition %q", name))
			continue
		}
		serviceNames[name] = struct{}{}

		normalizedService := WorkbenchComposeService{
			ServiceName:   name,
			Image:         strings.TrimSpace(service.Image),
			BuildSource:   strings.TrimSpace(service.BuildSource),
			RestartPolicy: strings.TrimSpace(service.RestartPolicy),
		}
		if normalizedService.Image == "" && normalizedService.BuildSource == "" {
			issues = append(issues, fmt.Sprintf("service %q must define image or build source", name))
		}
		model.services = append(model.services, normalizedService)
	}

	dependencySet := make(map[string]struct{})
	for _, dependency := range normalizedSnapshot.Dependencies {
		serviceName := strings.TrimSpace(dependency.ServiceName)
		dependsOn := strings.TrimSpace(dependency.DependsOn)
		if serviceName == "" || dependsOn == "" {
			issues = append(issues, "dependency entries must define serviceName and dependsOn")
			continue
		}
		if _, exists := serviceNames[serviceName]; !exists {
			issues = append(issues, fmt.Sprintf("dependency references unknown service %q", serviceName))
			continue
		}
		if _, exists := serviceNames[dependsOn]; !exists {
			issues = append(issues, fmt.Sprintf("dependency %q -> %q references unknown target service", serviceName, dependsOn))
			continue
		}

		key := serviceName + "|" + dependsOn
		if _, exists := dependencySet[key]; exists {
			continue
		}
		dependencySet[key] = struct{}{}
		model.dependencies[serviceName] = append(model.dependencies[serviceName], dependsOn)
	}

	portSet := make(map[string]struct{})
	hostBindings := []workbenchHostBinding{}
	for _, port := range normalizedSnapshot.Ports {
		serviceName := strings.TrimSpace(port.ServiceName)
		if _, exists := serviceNames[serviceName]; !exists {
			issues = append(issues, fmt.Sprintf("port entry references unknown service %q", serviceName))
			continue
		}
		if port.ContainerPort < 1 || port.ContainerPort > 65535 {
			issues = append(issues, fmt.Sprintf("service %q has invalid containerPort %d", serviceName, port.ContainerPort))
			continue
		}
		if port.HostPort != nil && (*port.HostPort < 1 || *port.HostPort > 65535) {
			issues = append(issues, fmt.Sprintf("service %q has invalid hostPort %d", serviceName, *port.HostPort))
			continue
		}

		normalizedPort := WorkbenchComposePort{
			ServiceName:   serviceName,
			ContainerPort: port.ContainerPort,
			HostPort:      port.HostPort,
			HostPortRaw:   strings.TrimSpace(port.HostPortRaw),
			Protocol:      strings.ToLower(strings.TrimSpace(port.Protocol)),
			HostIP:        normalizeHostIP(strings.TrimSpace(port.HostIP)),
		}
		if normalizedPort.Protocol == "" {
			normalizedPort.Protocol = "tcp"
		}

		hostPortValue := normalizedPort.HostPortRaw
		if normalizedPort.HostPort != nil {
			hostPortValue = strconv.Itoa(*normalizedPort.HostPort)
		}
		portKey := fmt.Sprintf(
			"%s|%d|%s|%s|%s",
			serviceName,
			normalizedPort.ContainerPort,
			normalizedPort.Protocol,
			normalizedPort.HostIP,
			hostPortValue,
		)
		if _, exists := portSet[portKey]; exists {
			issues = append(issues, fmt.Sprintf(
				"duplicate port mapping for service %q (container=%d host=%q protocol=%s hostIP=%q)",
				serviceName,
				normalizedPort.ContainerPort,
				hostPortValue,
				normalizedPort.Protocol,
				normalizedPort.HostIP,
			))
			continue
		}
		portSet[portKey] = struct{}{}

		if hostPortValue != "" {
			current := workbenchHostBinding{
				serviceName: serviceName,
				hostIP:      normalizedPort.HostIP,
				hostPort:    hostPortValue,
				protocol:    normalizedPort.Protocol,
			}
			for _, existing := range hostBindings {
				if !workbenchHostBindingConflicts(existing, current) {
					continue
				}
				issues = append(issues, fmt.Sprintf(
					"host port conflict between services %q and %q (protocol=%s hostPort=%q hostIPs=%q/%q)",
					existing.serviceName,
					current.serviceName,
					current.protocol,
					current.hostPort,
					existing.hostIP,
					current.hostIP,
				))
				break
			}
			hostBindings = append(hostBindings, current)
		}

		model.ports[serviceName] = append(model.ports[serviceName], normalizedPort)
	}

	for _, resource := range normalizedSnapshot.Resources {
		serviceName := strings.TrimSpace(resource.ServiceName)
		if _, exists := serviceNames[serviceName]; !exists {
			issues = append(issues, fmt.Sprintf("resource entry references unknown service %q", serviceName))
			continue
		}
		if _, exists := model.resources[serviceName]; exists {
			issues = append(issues, fmt.Sprintf("duplicate resource entry for service %q", serviceName))
			continue
		}
		normalizedResource := WorkbenchComposeResource{
			ServiceName:       serviceName,
			LimitCPUs:         strings.TrimSpace(resource.LimitCPUs),
			LimitMemory:       strings.TrimSpace(resource.LimitMemory),
			ReservationCPUs:   strings.TrimSpace(resource.ReservationCPUs),
			ReservationMemory: strings.TrimSpace(resource.ReservationMemory),
		}
		model.resources[serviceName] = normalizedResource
	}

	networkSet := make(map[string]struct{})
	perServiceNetworkSet := make(map[string]struct{})
	for _, networkRef := range normalizedSnapshot.NetworkRefs {
		serviceName := strings.TrimSpace(networkRef.ServiceName)
		networkName := strings.TrimSpace(networkRef.NetworkName)

		if _, exists := serviceNames[serviceName]; !exists {
			issues = append(issues, fmt.Sprintf("network ref references unknown service %q", serviceName))
			continue
		}
		if networkName == "" {
			issues = append(issues, fmt.Sprintf("service %q has an empty network reference", serviceName))
			continue
		}

		serviceKey := serviceName + "|" + networkName
		if _, exists := perServiceNetworkSet[serviceKey]; !exists {
			perServiceNetworkSet[serviceKey] = struct{}{}
			model.networkRefs[serviceName] = append(model.networkRefs[serviceName], networkName)
		}
		if _, exists := networkSet[networkName]; !exists {
			networkSet[networkName] = struct{}{}
			model.topLevelNetwork = append(model.topLevelNetwork, networkName)
		}
	}
	sort.Strings(model.topLevelNetwork)

	for _, volumeRef := range normalizedSnapshot.VolumeRefs {
		serviceName := strings.TrimSpace(volumeRef.ServiceName)
		volumeName := strings.TrimSpace(volumeRef.VolumeName)
		if serviceName == "" || volumeName == "" {
			issues = append(issues, "volume refs must include serviceName and volumeName")
			continue
		}
		issues = append(issues, fmt.Sprintf("service %q volume %q is not yet supported by generator baseline", serviceName, volumeName))
	}

	for _, module := range normalizedSnapshot.Modules {
		moduleType := strings.TrimSpace(module.ModuleType)
		serviceName := strings.TrimSpace(module.ServiceName)
		issues = append(issues, fmt.Sprintf("module %q for service %q is not yet supported by generator baseline", moduleType, serviceName))
	}

	if len(issues) > 0 {
		return workbenchComposeGenerationModel{}, workbenchComposeValidationError(normalizedSnapshot, issues)
	}

	return model, nil
}

func formatWorkbenchComposePort(port WorkbenchComposePort) string {
	container := strconv.Itoa(port.ContainerPort)
	hostPort := ""
	if port.HostPort != nil {
		hostPort = strconv.Itoa(*port.HostPort)
	} else if raw := strings.TrimSpace(port.HostPortRaw); raw != "" {
		hostPort = raw
	}

	value := container
	if hostPort != "" {
		hostSpec := hostPort
		if hostIP := strings.TrimSpace(port.HostIP); hostIP != "" {
			hostSpec = hostIP + ":" + hostSpec
		}
		value = hostSpec + ":" + container
	}

	protocol := strings.ToLower(strings.TrimSpace(port.Protocol))
	if protocol == "" {
		protocol = "tcp"
	}
	if protocol != "tcp" {
		value += "/" + protocol
	}

	return value
}

func encodeWorkbenchComposeYAML(root *yaml.Node) (string, error) {
	var buffer bytes.Buffer
	document := &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{root},
	}

	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}

	out := buffer.String()
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}

func workbenchComposeValidationError(snapshot WorkbenchStackSnapshot, issues []string) error {
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchValidationFailed, "invalid workbench snapshot for compose generation"),
		map[string]any{
			"project":           strings.TrimSpace(snapshot.ProjectName),
			"composePath":       strings.TrimSpace(snapshot.ComposePath),
			"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
			"revision":          snapshot.Revision,
			"issues":            append([]string{}, issues...),
		},
	)
}

func workbenchComposeGenerateError(snapshot WorkbenchStackSnapshot, message string, cause error) error {
	details := map[string]any{
		"project":           strings.TrimSpace(snapshot.ProjectName),
		"composePath":       strings.TrimSpace(snapshot.ComposePath),
		"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
		"revision":          snapshot.Revision,
	}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	return errs.WithDetails(errs.Wrap(errs.CodeWorkbenchGenerateFailed, message, cause), details)
}

func workbenchYAMLScalarNode(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
}

func workbenchYAMLMappingNode() *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
}

func workbenchYAMLSequenceNode() *yaml.Node {
	return &yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  "!!seq",
	}
}

func workbenchYAMLAddMapEntry(mapping *yaml.Node, key string, value *yaml.Node) {
	if mapping == nil || value == nil {
		return
	}
	mapping.Content = append(mapping.Content, workbenchYAMLScalarNode(key), value)
}

func workbenchHostBindingConflicts(left, right workbenchHostBinding) bool {
	if strings.TrimSpace(left.protocol) != strings.TrimSpace(right.protocol) {
		return false
	}
	if strings.TrimSpace(left.hostPort) != strings.TrimSpace(right.hostPort) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(left.serviceName), strings.TrimSpace(right.serviceName)) &&
		strings.EqualFold(strings.TrimSpace(left.hostIP), strings.TrimSpace(right.hostIP)) {
		return true
	}

	leftIP := strings.TrimSpace(left.hostIP)
	rightIP := strings.TrimSpace(right.hostIP)
	if leftIP == rightIP {
		return true
	}

	return workbenchIsWildcardHostIP(leftIP) || workbenchIsWildcardHostIP(rightIP)
}

func workbenchIsWildcardHostIP(hostIP string) bool {
	normalized := strings.ToLower(strings.TrimSpace(hostIP))
	return normalized == "" || normalized == "0.0.0.0" || normalized == "::"
}
