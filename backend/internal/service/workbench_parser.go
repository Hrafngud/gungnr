package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	workbenchWarningPassThrough = "WB-PARSE-PASSTHROUGH"
	workbenchWarningInvalidType = "WB-PARSE-INVALID-TYPE"
	workbenchWarningInvalidPort = "WB-PARSE-INVALID-PORT"
)

type WorkbenchComposeParseResult struct {
	ProjectName       string                       `json:"projectName"`
	ProjectDir        string                       `json:"projectDir"`
	ComposePath       string                       `json:"composePath"`
	SourceFingerprint string                       `json:"sourceFingerprint"`
	Services          []WorkbenchComposeService    `json:"services"`
	Dependencies      []WorkbenchComposeDependency `json:"dependencies"`
	Ports             []WorkbenchComposePort       `json:"ports"`
	Resources         []WorkbenchComposeResource   `json:"resources"`
	NetworkRefs       []WorkbenchComposeNetworkRef `json:"networkRefs"`
	VolumeRefs        []WorkbenchComposeVolumeRef  `json:"volumeRefs"`
	EnvRefs           []WorkbenchComposeEnvRef     `json:"envRefs"`
	Warnings          []WorkbenchComposeWarning    `json:"warnings"`
}

type WorkbenchComposeService struct {
	ServiceName   string `json:"serviceName"`
	Image         string `json:"image,omitempty"`
	BuildSource   string `json:"buildSource,omitempty"`
	RestartPolicy string `json:"restartPolicy,omitempty"`
}

type WorkbenchComposeDependency struct {
	ServiceName string `json:"serviceName"`
	DependsOn   string `json:"dependsOn"`
}

type WorkbenchComposePort struct {
	ServiceName   string `json:"serviceName"`
	ContainerPort int    `json:"containerPort"`
	HostPort      *int   `json:"hostPort,omitempty"`
	HostPortRaw   string `json:"hostPortRaw,omitempty"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIp,omitempty"`
}

type WorkbenchComposeResource struct {
	ServiceName       string `json:"serviceName"`
	LimitCPUs         string `json:"limitCpus,omitempty"`
	LimitMemory       string `json:"limitMemory,omitempty"`
	ReservationCPUs   string `json:"reservationCpus,omitempty"`
	ReservationMemory string `json:"reservationMemory,omitempty"`
}

type WorkbenchComposeNetworkRef struct {
	ServiceName string `json:"serviceName"`
	NetworkName string `json:"networkName"`
}

type WorkbenchComposeVolumeRef struct {
	ServiceName string `json:"serviceName"`
	VolumeName  string `json:"volumeName"`
}

type WorkbenchComposeEnvRef struct {
	ServiceName string `json:"serviceName,omitempty"`
	Path        string `json:"path"`
	Expression  string `json:"expression"`
	Variable    string `json:"variable"`
}

type WorkbenchComposeWarning struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type workbenchComposeCoreParser struct {
	result           WorkbenchComposeParseResult
	envRefSet        map[string]struct{}
	depSet           map[string]struct{}
	networkRefSet    map[string]struct{}
	volumeRefSet     map[string]struct{}
	topLevelNetworks map[string]struct{}
	topLevelVolumes  map[string]struct{}
}

func ParseWorkbenchComposeCore(normalizedSource string) (WorkbenchComposeParseResult, error) {
	parser := workbenchComposeCoreParser{
		envRefSet:        make(map[string]struct{}),
		depSet:           make(map[string]struct{}),
		networkRefSet:    make(map[string]struct{}),
		volumeRefSet:     make(map[string]struct{}),
		topLevelNetworks: make(map[string]struct{}),
		topLevelVolumes:  make(map[string]struct{}),
	}

	if strings.TrimSpace(normalizedSource) == "" {
		return parser.result, nil
	}

	var document yaml.Node
	if err := yaml.Unmarshal([]byte(normalizedSource), &document); err != nil {
		return WorkbenchComposeParseResult{}, err
	}

	root := workbenchDocumentRoot(&document)
	if root == nil {
		return parser.result, nil
	}
	if root.Kind != yaml.MappingNode {
		parser.warn(workbenchWarningInvalidType, "$", "compose source root must be a mapping")
		parser.sort()
		return parser.result, nil
	}

	var servicesNode *yaml.Node
	for idx := 0; idx+1 < len(root.Content); idx += 2 {
		keyNode := root.Content[idx]
		valueNode := root.Content[idx+1]
		if keyNode == nil {
			continue
		}

		key := strings.TrimSpace(keyNode.Value)
		if key == "" {
			parser.warn(workbenchWarningInvalidType, "$", "top-level key is empty")
			continue
		}

		switch key {
		case "services":
			servicesNode = valueNode
		case "networks":
			parser.parseTopLevelReferenceSet("$.networks", valueNode, parser.topLevelNetworks, "network")
		case "volumes":
			parser.parseTopLevelReferenceSet("$.volumes", valueNode, parser.topLevelVolumes, "volume")
		case "version", "name":
		default:
			parser.warnPassThrough(
				"$."+key,
				fmt.Sprintf("top-level key %q is pass-through and not parsed", key),
			)
		}
	}
	if servicesNode != nil {
		parser.parseServices("$.services", servicesNode)
	}

	parser.sort()
	return parser.result, nil
}

func workbenchDocumentRoot(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil
		}
		return node.Content[0]
	}
	return node
}

func (p *workbenchComposeCoreParser) parseServices(path string, node *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		p.warn(workbenchWarningInvalidType, path, "services must be a mapping")
		return
	}

	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		if keyNode == nil {
			continue
		}

		serviceName := strings.TrimSpace(keyNode.Value)
		if serviceName == "" {
			p.warn(workbenchWarningInvalidType, path, "service name is empty")
			continue
		}
		p.parseService(serviceName, path+"."+serviceName, valueNode)
	}
}

func (p *workbenchComposeCoreParser) parseService(serviceName, path string, node *yaml.Node) {
	service := WorkbenchComposeService{ServiceName: serviceName}
	if node == nil || node.Kind != yaml.MappingNode {
		p.warn(workbenchWarningInvalidType, path, "service definition must be a mapping")
		p.result.Services = append(p.result.Services, service)
		return
	}

	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		if keyNode == nil {
			continue
		}

		key := strings.TrimSpace(keyNode.Value)
		fieldPath := path + "." + key
		switch key {
		case "image":
			if valueNode != nil && valueNode.Kind == yaml.ScalarNode {
				service.Image = strings.TrimSpace(valueNode.Value)
				p.collectEnvRefs(serviceName, fieldPath, valueNode.Value)
			} else {
				p.warn(workbenchWarningInvalidType, fieldPath, "image must be a scalar")
			}
		case "build":
			service.BuildSource = p.parseBuildSource(serviceName, fieldPath, valueNode)
		case "restart":
			if valueNode != nil && valueNode.Kind == yaml.ScalarNode {
				service.RestartPolicy = strings.TrimSpace(valueNode.Value)
				p.collectEnvRefs(serviceName, fieldPath, valueNode.Value)
			} else {
				p.warn(workbenchWarningInvalidType, fieldPath, "restart must be a scalar")
			}
		case "depends_on":
			p.parseDependsOn(serviceName, fieldPath, valueNode)
		case "ports":
			p.parsePorts(serviceName, fieldPath, valueNode)
		case "environment":
			p.collectEnvRefsFromEnvironment(serviceName, fieldPath, valueNode)
		case "env_file":
			p.collectEnvRefsFromEnvFile(serviceName, fieldPath, valueNode)
		case "deploy":
			p.parseDeployResources(serviceName, fieldPath, valueNode)
		case "networks":
			p.parseServiceNetworks(serviceName, fieldPath, valueNode)
		case "volumes":
			p.parseServiceVolumes(serviceName, fieldPath, valueNode)
		default:
			p.warnPassThrough(
				fieldPath,
				fmt.Sprintf("service field %q is pass-through and not parsed", key),
			)
		}
	}

	p.result.Services = append(p.result.Services, service)
}

func (p *workbenchComposeCoreParser) parseBuildSource(serviceName, path string, node *yaml.Node) string {
	if node == nil {
		return ""
	}
	switch node.Kind {
	case yaml.ScalarNode:
		p.collectEnvRefs(serviceName, path, node.Value)
		return strings.TrimSpace(node.Value)
	case yaml.MappingNode:
		source := ""
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			keyNode := node.Content[idx]
			valueNode := node.Content[idx+1]
			if keyNode == nil {
				continue
			}
			key := strings.TrimSpace(keyNode.Value)
			keyPath := path + "." + key
			if valueNode != nil && valueNode.Kind == yaml.ScalarNode {
				p.collectEnvRefs(serviceName, keyPath, valueNode.Value)
			}
			switch key {
			case "context":
				if valueNode != nil && valueNode.Kind == yaml.ScalarNode {
					source = strings.TrimSpace(valueNode.Value)
				} else {
					p.warn(workbenchWarningInvalidType, keyPath, "build.context must be a scalar")
				}
			case "dockerfile":
				if source == "" && valueNode != nil && valueNode.Kind == yaml.ScalarNode {
					source = strings.TrimSpace(valueNode.Value)
				}
			}
		}
		return source
	default:
		p.warn(workbenchWarningInvalidType, path, "build must be a scalar or mapping")
		return ""
	}
}

func (p *workbenchComposeCoreParser) parseDependsOn(serviceName, path string, node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.SequenceNode:
		for idx, item := range node.Content {
			if item == nil || item.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fmt.Sprintf("%s[%d]", path, idx), "depends_on sequence values must be scalars")
				continue
			}
			dependency := strings.TrimSpace(item.Value)
			if dependency == "" {
				continue
			}
			p.appendDependency(serviceName, dependency)
		}
	case yaml.MappingNode:
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			keyNode := node.Content[idx]
			if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, path, "depends_on mapping keys must be scalars")
				continue
			}
			dependency := strings.TrimSpace(keyNode.Value)
			if dependency == "" {
				continue
			}
			p.appendDependency(serviceName, dependency)
		}
	default:
		p.warn(workbenchWarningInvalidType, path, "depends_on must be a sequence or mapping")
	}
}

func (p *workbenchComposeCoreParser) appendDependency(serviceName, dependency string) {
	key := serviceName + "->" + dependency
	if _, exists := p.depSet[key]; exists {
		return
	}
	p.depSet[key] = struct{}{}
	p.result.Dependencies = append(p.result.Dependencies, WorkbenchComposeDependency{
		ServiceName: serviceName,
		DependsOn:   dependency,
	})
}

func (p *workbenchComposeCoreParser) parsePorts(serviceName, path string, node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind != yaml.SequenceNode {
		p.warn(workbenchWarningInvalidType, path, "ports must be a sequence")
		return
	}

	for idx, item := range node.Content {
		itemPath := fmt.Sprintf("%s[%d]", path, idx)
		if item == nil {
			continue
		}
		switch item.Kind {
		case yaml.ScalarNode:
			port, ok := p.parseShortPort(serviceName, itemPath, item.Value)
			if ok {
				p.result.Ports = append(p.result.Ports, port)
			}
		case yaml.MappingNode:
			port, ok := p.parseLongPort(serviceName, itemPath, item)
			if ok {
				p.result.Ports = append(p.result.Ports, port)
			}
		default:
			p.warn(workbenchWarningInvalidType, itemPath, "port entry must be a scalar or mapping")
		}
	}
}

func (p *workbenchComposeCoreParser) parseShortPort(serviceName, path, raw string) (WorkbenchComposePort, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		p.warn(workbenchWarningInvalidPort, path, "port entry is empty")
		return WorkbenchComposePort{}, false
	}

	p.collectEnvRefs(serviceName, path, trimmed)

	mapping := WorkbenchComposePort{
		ServiceName: serviceName,
		Protocol:    "tcp",
	}

	valuePart := trimmed
	if slash := strings.LastIndex(trimmed, "/"); slash >= 0 {
		valuePart = strings.TrimSpace(trimmed[:slash])
		protocol := strings.TrimSpace(trimmed[slash+1:])
		if protocol != "" {
			mapping.Protocol = strings.ToLower(protocol)
		}
	}

	segments := splitPortSegments(valuePart)
	if len(segments) == 0 {
		p.warn(workbenchWarningInvalidPort, path, "port entry has no segments")
		return WorkbenchComposePort{}, false
	}

	var hostIP string
	var hostPortRaw string
	containerRaw := ""
	switch len(segments) {
	case 1:
		containerRaw = strings.TrimSpace(segments[0])
	case 2:
		hostPortRaw = strings.TrimSpace(segments[0])
		containerRaw = strings.TrimSpace(segments[1])
	default:
		hostIP = strings.TrimSpace(strings.Join(segments[:len(segments)-2], ":"))
		hostPortRaw = strings.TrimSpace(segments[len(segments)-2])
		containerRaw = strings.TrimSpace(segments[len(segments)-1])
	}

	containerPort, ok := parsePortLiteral(containerRaw)
	if !ok {
		p.warn(workbenchWarningInvalidPort, path, fmt.Sprintf("unsupported container port %q", containerRaw))
		return WorkbenchComposePort{}, false
	}
	mapping.ContainerPort = containerPort
	mapping.HostIP = normalizeHostIP(hostIP)

	if hostPortRaw != "" {
		if hostPort, parsed := parsePortLiteral(hostPortRaw); parsed {
			mapping.HostPort = &hostPort
		} else {
			mapping.HostPortRaw = hostPortRaw
			if !containsWorkbenchInterpolation(hostPortRaw) {
				p.warn(workbenchWarningInvalidPort, path, fmt.Sprintf("unsupported host port %q", hostPortRaw))
			}
		}
	}

	return mapping, true
}

func (p *workbenchComposeCoreParser) parseLongPort(serviceName, path string, node *yaml.Node) (WorkbenchComposePort, bool) {
	mapping := WorkbenchComposePort{
		ServiceName: serviceName,
		Protocol:    "tcp",
	}

	var targetRaw string
	var publishedRaw string
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		if keyNode == nil {
			continue
		}

		key := strings.TrimSpace(keyNode.Value)
		fieldPath := path + "." + key
		if valueNode != nil && valueNode.Kind == yaml.ScalarNode {
			p.collectEnvRefs(serviceName, fieldPath, valueNode.Value)
		}

		switch key {
		case "target":
			if valueNode == nil || valueNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fieldPath, "target must be a scalar")
				continue
			}
			targetRaw = strings.TrimSpace(valueNode.Value)
		case "published":
			if valueNode == nil || valueNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fieldPath, "published must be a scalar")
				continue
			}
			publishedRaw = strings.TrimSpace(valueNode.Value)
		case "host_ip":
			if valueNode == nil || valueNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fieldPath, "host_ip must be a scalar")
				continue
			}
			mapping.HostIP = normalizeHostIP(strings.TrimSpace(valueNode.Value))
		case "protocol":
			if valueNode == nil || valueNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fieldPath, "protocol must be a scalar")
				continue
			}
			protocol := strings.TrimSpace(valueNode.Value)
			if protocol != "" {
				mapping.Protocol = strings.ToLower(protocol)
			}
		case "mode", "name", "app_protocol":
		default:
			p.warnPassThrough(
				fieldPath,
				fmt.Sprintf("port mapping field %q is pass-through and not parsed", key),
			)
		}
	}

	if targetRaw == "" {
		p.warn(workbenchWarningInvalidPort, path, "port mapping target is required")
		return WorkbenchComposePort{}, false
	}
	targetPort, ok := parsePortLiteral(targetRaw)
	if !ok {
		p.warn(workbenchWarningInvalidPort, path, fmt.Sprintf("unsupported target port %q", targetRaw))
		return WorkbenchComposePort{}, false
	}
	mapping.ContainerPort = targetPort

	if publishedRaw != "" {
		if hostPort, parsed := parsePortLiteral(publishedRaw); parsed {
			mapping.HostPort = &hostPort
		} else {
			mapping.HostPortRaw = publishedRaw
			if !containsWorkbenchInterpolation(publishedRaw) {
				p.warn(workbenchWarningInvalidPort, path, fmt.Sprintf("unsupported published port %q", publishedRaw))
			}
		}
	}

	return mapping, true
}

func (p *workbenchComposeCoreParser) collectEnvRefsFromEnvironment(serviceName, path string, node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			keyNode := node.Content[idx]
			valueNode := node.Content[idx+1]
			if keyNode == nil {
				continue
			}
			key := strings.TrimSpace(keyNode.Value)
			entryPath := path + "." + key
			p.collectEnvRefs(serviceName, entryPath, keyNode.Value)
			if valueNode != nil && valueNode.Kind == yaml.ScalarNode {
				p.collectEnvRefs(serviceName, entryPath, valueNode.Value)
			}
		}
	case yaml.SequenceNode:
		for idx, item := range node.Content {
			if item == nil || item.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fmt.Sprintf("%s[%d]", path, idx), "environment sequence values must be scalars")
				continue
			}
			p.collectEnvRefs(serviceName, fmt.Sprintf("%s[%d]", path, idx), item.Value)
		}
	default:
		p.warn(workbenchWarningInvalidType, path, "environment must be a mapping or sequence")
	}
}

func (p *workbenchComposeCoreParser) collectEnvRefsFromEnvFile(serviceName, path string, node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.ScalarNode:
		p.collectEnvRefs(serviceName, path, node.Value)
	case yaml.SequenceNode:
		for idx, item := range node.Content {
			if item == nil || item.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fmt.Sprintf("%s[%d]", path, idx), "env_file sequence values must be scalars")
				continue
			}
			p.collectEnvRefs(serviceName, fmt.Sprintf("%s[%d]", path, idx), item.Value)
		}
	default:
		p.warn(workbenchWarningInvalidType, path, "env_file must be a scalar or sequence")
	}
}

func (p *workbenchComposeCoreParser) collectEnvRefs(serviceName, path, value string) {
	if value == "" {
		return
	}
	expressions := findWorkbenchEnvExpressions(value)
	for _, expression := range expressions {
		variable := extractWorkbenchEnvVariable(expression)
		key := serviceName + "|" + path + "|" + expression
		if _, exists := p.envRefSet[key]; exists {
			continue
		}
		p.envRefSet[key] = struct{}{}
		p.result.EnvRefs = append(p.result.EnvRefs, WorkbenchComposeEnvRef{
			ServiceName: serviceName,
			Path:        path,
			Expression:  expression,
			Variable:    variable,
		})
	}
}

func (p *workbenchComposeCoreParser) parseTopLevelReferenceSet(
	path string,
	node *yaml.Node,
	target map[string]struct{},
	kind string,
) {
	if node == nil {
		return
	}
	if node.Kind != yaml.MappingNode {
		p.warn(workbenchWarningInvalidType, path, fmt.Sprintf("%ss must be a mapping", kind))
		return
	}

	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
			p.warn(workbenchWarningInvalidType, path, fmt.Sprintf("%s name must be a scalar", kind))
			continue
		}

		name := strings.TrimSpace(keyNode.Value)
		if name == "" {
			p.warn(workbenchWarningInvalidType, path, fmt.Sprintf("%s name is empty", kind))
			continue
		}
		target[name] = struct{}{}

		entryPath := path + "." + name
		if valueNode == nil || isWorkbenchYAMLNull(valueNode) {
			continue
		}
		if valueNode.Kind != yaml.MappingNode {
			p.warn(workbenchWarningInvalidType, entryPath, fmt.Sprintf("top-level %s definition must be a mapping or null", kind))
			continue
		}
		if len(valueNode.Content) > 0 {
			p.warnPassThrough(
				entryPath,
				fmt.Sprintf("top-level %s options are pass-through and not parsed", kind),
			)
		}
	}
}

func (p *workbenchComposeCoreParser) parseDeployResources(serviceName, path string, node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind != yaml.MappingNode {
		p.warn(workbenchWarningInvalidType, path, "deploy must be a mapping")
		return
	}

	resource := WorkbenchComposeResource{ServiceName: serviceName}
	hasResource := false
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		if keyNode == nil {
			continue
		}

		key := strings.TrimSpace(keyNode.Value)
		fieldPath := path + "." + key
		switch key {
		case "resources":
			if p.parseDeployResourceBlock(serviceName, fieldPath, valueNode, &resource) {
				hasResource = true
			}
		default:
			p.warnPassThrough(
				fieldPath,
				fmt.Sprintf("deploy field %q is pass-through and not parsed", key),
			)
		}
	}

	if hasResource {
		p.result.Resources = append(p.result.Resources, resource)
	}
}

func (p *workbenchComposeCoreParser) parseDeployResourceBlock(
	serviceName, path string,
	node *yaml.Node,
	resource *WorkbenchComposeResource,
) bool {
	if node == nil {
		return false
	}
	if node.Kind != yaml.MappingNode {
		p.warn(workbenchWarningInvalidType, path, "deploy.resources must be a mapping")
		return false
	}

	hasResource := false
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		if keyNode == nil {
			continue
		}

		key := strings.TrimSpace(keyNode.Value)
		fieldPath := path + "." + key
		switch key {
		case "limits":
			if p.parseDeployResourceValues(serviceName, fieldPath, valueNode, &resource.LimitCPUs, &resource.LimitMemory, "limits") {
				hasResource = true
			}
		case "reservations":
			if p.parseDeployResourceValues(serviceName, fieldPath, valueNode, &resource.ReservationCPUs, &resource.ReservationMemory, "reservations") {
				hasResource = true
			}
		default:
			p.warnPassThrough(
				fieldPath,
				fmt.Sprintf("deploy.resources field %q is pass-through and not parsed", key),
			)
		}
	}
	return hasResource
}

func (p *workbenchComposeCoreParser) parseDeployResourceValues(
	serviceName, path string,
	node *yaml.Node,
	cpus *string,
	memory *string,
	blockName string,
) bool {
	if node == nil {
		return false
	}
	if node.Kind != yaml.MappingNode {
		p.warn(workbenchWarningInvalidType, path, fmt.Sprintf("deploy.resources.%s must be a mapping", blockName))
		return false
	}

	hasValue := false
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		if keyNode == nil {
			continue
		}

		key := strings.TrimSpace(keyNode.Value)
		fieldPath := path + "." + key
		if valueNode != nil && valueNode.Kind == yaml.ScalarNode {
			p.collectEnvRefs(serviceName, fieldPath, valueNode.Value)
		}

		switch key {
		case "cpus":
			if valueNode == nil || valueNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fieldPath, "cpus must be a scalar")
				continue
			}
			*cpus = strings.TrimSpace(valueNode.Value)
			if *cpus != "" {
				hasValue = true
			}
		case "memory":
			if valueNode == nil || valueNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fieldPath, "memory must be a scalar")
				continue
			}
			*memory = strings.TrimSpace(valueNode.Value)
			if *memory != "" {
				hasValue = true
			}
		default:
			p.warnPassThrough(
				fieldPath,
				fmt.Sprintf("deploy.resources.%s field %q is pass-through and not parsed", blockName, key),
			)
		}
	}
	return hasValue
}

func (p *workbenchComposeCoreParser) parseServiceNetworks(serviceName, path string, node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.SequenceNode:
		for idx, item := range node.Content {
			itemPath := fmt.Sprintf("%s[%d]", path, idx)
			if item == nil || item.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, itemPath, "network entry must be a scalar")
				continue
			}
			networkName := strings.TrimSpace(item.Value)
			if networkName == "" {
				continue
			}
			p.collectEnvRefs(serviceName, itemPath, item.Value)
			p.appendNetworkRef(serviceName, networkName)
		}
	case yaml.MappingNode:
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			keyNode := node.Content[idx]
			valueNode := node.Content[idx+1]
			if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, path, "network name must be a scalar")
				continue
			}

			networkName := strings.TrimSpace(keyNode.Value)
			if networkName == "" {
				continue
			}

			keyPath := path + "." + networkName
			p.collectEnvRefs(serviceName, keyPath, keyNode.Value)
			p.appendNetworkRef(serviceName, networkName)

			if valueNode == nil || isWorkbenchYAMLNull(valueNode) {
				continue
			}
			if valueNode.Kind != yaml.MappingNode {
				p.warn(workbenchWarningInvalidType, keyPath, "network attachment must be a mapping or null")
				continue
			}
			if len(valueNode.Content) > 0 {
				p.warnPassThrough(
					keyPath,
					fmt.Sprintf("service network %q options are pass-through and not parsed", networkName),
				)
			}
		}
	default:
		p.warn(workbenchWarningInvalidType, path, "networks must be a sequence or mapping")
	}
}

func (p *workbenchComposeCoreParser) parseServiceVolumes(serviceName, path string, node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind != yaml.SequenceNode {
		p.warn(workbenchWarningInvalidType, path, "volumes must be a sequence")
		return
	}

	for idx, item := range node.Content {
		itemPath := fmt.Sprintf("%s[%d]", path, idx)
		if item == nil {
			continue
		}
		switch item.Kind {
		case yaml.ScalarNode:
			p.parseShortVolumeRef(serviceName, itemPath, item.Value)
		case yaml.MappingNode:
			p.parseLongVolumeRef(serviceName, itemPath, item)
		default:
			p.warn(workbenchWarningInvalidType, itemPath, "volume entry must be a scalar or mapping")
		}
	}
}

func (p *workbenchComposeCoreParser) parseShortVolumeRef(serviceName, path, raw string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		p.warn(workbenchWarningInvalidType, path, "volume entry is empty")
		return
	}
	p.collectEnvRefs(serviceName, path, trimmed)

	source, ok := parseWorkbenchShortVolumeSource(trimmed)
	if !ok {
		return
	}
	p.appendVolumeRef(serviceName, source)
}

func (p *workbenchComposeCoreParser) parseLongVolumeRef(serviceName, path string, node *yaml.Node) {
	volumeType := "volume"
	source := ""

	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		if keyNode == nil {
			continue
		}
		key := strings.TrimSpace(keyNode.Value)
		fieldPath := path + "." + key
		if valueNode != nil && valueNode.Kind == yaml.ScalarNode {
			p.collectEnvRefs(serviceName, fieldPath, valueNode.Value)
		}

		switch key {
		case "type":
			if valueNode == nil || valueNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fieldPath, "type must be a scalar")
				continue
			}
			value := strings.TrimSpace(valueNode.Value)
			if value != "" {
				volumeType = strings.ToLower(value)
			}
		case "source":
			if valueNode == nil || valueNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fieldPath, "source must be a scalar")
				continue
			}
			source = strings.TrimSpace(valueNode.Value)
		case "target", "read_only":
			if valueNode == nil || valueNode.Kind != yaml.ScalarNode {
				p.warn(workbenchWarningInvalidType, fieldPath, fmt.Sprintf("%s must be a scalar", key))
			}
		case "bind", "volume", "tmpfs", "consistency", "nocopy", "subpath":
			p.warnPassThrough(
				fieldPath,
				fmt.Sprintf("volume mapping field %q is pass-through and not parsed", key),
			)
		default:
			p.warnPassThrough(
				fieldPath,
				fmt.Sprintf("volume mapping field %q is pass-through and not parsed", key),
			)
		}
	}

	if volumeType != "volume" {
		return
	}
	if source == "" || isWorkbenchPathLikeSource(source) || containsWorkbenchInterpolation(source) {
		return
	}
	p.appendVolumeRef(serviceName, source)
}

func (p *workbenchComposeCoreParser) appendNetworkRef(serviceName, networkName string) {
	if serviceName == "" || networkName == "" {
		return
	}
	if _, ok := p.topLevelNetworks[networkName]; !ok {
		return
	}
	key := serviceName + "|" + networkName
	if _, exists := p.networkRefSet[key]; exists {
		return
	}
	p.networkRefSet[key] = struct{}{}
	p.result.NetworkRefs = append(p.result.NetworkRefs, WorkbenchComposeNetworkRef{
		ServiceName: serviceName,
		NetworkName: networkName,
	})
}

func (p *workbenchComposeCoreParser) appendVolumeRef(serviceName, volumeName string) {
	if serviceName == "" || volumeName == "" {
		return
	}
	if _, ok := p.topLevelVolumes[volumeName]; !ok {
		return
	}
	key := serviceName + "|" + volumeName
	if _, exists := p.volumeRefSet[key]; exists {
		return
	}
	p.volumeRefSet[key] = struct{}{}
	p.result.VolumeRefs = append(p.result.VolumeRefs, WorkbenchComposeVolumeRef{
		ServiceName: serviceName,
		VolumeName:  volumeName,
	})
}

func extractWorkbenchEnvVariable(expression string) string {
	trimmed := strings.TrimSpace(expression)
	if strings.HasPrefix(trimmed, "$") && !strings.HasPrefix(trimmed, "${") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "$"))
	}
	if !strings.HasPrefix(trimmed, "${") || !strings.HasSuffix(trimmed, "}") {
		return trimmed
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, "${"), "}")
	for _, separator := range []string{":-", ":+", ":?", "-", "+", "?"} {
		if idx := strings.Index(inner, separator); idx >= 0 {
			return strings.TrimSpace(inner[:idx])
		}
	}
	return strings.TrimSpace(inner)
}

func containsWorkbenchInterpolation(value string) bool {
	return len(findWorkbenchEnvExpressions(value)) > 0
}

func parseWorkbenchShortVolumeSource(value string) (string, bool) {
	if value == "" || !strings.Contains(value, ":") {
		return "", false
	}
	parts := strings.SplitN(value, ":", 2)
	source := strings.TrimSpace(parts[0])
	if source == "" {
		return "", false
	}
	if containsWorkbenchInterpolation(source) || isWorkbenchPathLikeSource(source) {
		return "", false
	}
	return source, true
}

func isWorkbenchPathLikeSource(source string) bool {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "~") {
		return true
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") {
		return true
	}
	return false
}

func findWorkbenchEnvExpressions(value string) []string {
	if value == "" {
		return nil
	}

	expressions := make([]string, 0, 2)
	for idx := 0; idx < len(value); idx++ {
		if value[idx] != '$' {
			continue
		}

		if idx+1 < len(value) && value[idx+1] == '$' {
			idx++
			continue
		}

		if idx+1 < len(value) && value[idx+1] == '{' {
			end := idx + 2
			for end < len(value) && value[end] != '}' {
				end++
			}
			if end < len(value) && value[end] == '}' {
				expressions = append(expressions, value[idx:end+1])
				idx = end
			}
			continue
		}

		if idx+1 < len(value) && isWorkbenchEnvVarStart(value[idx+1]) {
			end := idx + 2
			for end < len(value) && isWorkbenchEnvVarPart(value[end]) {
				end++
			}
			expressions = append(expressions, value[idx:end])
			idx = end - 1
		}
	}
	return expressions
}

func isWorkbenchEnvVarStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isWorkbenchEnvVarPart(ch byte) bool {
	return isWorkbenchEnvVarStart(ch) || (ch >= '0' && ch <= '9')
}

func splitPortSegments(input string) []string {
	if input == "" {
		return nil
	}

	parts := make([]string, 0, 4)
	var builder strings.Builder
	bracketDepth := 0
	for _, r := range input {
		switch r {
		case '[':
			bracketDepth++
			builder.WriteRune(r)
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
			builder.WriteRune(r)
		case ':':
			if bracketDepth > 0 {
				builder.WriteRune(r)
				continue
			}
			parts = append(parts, builder.String())
			builder.Reset()
		default:
			builder.WriteRune(r)
		}
	}
	parts = append(parts, builder.String())
	return parts
}

func normalizeHostIP(hostIP string) string {
	trimmed := strings.TrimSpace(hostIP)
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")
	}
	return trimmed
}

func parsePortLiteral(value string) (int, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}
	for _, r := range trimmed {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil || port < 1 || port > 65535 {
		return 0, false
	}
	return port, true
}

func isWorkbenchYAMLNull(node *yaml.Node) bool {
	if node == nil {
		return true
	}
	if node.Kind != yaml.ScalarNode {
		return false
	}
	if node.Tag == "!!null" {
		return true
	}
	trimmed := strings.TrimSpace(node.Value)
	return trimmed == "" || strings.EqualFold(trimmed, "null") || trimmed == "~"
}

func (p *workbenchComposeCoreParser) warnPassThrough(path, message string) {
	p.warn(workbenchWarningPassThrough, path, message)
}

func (p *workbenchComposeCoreParser) warn(code, path, message string) {
	p.result.Warnings = append(p.result.Warnings, WorkbenchComposeWarning{
		Code:    strings.TrimSpace(code),
		Path:    strings.TrimSpace(path),
		Message: strings.TrimSpace(message),
	})
}

func (p *workbenchComposeCoreParser) sort() {
	sort.Slice(p.result.Services, func(i, j int) bool {
		return workbenchComposeServiceLess(p.result.Services[i], p.result.Services[j])
	})
	sort.Slice(p.result.Dependencies, func(i, j int) bool {
		return workbenchComposeDependencyLess(p.result.Dependencies[i], p.result.Dependencies[j])
	})
	sort.Slice(p.result.Ports, func(i, j int) bool {
		return workbenchComposePortLess(p.result.Ports[i], p.result.Ports[j])
	})
	sort.Slice(p.result.Resources, func(i, j int) bool {
		return workbenchComposeResourceLess(p.result.Resources[i], p.result.Resources[j])
	})
	sort.Slice(p.result.NetworkRefs, func(i, j int) bool {
		return workbenchComposeNetworkRefLess(p.result.NetworkRefs[i], p.result.NetworkRefs[j])
	})
	sort.Slice(p.result.VolumeRefs, func(i, j int) bool {
		return workbenchComposeVolumeRefLess(p.result.VolumeRefs[i], p.result.VolumeRefs[j])
	})
	sort.Slice(p.result.EnvRefs, func(i, j int) bool {
		return workbenchComposeEnvRefLess(p.result.EnvRefs[i], p.result.EnvRefs[j])
	})
	sort.Slice(p.result.Warnings, func(i, j int) bool {
		return workbenchComposeWarningLess(p.result.Warnings[i], p.result.Warnings[j])
	})
}

func workbenchComposeServiceLess(left, right WorkbenchComposeService) bool {
	if left.ServiceName != right.ServiceName {
		return left.ServiceName < right.ServiceName
	}
	if left.Image != right.Image {
		return left.Image < right.Image
	}
	if left.BuildSource != right.BuildSource {
		return left.BuildSource < right.BuildSource
	}
	return left.RestartPolicy < right.RestartPolicy
}

func workbenchComposeDependencyLess(left, right WorkbenchComposeDependency) bool {
	if left.ServiceName != right.ServiceName {
		return left.ServiceName < right.ServiceName
	}
	return left.DependsOn < right.DependsOn
}

func workbenchComposePortLess(left, right WorkbenchComposePort) bool {
	if left.ServiceName != right.ServiceName {
		return left.ServiceName < right.ServiceName
	}
	if left.ContainerPort != right.ContainerPort {
		return left.ContainerPort < right.ContainerPort
	}
	if left.Protocol != right.Protocol {
		return left.Protocol < right.Protocol
	}
	if left.HostIP != right.HostIP {
		return left.HostIP < right.HostIP
	}
	leftHostPort := 0
	if left.HostPort != nil {
		leftHostPort = *left.HostPort
	}
	rightHostPort := 0
	if right.HostPort != nil {
		rightHostPort = *right.HostPort
	}
	if leftHostPort != rightHostPort {
		return leftHostPort < rightHostPort
	}
	return left.HostPortRaw < right.HostPortRaw
}

func workbenchComposeResourceLess(left, right WorkbenchComposeResource) bool {
	if left.ServiceName != right.ServiceName {
		return left.ServiceName < right.ServiceName
	}
	if left.LimitCPUs != right.LimitCPUs {
		return left.LimitCPUs < right.LimitCPUs
	}
	if left.LimitMemory != right.LimitMemory {
		return left.LimitMemory < right.LimitMemory
	}
	if left.ReservationCPUs != right.ReservationCPUs {
		return left.ReservationCPUs < right.ReservationCPUs
	}
	return left.ReservationMemory < right.ReservationMemory
}

func workbenchComposeNetworkRefLess(left, right WorkbenchComposeNetworkRef) bool {
	if left.ServiceName != right.ServiceName {
		return left.ServiceName < right.ServiceName
	}
	return left.NetworkName < right.NetworkName
}

func workbenchComposeVolumeRefLess(left, right WorkbenchComposeVolumeRef) bool {
	if left.ServiceName != right.ServiceName {
		return left.ServiceName < right.ServiceName
	}
	return left.VolumeName < right.VolumeName
}

func workbenchComposeEnvRefLess(left, right WorkbenchComposeEnvRef) bool {
	if left.ServiceName != right.ServiceName {
		return left.ServiceName < right.ServiceName
	}
	if left.Path != right.Path {
		return left.Path < right.Path
	}
	if left.Expression != right.Expression {
		return left.Expression < right.Expression
	}
	return left.Variable < right.Variable
}

func workbenchComposeWarningLess(left, right WorkbenchComposeWarning) bool {
	if left.Code != right.Code {
		return left.Code < right.Code
	}
	if left.Path != right.Path {
		return left.Path < right.Path
	}
	return left.Message < right.Message
}
