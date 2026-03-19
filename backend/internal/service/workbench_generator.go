package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	serviceExtras   map[string]workbenchComposeServiceExtras
	topLevelNetwork []string
}

type workbenchHostBinding struct {
	serviceName string
	hostIP      string
	hostPort    string
	protocol    string
}

const (
	workbenchValidationClassSchema       = "schema"
	workbenchValidationClassDependency   = "dependency"
	workbenchValidationClassPortConflict = "port_conflict"
)

type WorkbenchValidationIssue struct {
	Class      string `json:"class"`
	Code       string `json:"code"`
	Path       string `json:"path"`
	Message    string `json:"message"`
	Service    string `json:"service,omitempty"`
	Dependency string `json:"dependency,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	HostIP     string `json:"hostIp,omitempty"`
	HostPort   string `json:"hostPort,omitempty"`
}

type WorkbenchComposePreviewRequest struct {
	ExpectedRevision *int `json:"expectedRevision,omitempty"`
}

type WorkbenchComposePreviewMetadata struct {
	Revision          int    `json:"revision"`
	SourceFingerprint string `json:"sourceFingerprint,omitempty"`
}

type WorkbenchComposePreviewResult struct {
	Compose  string                          `json:"compose"`
	Metadata WorkbenchComposePreviewMetadata `json:"metadata"`
}

type WorkbenchComposeApplyRequest struct {
	ExpectedRevision          *int   `json:"expectedRevision,omitempty"`
	ExpectedSourceFingerprint string `json:"expectedSourceFingerprint,omitempty"`
}

type WorkbenchComposeApplyMetadata struct {
	Revision          int    `json:"revision"`
	SourceFingerprint string `json:"sourceFingerprint,omitempty"`
	ComposePath       string `json:"composePath"`
}

type WorkbenchComposeApplyResult struct {
	Metadata     WorkbenchComposeApplyMetadata       `json:"metadata"`
	ComposeBytes int                                 `json:"composeBytes"`
	Backup       WorkbenchComposeBackupMetadata      `json:"backup"`
	Retention    WorkbenchComposeBackupRetentionInfo `json:"retention"`
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

	snapshot, err := s.loadStoredSnapshotForComposeLocked(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, "", err
	}

	compose, err := generateWorkbenchCompose(snapshot)
	if err != nil {
		return WorkbenchStackSnapshot{}, "", err
	}

	return snapshot, compose, nil
}

func (s *WorkbenchService) ValidateStoredSnapshotForCompose(
	ctx context.Context,
	projectName string,
) (WorkbenchStackSnapshot, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchStackSnapshot{}, err
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, err
	}
	defer release()

	snapshot, err := s.loadStoredSnapshotForComposeLocked(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, err
	}

	if err := validateWorkbenchSnapshotForCompose(snapshot); err != nil {
		return WorkbenchStackSnapshot{}, err
	}
	return snapshot, nil
}

func (s *WorkbenchService) PreviewComposeFromStoredSnapshot(
	ctx context.Context,
	projectName string,
	input WorkbenchComposePreviewRequest,
) (WorkbenchComposePreviewResult, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchComposePreviewResult{}, err
	}

	normalizedInput, err := normalizeWorkbenchComposePreviewRequest(input)
	if err != nil {
		return WorkbenchComposePreviewResult{}, err
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchComposePreviewResult{}, err
	}
	defer release()

	snapshot, err := s.loadStoredSnapshotForComposeLocked(ctx, normalizedProject)
	if err != nil {
		return WorkbenchComposePreviewResult{}, err
	}

	if normalizedInput.ExpectedRevision != nil && snapshot.Revision != *normalizedInput.ExpectedRevision {
		return WorkbenchComposePreviewResult{}, workbenchPreviewExpectedRevisionValidationError(
			snapshot,
			*normalizedInput.ExpectedRevision,
		)
	}

	currentSource, err := s.ResolveComposeSource(ctx, normalizedProject)
	if err != nil {
		return WorkbenchComposePreviewResult{}, err
	}
	compose, err := mergeWorkbenchSnapshotIntoComposeSource(snapshot, currentSource)
	if err != nil {
		return WorkbenchComposePreviewResult{}, err
	}

	return WorkbenchComposePreviewResult{
		Compose: compose,
		Metadata: WorkbenchComposePreviewMetadata{
			Revision:          snapshot.Revision,
			SourceFingerprint: strings.TrimSpace(snapshot.SourceFingerprint),
		},
	}, nil
}

func (s *WorkbenchService) ApplyComposeFromStoredSnapshot(
	ctx context.Context,
	projectName string,
	input WorkbenchComposeApplyRequest,
) (WorkbenchComposeApplyResult, error) {
	normalizedProject, err := normalizeWorkbenchProjectName(projectName)
	if err != nil {
		return WorkbenchComposeApplyResult{}, err
	}

	normalizedInput, err := normalizeWorkbenchComposeApplyRequest(input)
	if err != nil {
		return WorkbenchComposeApplyResult{}, err
	}

	release, err := s.AcquireProjectLock(ctx, normalizedProject)
	if err != nil {
		return WorkbenchComposeApplyResult{}, err
	}
	defer release()

	snapshot, err := s.loadStoredSnapshotForComposeLocked(ctx, normalizedProject)
	if err != nil {
		return WorkbenchComposeApplyResult{}, err
	}
	if snapshot.Revision != *normalizedInput.ExpectedRevision {
		return WorkbenchComposeApplyResult{}, workbenchApplyStaleRevisionError(
			snapshot,
			*normalizedInput.ExpectedRevision,
		)
	}

	currentSource, err := s.ResolveComposeSource(ctx, normalizedProject)
	if err != nil {
		return WorkbenchComposeApplyResult{}, err
	}
	if err := workbenchApplyDriftCheck(snapshot, currentSource, normalizedInput.ExpectedSourceFingerprint); err != nil {
		return WorkbenchComposeApplyResult{}, err
	}

	compose, err := mergeWorkbenchSnapshotIntoComposeSource(snapshot, currentSource)
	if err != nil {
		return WorkbenchComposeApplyResult{}, err
	}

	backup, retention, err := s.createComposeBackup(ctx, normalizedProject, snapshot, currentSource)
	if err != nil {
		return WorkbenchComposeApplyResult{}, err
	}

	normalizedCompose, appliedFingerprint := WorkbenchSourceFingerprint([]byte(compose))
	if err := replaceWorkbenchComposeAtomically(currentSource.ComposePath, []byte(normalizedCompose)); err != nil {
		return WorkbenchComposeApplyResult{}, workbenchComposeApplySourceInvalidError(
			snapshot,
			currentSource,
			"failed to replace compose source",
			err,
		)
	}

	updatedSnapshot := snapshot
	updatedSnapshot.ProjectName = normalizedProject
	updatedSnapshot.ProjectDir = currentSource.ProjectDir
	updatedSnapshot.ComposePath = currentSource.ComposePath
	updatedSnapshot.SourceFingerprint = appliedFingerprint
	if err := s.saveWorkbenchSnapshot(ctx, normalizedProject, updatedSnapshot); err != nil {
		restoreErr := replaceWorkbenchComposeAtomically(currentSource.ComposePath, currentSource.Raw)
		return WorkbenchComposeApplyResult{}, workbenchComposeApplyStorageError(
			updatedSnapshot,
			currentSource,
			"failed to persist updated workbench source fingerprint",
			err,
			restoreErr,
		)
	}

	return WorkbenchComposeApplyResult{
		Metadata: WorkbenchComposeApplyMetadata{
			Revision:          updatedSnapshot.Revision,
			SourceFingerprint: updatedSnapshot.SourceFingerprint,
			ComposePath:       currentSource.ComposePath,
		},
		ComposeBytes: len(normalizedCompose),
		Backup:       backup,
		Retention:    retention,
	}, nil
}

type workbenchComposePatchModel struct {
	snapshot      WorkbenchStackSnapshot
	services      map[string]WorkbenchComposeService
	dependencies  map[string][]string
	networkRefs   map[string][]string
	ports         map[string][]WorkbenchComposePort
	resources     map[string]WorkbenchComposeResource
	serviceExtras map[string]workbenchComposeServiceExtras
}

func mergeWorkbenchSnapshotIntoComposeSource(
	snapshot WorkbenchStackSnapshot,
	source WorkbenchComposeSource,
) (string, error) {
	model, err := buildWorkbenchComposePatchModel(snapshot)
	if err != nil {
		return "", err
	}

	var document yaml.Node
	if err := yaml.Unmarshal([]byte(source.Normalized), &document); err != nil {
		return "", workbenchComposeApplySourceInvalidError(
			model.snapshot,
			source,
			"failed to parse current compose source",
			err,
		)
	}

	root := workbenchDocumentRoot(&document)
	if root == nil || root.Kind != yaml.MappingNode {
		return "", workbenchComposeApplySourceInvalidError(
			model.snapshot,
			source,
			"current compose source root must be a mapping",
			nil,
		)
	}

	servicesNode, ok := workbenchYAMLFindMapValue(root, "services")
	if !ok || servicesNode == nil || servicesNode.Kind != yaml.MappingNode {
		return "", workbenchComposeApplySourceInvalidError(
			model.snapshot,
			source,
			"current compose source must define services as a mapping",
			nil,
		)
	}

	for serviceName, service := range model.services {
		serviceNode, ok := workbenchYAMLFindMapValue(servicesNode, serviceName)
		extras := model.serviceExtras[serviceName]
		if !ok || serviceNode == nil || serviceNode.Kind != yaml.MappingNode {
			if extras.Managed {
				workbenchYAMLAddMapEntry(
					servicesNode,
					serviceName,
					workbenchBuildServiceNode(service, model.dependencies[serviceName], model.ports[serviceName], model.resources[serviceName], model.networkRefs[serviceName], extras),
				)
				continue
			}
			return "", workbenchComposeApplySourceInvalidError(
				model.snapshot,
				source,
				fmt.Sprintf("current compose source is missing service %q", serviceName),
				nil,
			)
		}

		workbenchPatchServiceDefinition(serviceNode, service, model.dependencies[serviceName], model.networkRefs[serviceName])
		workbenchPatchServicePorts(serviceNode, model.ports[serviceName])
		workbenchPatchServiceResources(serviceNode, model.resources[serviceName])
		if extras.Managed {
			workbenchPatchServiceCommand(serviceNode, extras.Command)
			workbenchPatchServiceEnvironment(serviceNode, extras.Environment)
		}
	}
	workbenchPruneRemovedManagedServiceNodes(servicesNode, model.services, model.snapshot)

	encoded, err := encodeWorkbenchComposeYAML(root)
	if err != nil {
		return "", workbenchComposeApplySourceInvalidError(
			model.snapshot,
			source,
			"failed to encode merged compose source",
			err,
		)
	}
	return encoded, nil
}

func buildWorkbenchComposePatchModel(snapshot WorkbenchStackSnapshot) (workbenchComposePatchModel, error) {
	normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
	genModel, err := buildWorkbenchComposeGenerationModel(normalizedSnapshot)
	if err != nil {
		filteredErr := workbenchFilterValidationIssues(err, func(issue WorkbenchValidationIssue) bool {
			return strings.EqualFold(strings.TrimSpace(issue.Code), "WB-VAL-VOLUME-UNSUPPORTED")
		})
		if filteredErr != nil {
			return workbenchComposePatchModel{}, filteredErr
		}
		genModel = workbenchBuildComposePatchFallbackGenerationModel(normalizedSnapshot)
	}

	model := workbenchComposePatchModel{
		snapshot:      normalizedSnapshot,
		services:      make(map[string]WorkbenchComposeService, len(genModel.services)),
		dependencies:  make(map[string][]string),
		networkRefs:   make(map[string][]string),
		ports:         make(map[string][]WorkbenchComposePort),
		resources:     make(map[string]WorkbenchComposeResource),
		serviceExtras: make(map[string]workbenchComposeServiceExtras, len(genModel.serviceExtras)),
	}
	for _, service := range genModel.services {
		name := strings.TrimSpace(service.ServiceName)
		if name == "" {
			continue
		}
		model.services[name] = WorkbenchComposeService{
			ServiceName:   name,
			Image:         strings.TrimSpace(service.Image),
			BuildSource:   strings.TrimSpace(service.BuildSource),
			RestartPolicy: strings.TrimSpace(service.RestartPolicy),
		}
	}
	for serviceName, dependencies := range genModel.dependencies {
		model.dependencies[serviceName] = append([]string(nil), dependencies...)
	}
	for serviceName, networks := range genModel.networkRefs {
		model.networkRefs[serviceName] = append([]string(nil), networks...)
	}
	for serviceName, ports := range genModel.ports {
		model.ports[serviceName] = append([]WorkbenchComposePort(nil), ports...)
	}
	for serviceName, resource := range genModel.resources {
		model.resources[serviceName] = WorkbenchComposeResource{
			ServiceName:       serviceName,
			LimitCPUs:         strings.TrimSpace(resource.LimitCPUs),
			LimitMemory:       strings.TrimSpace(resource.LimitMemory),
			ReservationCPUs:   strings.TrimSpace(resource.ReservationCPUs),
			ReservationMemory: strings.TrimSpace(resource.ReservationMemory),
		}
	}
	for serviceName, extras := range genModel.serviceExtras {
		model.serviceExtras[serviceName] = extras
	}
	return model, nil
}

func workbenchBuildComposePatchFallbackGenerationModel(snapshot WorkbenchStackSnapshot) workbenchComposeGenerationModel {
	normalizedSnapshot := normalizeWorkbenchStackSnapshot(snapshot)
	model := workbenchComposeGenerationModel{
		snapshot:      normalizedSnapshot,
		services:      []WorkbenchComposeService{},
		dependencies:  make(map[string][]string),
		ports:         make(map[string][]WorkbenchComposePort),
		resources:     make(map[string]WorkbenchComposeResource),
		networkRefs:   make(map[string][]string),
		serviceExtras: make(map[string]workbenchComposeServiceExtras),
	}

	for _, service := range normalizedSnapshot.Services {
		name := strings.TrimSpace(service.ServiceName)
		if name == "" {
			continue
		}
		model.services = append(model.services, WorkbenchComposeService{
			ServiceName:   name,
			Image:         strings.TrimSpace(service.Image),
			BuildSource:   strings.TrimSpace(service.BuildSource),
			RestartPolicy: strings.TrimSpace(service.RestartPolicy),
		})
	}

	for _, managedService := range func() []workbenchManagedServiceModel {
		models, _ := workbenchBuildManagedServiceModels(normalizedSnapshot)
		return models
	}() {
		name := strings.TrimSpace(managedService.ServiceName)
		if name == "" {
			continue
		}
		model.services = append(model.services, managedService.Service)
		model.serviceExtras[name] = managedService.Extras
	}

	for _, dependency := range normalizedSnapshot.Dependencies {
		serviceName := strings.TrimSpace(dependency.ServiceName)
		dependsOn := strings.TrimSpace(dependency.DependsOn)
		if serviceName == "" || dependsOn == "" {
			continue
		}
		model.dependencies[serviceName] = append(model.dependencies[serviceName], dependsOn)
	}
	for _, networkRef := range normalizedSnapshot.NetworkRefs {
		serviceName := strings.TrimSpace(networkRef.ServiceName)
		networkName := strings.TrimSpace(networkRef.NetworkName)
		if serviceName == "" || networkName == "" {
			continue
		}
		model.networkRefs[serviceName] = append(model.networkRefs[serviceName], networkName)
	}
	for _, port := range normalizedSnapshot.Ports {
		serviceName := strings.TrimSpace(port.ServiceName)
		if serviceName == "" {
			continue
		}
		model.ports[serviceName] = append(model.ports[serviceName], normalizeWorkbenchComposePort(port))
	}
	managedServiceModels, _ := workbenchBuildManagedServiceModels(normalizedSnapshot)
	for _, managedService := range managedServiceModels {
		for _, port := range managedService.Ports {
			if workbenchHasPortMapping(model.ports[managedService.ServiceName], managedService.ServiceName, port.ContainerPort, port.Protocol, port.HostIP) {
				continue
			}
			model.ports[managedService.ServiceName] = append(model.ports[managedService.ServiceName], normalizeWorkbenchComposePort(port))
		}
	}
	for _, resource := range normalizedSnapshot.Resources {
		serviceName := strings.TrimSpace(resource.ServiceName)
		if serviceName == "" {
			continue
		}
		model.resources[serviceName] = WorkbenchComposeResource{
			ServiceName:       serviceName,
			LimitCPUs:         strings.TrimSpace(resource.LimitCPUs),
			LimitMemory:       strings.TrimSpace(resource.LimitMemory),
			ReservationCPUs:   strings.TrimSpace(resource.ReservationCPUs),
			ReservationMemory: strings.TrimSpace(resource.ReservationMemory),
		}
	}
	return model
}

func workbenchFilterValidationIssues(err error, drop func(issue WorkbenchValidationIssue) bool) error {
	if err == nil {
		return nil
	}

	typed, ok := errs.From(err)
	if !ok || typed.Code != errs.CodeWorkbenchValidationFailed {
		return err
	}

	details, ok := typed.Details.(map[string]any)
	if !ok {
		return err
	}

	issues, ok := details["issues"].([]WorkbenchValidationIssue)
	if !ok {
		return err
	}

	filtered := make([]WorkbenchValidationIssue, 0, len(issues))
	for _, issue := range issues {
		if drop != nil && drop(issue) {
			continue
		}
		filtered = append(filtered, issue)
	}
	if len(filtered) == 0 {
		return nil
	}
	return workbenchComposeValidationError(typedSnapshotFromValidationDetails(details), filtered)
}

func typedSnapshotFromValidationDetails(details map[string]any) WorkbenchStackSnapshot {
	snapshot := WorkbenchStackSnapshot{}
	if project, ok := details["project"].(string); ok {
		snapshot.ProjectName = project
	}
	if composePath, ok := details["composePath"].(string); ok {
		snapshot.ComposePath = composePath
	}
	if fingerprint, ok := details["sourceFingerprint"].(string); ok {
		snapshot.SourceFingerprint = fingerprint
	}
	if revision, ok := details["revision"].(int); ok {
		snapshot.Revision = revision
	}
	return snapshot
}

func workbenchPatchServicePorts(serviceNode *yaml.Node, ports []WorkbenchComposePort) {
	if len(ports) == 0 {
		workbenchYAMLDeleteMapEntry(serviceNode, "ports")
		return
	}

	portSequence := workbenchYAMLSequenceNode()
	for _, port := range ports {
		portSequence.Content = append(portSequence.Content, workbenchYAMLScalarNode(formatWorkbenchComposePort(port)))
	}
	workbenchYAMLSetMapEntry(serviceNode, "ports", portSequence)
}

func workbenchPatchServiceDefinition(
	serviceNode *yaml.Node,
	service WorkbenchComposeService,
	dependencies []string,
	networkRefs []string,
) {
	workbenchYAMLSetOrDeleteScalarEntry(serviceNode, "image", strings.TrimSpace(service.Image))
	workbenchPatchServiceBuild(serviceNode, strings.TrimSpace(service.BuildSource))
	workbenchYAMLSetOrDeleteScalarEntry(serviceNode, "restart", strings.TrimSpace(service.RestartPolicy))

	if len(dependencies) == 0 {
		workbenchYAMLDeleteMapEntry(serviceNode, "depends_on")
	} else {
		workbenchPatchServiceDependsOn(serviceNode, dependencies)
	}

	if len(networkRefs) == 0 {
		workbenchYAMLDeleteMapEntry(serviceNode, "networks")
	} else {
		workbenchPatchServiceNetworks(serviceNode, networkRefs)
	}
}

func workbenchPatchServiceBuild(serviceNode *yaml.Node, buildSource string) {
	currentNode, ok := workbenchYAMLFindMapValue(serviceNode, "build")
	if strings.TrimSpace(buildSource) == "" {
		workbenchYAMLDeleteMapEntry(serviceNode, "build")
		return
	}
	if ok && currentNode != nil && currentNode.Kind == yaml.MappingNode {
		return
	}
	workbenchYAMLSetMapEntry(serviceNode, "build", workbenchYAMLScalarNode(buildSource))
}

func workbenchPatchServiceDependsOn(serviceNode *yaml.Node, dependencies []string) {
	currentNode, ok := workbenchYAMLFindMapValue(serviceNode, "depends_on")
	if ok && currentNode != nil && currentNode.Kind == yaml.MappingNode {
		workbenchPatchNamedMappingEntries(currentNode, dependencies)
		return
	}

	dependsOnNode := workbenchYAMLSequenceNode()
	for _, dependency := range dependencies {
		dependsOnNode.Content = append(dependsOnNode.Content, workbenchYAMLScalarNode(dependency))
	}
	workbenchYAMLSetMapEntry(serviceNode, "depends_on", dependsOnNode)
}

func workbenchPatchServiceNetworks(serviceNode *yaml.Node, networkRefs []string) {
	currentNode, ok := workbenchYAMLFindMapValue(serviceNode, "networks")
	if ok && currentNode != nil && currentNode.Kind == yaml.MappingNode {
		workbenchPatchNamedMappingEntries(currentNode, networkRefs)
		return
	}

	networksNode := workbenchYAMLSequenceNode()
	for _, networkName := range networkRefs {
		networksNode.Content = append(networksNode.Content, workbenchYAMLScalarNode(networkName))
	}
	workbenchYAMLSetMapEntry(serviceNode, "networks", networksNode)
}

func workbenchPatchNamedMappingEntries(mappingNode *yaml.Node, names []string) {
	if mappingNode == nil || mappingNode.Kind != yaml.MappingNode {
		return
	}

	desired := make(map[string]struct{}, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		desired[trimmed] = struct{}{}
	}

	nextContent := make([]*yaml.Node, 0, len(mappingNode.Content))
	present := make(map[string]struct{}, len(desired))
	for idx := 0; idx+1 < len(mappingNode.Content); idx += 2 {
		keyNode := mappingNode.Content[idx]
		valueNode := mappingNode.Content[idx+1]
		if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
			continue
		}
		name := strings.TrimSpace(keyNode.Value)
		if _, keep := desired[name]; !keep {
			continue
		}
		nextContent = append(nextContent, keyNode, valueNode)
		present[name] = struct{}{}
	}

	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, exists := present[trimmed]; exists {
			continue
		}
		nextContent = append(nextContent, workbenchYAMLScalarNode(trimmed), workbenchYAMLMappingNode())
	}

	mappingNode.Content = nextContent
}

func workbenchPatchServiceResources(serviceNode *yaml.Node, resource WorkbenchComposeResource) {
	if workbenchIsEmptyResource(resource) {
		deployNode, ok := workbenchYAMLFindMapValue(serviceNode, "deploy")
		if !ok || deployNode == nil || deployNode.Kind != yaml.MappingNode {
			return
		}
		workbenchYAMLDeleteMapEntry(deployNode, "resources")
		if len(deployNode.Content) == 0 {
			workbenchYAMLDeleteMapEntry(serviceNode, "deploy")
		}
		return
	}

	deployNode, ok := workbenchYAMLFindMapValue(serviceNode, "deploy")
	if !ok || deployNode == nil {
		deployNode = workbenchYAMLMappingNode()
		workbenchYAMLSetMapEntry(serviceNode, "deploy", deployNode)
	} else if deployNode.Kind != yaml.MappingNode {
		deployNode.Kind = yaml.MappingNode
		deployNode.Style = 0
		deployNode.Tag = "!!map"
		deployNode.Content = nil
	}

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

	workbenchYAMLSetMapEntry(deployNode, "resources", resourcesNode)
}

func workbenchPatchServiceCommand(serviceNode *yaml.Node, command []string) {
	if len(command) == 0 {
		workbenchYAMLDeleteMapEntry(serviceNode, "command")
		return
	}
	workbenchYAMLSetMapEntry(serviceNode, "command", workbenchYAMLCommandNode(command))
}

func workbenchPatchServiceEnvironment(serviceNode *yaml.Node, environment []workbenchComposeEnvironmentEntry) {
	if len(environment) == 0 {
		workbenchYAMLDeleteMapEntry(serviceNode, "environment")
		return
	}
	workbenchYAMLSetMapEntry(serviceNode, "environment", workbenchYAMLEnvironmentNode(environment))
}

func workbenchBuildServiceNode(
	service WorkbenchComposeService,
	dependencies []string,
	ports []WorkbenchComposePort,
	resource WorkbenchComposeResource,
	networkRefs []string,
	extras workbenchComposeServiceExtras,
) *yaml.Node {
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
	if extras.Managed {
		workbenchAddServiceCommand(serviceNode, extras.Command)
		workbenchAddServiceEnvironment(serviceNode, extras.Environment)
	}
	if len(dependencies) > 0 {
		depSequence := workbenchYAMLSequenceNode()
		for _, dependency := range dependencies {
			depSequence.Content = append(depSequence.Content, workbenchYAMLScalarNode(dependency))
		}
		workbenchYAMLAddMapEntry(serviceNode, "depends_on", depSequence)
	}
	if len(ports) > 0 {
		portSequence := workbenchYAMLSequenceNode()
		for _, port := range ports {
			portSequence.Content = append(portSequence.Content, workbenchYAMLScalarNode(formatWorkbenchComposePort(port)))
		}
		workbenchYAMLAddMapEntry(serviceNode, "ports", portSequence)
	}
	if !workbenchIsEmptyResource(resource) {
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
	if len(networkRefs) > 0 {
		networkSequence := workbenchYAMLSequenceNode()
		for _, networkName := range networkRefs {
			networkSequence.Content = append(networkSequence.Content, workbenchYAMLScalarNode(networkName))
		}
		workbenchYAMLAddMapEntry(serviceNode, "networks", networkSequence)
	}
	return serviceNode
}

func workbenchAddServiceCommand(serviceNode *yaml.Node, command []string) {
	if len(command) == 0 {
		return
	}
	workbenchYAMLAddMapEntry(serviceNode, "command", workbenchYAMLCommandNode(command))
}

func workbenchAddServiceEnvironment(serviceNode *yaml.Node, environment []workbenchComposeEnvironmentEntry) {
	if len(environment) == 0 {
		return
	}
	workbenchYAMLAddMapEntry(serviceNode, "environment", workbenchYAMLEnvironmentNode(environment))
}

func workbenchPruneRemovedManagedServiceNodes(
	servicesNode *yaml.Node,
	desiredServices map[string]WorkbenchComposeService,
	snapshot WorkbenchStackSnapshot,
) {
	if servicesNode == nil || servicesNode.Kind != yaml.MappingNode {
		return
	}

	importedServiceNames := make(map[string]struct{}, len(snapshot.Services))
	for _, service := range snapshot.Services {
		name := strings.ToLower(strings.TrimSpace(service.ServiceName))
		if name == "" {
			continue
		}
		importedServiceNames[name] = struct{}{}
	}

	nextContent := make([]*yaml.Node, 0, len(servicesNode.Content))
	for idx := 0; idx+1 < len(servicesNode.Content); idx += 2 {
		keyNode := servicesNode.Content[idx]
		valueNode := servicesNode.Content[idx+1]
		if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
			continue
		}
		serviceName := strings.TrimSpace(keyNode.Value)
		if _, exists := desiredServices[serviceName]; exists {
			nextContent = append(nextContent, keyNode, valueNode)
			continue
		}
		normalizedServiceName := strings.ToLower(serviceName)
		if _, exists := importedServiceNames[normalizedServiceName]; exists {
			nextContent = append(nextContent, keyNode, valueNode)
			continue
		}
		if _, exists := workbenchOptionalServiceDefinitionByServiceName(serviceName); exists {
			continue
		}
		nextContent = append(nextContent, keyNode, valueNode)
	}
	servicesNode.Content = nextContent
}

func workbenchYAMLFindMapValue(node *yaml.Node, key string) (*yaml.Node, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, false
	}
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
			continue
		}
		if keyNode.Value == key {
			return node.Content[idx+1], true
		}
	}
	return nil, false
}

func workbenchYAMLSetMapEntry(node *yaml.Node, key string, value *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
			continue
		}
		if keyNode.Value == key {
			node.Content[idx+1] = value
			return
		}
	}
	workbenchYAMLAddMapEntry(node, key, value)
}

func workbenchYAMLDeleteMapEntry(node *yaml.Node, key string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		if keyNode == nil || keyNode.Kind != yaml.ScalarNode {
			continue
		}
		if keyNode.Value == key {
			node.Content = append(node.Content[:idx], node.Content[idx+2:]...)
			return
		}
	}
}

func workbenchYAMLSetOrDeleteScalarEntry(node *yaml.Node, key string, value string) {
	if strings.TrimSpace(value) == "" {
		workbenchYAMLDeleteMapEntry(node, key)
		return
	}
	workbenchYAMLSetMapEntry(node, key, workbenchYAMLScalarNode(value))
}

func normalizeWorkbenchComposePreviewRequest(input WorkbenchComposePreviewRequest) (WorkbenchComposePreviewRequest, error) {
	normalized := WorkbenchComposePreviewRequest{}
	if input.ExpectedRevision == nil {
		return normalized, nil
	}

	revision := *input.ExpectedRevision
	if revision <= 0 {
		return WorkbenchComposePreviewRequest{}, errs.New(
			errs.CodeProjectInvalidBody,
			"expectedRevision must be greater than zero",
		)
	}

	normalized.ExpectedRevision = intPtr(revision)
	return normalized, nil
}

func normalizeWorkbenchComposeApplyRequest(input WorkbenchComposeApplyRequest) (WorkbenchComposeApplyRequest, error) {
	normalized := WorkbenchComposeApplyRequest{}
	if input.ExpectedRevision == nil {
		return WorkbenchComposeApplyRequest{}, errs.New(
			errs.CodeProjectInvalidBody,
			"expectedRevision is required",
		)
	}

	revision := *input.ExpectedRevision
	if revision <= 0 {
		return WorkbenchComposeApplyRequest{}, errs.New(
			errs.CodeProjectInvalidBody,
			"expectedRevision must be greater than zero",
		)
	}

	fingerprint := strings.TrimSpace(input.ExpectedSourceFingerprint)
	if fingerprint == "" {
		return WorkbenchComposeApplyRequest{}, errs.New(
			errs.CodeProjectInvalidBody,
			"expectedSourceFingerprint is required",
		)
	}

	normalized.ExpectedRevision = intPtr(revision)
	normalized.ExpectedSourceFingerprint = fingerprint
	return normalized, nil
}

func (s *WorkbenchService) loadStoredSnapshotForComposeLocked(
	ctx context.Context,
	normalizedProject string,
) (WorkbenchStackSnapshot, error) {
	snapshot, exists, err := s.loadStoredWorkbenchSnapshot(ctx, normalizedProject)
	if err != nil {
		return WorkbenchStackSnapshot{}, err
	}
	if !exists {
		return WorkbenchStackSnapshot{}, errs.WithDetails(
			errs.New(errs.CodeWorkbenchSourceNotFound, fmt.Sprintf("workbench snapshot not found for project %q", normalizedProject)),
			map[string]any{
				"project": normalizedProject,
			},
		)
	}
	return snapshot, nil
}

func validateWorkbenchSnapshotForCompose(snapshot WorkbenchStackSnapshot) error {
	_, err := buildWorkbenchComposeGenerationModel(snapshot)
	return err
}

func workbenchPreviewExpectedRevisionValidationError(snapshot WorkbenchStackSnapshot, expectedRevision int) error {
	issue := WorkbenchValidationIssue{
		Class:   workbenchValidationClassSchema,
		Code:    "WB-VAL-EXPECTED-REVISION-MISMATCH",
		Path:    "$.expectedRevision",
		Message: fmt.Sprintf("expected revision %d does not match current revision %d", expectedRevision, snapshot.Revision),
	}
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchValidationFailed, "workbench preview blocked by validation errors"),
		map[string]any{
			"project":           strings.TrimSpace(snapshot.ProjectName),
			"composePath":       strings.TrimSpace(snapshot.ComposePath),
			"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
			"revision":          snapshot.Revision,
			"expectedRevision":  expectedRevision,
			"issueCount":        1,
			"issues":            []WorkbenchValidationIssue{issue},
		},
	)
}

func workbenchApplyStaleRevisionError(snapshot WorkbenchStackSnapshot, expectedRevision int) error {
	issue := WorkbenchValidationIssue{
		Class:   workbenchValidationClassSchema,
		Code:    "WB-STALE-EXPECTED-REVISION-MISMATCH",
		Path:    "$.expectedRevision",
		Message: fmt.Sprintf("expected revision %d does not match current revision %d", expectedRevision, snapshot.Revision),
	}
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchStaleRevision, "workbench apply blocked by stale revision"),
		map[string]any{
			"project":           strings.TrimSpace(snapshot.ProjectName),
			"composePath":       strings.TrimSpace(snapshot.ComposePath),
			"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
			"revision":          snapshot.Revision,
			"expectedRevision":  expectedRevision,
			"issueCount":        1,
			"issues":            []WorkbenchValidationIssue{issue},
		},
	)
}

func workbenchApplyDriftCheck(
	snapshot WorkbenchStackSnapshot,
	currentSource WorkbenchComposeSource,
	expectedSourceFingerprint string,
) error {
	issues := make([]WorkbenchValidationIssue, 0, 2)
	storedFingerprint := strings.TrimSpace(snapshot.SourceFingerprint)
	currentFingerprint := strings.TrimSpace(currentSource.Fingerprint)
	expectedFingerprint := strings.TrimSpace(expectedSourceFingerprint)

	if storedFingerprint != expectedFingerprint {
		issues = append(issues, WorkbenchValidationIssue{
			Class:   workbenchValidationClassSchema,
			Code:    "WB-DRIFT-EXPECTED-SOURCE-FINGERPRINT-MISMATCH",
			Path:    "$.expectedSourceFingerprint",
			Message: fmt.Sprintf("expected source fingerprint %q does not match current workbench fingerprint %q", expectedFingerprint, storedFingerprint),
		})
	}
	if storedFingerprint != currentFingerprint {
		issues = append(issues, WorkbenchValidationIssue{
			Class:   workbenchValidationClassSchema,
			Code:    "WB-DRIFT-COMPOSE-SOURCE-MISMATCH",
			Path:    "$.composeSource",
			Message: fmt.Sprintf("compose source fingerprint %q does not match stored workbench fingerprint %q", currentFingerprint, storedFingerprint),
		})
	}
	if len(issues) == 0 {
		return nil
	}

	sort.SliceStable(issues, func(i, j int) bool {
		return workbenchValidationIssueLess(issues[i], issues[j])
	})

	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchDriftDetected, "workbench apply blocked by compose drift"),
		map[string]any{
			"project":                   strings.TrimSpace(snapshot.ProjectName),
			"composePath":               strings.TrimSpace(currentSource.ComposePath),
			"projectPath":               strings.TrimSpace(currentSource.ProjectDir),
			"revision":                  snapshot.Revision,
			"expectedSourceFingerprint": expectedFingerprint,
			"sourceFingerprint":         storedFingerprint,
			"currentSourceFingerprint":  currentFingerprint,
			"issueCount":                len(issues),
			"issues":                    issues,
		},
	)
}

func replaceWorkbenchComposeAtomically(composePath string, content []byte) error {
	trimmedPath := strings.TrimSpace(composePath)
	if trimmedPath == "" {
		return fmt.Errorf("compose path is empty")
	}

	info, err := os.Stat(trimmedPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("compose path points to a directory")
	}

	dir := filepath.Dir(trimmedPath)
	tempFile, err := os.CreateTemp(dir, ".workbench-compose-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if err := tempFile.Chmod(info.Mode().Perm()); err != nil {
		_ = tempFile.Close()
		return err
	}
	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, trimmedPath); err != nil {
		return err
	}
	cleanupTemp = false
	return nil
}

func generateWorkbenchCompose(snapshot WorkbenchStackSnapshot) (string, error) {
	model, err := buildWorkbenchComposeGenerationModel(snapshot)
	if err != nil {
		return "", err
	}

	root := workbenchYAMLMappingNode()
	servicesNode := workbenchYAMLMappingNode()
	for _, service := range model.services {
		serviceNode := workbenchBuildServiceNode(
			service,
			model.dependencies[service.ServiceName],
			model.ports[service.ServiceName],
			model.resources[service.ServiceName],
			model.networkRefs[service.ServiceName],
			model.serviceExtras[service.ServiceName],
		)
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
		snapshot:      normalizedSnapshot,
		services:      []WorkbenchComposeService{},
		dependencies:  make(map[string][]string),
		ports:         make(map[string][]WorkbenchComposePort),
		resources:     make(map[string]WorkbenchComposeResource),
		networkRefs:   make(map[string][]string),
		serviceExtras: make(map[string]workbenchComposeServiceExtras),
	}

	issues := []WorkbenchValidationIssue{}
	addIssue := func(issue WorkbenchValidationIssue) {
		issue.Class = strings.TrimSpace(strings.ToLower(issue.Class))
		issue.Code = strings.TrimSpace(strings.ToUpper(issue.Code))
		issue.Path = strings.TrimSpace(issue.Path)
		issue.Message = strings.TrimSpace(issue.Message)
		issue.Service = strings.TrimSpace(issue.Service)
		issue.Dependency = strings.TrimSpace(issue.Dependency)
		issue.Protocol = strings.ToLower(strings.TrimSpace(issue.Protocol))
		issue.HostIP = normalizeHostIP(strings.TrimSpace(issue.HostIP))
		issue.HostPort = strings.TrimSpace(issue.HostPort)
		if issue.Class == "" || issue.Code == "" || issue.Path == "" || issue.Message == "" {
			return
		}
		issues = append(issues, issue)
	}
	serviceNames := make(map[string]struct{}, len(normalizedSnapshot.Services))
	for idx, service := range normalizedSnapshot.Services {
		path := fmt.Sprintf("$.services[%d]", idx)
		name := strings.TrimSpace(service.ServiceName)
		if name == "" {
			addIssue(WorkbenchValidationIssue{
				Class:   workbenchValidationClassSchema,
				Code:    "WB-VAL-SERVICE-NAME-REQUIRED",
				Path:    path + ".serviceName",
				Message: "service name is required",
			})
			continue
		}
		if _, exists := serviceNames[name]; exists {
			addIssue(WorkbenchValidationIssue{
				Class:   workbenchValidationClassSchema,
				Code:    "WB-VAL-SERVICE-DUPLICATE",
				Path:    path + ".serviceName",
				Message: fmt.Sprintf("duplicate service definition %q", name),
				Service: name,
			})
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
			addIssue(WorkbenchValidationIssue{
				Class:   workbenchValidationClassSchema,
				Code:    "WB-VAL-SERVICE-SOURCE-REQUIRED",
				Path:    path,
				Message: fmt.Sprintf("service %q must define image or build source", name),
				Service: name,
			})
		}
		model.services = append(model.services, normalizedService)
	}

	managedServiceModels, managedServiceIssues := workbenchBuildManagedServiceModels(normalizedSnapshot)
	for _, issue := range workbenchManagedServiceIssuesToValidation(managedServiceIssues) {
		addIssue(issue)
	}
	for _, managedService := range managedServiceModels {
		serviceName := strings.TrimSpace(managedService.Service.ServiceName)
		if serviceName == "" {
			continue
		}
		serviceNames[serviceName] = struct{}{}
		model.services = append(model.services, managedService.Service)
		model.serviceExtras[serviceName] = managedService.Extras
	}

	dependencySet := make(map[string]struct{})
	for idx, dependency := range normalizedSnapshot.Dependencies {
		path := fmt.Sprintf("$.dependencies[%d]", idx)
		serviceName := strings.TrimSpace(dependency.ServiceName)
		dependsOn := strings.TrimSpace(dependency.DependsOn)
		if serviceName == "" || dependsOn == "" {
			addIssue(WorkbenchValidationIssue{
				Class:      workbenchValidationClassSchema,
				Code:       "WB-VAL-DEPENDENCY-FIELDS-REQUIRED",
				Path:       path,
				Message:    "dependency entries must define serviceName and dependsOn",
				Service:    serviceName,
				Dependency: dependsOn,
			})
			continue
		}
		if _, exists := serviceNames[serviceName]; !exists {
			addIssue(WorkbenchValidationIssue{
				Class:      workbenchValidationClassDependency,
				Code:       "WB-VAL-DEPENDENCY-SERVICE-UNKNOWN",
				Path:       path + ".serviceName",
				Message:    fmt.Sprintf("dependency references unknown service %q", serviceName),
				Service:    serviceName,
				Dependency: dependsOn,
			})
			continue
		}
		if _, exists := serviceNames[dependsOn]; !exists {
			addIssue(WorkbenchValidationIssue{
				Class:      workbenchValidationClassDependency,
				Code:       "WB-VAL-DEPENDENCY-TARGET-UNKNOWN",
				Path:       path + ".dependsOn",
				Message:    fmt.Sprintf("dependency %q -> %q references unknown target service", serviceName, dependsOn),
				Service:    serviceName,
				Dependency: dependsOn,
			})
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
	for idx, port := range normalizedSnapshot.Ports {
		path := fmt.Sprintf("$.ports[%d]", idx)
		serviceName := strings.TrimSpace(port.ServiceName)
		if _, exists := serviceNames[serviceName]; !exists {
			addIssue(WorkbenchValidationIssue{
				Class:   workbenchValidationClassSchema,
				Code:    "WB-VAL-PORT-SERVICE-UNKNOWN",
				Path:    path + ".serviceName",
				Message: fmt.Sprintf("port entry references unknown service %q", serviceName),
				Service: serviceName,
			})
			continue
		}
		if port.ContainerPort < 1 || port.ContainerPort > 65535 {
			addIssue(WorkbenchValidationIssue{
				Class:   workbenchValidationClassSchema,
				Code:    "WB-VAL-PORT-CONTAINER-RANGE",
				Path:    path + ".containerPort",
				Message: fmt.Sprintf("service %q has invalid containerPort %d", serviceName, port.ContainerPort),
				Service: serviceName,
			})
			continue
		}
		if port.HostPort != nil && (*port.HostPort < 1 || *port.HostPort > 65535) {
			addIssue(WorkbenchValidationIssue{
				Class:    workbenchValidationClassSchema,
				Code:     "WB-VAL-PORT-HOST-RANGE",
				Path:     path + ".hostPort",
				Message:  fmt.Sprintf("service %q has invalid hostPort %d", serviceName, *port.HostPort),
				Service:  serviceName,
				HostPort: strconv.Itoa(*port.HostPort),
			})
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
			addIssue(WorkbenchValidationIssue{
				Class:    workbenchValidationClassSchema,
				Code:     "WB-VAL-PORT-DUPLICATE",
				Path:     path,
				Message:  fmt.Sprintf("duplicate port mapping for service %q (container=%d host=%q protocol=%s hostIP=%q)", serviceName, normalizedPort.ContainerPort, hostPortValue, normalizedPort.Protocol, normalizedPort.HostIP),
				Service:  serviceName,
				Protocol: normalizedPort.Protocol,
				HostIP:   normalizedPort.HostIP,
				HostPort: hostPortValue,
			})
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
				addIssue(WorkbenchValidationIssue{
					Class:      workbenchValidationClassPortConflict,
					Code:       "WB-VAL-PORT-HOST-CONFLICT",
					Path:       path,
					Message:    fmt.Sprintf("host port conflict between services %q and %q (protocol=%s hostPort=%q hostIPs=%q/%q)", existing.serviceName, current.serviceName, current.protocol, current.hostPort, existing.hostIP, current.hostIP),
					Service:    current.serviceName,
					Dependency: existing.serviceName,
					Protocol:   current.protocol,
					HostIP:     current.hostIP,
					HostPort:   current.hostPort,
				})
				break
			}
			hostBindings = append(hostBindings, current)
		}

		model.ports[serviceName] = append(model.ports[serviceName], normalizedPort)
	}
	for _, managedService := range managedServiceModels {
		for _, port := range managedService.Ports {
			serviceName := strings.TrimSpace(port.ServiceName)
			if serviceName == "" {
				continue
			}
			if workbenchHasPortMapping(model.ports[serviceName], serviceName, port.ContainerPort, port.Protocol, port.HostIP) {
				continue
			}

			normalizedPort := normalizeWorkbenchComposePort(port)
			hostPortValue := normalizedPort.HostPortRaw
			if normalizedPort.HostPort != nil {
				hostPortValue = strconv.Itoa(*normalizedPort.HostPort)
			}
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
					addIssue(WorkbenchValidationIssue{
						Class:      workbenchValidationClassPortConflict,
						Code:       "WB-VAL-PORT-HOST-CONFLICT",
						Path:       "$.managedServices",
						Message:    fmt.Sprintf("host port conflict between services %q and %q (protocol=%s hostPort=%q hostIPs=%q/%q)", existing.serviceName, current.serviceName, current.protocol, current.hostPort, existing.hostIP, current.hostIP),
						Service:    current.serviceName,
						Dependency: existing.serviceName,
						Protocol:   current.protocol,
						HostIP:     current.hostIP,
						HostPort:   current.hostPort,
					})
					break
				}
				hostBindings = append(hostBindings, current)
			}

			model.ports[serviceName] = append(model.ports[serviceName], normalizedPort)
		}
	}

	for idx, resource := range normalizedSnapshot.Resources {
		path := fmt.Sprintf("$.resources[%d]", idx)
		serviceName := strings.TrimSpace(resource.ServiceName)
		if _, exists := serviceNames[serviceName]; !exists {
			addIssue(WorkbenchValidationIssue{
				Class:   workbenchValidationClassSchema,
				Code:    "WB-VAL-RESOURCE-SERVICE-UNKNOWN",
				Path:    path + ".serviceName",
				Message: fmt.Sprintf("resource entry references unknown service %q", serviceName),
				Service: serviceName,
			})
			continue
		}
		if _, exists := model.resources[serviceName]; exists {
			addIssue(WorkbenchValidationIssue{
				Class:   workbenchValidationClassSchema,
				Code:    "WB-VAL-RESOURCE-DUPLICATE",
				Path:    path + ".serviceName",
				Message: fmt.Sprintf("duplicate resource entry for service %q", serviceName),
				Service: serviceName,
			})
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
	for idx, networkRef := range normalizedSnapshot.NetworkRefs {
		path := fmt.Sprintf("$.networkRefs[%d]", idx)
		serviceName := strings.TrimSpace(networkRef.ServiceName)
		networkName := strings.TrimSpace(networkRef.NetworkName)

		if _, exists := serviceNames[serviceName]; !exists {
			addIssue(WorkbenchValidationIssue{
				Class:   workbenchValidationClassSchema,
				Code:    "WB-VAL-NETWORK-SERVICE-UNKNOWN",
				Path:    path + ".serviceName",
				Message: fmt.Sprintf("network ref references unknown service %q", serviceName),
				Service: serviceName,
			})
			continue
		}
		if networkName == "" {
			addIssue(WorkbenchValidationIssue{
				Class:   workbenchValidationClassSchema,
				Code:    "WB-VAL-NETWORK-NAME-REQUIRED",
				Path:    path + ".networkName",
				Message: fmt.Sprintf("service %q has an empty network reference", serviceName),
				Service: serviceName,
			})
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

	for idx, volumeRef := range normalizedSnapshot.VolumeRefs {
		path := fmt.Sprintf("$.volumeRefs[%d]", idx)
		serviceName := strings.TrimSpace(volumeRef.ServiceName)
		volumeName := strings.TrimSpace(volumeRef.VolumeName)
		if serviceName == "" || volumeName == "" {
			addIssue(WorkbenchValidationIssue{
				Class:   workbenchValidationClassSchema,
				Code:    "WB-VAL-VOLUME-FIELDS-REQUIRED",
				Path:    path,
				Message: "volume refs must include serviceName and volumeName",
				Service: serviceName,
			})
			continue
		}
		addIssue(WorkbenchValidationIssue{
			Class:   workbenchValidationClassSchema,
			Code:    "WB-VAL-VOLUME-UNSUPPORTED",
			Path:    path,
			Message: fmt.Sprintf("service %q volume %q is not yet supported by generator baseline", serviceName, volumeName),
			Service: serviceName,
		})
	}

	for idx, module := range normalizedSnapshot.Modules {
		path := fmt.Sprintf("$.modules[%d]", idx)
		moduleType := strings.TrimSpace(module.ModuleType)
		serviceName := strings.TrimSpace(module.ServiceName)
		addIssue(WorkbenchValidationIssue{
			Class:   workbenchValidationClassSchema,
			Code:    "WB-VAL-MODULE-UNSUPPORTED",
			Path:    path,
			Message: fmt.Sprintf("module %q for service %q is not yet supported by generator baseline", moduleType, serviceName),
			Service: serviceName,
		})
	}

	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return workbenchValidationIssueLess(issues[i], issues[j])
		})
		return workbenchComposeGenerationModel{}, workbenchComposeValidationError(normalizedSnapshot, issues)
	}

	return model, nil
}

func workbenchValidationIssueLess(left, right WorkbenchValidationIssue) bool {
	leftClass := strings.TrimSpace(strings.ToLower(left.Class))
	rightClass := strings.TrimSpace(strings.ToLower(right.Class))
	if leftClass != rightClass {
		return leftClass < rightClass
	}

	leftCode := strings.TrimSpace(strings.ToUpper(left.Code))
	rightCode := strings.TrimSpace(strings.ToUpper(right.Code))
	if leftCode != rightCode {
		return leftCode < rightCode
	}

	leftPath := strings.TrimSpace(left.Path)
	rightPath := strings.TrimSpace(right.Path)
	if leftPath != rightPath {
		return leftPath < rightPath
	}

	leftService := strings.TrimSpace(strings.ToLower(left.Service))
	rightService := strings.TrimSpace(strings.ToLower(right.Service))
	if leftService != rightService {
		return leftService < rightService
	}

	leftDependency := strings.TrimSpace(strings.ToLower(left.Dependency))
	rightDependency := strings.TrimSpace(strings.ToLower(right.Dependency))
	if leftDependency != rightDependency {
		return leftDependency < rightDependency
	}

	leftProtocol := strings.TrimSpace(strings.ToLower(left.Protocol))
	rightProtocol := strings.TrimSpace(strings.ToLower(right.Protocol))
	if leftProtocol != rightProtocol {
		return leftProtocol < rightProtocol
	}

	leftHostIP := strings.TrimSpace(strings.ToLower(left.HostIP))
	rightHostIP := strings.TrimSpace(strings.ToLower(right.HostIP))
	if leftHostIP != rightHostIP {
		return leftHostIP < rightHostIP
	}

	leftHostPort := strings.TrimSpace(left.HostPort)
	rightHostPort := strings.TrimSpace(right.HostPort)
	if leftHostPort != rightHostPort {
		return leftHostPort < rightHostPort
	}

	return strings.TrimSpace(left.Message) < strings.TrimSpace(right.Message)
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

func workbenchComposeValidationError(snapshot WorkbenchStackSnapshot, issues []WorkbenchValidationIssue) error {
	normalized := append([]WorkbenchValidationIssue(nil), issues...)
	return errs.WithDetails(
		errs.New(errs.CodeWorkbenchValidationFailed, "invalid workbench snapshot for compose generation"),
		map[string]any{
			"project":           strings.TrimSpace(snapshot.ProjectName),
			"composePath":       strings.TrimSpace(snapshot.ComposePath),
			"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
			"revision":          snapshot.Revision,
			"issueCount":        len(normalized),
			"issues":            normalized,
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

func workbenchComposeApplySourceInvalidError(
	snapshot WorkbenchStackSnapshot,
	source WorkbenchComposeSource,
	message string,
	cause error,
) error {
	details := map[string]any{
		"project":           strings.TrimSpace(snapshot.ProjectName),
		"projectPath":       strings.TrimSpace(source.ProjectDir),
		"composePath":       strings.TrimSpace(source.ComposePath),
		"sourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
		"revision":          snapshot.Revision,
	}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	return errs.WithDetails(errs.Wrap(errs.CodeWorkbenchSourceInvalid, message, cause), details)
}

func workbenchComposeApplyStorageError(
	snapshot WorkbenchStackSnapshot,
	source WorkbenchComposeSource,
	message string,
	cause error,
	restoreErr error,
) error {
	details := map[string]any{
		"project":                    strings.TrimSpace(snapshot.ProjectName),
		"projectPath":                strings.TrimSpace(source.ProjectDir),
		"composePath":                strings.TrimSpace(source.ComposePath),
		"sourceFingerprint":          strings.TrimSpace(source.Fingerprint),
		"attemptedSourceFingerprint": strings.TrimSpace(snapshot.SourceFingerprint),
		"revision":                   snapshot.Revision,
		"restoredCompose":            restoreErr == nil,
	}
	if cause != nil {
		details["cause"] = cause.Error()
	}
	if restoreErr != nil {
		details["restoreError"] = restoreErr.Error()
	}
	return errs.WithDetails(errs.Wrap(errs.CodeWorkbenchStorageFailed, message, cause), details)
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

func workbenchYAMLCommandNode(command []string) *yaml.Node {
	node := workbenchYAMLSequenceNode()
	for _, part := range command {
		node.Content = append(node.Content, workbenchYAMLScalarNode(part))
	}
	return node
}

func workbenchYAMLEnvironmentNode(environment []workbenchComposeEnvironmentEntry) *yaml.Node {
	node := workbenchYAMLMappingNode()
	for _, entry := range environment {
		workbenchYAMLAddMapEntry(node, entry.Key, workbenchYAMLScalarNode(entry.Value))
	}
	return node
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
