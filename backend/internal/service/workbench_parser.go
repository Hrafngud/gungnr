package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	workbenchWarningUnsupportedTopLevel = "WB-PARSE-UNSUPPORTED-TOPLEVEL"
	workbenchWarningUnsupportedField    = "WB-PARSE-UNSUPPORTED-FIELD"
	workbenchWarningInvalidType         = "WB-PARSE-INVALID-TYPE"
	workbenchWarningInvalidPort         = "WB-PARSE-INVALID-PORT"
)

type WorkbenchComposeParseResult struct {
	ProjectName       string                       `json:"projectName"`
	ProjectDir        string                       `json:"projectDir"`
	ComposePath       string                       `json:"composePath"`
	SourceFingerprint string                       `json:"sourceFingerprint"`
	Services          []WorkbenchComposeService    `json:"services"`
	Dependencies      []WorkbenchComposeDependency `json:"dependencies"`
	Ports             []WorkbenchComposePort       `json:"ports"`
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
	result    WorkbenchComposeParseResult
	envRefSet map[string]struct{}
	depSet    map[string]struct{}
}

func ParseWorkbenchComposeCore(normalizedSource string) (WorkbenchComposeParseResult, error) {
	parser := workbenchComposeCoreParser{
		envRefSet: make(map[string]struct{}),
		depSet:    make(map[string]struct{}),
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
			parser.parseServices("$.services", valueNode)
		case "version", "name":
		default:
			parser.warn(
				workbenchWarningUnsupportedTopLevel,
				"$."+key,
				fmt.Sprintf("top-level key %q is not parsed in core slice", key),
			)
		}
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
		default:
			p.warn(
				workbenchWarningUnsupportedField,
				fieldPath,
				fmt.Sprintf("service field %q is not parsed in core slice", key),
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
			p.warn(
				workbenchWarningUnsupportedField,
				fieldPath,
				fmt.Sprintf("port mapping field %q is not parsed in core slice", key),
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
