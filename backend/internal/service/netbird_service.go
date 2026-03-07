package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go-notes/internal/config"
	"go-notes/internal/errs"
	"go-notes/internal/repository"
)

const (
	netBirdOpCreate         = "create"
	netBirdOpUpdate         = "update"
	netBirdOpDelete         = "delete"
	netBirdOpDisableDefault = "disable-default"
)

type NetBirdService struct {
	cfg                     config.Config
	settings                *SettingsService
	projects                repository.ProjectRepository
	jobs                    repository.JobRepository
	liveStatusClientFactory func(baseURL, token string) netBirdVisibilityClient
}

type NetBirdModePlan struct {
	CurrentMode                NetBirdMode                        `json:"currentMode"`
	TargetMode                 NetBirdMode                        `json:"targetMode"`
	AllowLocalhost             bool                               `json:"allowLocalhost"`
	CurrentModeBProjectIDs     []uint                             `json:"currentModeBProjectIds"`
	TargetModeBProjectIDs      []uint                             `json:"targetModeBProjectIds"`
	Catalog                    NetBirdCatalog                     `json:"catalog"`
	GroupOperations            []NetBirdGroupOperation            `json:"groupOperations"`
	PolicyOperations           []NetBirdPolicyOperation           `json:"policyOperations"`
	ServiceRebindingOperations []NetBirdServiceRebindingOperation `json:"serviceRebindingOperations"`
	RedeployTargets            NetBirdRedeployTargets             `json:"redeployTargets"`
	Warnings                   []string                           `json:"warnings"`
}

type NetBirdGroupOperation struct {
	Operation string               `json:"operation"`
	Name      string               `json:"name"`
	Match     string               `json:"match,omitempty"`
	Payload   *NetBirdGroupPayload `json:"payload,omitempty"`
	Reason    string               `json:"reason,omitempty"`
}

type NetBirdPolicyOperation struct {
	Operation string                `json:"operation"`
	Name      string                `json:"name"`
	Match     string                `json:"match,omitempty"`
	Payload   *NetBirdPolicyPayload `json:"payload,omitempty"`
	Reason    string                `json:"reason,omitempty"`
}

type NetBirdServiceRebindingOperation struct {
	Service       string   `json:"service"`
	ProjectID     uint     `json:"projectId,omitempty"`
	ProjectName   string   `json:"projectName,omitempty"`
	Port          int      `json:"port"`
	FromListeners []string `json:"fromListeners"`
	ToListeners   []string `json:"toListeners"`
	Reason        string   `json:"reason"`
}

type NetBirdRedeployTargets struct {
	Panel    bool                           `json:"panel"`
	Projects []NetBirdRedeployProjectTarget `json:"projects"`
}

type NetBirdRedeployProjectTarget struct {
	ProjectID   uint   `json:"projectId"`
	ProjectName string `json:"projectName"`
	Port        int    `json:"port"`
	Reason      string `json:"reason"`
}

func NewNetBirdService(cfg config.Config, settings *SettingsService, projects repository.ProjectRepository, jobs repository.JobRepository) *NetBirdService {
	return &NetBirdService{
		cfg:      cfg,
		settings: settings,
		projects: projects,
		jobs:     jobs,
	}
}

func (s *NetBirdService) ResolveModeApplyExecutionRequest(ctx context.Context, input NetBirdModeApplyJobRequest) (NetBirdModeApplyRequest, bool, error) {
	request := NormalizeNetBirdModeApplyRequest(NetBirdModeApplyRequest{
		TargetMode:      input.TargetMode,
		AllowLocalhost:  input.AllowLocalhost,
		ModeBProjectIDs: input.ModeBProjectIDs,
		APIBaseURL:      input.APIBaseURL,
		APIToken:        input.APIToken,
		HostPeerID:      input.HostPeerID,
		AdminPeerIDs:    input.AdminPeerIDs,
	})
	if s == nil || s.settings == nil {
		return request, false, nil
	}
	return s.settings.ResolveNetBirdModeApplyJobRequest(ctx, input)
}

func (s *NetBirdService) PlanMode(ctx context.Context, targetModeRaw string, allowLocalhost bool, modeBProjectIDs []uint) (NetBirdModePlan, error) {
	if s == nil || s.projects == nil {
		return NetBirdModePlan{}, errs.New(errs.CodeNetBirdUnavailable, "netbird service unavailable")
	}

	targetMode, err := ParseNetBirdMode(targetModeRaw)
	if err != nil {
		return NetBirdModePlan{}, err
	}

	runtimeState := s.resolveNetBirdRuntimeState(ctx)
	currentMode := runtimeState.EffectiveMode
	currentModeBProjectIDs := normalizeUintList(runtimeState.EffectiveModeBProjectIDs)
	targetModeBProjectIDs := normalizeUintList(modeBProjectIDs)

	panelPort, panelPortFallback := resolvePanelPort(s.cfg.Port)
	projectInputs, projectWarnings, err := s.loadProjectCatalogInputs(ctx)
	if err != nil {
		return NetBirdModePlan{}, errs.Wrap(errs.CodeNetBirdPlanFailed, "failed to load projects for netbird planner", err)
	}
	currentModeProjects, currentProjectWarnings := selectModeBProjects(projectInputs, currentModeBProjectIDs)
	targetModeProjects, targetProjectWarnings := selectModeBProjects(projectInputs, targetModeBProjectIDs)

	if currentMode != NetBirdModeB {
		currentModeBProjectIDs = nil
		currentModeProjects = nil
		currentProjectWarnings = nil
	}
	if targetMode != NetBirdModeB {
		targetModeBProjectIDs = nil
		targetModeProjects = nil
		targetProjectWarnings = nil
	}

	currentCatalog := BuildNetBirdCatalog(NetBirdCatalogInput{
		Mode:      currentMode,
		PanelPort: panelPort,
		Projects:  currentModeProjects,
	})
	targetCatalog := BuildNetBirdCatalog(NetBirdCatalogInput{
		Mode:      targetMode,
		PanelPort: panelPort,
		Projects:  targetModeProjects,
	})

	groupOps := buildGroupOperations(currentCatalog.Groups, targetCatalog.Groups, targetMode)
	policyOps := buildPolicyOperations(currentCatalog.Policies, targetCatalog.Policies, targetMode)
	rebindings := buildServiceRebindings(
		currentMode,
		targetMode,
		runtimeState.EffectiveAllowLocalhost,
		allowLocalhost,
		panelPort,
		projectInputs,
		currentModeBProjectIDs,
		targetModeBProjectIDs,
	)
	redeployTargets := buildRedeployTargets(rebindings)

	warnings := make([]string, 0, 5+len(projectWarnings)+len(currentProjectWarnings)+len(targetProjectWarnings)+len(runtimeState.Warnings))
	warnings = append(warnings, runtimeState.Warnings...)
	if panelPortFallback {
		warnings = append(warnings, "Panel port was not a valid integer; planner used default port 8080.")
	}
	warnings = append(warnings, projectWarnings...)
	warnings = append(warnings, currentProjectWarnings...)
	warnings = append(warnings, targetProjectWarnings...)
	if targetMode == NetBirdModeB && len(targetModeBProjectIDs) == 0 {
		warnings = append(warnings, "Mode B selected with no assigned projects; only panel isolation policy will be planned.")
	}
	if targetMode != NetBirdModeB && len(modeBProjectIDs) > 0 {
		warnings = append(warnings, "Mode B project IDs were provided for a non-Mode B target and were ignored.")
	}
	if targetMode == currentMode && len(rebindings) == 0 {
		warnings = append(warnings, "Target mode matches current mode; plan is a policy reconcile only.")
	}

	return NetBirdModePlan{
		CurrentMode:                currentMode,
		TargetMode:                 targetMode,
		AllowLocalhost:             allowLocalhost,
		CurrentModeBProjectIDs:     normalizeUintList(currentModeBProjectIDs),
		TargetModeBProjectIDs:      normalizeUintList(targetModeBProjectIDs),
		Catalog:                    targetCatalog,
		GroupOperations:            groupOps,
		PolicyOperations:           policyOps,
		ServiceRebindingOperations: rebindings,
		RedeployTargets:            redeployTargets,
		Warnings:                   warnings,
	}, nil
}

func (s *NetBirdService) loadProjectCatalogInputs(ctx context.Context) ([]NetBirdProjectCatalogInput, []string, error) {
	projects, err := s.projects.List(ctx)
	if err != nil {
		return nil, nil, err
	}

	inputs := make([]NetBirdProjectCatalogInput, 0, len(projects))
	warnings := make([]string, 0)
	for _, project := range projects {
		port := project.ProxyPort
		if port <= 0 {
			port = defaultIngressPort
			warnings = append(warnings, fmt.Sprintf("Project %q has no proxy port; planner used default ingress port 80.", strings.TrimSpace(project.Name)))
		}
		inputs = append(inputs, NetBirdProjectCatalogInput{
			ProjectID:   project.ID,
			ProjectName: strings.TrimSpace(project.Name),
			IngressPort: port,
		})
	}

	return normalizeCatalogProjects(inputs), warnings, nil
}

func resolvePanelPort(raw string) (int, bool) {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || parsed <= 0 {
		return defaultPanelPort, true
	}
	return parsed, false
}

func selectModeBProjects(all []NetBirdProjectCatalogInput, selectedIDs []uint) ([]NetBirdProjectCatalogInput, []string) {
	if len(selectedIDs) == 0 {
		return []NetBirdProjectCatalogInput{}, nil
	}

	byID := make(map[uint]NetBirdProjectCatalogInput, len(all))
	for _, project := range all {
		byID[project.ProjectID] = project
	}

	selected := make([]NetBirdProjectCatalogInput, 0, len(selectedIDs))
	warnings := make([]string, 0)
	for _, projectID := range normalizeUintList(selectedIDs) {
		project, ok := byID[projectID]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("Mode B project assignment %d was not found and was ignored.", projectID))
			continue
		}
		selected = append(selected, project)
	}

	return normalizeCatalogProjects(selected), warnings
}

func buildGroupOperations(current, target []NetBirdGroupPayload, targetMode NetBirdMode) []NetBirdGroupOperation {
	currentByName := make(map[string]NetBirdGroupPayload, len(current))
	for _, entry := range current {
		currentByName[entry.Name] = entry
	}

	targetByName := make(map[string]NetBirdGroupPayload, len(target))
	targetNames := make([]string, 0, len(target))
	for _, entry := range target {
		targetByName[entry.Name] = entry
		targetNames = append(targetNames, entry.Name)
	}
	sort.Strings(targetNames)

	ops := make([]NetBirdGroupOperation, 0, len(current)+len(target)+4)
	for _, name := range targetNames {
		payload := targetByName[name]
		operation := netBirdOpCreate
		if _, exists := currentByName[name]; exists {
			operation = netBirdOpUpdate
		}
		ops = append(ops, NetBirdGroupOperation{
			Operation: operation,
			Name:      name,
			Payload:   &payload,
			Reason:    "Ensure managed group matches target mode catalog.",
		})
	}

	ops = append(ops, cleanupGroupDeletes(targetMode)...)

	currentNames := make([]string, 0, len(currentByName))
	for name := range currentByName {
		if _, keep := targetByName[name]; !keep {
			currentNames = append(currentNames, name)
		}
	}
	sort.Strings(currentNames)
	for _, name := range currentNames {
		if coveredByGroupPrefixDelete(ops, name) {
			continue
		}
		ops = append(ops, NetBirdGroupOperation{
			Operation: netBirdOpDelete,
			Name:      name,
			Reason:    "Remove managed group absent from target mode catalog.",
		})
	}

	return dedupeGroupOperations(ops)
}

func buildPolicyOperations(current, target []NetBirdPolicyPayload, targetMode NetBirdMode) []NetBirdPolicyOperation {
	currentByName := make(map[string]NetBirdPolicyPayload, len(current))
	for _, entry := range current {
		currentByName[entry.Name] = entry
	}

	targetByName := make(map[string]NetBirdPolicyPayload, len(target))
	targetNames := make([]string, 0, len(target))
	for _, entry := range target {
		targetByName[entry.Name] = entry
		targetNames = append(targetNames, entry.Name)
	}
	sort.Strings(targetNames)

	ops := make([]NetBirdPolicyOperation, 0, len(current)+len(target)+6)
	if targetMode != NetBirdModeLegacy {
		ops = append(ops, NetBirdPolicyOperation{
			Operation: netBirdOpDisableDefault,
			Name:      netBirdDefaultPolicyName,
			Reason:    "Enforce deny-by-default baseline before reconciling managed policies.",
		})
	}

	for _, name := range targetNames {
		payload := targetByName[name]
		operation := netBirdOpCreate
		if _, exists := currentByName[name]; exists {
			operation = netBirdOpUpdate
		}
		ops = append(ops, NetBirdPolicyOperation{
			Operation: operation,
			Name:      name,
			Payload:   &payload,
			Reason:    "Ensure managed policy matches target mode catalog.",
		})
	}

	ops = append(ops, cleanupPolicyDeletes(targetMode)...)

	currentNames := make([]string, 0, len(currentByName))
	for name := range currentByName {
		if _, keep := targetByName[name]; !keep {
			currentNames = append(currentNames, name)
		}
	}
	sort.Strings(currentNames)
	for _, name := range currentNames {
		if coveredByPolicyPrefixDelete(ops, name) {
			continue
		}
		ops = append(ops, NetBirdPolicyOperation{
			Operation: netBirdOpDelete,
			Name:      name,
			Reason:    "Remove managed policy absent from target mode catalog.",
		})
	}

	return dedupePolicyOperations(ops)
}

func cleanupGroupDeletes(targetMode NetBirdMode) []NetBirdGroupOperation {
	switch targetMode {
	case NetBirdModeA:
		return []NetBirdGroupOperation{
			{
				Operation: netBirdOpDelete,
				Name:      netBirdProjectGroupPrefix,
				Match:     "prefix",
				Reason:    "Remove stale Mode B per-project groups.",
			},
		}
	case NetBirdModeLegacy:
		return []NetBirdGroupOperation{
			{
				Operation: netBirdOpDelete,
				Name:      netBirdGroupAdminsName,
				Reason:    "Legacy mode does not keep managed NetBird groups.",
			},
			{
				Operation: netBirdOpDelete,
				Name:      netBirdGroupPanelName,
				Reason:    "Legacy mode does not keep managed NetBird groups.",
			},
			{
				Operation: netBirdOpDelete,
				Name:      netBirdProjectGroupPrefix,
				Match:     "prefix",
				Reason:    "Remove stale Mode B per-project groups.",
			},
		}
	default:
		return nil
	}
}

func cleanupPolicyDeletes(targetMode NetBirdMode) []NetBirdPolicyOperation {
	switch targetMode {
	case NetBirdModeA:
		return []NetBirdPolicyOperation{
			{
				Operation: netBirdOpDelete,
				Name:      netBirdModeBPanelPolicy,
				Reason:    "Remove stale Mode B panel policy.",
			},
			{
				Operation: netBirdOpDelete,
				Name:      netBirdModeBProjectPrefix,
				Match:     "prefix",
				Reason:    "Remove stale Mode B per-project policies.",
			},
		}
	case NetBirdModeB:
		return []NetBirdPolicyOperation{
			{
				Operation: netBirdOpDelete,
				Name:      netBirdModeAPolicyName,
				Reason:    "Remove stale Mode A panel policy.",
			},
		}
	case NetBirdModeLegacy:
		return []NetBirdPolicyOperation{
			{
				Operation: netBirdOpDelete,
				Name:      netBirdModeAPolicyName,
				Reason:    "Legacy mode does not keep managed NetBird policies.",
			},
			{
				Operation: netBirdOpDelete,
				Name:      netBirdModeBPanelPolicy,
				Reason:    "Legacy mode does not keep managed NetBird policies.",
			},
			{
				Operation: netBirdOpDelete,
				Name:      netBirdModeBProjectPrefix,
				Match:     "prefix",
				Reason:    "Legacy mode removes all per-project NetBird policies.",
			},
		}
	default:
		return nil
	}
}

func dedupeGroupOperations(ops []NetBirdGroupOperation) []NetBirdGroupOperation {
	seen := make(map[string]struct{}, len(ops))
	out := make([]NetBirdGroupOperation, 0, len(ops))
	for _, op := range ops {
		key := op.Operation + "|" + op.Match + "|" + op.Name
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, op)
	}
	return out
}

func coveredByGroupPrefixDelete(ops []NetBirdGroupOperation, name string) bool {
	for _, op := range ops {
		if op.Operation != netBirdOpDelete || op.Match != "prefix" {
			continue
		}
		if strings.HasPrefix(name, op.Name) {
			return true
		}
	}
	return false
}

func dedupePolicyOperations(ops []NetBirdPolicyOperation) []NetBirdPolicyOperation {
	seen := make(map[string]struct{}, len(ops))
	out := make([]NetBirdPolicyOperation, 0, len(ops))
	for _, op := range ops {
		key := op.Operation + "|" + op.Match + "|" + op.Name
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, op)
	}
	return out
}

func coveredByPolicyPrefixDelete(ops []NetBirdPolicyOperation, name string) bool {
	for _, op := range ops {
		if op.Operation != netBirdOpDelete || op.Match != "prefix" {
			continue
		}
		if strings.HasPrefix(name, op.Name) {
			return true
		}
	}
	return false
}

func buildServiceRebindings(
	currentMode NetBirdMode,
	targetMode NetBirdMode,
	currentAllowLocalhost bool,
	targetAllowLocalhost bool,
	panelPort int,
	projects []NetBirdProjectCatalogInput,
	currentModeBProjectIDs []uint,
	targetModeBProjectIDs []uint,
) []NetBirdServiceRebindingOperation {
	currentPanelListeners := panelListeners(currentMode, currentAllowLocalhost)
	targetPanelListeners := panelListeners(targetMode, targetAllowLocalhost)

	ops := make([]NetBirdServiceRebindingOperation, 0, len(projects)+1)
	if !listenersEqual(currentPanelListeners, targetPanelListeners) {
		ops = append(ops, NetBirdServiceRebindingOperation{
			Service:       "panel",
			Port:          panelPort,
			FromListeners: currentPanelListeners,
			ToListeners:   targetPanelListeners,
			Reason:        "Apply target panel listener binding for requested NetBird mode.",
		})
	}

	currentModeBProjectSet := modeBProjectIDSet(currentModeBProjectIDs)
	targetModeBProjectSet := modeBProjectIDSet(targetModeBProjectIDs)

	for _, project := range projects {
		currentProjectListeners := projectListeners(currentMode, currentAllowLocalhost, project.ProjectID, currentModeBProjectSet)
		targetProjectListeners := projectListeners(targetMode, targetAllowLocalhost, project.ProjectID, targetModeBProjectSet)
		if listenersEqual(currentProjectListeners, targetProjectListeners) {
			continue
		}
		ops = append(ops, NetBirdServiceRebindingOperation{
			Service:       "project_ingress",
			ProjectID:     project.ProjectID,
			ProjectName:   project.ProjectName,
			Port:          project.IngressPort,
			FromListeners: currentProjectListeners,
			ToListeners:   targetProjectListeners,
			Reason:        "Apply target project ingress binding for requested NetBird mode.",
		})
	}

	return ops
}

func panelListeners(mode NetBirdMode, allowLocalhost bool) []string {
	switch mode {
	case NetBirdModeA, NetBirdModeB:
		if allowLocalhost {
			return []string{"wg0", "127.0.0.1"}
		}
		return []string{"wg0"}
	default:
		return []string{"0.0.0.0"}
	}
}

func projectListeners(mode NetBirdMode, allowLocalhost bool, projectID uint, modeBProjectSet map[uint]struct{}) []string {
	if mode == NetBirdModeB {
		if _, selected := modeBProjectSet[projectID]; !selected {
			return []string{"0.0.0.0"}
		}
		if allowLocalhost {
			return []string{"wg0", "127.0.0.1"}
		}
		return []string{"wg0"}
	}
	return []string{"0.0.0.0"}
}

func modeBProjectIDSet(values []uint) map[uint]struct{} {
	set := make(map[uint]struct{}, len(values))
	for _, value := range normalizeUintList(values) {
		set[value] = struct{}{}
	}
	return set
}

func listenersEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func buildRedeployTargets(ops []NetBirdServiceRebindingOperation) NetBirdRedeployTargets {
	result := NetBirdRedeployTargets{
		Panel:    false,
		Projects: []NetBirdRedeployProjectTarget{},
	}
	projectSeen := map[uint]struct{}{}

	for _, op := range ops {
		switch op.Service {
		case "panel":
			result.Panel = true
		case "project_ingress":
			if _, exists := projectSeen[op.ProjectID]; exists {
				continue
			}
			projectSeen[op.ProjectID] = struct{}{}
			result.Projects = append(result.Projects, NetBirdRedeployProjectTarget{
				ProjectID:   op.ProjectID,
				ProjectName: op.ProjectName,
				Port:        op.Port,
				Reason:      "Ingress listener binding change requires project stack redeploy.",
			})
		}
	}

	sort.Slice(result.Projects, func(i, j int) bool {
		if result.Projects[i].ProjectID == result.Projects[j].ProjectID {
			return result.Projects[i].ProjectName < result.Projects[j].ProjectName
		}
		return result.Projects[i].ProjectID < result.Projects[j].ProjectID
	})

	return result
}
